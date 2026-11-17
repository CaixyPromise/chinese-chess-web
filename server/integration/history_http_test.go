package integration

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/pvp"
	"chinesechess/internal/store"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestHistoryHTTPAuthorizationAndArchivedSnapshots(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for real PostgreSQL HTTP history tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	schema := "history_http_" + game.ID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	repo, err := store.Open(ctx, schemaDSN(dsn, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	owner, other := game.ID(), game.ID()
	old := game.Record{Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Updated: 30, Fen: chess.Initial, Moves: []string{}, Legal: []string{}, Phase: "finished", Outcome: "black_win", Reason: "saved-original-result", Human: "red", Difficulty: "easy"}, Owner: owner, Receipts: map[string]game.Receipt{}}
	if err = repo.Save(ctx, old, 0); err != nil {
		t.Fatal(err)
	}
	r := pvp.Record{State: pvp.State{ID: game.ID(), Version: 1, Updated: 20, Fen: chess.Initial, Moves: []string{}, Legal: []string{}, Phase: "finished", Outcome: "draw", Reason: "agreement"}, Host: owner, RedOwner: owner, BlackOwner: other, Invite: game.ID(), Receipts: map[string]pvp.Receipt{}}
	if err = repo.SavePVP(ctx, r, 0); err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs("../..")
	rules := engine.NewPool(filepath.Join(root, "build/engine/xiangqi-engine"), 1, 4)
	bots := engine.NewPool(filepath.Join(root, "build/engine/xiangqi-engine"), 1, 0)
	defer rules.Close()
	defer bots.Close()
	m, err := game.New(repo, rules, bots, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	rooms, err := pvp.New(repo, rules)
	if err != nil {
		t.Fatal(err)
	}
	defer rooms.Close()
	server := httptest.NewServer(httpapi.New(m, root, slog.New(slog.NewTextHandler(io.Discard, nil)), rooms))
	defer server.Close()
	get := func(owner, path string, status int) []byte {
		t.Helper()
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+path, nil)
		if owner != "" {
			req.AddCookie(&http.Cookie{Name: "chess_guest", Value: owner})
		}
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		body, e := io.ReadAll(response.Body)
		if e != nil || response.StatusCode != status {
			t.Fatalf("%s: status=%d body=%s err=%v", path, response.StatusCode, body, e)
		}
		return body
	}
	get("", "/api/history", 401)
	for _, query := range []string{"limit=no", "limit=0", "limit=-1", "limit=51", "mode=secret", "cursor=broken"} {
		var failure map[string]string
		if err = json.Unmarshal(get(owner, "/api/history?"+query, 400), &failure); err != nil || failure["code"] != "INVALID_HISTORY_QUERY" {
			t.Fatal("unsafe query error", failure, err)
		}
	}
	var first, second, foreign history.Page
	if err = json.Unmarshal(get(owner, "/api/history?limit=1", 200), &first); err != nil || len(first.Items) != 1 || first.Items[0].ID != old.ID || first.NextCursor == "" {
		t.Fatal("first page", first, err)
	}
	if err = json.Unmarshal(get(owner, "/api/history?limit=1&cursor="+url.QueryEscape(first.NextCursor), 200), &second); err != nil || len(second.Items) != 1 || second.Items[0].ID != r.ID || second.NextCursor != "" {
		t.Fatal("second page", second, err)
	}
	if err = json.Unmarshal(get(other, "/api/history?limit=1&cursor="+url.QueryEscape(first.NextCursor), 200), &foreign); err != nil || len(foreign.Items) != 1 || foreign.Items[0].ID != r.ID || foreign.Items[0].Side != "black" {
		t.Fatal("cookie scope", foreign, err)
	}
	get(other, "/api/games/"+old.ID, 404)
	get(game.ID(), "/api/rooms/"+r.ID, 404)
	var archived game.Snapshot
	if err = json.Unmarshal(get(owner, "/api/games/"+old.ID, 200), &archived); err != nil || archived.Rules != chess.LegacyRules || archived.Outcome != old.Outcome || archived.Reason != old.Reason || archived.Version != 1 {
		t.Fatal("old PVE changed", archived, err)
	}
	var room pvp.View
	if err = json.Unmarshal(get(other, "/api/rooms/"+r.ID, 200), &room); err != nil || room.Rules != chess.CasualRules || room.You != "black" || room.Outcome != "draw" {
		t.Fatal("old PVP changed", room, err)
	}
	saved, err := repo.FindGame(ctx, owner, old.ID)
	if err != nil || saved.Version != 1 || saved.Outcome != old.Outcome || saved.Reason != old.Reason {
		t.Fatal("GET wrote archive", saved, err)
	}
}
