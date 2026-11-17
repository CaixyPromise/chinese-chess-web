package game

import (
	"chinesechess/internal/chess"
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPracticePrefixRulesAndUndoBoundary(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	m := testManager(t, repo, 1)
	ctx, owner := context.Background(), ID()
	source, err := m.Create(ctx, owner, "red", "easy")
	if err != nil {
		t.Fatal(err)
	}
	prefix := []string{"b0c2", "b9c7"}
	v, err := m.CreatePractice(ctx, owner, "red", "easy", "pve", source.ID, source.Version, prefix, chess.CasualRules)
	if err != nil {
		t.Fatal(err)
	}
	if v.ID == source.ID || v.Phase != "playing" || !reflect.DeepEqual(v.Moves, prefix) || v.Practice == nil || v.Practice.Ply != 2 || v.Practice.SourceID != source.ID || v.Practice.SourceVersion != source.Version {
		t.Fatal("incorrect practice snapshot", v)
	}
	_, err = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "undo", ""})
	expectCode(t, err, "NO_HISTORY")
	v, err = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "move", v.Legal[0]})
	if err != nil {
		t.Fatal(err)
	}
	v = waitView(t, m, owner, v.ID, func(s Snapshot) bool { return s.Phase == "playing" && len(s.Moves) == 4 })
	v, err = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "undo", ""})
	if err != nil || !reflect.DeepEqual(v.Moves, prefix) {
		t.Fatal("undo escaped prefix", v, err)
	}
	again, err := m.Get(owner, source.ID)
	if err != nil || !reflect.DeepEqual(source, again) {
		t.Fatal("source changed", source, again, err)
	}
	if _, err = m.Get(ID(), v.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("owner isolation", err)
	}
	stored, err := repo.FindGame(ctx, owner, v.ID)
	if err != nil || len(stored.Keys) != 3 || len(stored.Checks) != 2 {
		t.Fatal("rule history", stored, err)
	}
}

func TestPracticeBotFirstAndRestart(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	ctx, owner, source := context.Background(), ID(), ID()
	paused := testManager(t, repo, 0)
	v, err := paused.CreatePractice(ctx, owner, "black", "easy", "pvp", source, 3, []string{"b0c2", "b9c7"}, chess.LegacyRules)
	if err != nil || v.Phase != "queued" {
		t.Fatal(v, err)
	}
	paused.Close()
	m := testManager(t, repo, 1)
	v = waitView(t, m, owner, v.ID, func(s Snapshot) bool { return s.Phase == "playing" && len(s.Moves) == 3 })
	if v.Practice == nil || v.Practice.SourceMode != "pvp" || v.Practice.SourceVersion != 3 || v.Rules != chess.LegacyRules {
		t.Fatal("restart lost metadata", v)
	}
	m.Close()
	noBots := testManager(t, repo, 0)
	v, err = noBots.Command(ctx, owner, v.ID, Command{ID(), v.Version, "undo", ""})
	if err != nil || len(v.Moves) != 2 || v.Phase != "queued" {
		t.Fatal("bot-first undo boundary", v, err)
	}
	_, err = noBots.Command(ctx, owner, v.ID, Command{ID(), v.Version, "undo", ""})
	expectCode(t, err, "NO_HISTORY")
}

func TestPracticeTerminalRulesValidationAndAdmission(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	m := testManager(t, repo, 0)
	ctx, owner, source := context.Background(), ID(), ID()
	repetition := []string{"b0c2", "b9c7", "c2b0", "c7b9", "b0c2", "b9c7", "c2b0", "c7b9"}
	_, err := m.CreatePractice(ctx, owner, "red", "easy", "pve", source, 1, repetition, chess.CasualRules)
	expectCode(t, err, "PRACTICE_TERMINAL")
	v, err := m.CreatePractice(ctx, owner, "red", "easy", "pve", source, 1, repetition, "")
	if err != nil || v.Rules != chess.LegacyRules || v.Phase != "playing" {
		t.Fatal("legacy repetition", v, err)
	}
	_, err = m.CreatePractice(ctx, owner, "red", "easy", "pve", source, 1, []string{"a0a9"}, chess.LegacyRules)
	expectCode(t, err, "INVALID_REQUEST")
	_, err = m.CreatePractice(ctx, owner, "red", "easy", "pve", source, 1, nil, "unknown")
	expectCode(t, err, "UNSUPPORTED_RULES")
	for i := 0; i < 7; i++ {
		if _, err = m.CreatePractice(ctx, owner, "red", "easy", "pve", source, 1, nil, chess.CasualRules); err != nil {
			t.Fatal(err)
		}
	}
	_, err = m.CreatePractice(ctx, owner, "red", "easy", "pve", source, 1, nil, chess.CasualRules)
	expectCode(t, err, "ACTIVE_GAME_LIMIT")
}

func TestPracticeGlobalAdmission(t *testing.T) {
	m := testManager(t, &memoryRepo{rows: map[string][]byte{}}, 0)
	for i := 0; i < 256; i++ {
		id := ID()
		m.rooms[id] = &room{record: Record{Snapshot: Snapshot{ID: id, Phase: "playing"}, Owner: ID()}}
	}
	_, err := m.CreatePractice(context.Background(), ID(), "red", "easy", "pve", ID(), 1, nil, chess.CasualRules)
	expectCode(t, err, "GAME_LIMIT")
}
