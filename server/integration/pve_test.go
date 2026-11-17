// These tests require an explicitly selected PostgreSQL database; each run owns an isolated schema.
package integration

import (
	"bytes"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPostgresHTTPWebSocketAndRestart(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(context.Background())
	schema := "pve_test_" + game.ID()
	if _, e = admin.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	repo, e := store.Open(ctx, schemaDSN(dsn, schema))
	if e != nil {
		t.Fatal(e)
	}
	defer repo.Close()
	root, _ := filepath.Abs("../..")
	binary := filepath.Join(root, "build/engine/xiangqi-engine")
	start := func(workers int) (*game.Manager, func()) {
		rules, bots := engine.NewPool(binary, 2, 16), engine.NewPool(binary, 2, 0)
		m, e := game.New(repo, rules, bots, workers)
		if e != nil {
			t.Fatal(e)
		}
		return m, func() { m.Close(); rules.Close(); bots.Close() }
	}
	m, stop := start(2)
	server := httptest.NewServer(httpapi.New(m, root, slog.New(slog.NewTextHandler(io.Discard, nil))))
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 5 * time.Second}
	call := func(client *http.Client, method, path, body, origin string, status int) []byte {
		t.Helper()
		r, _ := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		resp, e := client.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != status {
			t.Fatalf("%s %s: %d %s", method, path, resp.StatusCode, b)
		}
		return b
	}
	call(client, "GET", "/api/session", "", "", 200)
	cookies := jar.Cookies(mustURL(t, server.URL))
	if len(cookies) != 1 {
		t.Fatal("no guest session")
	}
	owner := cookies[0].Value
	var view game.Snapshot
	_ = json.Unmarshal(call(client, "POST", "/api/games", `{"human":"red","difficulty":"easy"}`, server.URL, 201), &view)
	path := "/api/games/" + view.ID
	other := &http.Client{Timeout: 5 * time.Second}
	call(other, "GET", "/api/session", "", "", 200)
	call(other, "GET", path, "", "", 401)
	otherJar, _ := cookiejar.New(nil)
	other.Jar = otherJar
	call(other, "GET", "/api/session", "", "", 200)
	call(other, "GET", path, "", "", 404)
	call(client, "POST", path+"/commands", `{}`, "https://foreign.invalid", 403)
	call(client, "POST", path+"/commands", `{"unknown":true}`, server.URL, 400)
	ws, eResp, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+path+"/socket", &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
	if e != nil {
		t.Fatalf("socket: %v %v", e, eResp)
	}
	read := func() game.Snapshot {
		t.Helper()
		_, b, e := ws.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		var v game.Snapshot
		if e = json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if read().Version != view.Version {
		t.Fatal("socket initial snapshot")
	}
	request := game.Command{RequestID: game.ID(), Version: view.Version, Type: "move", Move: "b0c2"}
	b, _ := json.Marshal(request)
	accepted := call(client, "POST", path+"/commands", string(b), server.URL, 200)
	again := call(client, "POST", path+"/commands", string(b), server.URL, 200)
	if !bytes.Equal(accepted, again) {
		t.Fatal("idempotent receipt changed")
	}
	var final game.Snapshot
	for i := 0; i < 5; i++ {
		final = read()
		if len(final.Moves) == 2 && final.Phase == "playing" {
			break
		}
	}
	if len(final.Moves) != 2 {
		t.Fatal("no native reply", final)
	}
	_ = ws.Close(websocket.StatusNormalClosure, "")
	var refreshed game.Snapshot
	_ = json.Unmarshal(call(client, "GET", path, "", "", 200), &refreshed)
	if refreshed.Version != final.Version {
		t.Fatal("refresh mismatch")
	}
	server.Close()
	stop()
	// Persist a queued black-side game and restart into active C++ workers.
	paused, pauseStop := start(0)
	queued, e := paused.Create(ctx, owner, "black", "easy")
	if e != nil {
		t.Fatal(e)
	}
	pauseStop()
	restored, restoreStop := start(2)
	defer restoreStop()
	loaded, e := restored.Get(owner, final.ID)
	if e != nil || loaded.Fen != final.Fen || len(loaded.Moves) != 2 {
		t.Fatal("SQL restart failed", e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, _ := restored.Get(owner, queued.ID)
		if len(v.Moves) == 1 && v.Phase == "playing" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(fmt.Sprintf("queued game did not resume: %s", queued.ID))
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, e := url.Parse(s)
	if e != nil {
		t.Fatal(e)
	}
	return u
}
