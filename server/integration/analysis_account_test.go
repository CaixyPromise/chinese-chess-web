package integration

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"chinesechess/internal/pvp"
	"context"
	"testing"
	"time"
)

func TestAccountAnalysisSeatProjectionAndEngineResume(t *testing.T) {
	ctx, repo, db := analysisDatabase(t)
	red, black, account := game.ID(), game.ID(), game.ID()
	if _, err := db.Exec(ctx, `INSERT INTO auth_owner_links VALUES($1,$3,1),($2,$3,1),($3,$3,1)`, red, black, account); err != nil {
		t.Fatal(err)
	}
	if err := repo.ReloadIdentity(ctx); err != nil {
		t.Fatal(err)
	}
	r := pvp.Record{Host: red, RedOwner: red, BlackOwner: black, State: pvp.State{ID: game.ID(), Version: 1, Phase: "finished", Outcome: "draw", Reason: "agreement", Rules: chess.CasualRules, Fen: chess.Initial, Moves: []string{}, Updated: time.Now().UnixMilli()}}
	if err := repo.SavePVP(ctx, r, 0); err != nil {
		t.Fatal(err)
	}
	source := analysis.Source{Mode: "pvp", ID: r.ID, Version: 1, Phase: "finished", Outcome: "draw", Rules: chess.CasualRules, Fen: chess.Initial, Moves: []string{}, GroupID: "pvp:" + r.ID, Origin: "game"}
	for _, owner := range []string{red, black} {
		j := analysis.Job{ID: game.ID(), Owner: owner, Source: source, Digest: analysis.Digest(source), Engine: "old-test", Scope: "game", Profile: "standard", State: "paused", Generation: 1, Created: 1, Updated: 1}
		if _, err := repo.CreateAnalysis(ctx, j); err != nil {
			t.Fatal(err)
		}
	}
	jobs, err := repo.ListAnalyses(ctx, account, "pvp", r.ID, 1)
	if err != nil || len(jobs) != 2 {
		t.Fatal(jobs, err)
	}
	for _, j := range jobs {
		want := "red"
		if j.Owner == black {
			want = "black"
		}
		if j.ViewerSide != want {
			t.Fatalf("historical task side %+v", j)
		}
	}
	// Zero-move whole-game tasks finish without a native search, so this tests
	// owner provenance on version replacement without a background calculator.
	m, err := analysis.New(repo, nil, nil, func(_ context.Context, who, mode, id string) (analysis.Source, error) {
		if who != account {
			t.Fatalf("loader identity %s", who)
		}
		return source, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	updated, err := m.Control(ctx, account, jobs[0].ID, "resume")
	if err != nil || updated.Owner != jobs[0].Owner || updated.Engine != analysis.EngineVersion || updated.State != "completed" {
		t.Fatalf("replacement provenance %+v %v", updated, err)
	}
}
