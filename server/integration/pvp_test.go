package integration

import (
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/pvp"
	"chinesechess/internal/store"
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/jackc/pgx/v5"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRealPostgresTwoPlayerHTTPAndSockets(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for actual PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	admin, e := pgx.Connect(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(context.Background())
	schema := "pvp_test_" + game.ID()
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
	rules, bots := engine.NewPool(binary, 2, 8), engine.NewPool(binary, 1, 0)
	defer rules.Close()
	defer bots.Close()
	games, e := game.New(repo, rules, bots, 1)
	if e != nil {
		t.Fatal(e)
	}
	defer games.Close()
	var now atomic.Int64
	now.Store(1000000)
	rooms, e := pvp.New(repo, rules, now.Load)
	if e != nil {
		t.Fatal(e)
	}
	defer rooms.Close()
	server := httptest.NewServer(httpapi.New(games, root, slog.New(slog.NewTextHandler(io.Discard, nil)), rooms))
	defer server.Close()
	newClient := func() *http.Client {
		j, _ := cookiejar.New(nil)
		c := &http.Client{Jar: j, Timeout: 4 * time.Second}
		r, e := c.Get(server.URL + "/api/session")
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		return c
	}
	red, black, outsider := newClient(), newClient(), newClient()
	call := func(c *http.Client, path, body string, status int) pvp.View {
		t.Helper()
		method := "GET"
		if body != "" {
			method = "POST"
		}
		r, _ := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", server.URL)
		resp, e := c.Do(r)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		if resp.StatusCode != status {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("HTTP %d %s", resp.StatusCode, b)
		}
		var v pvp.View
		_ = json.NewDecoder(resp.Body).Decode(&v)
		return v
	}
	host := call(red, "/api/rooms", `{"side":"red"}`, 201)
	path := "/api/rooms/" + host.ID
	request, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/rooms/join", strings.NewReader(`{"code":"`+host.Invite+`","nickname":"受邀者","requestId":"`+game.ID()+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", server.URL)
	response, e := black.Do(request)
	if e != nil {
		t.Fatal(e)
	}
	var application pvp.ApplicationView
	e = json.NewDecoder(response.Body).Decode(&application)
	response.Body.Close()
	if e != nil || response.StatusCode != 200 || application.State != "pending" {
		t.Fatalf("application: %+v %v", application, e)
	}
	call(black, path, "", 404)
	current := call(red, path, "", 200)
	approval, _ := json.Marshal(pvp.Command{RequestID: game.ID(), Version: current.Version, Type: "approve_join", ApplicationID: application.ApplicationID})
	call(red, path+"/commands", string(approval), 200)
	guest := call(black, path, "", 200)
	if guest.You != "black" {
		t.Fatal("wrong seat")
	}
	call(outsider, path, "", 404)
	call(outsider, "/api/rooms/join", `{"code":"`+host.Invite+`","nickname":"旁观者","requestId":"`+game.ID()+`"}`, 400)
	sockets := []*websocket.Conn{}
	for _, c := range []*http.Client{red, black} {
		ws, _, e := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+path+"/socket", &websocket.DialOptions{HTTPClient: c, HTTPHeader: http.Header{"Origin": []string{server.URL}}})
		if e != nil {
			t.Fatal(e)
		}
		sockets = append(sockets, ws)
		defer ws.Close(websocket.StatusNormalClosure, "")
	}
	read := func(ws *websocket.Conn) pvp.View {
		t.Helper()
		_, b, e := ws.Read(ctx)
		if e != nil {
			t.Fatal(e)
		}
		var v pvp.View
		if e = json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	if read(sockets[0]).You != "red" || read(sockets[1]).You != "black" {
		t.Fatal("socket identity leaked")
	}
	cmd := func(c *http.Client, kind, move string) pvp.View {
		t.Helper()
		v := call(c, path, "", 200)
		b, _ := json.Marshal(pvp.Command{RequestID: game.ID(), Version: v.Version, Type: kind, Move: move})
		return call(c, path+"/commands", string(b), 200)
	}
	cmd(red, "ready", "")
	v := cmd(black, "ready", "")
	if v.Phase != "playing" {
		t.Fatal(v.Phase)
	}
	now.Add(500)
	v = cmd(red, "move", "b0c2")
	if v.RedMillis != 604500 {
		t.Fatal("increment not authoritative")
	}
	now.Add(300)
	v = cmd(black, "move", "b9c7")
	if len(v.Moves) != 2 {
		t.Fatal("missing plies")
	}
	for i, ws := range sockets {
		var received pvp.View
		for n := 0; n < 6; n++ {
			received = read(ws)
			if received.Version == v.Version {
				break
			}
		}
		if received.Fen != v.Fen || received.Version != v.Version || received.You != []string{"red", "black"}[i] {
			t.Fatal("two clients diverged")
		}
	}
	rows, e := repo.LoadPVP(ctx)
	if e != nil || len(rows) != 1 || rows[0].Fen != v.Fen || len(rows[0].Moves) != 2 {
		t.Fatal("PVP not persisted", e)
	}
	now.Add(604501)
	finished := call(red, path, "", 200)
	if finished.Phase != "finished" || finished.Reason != "timeout" || finished.Outcome != "black_win" {
		t.Fatal("clock did not adjudicate")
	}
}
