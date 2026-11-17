package integration

import (
	"chinesechess/internal/account"
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"chinesechess/internal/pvp"
	"testing"
	"time"
)

func TestAccountStatisticsActivityAndAtomicFacts(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	owner, a, b, other := game.ID(), game.ID(), game.ID(), game.ID()
	now := time.Now().UnixMilli()
	if _, err := conn.Exec(ctx, `INSERT INTO auth_accounts VALUES($1,$2,'fixture','棋友',$3)`, owner, owner+"@example.test", now); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{owner, a, b} {
		if _, err := conn.Exec(ctx, `INSERT INTO auth_owner_links VALUES($1,$2,$3)`, key, owner, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := repo.ReloadIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	addGame := func(who, human, outcome string, practice bool, phase string) game.Record {
		t.Helper()
		r := game.Record{Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Human: human, Phase: phase, Outcome: outcome, Reason: "resign", Rules: chess.CasualRules, Fen: chess.Initial, Moves: []string{"b0c2"}, Updated: now, Difficulty: "easy"}, Owner: who, Receipts: map[string]game.Receipt{}}
		if practice {
			r.Practice = &game.Practice{SourceMode: "pve", SourceID: game.ID(), SourceVersion: 1, Ply: 0, SourceGroupID: "lineage"}
		}
		if err := repo.Save(ctx, r, 0); err != nil {
			t.Fatal(err)
		}
		return r
	}
	first := addGame(a, "red", "red_win", false, "finished")
	addGame(a, "red", "black_win", false, "finished")
	addGame(b, "black", "draw", false, "finished")
	addGame(a, "black", "black_win", true, "finished")
	addGame(other, "red", "red_win", false, "finished")
	for i := 0; i < 9; i++ {
		addGame(a, "red", "ongoing", false, "playing")
	}
	addRoom := func(red, black, outcome, reason string) pvp.Record {
		t.Helper()
		r := pvp.Record{State: pvp.State{ID: game.ID(), Version: 1, Rules: chess.CasualRules, Fen: chess.Initial, Phase: "finished", Outcome: outcome, Reason: reason, Moves: []string{}, Updated: now}, Host: red, RedOwner: red, BlackOwner: black, Receipts: map[string]pvp.Receipt{}}
		if err := repo.SavePVP(ctx, r, 0); err != nil {
			t.Fatal(err)
		}
		return r
	}
	addRoom(a, other, "red_win", "resign")
	addRoom(other, a, "draw", "agreement")
	dual := addRoom(a, b, "black_win", "resign")
	addRoom(a, "", "draw", "cancelled")
	for i, state := range []string{"completed", "queued", "running", "paused", "failed", "completed"} {
		j := analysisJob(a)
		j.Created = now
		j.Updated = now
		j.State = state
		j.Automatic = i == 5
		saved, err := repo.CreateAnalysis(ctx, j)
		if err != nil {
			t.Fatal(err)
		}
		again, err := repo.CreateAnalysis(ctx, j)
		if err != nil || again.ID != saved.ID {
			t.Fatal("repeat task", again, err)
		}
	}
	for mode, want := range map[string]account.Counts{"pve": {Win: 1, Draw: 1, Loss: 1, Total: 3}, "practice": {Win: 1, Total: 1}, "pvp": {Win: 1, Draw: 1, Total: 2}} {
		stats, err := repo.AccountStatistics(ctx, owner, mode)
		if err != nil {
			t.Fatal(err)
		}
		if stats.Results != want {
			t.Fatalf("%s results %+v want %+v", mode, stats, want)
		}
		if stats.Analysis.Total != 6 || stats.Analysis.Completed != 2 || stats.Analysis.Queued != 1 || stats.Analysis.Running != 1 || stats.Analysis.Paused != 1 || stats.Analysis.Failed != 1 {
			t.Fatal("analysis distribution", stats.Analysis)
		}
		if mode == "pvp" && (stats.ExcludedSelfGames != 1 || stats.Sides.Red.Win != 1 || stats.Sides.Black.Draw != 1) {
			t.Fatal("side ownership", stats)
		}
	}
	overview, err := repo.AccountOverview(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	if len(overview.Active) != 9 || len(overview.Recent) != 6 || len(overview.Analyses) != 6 {
		t.Fatalf("overview incomplete: %+v", overview)
	}
	activity, err := repo.AccountActivity(ctx, owner, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(activity.Days) != 365 || activity.Totals != (account.Totals{Game: 7, Practice: 1, Manual: 5, Automatic: 1}) {
		t.Fatalf("activity counts: %+v", activity)
	}
	var bodyOwner string
	if err = conn.QueryRow(ctx, `SELECT body->>'Owner' FROM games WHERE id=$1`, first.ID).Scan(&bodyOwner); err != nil || bodyOwner != a {
		t.Fatalf("source rewritten %q %v", bodyOwner, err)
	}
	// Both historical seats paginate independently even with the same id/time.
	seen := map[string]bool{}
	var cursor string
	for {
		q, e := history.ParseQuery("pvp", cursor, 1)
		if e != nil {
			t.Fatal(e)
		}
		page, e := repo.History(ctx, owner, q)
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range page.Items {
			key := row.ID + ":" + row.Side
			if seen[key] {
				t.Fatal("duplicate history", key)
			}
			seen[key] = true
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if !seen[dual.ID+":red"] || !seen[dual.ID+":black"] {
		t.Fatal("lost historical side", seen)
	}
	// Reopening removes the game from current results; finishing again does not
	// create another daily activity for the same original game.
	first.Version = 2
	first.Phase = "playing"
	first.Outcome = "ongoing"
	if err = repo.Save(ctx, first, 1); err != nil {
		t.Fatal(err)
	}
	stats, err := repo.AccountStatistics(ctx, owner, "pve")
	if err != nil || stats.Results.Total != 2 {
		t.Fatal("reopened game counted", stats, err)
	}
	first.Version = 3
	first.Phase = "finished"
	first.Outcome = "red_win"
	if err = repo.Save(ctx, first, 2); err != nil {
		t.Fatal(err)
	}
	again, err := repo.AccountActivity(ctx, owner, "", "")
	if err != nil || again.Totals != activity.Totals {
		t.Fatal("duplicate finish inflated activity", again.Totals, err)
	}
	// A failed event write must also roll back creation of its practice game.
	if _, err = conn.Exec(ctx, `ALTER TABLE account_activity ADD CONSTRAINT reject_practice CHECK(kind<>'practice') NOT VALID`); err != nil {
		t.Fatal(err)
	}
	failing := game.Record{Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Human: "red", Phase: "playing", Outcome: "ongoing", Updated: now, Practice: &game.Practice{SourceMode: "pve", SourceID: game.ID(), Ply: 0}}, Owner: a}
	if err = repo.Save(ctx, failing, 0); err == nil {
		t.Fatal("accepted failed activity transaction")
	}
	var count int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM games WHERE id=$1`, failing.ID).Scan(&count); err != nil || count != 0 {
		t.Fatal("partial practice commit", count, err)
	}
	if _, err = conn.Exec(ctx, `ALTER TABLE account_activity DROP CONSTRAINT reject_practice`); err != nil {
		t.Fatal(err)
	}
	private, err := repo.AccountOverview(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range private.Active {
		if row.ID == first.ID {
			t.Fatal("foreign active record leaked")
		}
	}
	if _, err = repo.AccountStatistics(ctx, owner, "unsupported"); err == nil {
		t.Fatal("accepted mode")
	}
	if _, err = repo.AccountActivity(ctx, owner, "2027-01-01", "2027-01-02"); err == nil {
		t.Fatal("accepted future range")
	}
}
