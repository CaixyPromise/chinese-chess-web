package integration

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"chinesechess/internal/httpapi"
	"chinesechess/internal/pvp"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLinkedDomainAuthorizationPreservesFactsAndAnalysisOwner(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	first, second, account, foreign := game.ID(), game.ID(), game.ID(), game.ID()
	g := game.Record{Owner: first, Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Rules: chess.CasualRules, Human: "red", Fen: chess.Initial, Moves: []string{"b0c2"}, Phase: "finished", Outcome: "black_win", Reason: "resign", Updated: 100}}
	r := pvp.Record{Host: first, RedOwner: first, BlackOwner: second, Invite: game.ID(), State: pvp.State{ID: game.ID(), Version: 1, Rules: chess.CasualRules, Fen: chess.Initial, Moves: []string{"b0c2"}, Phase: "finished", Outcome: "red_win", Reason: "resign", Updated: 101}}
	if e := repo.Save(ctx, g, 0); e != nil {
		t.Fatal(e)
	}
	if e := repo.SavePVP(ctx, r, 0); e != nil {
		t.Fatal(e)
	}
	source := analysis.Source{Mode: "pve", ID: g.ID, Version: g.Version, Moves: g.Moves, Rules: g.Rules, Phase: g.Phase, Fen: g.Fen, GroupID: "pve:" + g.ID, Outcome: g.Outcome, Origin: "game"}
	snapshot, e := repo.SaveReplaySnapshot(ctx, first, source)
	if e != nil {
		t.Fatal(e)
	}
	if e = repo.MaterializeAnalyses(ctx, analysis.EngineVersion); e != nil {
		t.Fatal(e)
	}
	originalJobs, e := repo.ListAnalyses(ctx, first, "pve", g.ID, 1)
	if e != nil || len(originalJobs) != 1 {
		t.Fatal(originalJobs, e)
	}
	original := originalJobs[0]
	original.State = "running"
	if e = repo.SaveAnalysis(ctx, original, 1, "queued"); e != nil {
		t.Fatal(e)
	}
	original.State = "queued"
	original.Completed = 1
	original.NextPly = 1
	position := analysis.Position{Ply: 0, SampleID: "stable-sample", PositionID: "stable-position", GroupID: source.GroupID, SourceID: g.ID, SourceVersion: 1, SourceMode: "pve", Outcome: g.Outcome, Final: true, Budget: original.Budget, Teacher: original.Engine}
	if e = repo.CompletePosition(ctx, original, 1, position, ""); e != nil {
		t.Fatal(e)
	}
	var gameBefore, roomBefore, eventBefore []byte
	if e = conn.QueryRow(ctx, `SELECT body FROM games WHERE id=$1`, g.ID).Scan(&gameBefore); e != nil {
		t.Fatal(e)
	}
	if e = conn.QueryRow(ctx, `SELECT body FROM pvp_rooms WHERE id=$1`, r.ID).Scan(&roomBefore); e != nil {
		t.Fatal(e)
	}
	if e = conn.QueryRow(ctx, `SELECT frozen_record FROM analysis_finished_events WHERE source_id=$1`, g.ID).Scan(&eventBefore); e != nil {
		t.Fatal(e)
	}
	if _, e = repo.FindGame(ctx, account, g.ID); !errors.Is(e, game.ErrNotFound) {
		t.Fatal("unlinked read", e)
	}
	if _, e = conn.Exec(ctx, `INSERT INTO auth_owner_links(owner_key,account_id,created_at) VALUES($1,$3,1),($2,$3,1),($3,$3,1)`, first, second, account); e != nil {
		t.Fatal(e)
	}
	if e = repo.ReloadIdentity(ctx); e != nil {
		t.Fatal(e)
	}
	if !repo.SameOwner(first, second) || !repo.SameOwner(first, account) || repo.SameOwner(first, foreign) || repo.SameOwner("", "") {
		t.Fatal("identity relation")
	}
	if got, e := repo.FindGame(ctx, account, g.ID); e != nil || !reflect.DeepEqual(g, got) {
		t.Fatal("linked game/facts", got, e)
	}
	if got, e := repo.FindPVP(ctx, account, r.ID); e != nil || got.RedOwner != first || got.BlackOwner != second {
		t.Fatal("linked room", got, e)
	}
	if rows, e := repo.RecentGames(ctx, account); e != nil || len(rows) != 1 {
		t.Fatal("linked recent games", rows, e)
	}
	if rows, e := repo.RecentPVP(ctx, account); e != nil || len(rows) != 1 {
		t.Fatal("linked recent rooms must not duplicate sides", rows, e)
	}
	if frozen, e := repo.FindReplaySnapshot(ctx, account, snapshot); e != nil || !reflect.DeepEqual(source, frozen) {
		t.Fatal("linked frozen source", frozen, e)
	}
	jobs, e := repo.ListAnalyses(ctx, account, "pvp", r.ID, 1)
	if e != nil || len(jobs) != 2 || jobs[0].Owner == jobs[1].Owner {
		t.Fatal("dual original viewer jobs", jobs, e)
	}
	got, e := repo.FindAnalysis(ctx, account, original.ID)
	if e != nil || got.Owner != first || got.Digest != original.Digest {
		t.Fatal("analysis owner was rewritten", got, e)
	}
	page, e := repo.AnalysisPositions(ctx, account, original.ID, 0, 100)
	if e != nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], position) {
		t.Fatal("linked immutable sample", page, e)
	}
	// Analysis control uses the original job owner returned by FindAnalysis for CAS.
	got.State = "paused"
	got.Generation++
	if e = repo.SaveAnalysis(ctx, got, 1, "queued"); e != nil {
		t.Fatal("linked cancel CAS", e)
	}
	again, e := repo.FindAnalysis(ctx, account, original.ID)
	if e != nil || again.Owner != first || again.State != "paused" {
		t.Fatal(again, e)
	}
	again.State = "queued"
	again.Generation++
	if e = repo.SaveAnalysis(ctx, again, 2, "paused"); e != nil {
		t.Fatal("linked resume CAS", e)
	}
	for _, who := range []string{foreign, ""} {
		if _, e = repo.FindGame(ctx, who, g.ID); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("foreign game", e)
		}
		if _, e = repo.FindPVP(ctx, who, r.ID); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("foreign room", e)
		}
		if _, e = repo.FindAnalysis(ctx, who, original.ID); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("foreign job", e)
		}
		if _, e = repo.FindReplaySnapshot(ctx, who, snapshot); !errors.Is(e, game.ErrNotFound) {
			t.Fatal("foreign snapshot", e)
		}
	}
	// Two sides survive a one-row pagination boundary with the same source tuple.
	var seen []history.Entry
	cursor := ""
	for i := 0; i < 4; i++ {
		q, e := history.ParseQuery("pvp", cursor, 1)
		if e != nil {
			t.Fatal(e)
		}
		p, e := repo.History(ctx, account, q)
		if e != nil {
			t.Fatal(e)
		}
		seen = append(seen, p.Items...)
		if p.NextCursor == "" {
			break
		}
		cursor = p.NextCursor
	}
	if len(seen) != 2 || seen[0].Side != "red" || seen[1].Side != "black" || seen[0].ID != r.ID || seen[1].ID != r.ID {
		t.Fatal("dual-side pagination", seen)
	}
	for table, want := range map[string][]byte{"games": gameBefore, "pvp_rooms": roomBefore, "analysis_finished_events": eventBefore} {
		var after []byte
		query := `SELECT body FROM ` + table + ` WHERE id=$1`
		id := g.ID
		if table == "pvp_rooms" {
			id = r.ID
		}
		if table == "analysis_finished_events" {
			query = `SELECT frozen_record FROM analysis_finished_events WHERE source_id=$1`
		}
		if e = conn.QueryRow(ctx, query, id).Scan(&after); e != nil || !reflect.DeepEqual(want, after) {
			t.Fatal("claim rewrote facts", table, e)
		}
	}
}

func TestLinkedManagersQuotaAndDualFinishedPerspective(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	first, second, account := game.ID(), game.ID(), game.ID()
	binary, _ := filepath.Abs("../../build/engine/xiangqi-engine")
	rules := engine.NewPool(binary, 1, 16)
	bots := engine.NewPool(binary, 1, 0)
	defer rules.Close()
	defer bots.Close()
	for i := 0; i < 8; i++ {
		owner := first
		if i%2 == 1 {
			owner = second
		}
		g := game.Record{Owner: owner, Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Rules: chess.CasualRules, Human: "red", Fen: chess.Initial, Moves: []string{}, Phase: "playing", Outcome: "ongoing"}}
		if e := repo.Save(ctx, g, 0); e != nil {
			t.Fatal(e)
		}
	}
	room := pvp.Record{Host: first, RedOwner: first, BlackOwner: second, Invite: game.ID(), State: pvp.State{ID: game.ID(), Version: 1, Rules: chess.CasualRules, Fen: chess.Initial, Moves: []string{}, Phase: "finished", Outcome: "draw", Reason: "agreement", Updated: 101}}
	if e := repo.SavePVP(ctx, room, 0); e != nil {
		t.Fatal(e)
	}
	if _, e := conn.Exec(ctx, `INSERT INTO auth_owner_links(owner_key,account_id,created_at) VALUES($1,$3,1),($2,$3,1),($3,$3,1)`, first, second, account); e != nil {
		t.Fatal(e)
	}
	if e := repo.ReloadIdentity(ctx); e != nil {
		t.Fatal(e)
	}
	gm, e := game.New(repo, rules, bots, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer gm.Close()
	_, e = gm.Create(ctx, account, "red", "easy")
	assertDomainCode(t, e, "ACTIVE_GAME_LIMIT")
	pm, e := pvp.New(repo, rules)
	if e != nil {
		t.Fatal(e)
	}
	defer pm.Close()
	both, e := pm.Get(account, room.ID)
	if e != nil || both.You != "" || !reflect.DeepEqual(both.ViewChoices, []string{"red", "black"}) {
		t.Fatal("silent side choice", both, e)
	}
	red, e := pm.GetSide(account, room.ID, "red")
	if e != nil || red.You != "red" {
		t.Fatal(red, e)
	}
	black, _, stop, e := pm.WatchSide(account, room.ID, "black")
	if e != nil {
		t.Fatal(e)
	}
	stop()
	if black.You != "black" {
		t.Fatal(black)
	}
	loader := httpapi.AnalysisLoader(gm, pm)
	if _, e = loader(ctx, account, "pvp", room.ID); e != nil {
		t.Fatal("dual source read authorization", e)
	}
}
func assertDomainCode(t *testing.T, e error, want string) {
	t.Helper()
	var p *game.Error
	if !errors.As(e, &p) || p.Code != want {
		t.Fatalf("want %s got %v", want, e)
	}
}

func TestPVPInvitationApplicationAndInvalidationPersistInPostgres(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	host, guest := game.ID(), game.ID()
	binary, _ := filepath.Abs("../../build/engine/xiangqi-engine")
	rules := engine.NewPool(binary, 1, 8)
	defer rules.Close()
	manager, e := pvp.New(repo, rules)
	if e != nil {
		t.Fatal(e)
	}
	v, e := manager.Create(ctx, host, "black", "房主")
	if e != nil {
		t.Fatal(e)
	}
	invite := v.Invite
	a, e := manager.ApplyJoin(guest, invite, "访客", game.ID())
	if e != nil {
		t.Fatal(e)
	}
	manager.Close()
	recovered, e := pvp.New(repo, rules)
	if e != nil {
		t.Fatal(e)
	}
	defer recovered.Close()
	p, e := recovered.Preview(guest, invite)
	if e != nil || p.Application == nil || p.Application.ApplicationID != a.ApplicationID || p.Application.State != "pending" {
		t.Fatal("application did not survive restart", p, e)
	}
	before, e := recovered.Get(host, v.ID)
	if e != nil || before.PendingApplication == nil {
		t.Fatal(before, e)
	}
	_, e = recovered.Command(ctx, host, v.ID, pvp.Command{RequestID: game.ID(), Version: before.Version, Type: "leave"})
	if e != nil {
		t.Fatal(e)
	}
	p, e = recovered.Preview(guest, invite)
	if e != nil || p.Status != "cancelled" || p.Application.State != "cancelled" {
		t.Fatal("durable invitation-only lookup", p, e)
	}
	if _, e = recovered.Get(guest, v.ID); !errors.Is(e, game.ErrNotFound) {
		t.Fatal("cancelled applicant got private room", e)
	}
	// Fresh PostgreSQL instance reads its own committed identity map, with no global leakage.
	if repo.SameOwner(host, guest) {
		t.Fatal("unlinked room owners canonicalized")
	}
}
