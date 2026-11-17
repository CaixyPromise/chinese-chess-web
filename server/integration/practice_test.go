package integration

import (
	"bytes"
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/pvp"
	"chinesechess/internal/store"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPracticePostgresHTTP(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL for real PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	schema := "practice_" + game.ID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	repo, err := store.Open(ctx, schemaDSN(dsn, schema))
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	owner, opponent, stranger := game.ID(), game.ID(), game.ID()
	source := game.Record{Snapshot: game.Snapshot{ID: game.ID(), Version: 7, Moves: []string{"b0c2", "b9c7"}, Fen: chess.Initial, Rules: chess.LegacyRules, Phase: "finished", Outcome: "black_win", Reason: "resign", Human: "red", Difficulty: "easy"}, Owner: owner, Receipts: map[string]game.Receipt{}}
	if err = repo.Save(ctx, source, 0); err != nil {
		t.Fatal(err)
	}
	finished := pvp.Record{State: pvp.State{ID: game.ID(), Version: 4, Moves: []string{"b0c2"}, Fen: chess.Initial, Rules: chess.CasualRules, Phase: "finished", Outcome: "draw", Reason: "agreement"}, Host: owner, RedOwner: owner, BlackOwner: opponent, Invite: game.ID(), Receipts: map[string]pvp.Receipt{}}
	active := pvp.Record{State: pvp.State{ID: game.ID(), Version: 1, Moves: []string{}, Fen: chess.Initial, Rules: chess.CasualRules, Phase: "waiting", Outcome: "ongoing"}, Host: owner, RedOwner: owner, Invite: game.ID(), Receipts: map[string]pvp.Receipt{}}
	for _, r := range []pvp.Record{finished, active} {
		if err = repo.SavePVP(ctx, r, 0); err != nil {
			t.Fatal(err)
		}
	}
	root, _ := filepath.Abs("../..")
	binary := filepath.Join(root, "build/engine/xiangqi-engine")
	rules, bots := engine.NewPool(binary, 2, 8), engine.NewPool(binary, 1, 0)
	defer rules.Close()
	defer bots.Close()
	games, err := game.New(repo, rules, bots, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer games.Close()
	rooms, err := pvp.New(repo, rules)
	if err != nil {
		t.Fatal(err)
	}
	defer rooms.Close()
	server := httptest.NewServer(httpapi.New(games, root, slog.New(slog.NewTextHandler(io.Discard, nil)), rooms))
	defer server.Close()
	post := func(who, mode, id string, version int64, ply int, status int, code string) game.Snapshot {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"sourceMode": mode, "sourceId": id, "expectedVersion": version, "ply": ply, "human": "red", "difficulty": "easy"})
		req, _ := http.NewRequestWithContext(ctx, "POST", server.URL+"/api/games/practice", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(&http.Cookie{Name: "chess_guest", Value: who})
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != status {
			t.Fatalf("status %d, body %s", resp.StatusCode, data)
		}
		if code != "" {
			var failure map[string]string
			_ = json.Unmarshal(data, &failure)
			if failure["code"] != code {
				t.Fatal(string(data))
			}
		}
		var v game.Snapshot
		_ = json.Unmarshal(data, &v)
		return v
	}
	post(stranger, "pve", source.ID, 7, 1, 404, "GAME_NOT_FOUND")
	post(stranger, "pvp", finished.ID, 4, 1, 404, "GAME_NOT_FOUND")
	post(owner, "pvp", active.ID, 1, 0, 400, "ANALYSIS_NOT_AVAILABLE")
	post(owner, "pve", source.ID, 6, 1, 409, "VERSION_CONFLICT")
	post(owner, "pve", source.ID, 7, 3, 400, "INVALID_REQUEST")
	v := post(owner, "pve", source.ID, 7, 2, 201, "")
	if v.Practice == nil || v.Practice.Ply != 2 || v.Rules != chess.LegacyRules || v.Outcome != "ongoing" || v.Reason != "" || !reflect.DeepEqual(v.Moves, source.Moves) {
		t.Fatal(v)
	}
	other := post(opponent, "pvp", finished.ID, 4, 1, 201, "")
	if other.Practice == nil || other.Practice.SourceID != finished.ID || other.Phase != "queued" || other.Rules != chess.CasualRules {
		t.Fatal(other)
	}
	live, err := games.Create(ctx, owner, "red", "easy")
	if err != nil {
		t.Fatal(err)
	}
	live, err = games.Command(ctx, owner, live.ID, game.Command{RequestID: game.ID(), Version: live.Version, Type: "move", Move: "b0c2"})
	if err != nil {
		t.Fatal(err)
	}
	livePractice := post(owner, "pve", live.ID, live.Version, 1, 201, "")
	if livePractice.Practice == nil || livePractice.Practice.SourceVersion != live.Version || livePractice.Rules != chess.CasualRules || !reflect.DeepEqual(livePractice.Moves, live.Moves) {
		t.Fatal("active source", livePractice)
	}
	stillLive, err := games.Get(owner, live.ID)
	if err != nil || !reflect.DeepEqual(stillLive, live) {
		t.Fatal("active source changed", stillLive, err)
	}
	saved, err := repo.FindGame(ctx, owner, v.ID)
	if err != nil || !reflect.DeepEqual(saved.Practice, v.Practice) || !reflect.DeepEqual(saved.Moves, source.Moves) {
		t.Fatal("practice persistence", saved, err)
	}
	savedSource, err := repo.FindGame(ctx, owner, source.ID)
	if err != nil || savedSource.Version != source.Version || savedSource.Reason != source.Reason || !reflect.DeepEqual(savedSource.Moves, source.Moves) {
		t.Fatal("source changed", savedSource, err)
	}
	games.Close()
	recovered, err := game.New(repo, rules, bots, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer recovered.Close()
	reloaded, err := recovered.Get(owner, v.ID)
	if err != nil || !reflect.DeepEqual(reloaded.Practice, v.Practice) || !reflect.DeepEqual(reloaded.Moves, source.Moves) {
		t.Fatal("restart lost practice", reloaded, err)
	}
}
