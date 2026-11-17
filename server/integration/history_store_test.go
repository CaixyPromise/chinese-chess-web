package integration

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"chinesechess/internal/pvp"
	"chinesechess/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"
)

func TestHistoryPostgresPaginationAndAuthorization(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL history integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	schema := "history_test_" + game.ID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	scopedDSN := schemaDSN(dsn, schema)
	conn, err := pgx.Connect(ctx, scopedDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(context.Background())
	// Exercise the real upgrade from the two original migrations, including their
	// historic capitalized seat keys and unknown JSON fields.
	oldMigrations := fstest.MapFS{}
	for _, name := range []string{"00001_games.sql", "00002_pvp.sql"} {
		data, e := os.ReadFile("../internal/store/migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		oldMigrations[name] = &fstest.MapFile{Data: data}
	}
	db, err := sql.Open("pgx", scopedDSN)
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, oldMigrations)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	owner, other := "history-owner-a", "history-owner-b"
	oldID := fmt.Sprintf("%032x", 900)
	explicitID := fmt.Sprintf("%032x", 901)
	oldPVE := fmt.Sprintf(`{"id":%q,"Owner":%q,"version":1,"updatedAt":7000,"phase":"finished","human":"black","outcome":"human","reason":"legacy-recorded","moves":["a0a1"],"unknownEvidence":{"kept":true}}`, oldID, owner)
	oldPVP := fmt.Sprintf(`{"id":%q,"RedOwner":%q,"BlackOwner":%q,"Host":"former-host","version":1,"updatedAt":7000,"phase":"finished","outcome":"red","reason":"recorded-timeout","moves":["a0a1","a9a8"],"unknownEvidence":123}`, oldID, owner, other)
	if _, err = conn.Exec(ctx, `INSERT INTO games(id,owner_key,version,updated_at,body) VALUES($1,$2,1,7000,$3)`, oldID, owner, oldPVE); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO pvp_rooms(id,version,updated_at,body) VALUES($1,1,7000,$2)`, oldID, oldPVP); err != nil {
		t.Fatal(err)
	}
	var pveBody, pvpBody string
	if err = conn.QueryRow(ctx, `SELECT body::text FROM games WHERE id=$1`, oldID).Scan(&pveBody); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, `SELECT body::text FROM pvp_rooms WHERE id=$1`, oldID).Scan(&pvpBody); err != nil {
		t.Fatal(err)
	}
	repo, err := store.Open(ctx, scopedDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	expected := []history.Entry{
		{ID: oldID, Mode: "pve", Side: "black", RulesVersion: "legacy-java-v1", Outcome: "human", Reason: "legacy-recorded", MoveCount: 1, UpdatedAt: 7000},
		{ID: oldID, Mode: "pvp", Side: "red", RulesVersion: "casual-xiangqi-v1", Outcome: "red", Reason: "recorded-timeout", MoveCount: 2, UpdatedAt: 7000},
	}
	// Both tables intentionally reuse IDs and timestamps, requiring mode as the
	// third stable cursor component. More than 256 finished records per mode.
	for i := 1; i <= 300; i++ {
		id := fmt.Sprintf("%032x", i)
		updated := int64(10000 + i%3)
		g := game.Record{Owner: owner, Snapshot: game.Snapshot{ID: id, Version: 1, Updated: updated, Phase: "finished", Human: "red", Outcome: "draw", Reason: "saved-pve", Moves: []string{"a0a1"}}}
		r := pvp.Record{RedOwner: owner, BlackOwner: other, State: pvp.State{ID: id, Version: 1, Updated: updated, Phase: "finished", Outcome: "black", Reason: "saved-pvp", Moves: []string{"a0a1", "a9a8"}}}
		if err = repo.Save(ctx, g, 0); err != nil {
			t.Fatal(err)
		}
		if err = repo.SavePVP(ctx, r, 0); err != nil {
			t.Fatal(err)
		}
		expected = append(expected,
			history.Entry{ID: id, Mode: "pve", Side: "red", RulesVersion: "legacy-java-v1", Outcome: "draw", Reason: "saved-pve", MoveCount: 1, UpdatedAt: updated},
			history.Entry{ID: id, Mode: "pvp", Side: "red", RulesVersion: "casual-xiangqi-v1", Outcome: "black", Reason: "saved-pvp", MoveCount: 2, UpdatedAt: updated})
	}
	// Explicit rule versions and stored adjudication survive all read paths.
	explicitGame := game.Record{Owner: owner, Snapshot: game.Snapshot{ID: explicitID, Version: 1, Updated: 6000, Rules: "historical-pve-version", Phase: "finished", Human: "red", Outcome: "engine", Reason: "archived-result"}}
	explicitPVP := pvp.Record{RedOwner: owner, State: pvp.State{ID: explicitID, Version: 1, Updated: 6000, Rules: "historical-pvp-version", Phase: "finished", Outcome: "draw", Reason: "archived-agreement"}}
	if err = repo.Save(ctx, explicitGame, 0); err != nil {
		t.Fatal(err)
	}
	if err = repo.SavePVP(ctx, explicitPVP, 0); err != nil {
		t.Fatal(err)
	}
	expected = append(expected,
		history.Entry{ID: explicitID, Mode: "pve", Side: "red", RulesVersion: "historical-pve-version", Outcome: "engine", Reason: "archived-result", UpdatedAt: 6000},
		history.Entry{ID: explicitID, Mode: "pvp", Side: "red", RulesVersion: "historical-pvp-version", Outcome: "draw", Reason: "archived-agreement", UpdatedAt: 6000})
	activeID := fmt.Sprintf("%032x", 902)
	activeGame := game.Record{Owner: owner, Snapshot: game.Snapshot{ID: activeID, Version: 1, Updated: 20000, Phase: "playing", Outcome: "ongoing"}}
	activePVP := pvp.Record{RedOwner: owner, State: pvp.State{ID: activeID, Version: 1, Updated: 20000, Phase: "waiting", Outcome: "ongoing"}}
	if err = repo.Save(ctx, activeGame, 0); err != nil {
		t.Fatal(err)
	}
	if err = repo.SavePVP(ctx, activePVP, 0); err != nil {
		t.Fatal(err)
	}
	// A former host with no current seat cannot retrieve even their old room.
	departedID := fmt.Sprintf("%032x", 903)
	departed := pvp.Record{Host: owner, RedOwner: other, State: pvp.State{ID: departedID, Version: 1, Updated: 15000, Phase: "finished", Outcome: "red"}}
	if err = repo.SavePVP(ctx, departed, 0); err != nil {
		t.Fatal(err)
	}
	otherGameID := fmt.Sprintf("%032x", 904)
	if err = repo.Save(ctx, game.Record{Owner: other, Snapshot: game.Snapshot{ID: otherGameID, Version: 1, Updated: 5000, Phase: "finished", Outcome: "draw"}}, 0); err != nil {
		t.Fatal(err)
	}
	games, err := repo.Load(ctx)
	if err != nil || len(games) != 1 || games[0].ID != activeID {
		t.Fatalf("active-only PVE: len=%d err=%v", len(games), err)
	}
	rooms, err := repo.LoadPVP(ctx)
	if err != nil || len(rooms) != 1 || rooms[0].ID != activeID {
		t.Fatalf("active-only PVP: len=%d err=%v", len(rooms), err)
	}
	if _, err = repo.FindGame(ctx, other, oldID); !errors.Is(err, game.ErrNotFound) {
		t.Fatalf("foreign PVE: %v", err)
	}
	for _, foreign := range []string{owner, "", "unknown"} {
		if _, err = repo.FindPVP(ctx, foreign, departedID); !errors.Is(err, game.ErrNotFound) {
			t.Fatalf("departed/foreign PVP: %q %v", foreign, err)
		}
	}
	if _, err = repo.FindPVP(ctx, other, oldID); err != nil {
		t.Fatalf("black-seat read: %v", err)
	}
	oldGame, err := repo.FindGame(ctx, owner, oldID)
	if err != nil || oldGame.Rules != "legacy-java-v1" || oldGame.Reason != "legacy-recorded" {
		t.Fatalf("legacy PVE: %#v %v", oldGame, err)
	}
	oldRoom, err := repo.FindPVP(ctx, owner, oldID)
	if err != nil || oldRoom.Rules != "casual-xiangqi-v1" || oldRoom.Outcome != "red" || oldRoom.Reason != "recorded-timeout" {
		t.Fatalf("legacy PVP: %#v %v", oldRoom, err)
	}
	foundGame, err := repo.FindGame(ctx, owner, explicitID)
	if err != nil || foundGame.Rules != explicitGame.Rules {
		t.Fatalf("explicit PVE: %#v %v", foundGame, err)
	}
	foundRoom, err := repo.FindPVP(ctx, owner, explicitID)
	if err != nil || foundRoom.Rules != explicitPVP.Rules {
		t.Fatalf("explicit PVP: %#v %v", foundRoom, err)
	}
	recentGames, err := repo.RecentGames(ctx, owner)
	if err != nil || len(recentGames) != 8 || recentGames[0].ID != activeID || recentGames[1].Phase != "finished" {
		t.Fatalf("recent PVE: %#v %v", recentGames, err)
	}
	recentRooms, err := repo.RecentPVP(ctx, owner)
	if err != nil || len(recentRooms) != 8 || recentRooms[0].ID != activeID || recentRooms[1].Phase != "finished" {
		t.Fatalf("recent PVP: %#v %v", recentRooms, err)
	}
	sort.Slice(expected, func(i, j int) bool {
		a, b := expected[i], expected[j]
		if a.UpdatedAt != b.UpdatedAt {
			return a.UpdatedAt > b.UpdatedAt
		}
		if a.ID != b.ID {
			return a.ID > b.ID
		}
		return a.Mode > b.Mode
	})
	var firstCursor string
	for _, mode := range []string{"all", "pve", "pvp"} {
		want := []history.Entry{}
		for _, e := range expected {
			if mode == "all" || mode == e.Mode {
				want = append(want, e)
			}
		}
		got := []history.Entry{}
		cursor := ""
		for pages := 0; ; pages++ {
			if pages > 1000 {
				t.Fatal("pagination did not terminate")
			}
			q, e := history.ParseQuery(mode, cursor, 7)
			if e != nil {
				t.Fatal(e)
			}
			page, e := repo.History(ctx, owner, q)
			if e != nil {
				t.Fatal(e)
			}
			if page.Items == nil || len(page.Items) > 7 {
				t.Fatalf("bad page: %#v", page)
			}
			got = append(got, page.Items...)
			if pages == 0 && mode == "all" {
				firstCursor = page.NextCursor
			}
			if page.NextCursor == "" {
				break
			}
			if len(page.Items) != 7 || page.NextCursor == cursor {
				t.Fatal("invalid next cursor")
			}
			cursor = page.NextCursor
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s pagination mismatch: got %d want %d; first got=%#v want=%#v", mode, len(got), len(want), got[:min(3, len(got))], want[:min(3, len(want))])
		}
	}
	// A full final page must not advertise a nonexistent next page.
	lastQuery, _ := history.ParseQuery("pve", history.Cursor("pve", history.Entry{ID: oldID, Mode: "pve", UpdatedAt: 7000}), 1)
	lastPage, err := repo.History(ctx, owner, lastQuery)
	if err != nil || len(lastPage.Items) != 1 || lastPage.Items[0].ID != explicitID || lastPage.NextCursor != "" {
		t.Fatalf("exact final page: %#v %v", lastPage, err)
	}
	emptyQuery, _ := history.ParseQuery("pve", history.Cursor("pve", lastPage.Items[0]), 1)
	emptyPage, err := repo.History(ctx, owner, emptyQuery)
	if err != nil || emptyPage.Items == nil || len(emptyPage.Items) != 0 || emptyPage.NextCursor != "" {
		t.Fatalf("past final page: %#v %v", emptyPage, err)
	}
	// Cursor contains only a position: changing authenticated visitors still scopes
	// the SQL to that visitor's currently owned rows, with their own seat perspective.
	q, err := history.ParseQuery("all", firstCursor, 50)
	if err != nil {
		t.Fatal(err)
	}
	foreignPage, err := repo.History(ctx, other, q)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range foreignPage.Items {
		if e.Mode == "pve" && e.ID != otherGameID {
			t.Fatalf("cursor leaked PVE: %#v", e)
		}
		if e.Mode == "pvp" && e.Side != "black" {
			t.Fatalf("cursor leaked PVP perspective: %#v", e)
		}
	}
	rawCursor, err := base64.RawURLEncoding.DecodeString(firstCursor)
	if err != nil || strings.Contains(string(rawCursor), owner) || strings.Contains(string(rawCursor), "Invite") {
		t.Fatalf("cursor leaks credentials: %s %v", rawCursor, err)
	}
	for _, visitor := range []string{"unknown", ""} {
		q, _ := history.ParseQuery("all", "", 20)
		empty, err := repo.History(ctx, visitor, q)
		body, _ := json.Marshal(empty)
		if err != nil || string(body) != `{"items":[]}` {
			t.Fatalf("empty page: %s %v", body, err)
		}
		rg, e := repo.RecentGames(ctx, visitor)
		if e != nil || len(rg) != 0 {
			t.Fatalf("foreign recent games: %v %v", rg, e)
		}
		rp, e := repo.RecentPVP(ctx, visitor)
		if e != nil || len(rp) != 0 {
			t.Fatalf("foreign recent rooms: %v %v", rp, e)
		}
	}
	if _, err = repo.History(ctx, owner, history.Query{Mode: "all", Limit: 51}); !errors.Is(err, history.ErrInvalidQuery) {
		t.Fatalf("unchecked direct query: %v", err)
	}
	// Existing optimistic concurrency guards and normalized seat columns remain live.
	activeGame.Version = 2
	if err = repo.Save(ctx, activeGame, 1); err != nil {
		t.Fatal(err)
	}
	if err = repo.Save(ctx, activeGame, 1); err == nil {
		t.Fatal("PVE CAS accepted stale version")
	}
	activeGame.Owner = other
	activeGame.Version = 3
	if err = repo.Save(ctx, activeGame, 2); err == nil {
		t.Fatal("PVE CAS accepted owner change")
	}
	activePVP.Version = 2
	activePVP.RedOwner = other
	if err = repo.SavePVP(ctx, activePVP, 1); err != nil {
		t.Fatal(err)
	}
	if err = repo.SavePVP(ctx, activePVP, 1); err == nil {
		t.Fatal("PVP CAS accepted stale version")
	}
	if _, err = repo.FindPVP(ctx, owner, activeID); !errors.Is(err, game.ErrNotFound) {
		t.Fatalf("old seat still authorized: %v", err)
	}
	if _, err = repo.FindPVP(ctx, other, activeID); err != nil {
		t.Fatalf("new seat not authorized: %v", err)
	}
	activeGame.Owner = owner
	activeGame.Phase = "finished"
	if err = repo.Save(ctx, activeGame, 2); err != nil {
		t.Fatal(err)
	}
	activePVP.Version = 3
	activePVP.Phase = "finished"
	if err = repo.SavePVP(ctx, activePVP, 2); err != nil {
		t.Fatal(err)
	}
	if remaining, e := repo.Load(ctx); e != nil || len(remaining) != 0 {
		t.Fatalf("PVE phase update not normalized: %v %v", remaining, e)
	}
	if remaining, e := repo.LoadPVP(ctx); e != nil || len(remaining) != 0 {
		t.Fatalf("PVP phase update not normalized: %v %v", remaining, e)
	}
	var afterPVE, afterPVP string
	if err = conn.QueryRow(ctx, `SELECT body::text FROM games WHERE id=$1`, oldID).Scan(&afterPVE); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, `SELECT body::text FROM pvp_rooms WHERE id=$1`, oldID).Scan(&afterPVP); err != nil {
		t.Fatal(err)
	}
	if pveBody != afterPVE || pvpBody != afterPVP {
		t.Fatal("migration or reads rewrote historic body")
	}
}
