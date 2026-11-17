package integration

import (
	"errors"
	"reflect"
	"testing"

	"chinesechess/internal/analysis"
	"chinesechess/internal/game"
)

func TestReplaySnapshotOwnerScopedIdempotenceAndImmutableBody(t *testing.T) {
	ctx, repo, conn := analysisDatabase(t)
	source := analysis.Source{Mode: "pve", ID: game.ID(), Version: 3, Moves: []string{"b0c2"}, Rules: "legacy-java-v1", Phase: "thinking", Fen: game.Initial, Outcome: "ongoing", Reason: "", Origin: "practice", GroupID: "pvp:original", Keys: []string{"initial", "one-move"}, Checks: []bool{false}}
	id, err := repo.SaveReplaySnapshot(ctx, "owner-a", source)
	if err != nil || !game.ValidID(id) {
		t.Fatalf("save snapshot id=%q err=%v", id, err)
	}
	repeated, err := repo.SaveReplaySnapshot(ctx, "owner-a", source)
	if err != nil || repeated != id {
		t.Fatalf("repeat created a new frame %q %v", repeated, err)
	}
	// Rebuilt helper rule metadata is outside the source digest. A repeated read
	// must still preserve the exact original body instead of replacing it.
	refreshed := source
	refreshed.Keys = []string{"rebuilt-initial", "rebuilt-one"}
	refreshed.Checks = []bool{true}
	repeated, err = repo.SaveReplaySnapshot(ctx, "owner-a", refreshed)
	if err != nil || repeated != id {
		t.Fatalf("helper metadata changed snapshot identity %q %v", repeated, err)
	}
	found, err := repo.FindReplaySnapshot(ctx, "owner-a", id)
	if err != nil || !reflect.DeepEqual(found, source) {
		t.Fatalf("original source changed %#v %v", found, err)
	}
	for _, owner := range []string{"owner-b", "", "unknown"} {
		if _, err = repo.FindReplaySnapshot(ctx, owner, id); !errors.Is(err, game.ErrNotFound) {
			t.Fatalf("foreign snapshot read owner=%q err=%v", owner, err)
		}
	}
	other, err := repo.SaveReplaySnapshot(ctx, "owner-b", source)
	if err != nil || !game.ValidID(other) || other == id {
		t.Fatalf("snapshot identity crossed owner boundary %q %v", other, err)
	}
	if _, err = repo.FindReplaySnapshot(ctx, "owner-a", other); !errors.Is(err, game.ErrNotFound) {
		t.Fatalf("other owner snapshot readable %v", err)
	}
	var n int
	if err = conn.QueryRow(ctx, `SELECT count(*) FROM analysis_replay_snapshots`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("snapshot dedup count=%d err=%v", n, err)
	}
	// A changed prefix or source version must get a separate immutable identity.
	changed := source
	changed.Version++
	changed.Moves = append(append([]string{}, source.Moves...), "b9c7")
	newer, err := repo.SaveReplaySnapshot(ctx, "owner-a", changed)
	if err != nil || newer == id {
		t.Fatalf("different source reused old frame %q %v", newer, err)
	}
}

func TestReplaySnapshotSurvivesLiveAppendUndoAndPreservesFinalMetadata(t *testing.T) {
	ctx, repo, _ := analysisDatabase(t)
	live := game.Record{Owner: "snapshot-owner", Snapshot: game.Snapshot{ID: game.ID(), Version: 3, Phase: "thinking", Rules: "legacy-java-v1", Fen: game.Initial, Outcome: "ongoing", Moves: []string{"b0c2"}}}
	if err := repo.Save(ctx, live, 0); err != nil {
		t.Fatal(err)
	}
	frozen := analysis.Source{Mode: "pve", ID: live.ID, Version: live.Version, Phase: live.Phase, Rules: live.Rules, Fen: live.Fen, Outcome: live.Outcome, Moves: append([]string{}, live.Moves...), Origin: "game", GroupID: "pve:" + live.ID}
	id, err := repo.SaveReplaySnapshot(ctx, live.Owner, frozen)
	if err != nil {
		t.Fatal(err)
	}
	// The bot appends after the frame was read, before the user requests analysis.
	live.Version = 4
	live.Phase = "playing"
	live.Moves = append(live.Moves, "b9c7")
	if err = repo.Save(ctx, live, 3); err != nil {
		t.Fatal(err)
	}
	// Undo may shorten the mutable game below the frozen prefix as well.
	live.Version = 5
	live.Moves = []string{}
	if err = repo.Save(ctx, live, 4); err != nil {
		t.Fatal(err)
	}
	found, err := repo.FindReplaySnapshot(ctx, live.Owner, id)
	if err != nil || !reflect.DeepEqual(found, frozen) {
		t.Fatalf("live append/undo changed frozen frame %#v %v", found, err)
	}
	current, err := repo.FindGame(ctx, live.Owner, live.ID)
	if err != nil || current.Version != 5 || len(current.Moves) != 0 {
		t.Fatalf("live game unexpectedly changed %#v %v", current, err)
	}
	final := frozen
	final.Version = 6
	final.Phase = "finished"
	final.Outcome = "human"
	final.Reason = "resigned"
	finalID, err := repo.SaveReplaySnapshot(ctx, live.Owner, final)
	if err != nil {
		t.Fatal(err)
	}
	finalFound, err := repo.FindReplaySnapshot(ctx, live.Owner, finalID)
	if err != nil || !reflect.DeepEqual(finalFound, final) {
		t.Fatalf("final source metadata lost %#v %v", finalFound, err)
	}
}
