package pvp

import (
	"chinesechess/internal/game"
	"context"
	"encoding/json"
	"testing"
)

func TestRecoveredPVPUndoRebuildsMissingMetadata(t *testing.T) {
	for _, keepKeys := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing both", true: "missing checks"}[keepKeys], func(t *testing.T) {
			m, clock, repo := setup(t)
			v, red, black := started(t, m)
			for _, move := range []string{"b0c2", "b9c7", "h0g2", "h9g7"} {
				actor := red
				if len(v.Moves)%2 == 1 {
					actor = black
				}
				v = command(t, m, actor, v.ID, "move", move)
			}
			m.Close()
			repo.mu.Lock()
			var old Record
			if err := json.Unmarshal(repo.rows[v.ID], &old); err != nil {
				t.Fatal(err)
			}
			old.Checks = nil
			if !keepKeys {
				old.Keys = nil
			}
			old.Rules = ""
			repo.rows[v.ID], _ = json.Marshal(old)
			repo.mu.Unlock()
			recovered, err := New(repo, m.rules, clock.Load)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(recovered.Close)
			command(t, recovered, red, v.ID, "undo", "")
			v = command(t, recovered, black, v.ID, "accept", "")
			if len(v.Moves) != 2 || v.Rules != RulesVersion {
				t.Fatal("undo", v)
			}
			repo.mu.Lock()
			var saved Record
			err = json.Unmarshal(repo.rows[v.ID], &saved)
			repo.mu.Unlock()
			if err != nil || len(saved.Keys) != 3 || len(saved.Checks) != 2 {
				t.Fatal("undo metadata", saved.Keys, saved.Checks, err)
			}
		})
	}
}

func TestOldPVPReceiptInheritsOnlyMissingRuleVersion(t *testing.T) {
	m, clock, repo := setup(t)
	v, red, _ := started(t, m)
	request := Command{game.ID(), v.Version, "draw", "", ""}
	first, err := m.Command(context.Background(), red, v.ID, request)
	if err != nil {
		t.Fatal(err)
	}
	m.Close()
	repo.mu.Lock()
	var saved Record
	err = json.Unmarshal(repo.rows[v.ID], &saved)
	if err != nil {
		t.Fatal(err)
	}
	saved.Rules = ""
	key := red + ":" + request.RequestID
	receipt := saved.Receipts[key]
	receipt.State.Rules = ""
	saved.Receipts[key] = receipt
	repo.rows[v.ID], _ = json.Marshal(saved)
	repo.mu.Unlock()
	recovered, err := New(repo, m.rules, clock.Load)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(recovered.Close)
	got, err := recovered.Command(context.Background(), red, v.ID, request)
	if err != nil || got.Rules != RulesVersion || got.Version != first.Version || got.Outcome != first.Outcome {
		t.Fatal("old receipt", got, err)
	}
	g := recovered.rooms[v.ID]
	g.mu.Lock()
	receipt = g.record.Receipts[key]
	receipt.State.Rules = "legacy-java-v1"
	g.record.Receipts[key] = receipt
	g.mu.Unlock()
	got, err = recovered.Command(context.Background(), red, v.ID, request)
	if err != nil || got.Rules != "legacy-java-v1" {
		t.Fatal("explicit receipt version overwritten", got, err)
	}
}
