package integration

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"chinesechess/internal/pvp"
	"chinesechess/internal/store"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
)

func analysisDatabase(t *testing.T) (context.Context, *store.Postgres, *pgx.Conn) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run real PostgreSQL analysis integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	t.Cleanup(cancel)
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	schema := "analysis_test_" + game.ID()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close(context.Background())
	})
	scoped := schemaDSN(dsn, schema)
	repo, err := store.Open(ctx, scoped)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(repo.Close)
	conn, err := pgx.Connect(ctx, scoped)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close(context.Background()) })
	return ctx, repo, conn
}
func analysisJob(owner string) analysis.Job {
	src := analysis.Source{Mode: "pve", ID: game.ID(), Version: 7, Moves: []string{"b0c2", "b9c7"}, Rules: "legacy-java-v1", Phase: "finished"}
	return analysis.Job{ID: game.ID(), Owner: owner, Source: src, Digest: analysis.Digest(src), Engine: analysis.EngineVersion, Scope: "game", Profile: "standard", State: "queued", Generation: 1, Total: 3, Created: 1, Updated: 1}
}
func assertAnalysisConflict(t *testing.T, err error) {
	t.Helper()
	var ge *game.Error
	if !errors.As(err, &ge) || ge.Code != "VERSION_CONFLICT" {
		t.Fatalf("expected VERSION_CONFLICT, got %v", err)
	}
}
func TestAnalysisFinishedOutboxFrozenAndAtomic(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	count := func(table string) int {
		t.Helper()
		var n int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	old := game.Record{Owner: "historic", Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Phase: "finished", Moves: []string{"b0c2"}}}
	body, _ := json.Marshal(old)
	if _, err := conn.Exec(ctx, `INSERT INTO games(id,owner_key,version,updated_at,body,phase)VALUES($1,$2,1,1,$3,'finished')`, old.ID, old.Owner, body); err != nil {
		t.Fatal(err)
	}
	if err := repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	if count("analysis_jobs") != 0 {
		t.Fatal("historic records backfilled")
	}
	g := game.Record{Owner: "owner-a", Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Phase: "playing", Moves: []string{"b0c2", "b9c7"}}}
	r := pvp.Record{RedOwner: "red-owner", BlackOwner: "black-owner", State: pvp.State{ID: game.ID(), Version: 1, Phase: "playing", Moves: []string{"b0c2"}}}
	if err := repo.Save(ctx, g, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.SavePVP(ctx, r, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE analysis_finished_events ADD CONSTRAINT reject_events CHECK(false)`); err != nil {
		t.Fatal(err)
	}
	g.Version = 2
	g.Phase = "finished"
	r.Version = 2
	r.Phase = "finished"
	if err := repo.Save(ctx, g, 1); err == nil {
		t.Fatal("outbox failure accepted PVE save")
	}
	if err := repo.SavePVP(ctx, r, 1); err == nil {
		t.Fatal("outbox failure accepted PVP save")
	}
	got, _ := repo.FindGame(ctx, g.Owner, g.ID)
	room, _ := repo.FindPVP(ctx, r.RedOwner, r.ID)
	if got.Version != 1 || got.Phase != "playing" || room.Version != 1 || room.Phase != "playing" || count("analysis_finished_events") != 0 {
		t.Fatal("transaction partially committed")
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE analysis_finished_events DROP CONSTRAINT reject_events`); err != nil {
		t.Fatal(err)
	}
	if err := repo.Save(ctx, g, 0); err == nil {
		t.Fatal("stale PVE save accepted")
	}
	if err := repo.SavePVP(ctx, r, 0); err == nil {
		t.Fatal("stale PVP save accepted")
	}
	if count("analysis_finished_events") != 0 {
		t.Fatal("failed CAS left event")
	}
	if err := repo.Save(ctx, g, 1); err != nil {
		t.Fatal(err)
	}
	if err := repo.SavePVP(ctx, r, 1); err != nil {
		t.Fatal(err)
	}
	g.Version = 3
	r.Version = 3
	if err := repo.Save(ctx, g, 2); err != nil {
		t.Fatal(err)
	}
	if err := repo.SavePVP(ctx, r, 2); err != nil {
		t.Fatal(err)
	}
	if count("analysis_finished_events") != 2 {
		t.Fatal("metadata update duplicated event")
	}
	g.Version = 4
	g.Phase = "playing"
	g.Moves = g.Moves[:1]
	if err := repo.Save(ctx, g, 3); err != nil {
		t.Fatal(err)
	}
	// A failed materialization rolls back both all created jobs and the event
	// acknowledgment, so the next worker can retry the exact frozen inputs.
	if _, err := conn.Exec(ctx, `ALTER TABLE analysis_jobs ADD CONSTRAINT reject_jobs CHECK(false)`); err != nil {
		t.Fatal(err)
	}
	if err := repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err == nil {
		t.Fatal("materialization failure accepted")
	}
	if count("analysis_jobs") != 0 || count("analysis_finished_events") != 2 {
		t.Fatal("materialization partially committed")
	}
	if _, err := conn.Exec(ctx, `ALTER TABLE analysis_jobs DROP CONSTRAINT reject_jobs`); err != nil {
		t.Fatal(err)
	}
	for n := 0; n < 2; n++ {
		if err := repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
			t.Fatal(err)
		}
	}
	if count("analysis_jobs") != 3 || count("analysis_finished_events") != 2 {
		t.Fatal("materialization duplicated or lost jobs")
	}
	jobs, err := repo.ListAnalyses(ctx, g.Owner, "pve", g.ID, 2)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("frozen PVE %#v %v", jobs, err)
	}
	j := jobs[0]
	if !reflect.DeepEqual(j.Source.Moves, []string{"b0c2", "b9c7"}) || j.Source.Phase != "finished" || j.Source.Rules != "legacy-java-v1" || j.Total != 3 || j.NextPly != 0 || !j.Automatic || j.State != "queued" {
		t.Fatalf("bad frozen job %#v", j)
	}
	red, err := repo.ListAnalyses(ctx, r.RedOwner, "pvp", r.ID, 2)
	if err != nil || len(red) != 1 {
		t.Fatalf("red job %v %v", red, err)
	}
	black, err := repo.ListAnalyses(ctx, r.BlackOwner, "pvp", r.ID, 2)
	if err != nil || len(black) != 1 {
		t.Fatalf("black job %v %v", black, err)
	}
	if red[0].ID == black[0].ID || red[0].Digest != black[0].Digest || red[0].Source.Rules != "casual-xiangqi-v1" {
		t.Fatal("seat identity wrong")
	}
	pause := red[0]
	pause.State = "paused"
	pause.Generation++
	if err = repo.SaveAnalysis(ctx, pause, 1, "queued"); err != nil {
		t.Fatal(err)
	}
	unaffected, _ := repo.FindAnalysis(ctx, r.BlackOwner, black[0].ID)
	if unaffected.State != "queued" || unaffected.Generation != 1 {
		t.Fatal("cancel affected opponent")
	}
	empty := game.Record{Owner: "empty-owner", Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Phase: "finished"}}
	if err = repo.Save(ctx, empty, 0); err != nil {
		t.Fatal(err)
	}
	if err = repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	zeros, err := repo.ListAnalyses(ctx, empty.Owner, "pve", empty.ID, 1)
	if err != nil || len(zeros) != 1 || zeros[0].State != "completed" || zeros[0].Total != 0 || zeros[0].Completed != 0 {
		t.Fatalf("zero move job %#v %v", zeros, err)
	}
}
func TestAnalysisPositionCASAuthorizationCacheAndRecovery(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	j := analysisJob("owner")
	var err error
	j, err = repo.CreateAnalysis(ctx, j)
	if err != nil {
		t.Fatal(err)
	}
	repeated := j
	repeated.ID = game.ID()
	repeated.Completed = 99
	saved, err := repo.CreateAnalysis(ctx, repeated)
	if err != nil || saved.ID != j.ID || saved.Completed != 0 || saved.Owner != j.Owner {
		t.Fatalf("idempotent create %#v %v", saved, err)
	}
	for _, owner := range []string{"other", ""} {
		if _, err = repo.FindAnalysis(ctx, owner, j.ID); !errors.Is(err, game.ErrNotFound) {
			t.Fatalf("foreign read %v", err)
		}
		if _, err = repo.AnalysisPositions(ctx, owner, j.ID, 0, 5); !errors.Is(err, game.ErrNotFound) {
			t.Fatalf("foreign positions %v", err)
		}
		items, e := repo.ListAnalyses(ctx, owner, j.Source.Mode, j.Source.ID, j.Source.Version)
		if e != nil || items == nil || len(items) != 0 {
			t.Fatalf("foreign list %v %v", items, e)
		}
	}
	j.State = "running"
	if err = repo.SaveAnalysis(ctx, j, 1, "queued"); err != nil {
		t.Fatal(err)
	}
	assertAnalysisConflict(t, repo.SaveAnalysis(ctx, j, 1, "queued"))
	key := "history-current-engine-standard"
	for ply := 0; ply < 3; ply++ {
		j.Completed = ply + 1
		j.NextPly = ply + 1
		pos := analysis.Position{Ply: ply, Best: analysis.Line{Score: ply, PV: []string{"b0c2"}, Reliable: true, Depth: 6}}
		if err = repo.CompletePosition(ctx, j, 1, pos, key); err != nil {
			t.Fatal(err)
		}
	}
	inflated := j
	inflated.Completed = 99
	if err = repo.CompletePosition(ctx, inflated, 1, analysis.Position{Ply: 2, Best: analysis.Line{Score: 999}}, key); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.FindAnalysis(ctx, j.Owner, j.ID)
	if err != nil || saved.Completed != 3 {
		t.Fatalf("duplicate advanced progress %#v %v", saved, err)
	}
	page, err := repo.AnalysisPositions(ctx, j.Owner, j.ID, 0, 2)
	if err != nil || len(page.Items) != 2 || page.Next != 2 {
		t.Fatalf("first page %#v %v", page, err)
	}
	page, err = repo.AnalysisPositions(ctx, j.Owner, j.ID, page.Next, 2)
	if err != nil || len(page.Items) != 1 || page.Items[0].Ply != 2 || page.Next != 0 {
		t.Fatalf("last page %#v %v", page, err)
	}
	page, err = repo.AnalysisPositions(ctx, j.Owner, j.ID, 3, 100)
	if err != nil || page.Items == nil || len(page.Items) != 0 {
		t.Fatalf("empty page %#v %v", page, err)
	}
	cached, found, err := repo.CachedAnalysisPosition(ctx, key, 2)
	if err != nil || !found || cached.Best.Score != 2 {
		t.Fatalf("cache overwrite %#v %v %v", cached, found, err)
	}
	if _, found, err = repo.CachedAnalysisPosition(ctx, key+":old-engine", 2); err != nil || found {
		t.Fatal("mixed engine cache")
	}
	paused := analysisJob("paused")
	paused.State = "paused"
	paused, err = repo.CreateAnalysis(ctx, paused)
	if err != nil {
		t.Fatal(err)
	}
	old := analysisJob("old")
	old.Engine = "0.5.1"
	old.State = "running"
	old, err = repo.CreateAnalysis(ctx, old)
	if err != nil {
		t.Fatal(err)
	}
	terminal := analysisJob("terminal")
	terminal.Engine = "0.5.1"
	terminal.State = "completed"
	terminal, err = repo.CreateAnalysis(ctx, terminal)
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.RecoverAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	saved, err = repo.FindAnalysis(ctx, j.Owner, j.ID)
	if err != nil || saved.State != "queued" || saved.Generation != 2 || saved.Completed != 3 {
		t.Fatalf("restart lost progress %#v %v", saved, err)
	}
	saved, _ = repo.FindAnalysis(ctx, paused.Owner, paused.ID)
	if saved.State != "paused" || saved.Generation != 1 {
		t.Fatal("cancelled resumed during restart")
	}
	saved, _ = repo.FindAnalysis(ctx, old.Owner, old.ID)
	if saved.State != "paused" || saved.Generation != 2 || saved.ErrorCode != "ENGINE_CHANGED" {
		t.Fatalf("old engine recovery %#v", saved)
	}
	if err = repo.RecoverAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	saved, _ = repo.FindAnalysis(ctx, old.Owner, old.ID)
	if saved.Generation != 2 {
		t.Fatal("repeat recovery changed generation")
	}
	saved, _ = repo.FindAnalysis(ctx, terminal.Owner, terminal.ID)
	if saved.State != "completed" || saved.Generation != 1 {
		t.Fatal("completed old engine changed")
	}
	assertAnalysisConflict(t, repo.CompletePosition(ctx, j, 1, analysis.Position{Ply: 3}, key))
}
func TestAnalysisCancelRacesPositionCommit(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	for i := 0; i < 20; i++ {
		j := analysisJob("race-owner")
		j.State = "running"
		var err error
		j, err = repo.CreateAnalysis(ctx, j)
		if err != nil {
			t.Fatal(err)
		}
		cancel := j
		cancel.State = "paused"
		cancel.Generation = 2
		complete := j
		complete.Completed = 1
		complete.NextPly = 1
		complete.State = "queued"
		key := "race-" + j.ID
		var cancelErr, completeErr error
		var wg sync.WaitGroup
		start := make(chan struct{})
		wg.Add(2)
		go func() { defer wg.Done(); <-start; cancelErr = repo.SaveAnalysis(ctx, cancel, 1, "running") }()
		go func() {
			defer wg.Done()
			<-start
			completeErr = repo.CompletePosition(ctx, complete, 1, analysis.Position{Ply: 0}, key)
		}()
		close(start)
		wg.Wait()
		if (cancelErr == nil) == (completeErr == nil) {
			t.Fatalf("CAS race winners cancel=%v complete=%v", cancelErr, completeErr)
		}
		page, err := repo.AnalysisPositions(ctx, j.Owner, j.ID, 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		_, found, err := repo.CachedAnalysisPosition(ctx, key, 0)
		if err != nil {
			t.Fatal(err)
		}
		if cancelErr == nil {
			assertAnalysisConflict(t, completeErr)
			if len(page.Items) != 0 || found {
				t.Fatal("late cancelled result/cache committed")
			}
		} else {
			assertAnalysisConflict(t, cancelErr)
			if len(page.Items) != 1 || !found {
				t.Fatal("winning completion not atomic")
			}
		}
	}
}
func TestAnalysisPracticeHistoryAndFrozenRules(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	practice := &game.Practice{SourceMode: "pvp", SourceID: game.ID(), SourceVersion: 9, Ply: 1, SourceGroupID: "pvp:original-group"}
	g := game.Record{Owner: "practice-owner", Keys: []string{"start", chess.PositionKey(game.Initial)}, Checks: []bool{false}, Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Fen: game.Initial, Phase: "finished", Practice: practice, Moves: []string{"b0c2"}, Updated: 10}}
	if err := repo.Save(ctx, g, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	jobs, err := repo.ListAnalyses(ctx, g.Owner, "pve", g.ID, 1)
	if err != nil || len(jobs) != 1 || !reflect.DeepEqual(jobs[0].Source.Keys, g.Keys) || !reflect.DeepEqual(jobs[0].Source.Checks, g.Checks) {
		t.Fatalf("frozen rules %#v %v", jobs, err)
	}
	if jobs[0].Source.Origin != "practice" || jobs[0].Source.GroupID != practice.SourceGroupID {
		t.Fatalf("practice grouping %#v", jobs[0].Source)
	}
	query, _ := history.ParseQuery("pve", "", 20)
	page, err := repo.History(ctx, g.Owner, query)
	if err != nil || len(page.Items) != 1 || page.Items[0].Practice == nil {
		t.Fatalf("practice summary %#v %v", page, err)
	}
	got := page.Items[0].Practice
	if got.SourceID != practice.SourceID || got.SourceMode != practice.SourceMode || got.SourceVersion != practice.SourceVersion || got.Ply != practice.Ply || got.SourceGroupID != practice.SourceGroupID {
		t.Fatalf("practice provenance %#v", got)
	}
}

func TestAnalysisReadyBoundedOwnerPriority(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	for i := 0; i < 12; i++ {
		for n := 0; n < 2; n++ {
			j := analysisJob(string(rune('a' + i)))
			j.Automatic = n == 0
			j.Updated = int64(i + 1)
			if n == 1 {
				j.Scope = "position"
				j.Updated += 100
			}
			if _, err := repo.CreateAnalysis(ctx, j); err != nil {
				t.Fatal(err)
			}
		}
	}
	ready, err := repo.ReadyAnalyses(ctx, 100)
	if err != nil || len(ready) != 8 {
		t.Fatalf("bounded ready %v %v", ready, err)
	}
	seen := map[string]bool{}
	for i, j := range ready {
		if j.Scope != "position" || seen[j.Owner] || j.Owner != string(rune('a'+i)) {
			t.Fatalf("position priority/owner order %#v", ready)
		}
		seen[j.Owner] = true
	}
	var queued int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM analysis_jobs WHERE state='queued'`).Scan(&queued); err != nil || queued != 24 {
		t.Fatalf("bounded read lost database pending work: count=%d err=%v", queued, err)
	}
}

func TestAnalysisWholeGameFairnessIgnoresAutomaticOrigin(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	manual := analysisJob("manual-game-owner")
	manual.Updated = 1
	automatic := analysisJob("automatic-game-owner")
	automatic.Automatic = true
	automatic.Updated = 2
	var err error
	manual, err = repo.CreateAnalysis(ctx, manual)
	if err != nil {
		t.Fatal(err)
	}
	automatic, err = repo.CreateAnalysis(ctx, automatic)
	if err != nil {
		t.Fatal(err)
	}
	ready, err := repo.ReadyAnalyses(ctx, 8)
	if err != nil || len(ready) != 2 || ready[0].ID != manual.ID {
		t.Fatalf("oldest whole-game task first %#v %v", ready, err)
	}
	// Completing one position returns A to the queue with its new checkpoint time;
	// B's older checkpoint must win even though B was automatically created.
	manual.State = "running"
	if err = repo.SaveAnalysis(ctx, manual, 1, "queued"); err != nil {
		t.Fatal(err)
	}
	manual.State = "queued"
	manual.Updated = 3
	manual.Completed = 1
	manual.NextPly = 1
	if err = repo.CompletePosition(ctx, manual, 1, analysis.Position{Ply: 0}, ""); err != nil {
		t.Fatal(err)
	}
	ready, err = repo.ReadyAnalyses(ctx, 8)
	if err != nil || len(ready) != 2 || ready[0].ID != automatic.ID || ready[1].ID != manual.ID {
		t.Fatalf("manual whole-game task starved automatic game %#v %v", ready, err)
	}
	automatic.State = "running"
	if err = repo.SaveAnalysis(ctx, automatic, 1, "queued"); err != nil {
		t.Fatal(err)
	}
	automatic.State = "queued"
	automatic.Updated = 4
	automatic.Completed = 1
	automatic.NextPly = 1
	if err = repo.CompletePosition(ctx, automatic, 1, analysis.Position{Ply: 0}, ""); err != nil {
		t.Fatal(err)
	}
	ready, err = repo.ReadyAnalyses(ctx, 8)
	if err != nil || len(ready) != 2 || ready[0].ID != manual.ID {
		t.Fatalf("whole-game checkpoints did not rotate %#v %v", ready, err)
	}
}

func TestAnalysisMaterializationBatchBound(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	for n := 0; n < 35; n++ {
		r := game.Record{Owner: "batch-owner", Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Phase: "finished", Moves: []string{"b0c2"}}}
		if err := repo.Save(ctx, r, 0); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []int{32, 35, 35} {
		if err := repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
			t.Fatal(err)
		}
		var jobs, events int
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM analysis_jobs`).Scan(&jobs); err != nil {
			t.Fatal(err)
		}
		if err := conn.QueryRow(ctx, `SELECT count(*) FROM analysis_finished_events WHERE processed_at=0`).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if jobs != want || events != 35-want {
			t.Fatalf("batch materialization jobs=%d events=%d want jobs=%d", jobs, events, want)
		}
	}
}

func TestAnalysisTrainingMetadataRetainsFinalRecordAndSample(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	g := game.Record{Owner: "training-owner", Snapshot: game.Snapshot{ID: game.ID(), Version: 1, Phase: "finished", Fen: game.Initial, Moves: []string{"b0c2", "b9c7"}, Outcome: "human", Reason: "resigned", Rules: "legacy-java-v1"}}
	if err := repo.Save(ctx, g, 0); err != nil {
		t.Fatal(err)
	}
	if err := repo.MaterializeAnalyses(ctx, analysis.EngineVersion); err != nil {
		t.Fatal(err)
	}
	// Consuming the queue leaves the exact completed record and outcome available.
	var processed int64
	var sourceBody, recordBody []byte
	if err := conn.QueryRow(ctx, `SELECT processed_at,source,frozen_record FROM analysis_finished_events WHERE mode='pve' AND source_id=$1 AND source_version=1`, g.ID).Scan(&processed, &sourceBody, &recordBody); err != nil {
		t.Fatal(err)
	}
	var source analysis.Source
	var frozen game.Record
	if err := json.Unmarshal(sourceBody, &source); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(recordBody, &frozen); err != nil {
		t.Fatal(err)
	}
	if processed <= 0 || source.Fen != g.Fen || source.Outcome != g.Outcome || source.Reason != g.Reason || source.Origin != "game" || source.GroupID != "pve:"+g.ID || !reflect.DeepEqual(frozen, g) {
		t.Fatalf("retained final metadata source=%#v frozen=%#v", source, frozen)
	}
	// An undo may replace the mutable game record but must retain the final label
	// and original full move history in the acknowledged completion event.
	g.Version = 2
	g.Phase = "playing"
	g.Outcome = "ongoing"
	g.Reason = ""
	g.Moves = g.Moves[:1]
	if err := repo.Save(ctx, g, 1); err != nil {
		t.Fatal(err)
	}
	var retained []byte
	if err := conn.QueryRow(ctx, `SELECT frozen_record FROM analysis_finished_events WHERE source_id=$1`, g.ID).Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retained, recordBody) {
		t.Fatal("source undo rewrote completion evidence")
	}
	jobs, err := repo.ListAnalyses(ctx, g.Owner, "pve", g.ID, 1)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("training job %v %v", jobs, err)
	}
	j := jobs[0]
	if j.Budget != analysis.ProfileBudget("standard") {
		t.Fatalf("automatic search budget %#v", j.Budget)
	}
	j.State = "running"
	if err = repo.SaveAnalysis(ctx, j, 1, "queued"); err != nil {
		t.Fatal(err)
	}
	actual := analysis.Line{Score: -412, NativeScore: 412, Depth: 6, Reliable: true, PV: []string{"b0c2"}, Kind: "evaluation", StopReason: "depth", CompletedNanos: 1000}
	teacher := analysis.Line{Score: 725, NativeScore: -725, Depth: 6, Reliable: true, PV: []string{"h2e2"}, Kind: "evaluation", StopReason: "depth", CompletedNanos: 2000}
	pos := analysis.Position{Ply: 1, SampleID: "shared-sample-1", PositionID: "history-position-1", GroupID: j.Source.GroupID, SourceMode: "pve", SourceID: g.ID, SourceVersion: 1, Origin: "game", Fen: game.Initial, Turn: "black", History: []string{"b0c2"}, Rules: j.Source.Rules, Teacher: j.Engine, Budget: j.Budget, ScorePerspective: "red", ScoreUnits: "legacy_native", Outcome: "human", EndReason: "resigned", Final: true, Candidates: []analysis.Candidate{{Move: "h2e2", Role: "teacher_recommended", Line: teacher}}, Best: teacher, Actual: &actual, ActualMove: "b9c7", Comparable: true, Loss: 1137}
	j.Completed = 1
	j.NextPly = 2
	j.State = "queued"
	if err = repo.CompletePosition(ctx, j, 1, pos, analysis.CacheKey(j)); err != nil {
		t.Fatal(err)
	}
	page, err := repo.AnalysisPositions(ctx, j.Owner, j.ID, 0, 100)
	if err != nil || len(page.Items) != 1 || !reflect.DeepEqual(page.Items[0], pos) {
		t.Fatalf("sample metadata roundtrip %#v %v", page, err)
	}
	cached, found, err := repo.CachedAnalysisPosition(ctx, analysis.CacheKey(j), 1)
	if err != nil || !found || !reflect.DeepEqual(cached, pos) {
		t.Fatalf("training cache roundtrip %#v %v", cached, err)
	}
	// Different viewer jobs may retain the same sample ID; it is indexed for later
	// deduplication and is deliberately not unique across authorized jobs.
	other := j
	other.ID = game.ID()
	other.Owner = "other-viewer"
	other.State = "running"
	other.Completed = 0
	other.NextPly = 1
	other, err = repo.CreateAnalysis(ctx, other)
	if err != nil {
		t.Fatal(err)
	}
	other.Completed = 1
	other.NextPly = 2
	other.State = "queued"
	if err = repo.CompletePosition(ctx, other, 1, pos, analysis.CacheKey(other)); err != nil {
		t.Fatal(err)
	}
	var samples int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM analysis_positions WHERE sample_id=$1`, pos.SampleID).Scan(&samples); err != nil || samples != 2 {
		t.Fatalf("duplicate viewer sample count=%d err=%v", samples, err)
	}
	var unique bool
	if err = conn.QueryRow(ctx, `SELECT indisunique FROM pg_index WHERE indexrelid='analysis_positions_sample'::regclass`).Scan(&unique); err != nil || unique {
		t.Fatalf("sample index must not be unique: %v %v", unique, err)
	}
}
