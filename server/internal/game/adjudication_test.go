package game

import (
	"chinesechess/internal/chess"
	"context"
	"encoding/json"
	"testing"
)

func TestVersionedPVERepetitionUndoAndRestart(t *testing.T) {
	for _, version := range []string{chess.CasualRules, chess.LegacyRules, ""} {
		t.Run("version="+version, func(t *testing.T) {
			repo := &memoryRepo{rows: map[string][]byte{}}
			m := testManager(t, repo, 0)
			ctx, owner := context.Background(), ID()
			moves := []string{"b0c2", "b9c7", "c2b0", "c7b9", "b0c2", "b9c7", "c2b0"}
			p, err := chess.Replay(ctx, m.rules, moves, "")
			if err != nil {
				t.Fatal(err)
			}
			r := Record{Snapshot: Snapshot{ID: ID(), Version: 1, Rules: version, Fen: p.Fen, Moves: moves, Legal: p.Legal,
				Human: "black", Difficulty: "easy", Phase: "playing", Outcome: "ongoing"}, Owner: owner, Generation: 1, Receipts: map[string]Receipt{}}
			if err = repo.Save(ctx, r, 0); err != nil {
				t.Fatal(err)
			}
			recovered := testManager(t, repo, 0)
			before, err := recovered.Get(owner, r.ID)
			if err != nil {
				t.Fatal(err)
			}
			wantVersion := version
			if wantVersion == "" {
				wantVersion = chess.LegacyRules
			}
			if before.Rules != wantVersion {
				t.Fatal("version changed", before.Rules)
			}
			v, err := recovered.Command(ctx, owner, r.ID, Command{ID(), before.Version, "move", "c7b9"})
			if err != nil {
				t.Fatal(err)
			}
			if version == chess.CasualRules {
				if v.Phase != "finished" || v.Outcome != "draw" || v.Reason != "repetition" || len(v.Legal) != 0 {
					t.Fatal("casual result", v)
				}
			} else if v.Phase == "finished" || v.Rules != chess.LegacyRules {
				t.Fatal("legacy reinterpreted", v)
			}
			v, err = recovered.Command(ctx, owner, r.ID, Command{ID(), v.Version, "undo", ""})
			if err != nil || len(v.Moves) != 7 || v.Reason != "" || v.Outcome != "ongoing" {
				t.Fatal("undo", v, err)
			}
			repo.mu.Lock()
			var saved Record
			err = json.Unmarshal(repo.rows[r.ID], &saved)
			repo.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			if version == chess.CasualRules && (len(saved.Keys) != 8 || len(saved.Checks) != 7) {
				t.Fatal("undo metadata", saved.Keys, saved.Checks)
			}
			if saved.Rules != wantVersion {
				t.Fatal("persisted version")
			}
		})
	}
}

func TestNewPVEPreservesBotRuleHistory(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	m := testManager(t, repo, 1)
	owner := ID()
	v, err := m.Create(context.Background(), owner, "black", "easy")
	if err != nil {
		t.Fatal(err)
	}
	v = waitView(t, m, owner, v.ID, func(s Snapshot) bool { return len(s.Moves) == 1 && s.Phase == "playing" })
	if v.Rules != chess.CasualRules {
		t.Fatal("new PVE rule", v.Rules)
	}
	repo.mu.Lock()
	var saved Record
	err = json.Unmarshal(repo.rows[v.ID], &saved)
	repo.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Keys) != 2 || len(saved.Checks) != 1 || saved.Keys[1] != chess.PositionKey(v.Fen) || saved.Checks[0] != v.InCheck {
		t.Fatal("bot metadata", saved)
	}
}

func TestUnknownRuleVersionDoesNotResumeBot(t *testing.T) {
	owner := ID()
	r := Record{Snapshot: Snapshot{ID: ID(), Version: 1, Rules: "future-rules-v2", Fen: Initial,
		Moves: []string{}, Phase: "queued", Outcome: "ongoing", Human: "black", Difficulty: "easy"}, Owner: owner, Generation: 1, Receipts: map[string]Receipt{}}
	b, _ := json.Marshal(r)
	m := testManager(t, &memoryRepo{rows: map[string][]byte{r.ID: b}}, 1)
	g := m.rooms[r.ID]
	// Exercise the worker entry directly in addition to checking load scheduling.
	m.compute(job{room: g, generation: 1, ctx: context.Background()})
	v, err := m.Get(owner, r.ID)
	if err != nil || v.Rules != r.Rules || v.Version != 1 || len(v.Moves) != 0 || v.Outcome != r.Outcome {
		t.Fatal("unknown rules resumed", v, err)
	}
	_, err = m.Command(context.Background(), owner, r.ID, Command{ID(), v.Version, "resign", ""})
	expectCode(t, err, "UNSUPPORTED_RULES")
}
