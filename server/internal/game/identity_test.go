package game

import (
	"chinesechess/internal/engine"
	"context"
	"path/filepath"
	"testing"
)

type linkedRepo struct {
	*memoryRepo
	first, second, account string
}

func (r *linkedRepo) SameOwner(a, b string) bool {
	if a == r.first || a == r.second {
		a = r.account
	}
	if b == r.first || b == r.second {
		b = r.account
	}
	return a != "" && b != "" && a == b
}
func TestLinkedSourcesAuthorizeAndShareAdmission(t *testing.T) {
	repo := &linkedRepo{memoryRepo: &memoryRepo{rows: map[string][]byte{}}, first: ID(), second: ID(), account: ID()}
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	rules, bots := engine.NewPool(binary, 1, 8), engine.NewPool(binary, 1, 0)
	defer rules.Close()
	defer bots.Close()
	m, e := New(repo, rules, bots, 0)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	var first Snapshot
	for i := 0; i < 8; i++ {
		owner := repo.first
		if i%2 == 1 {
			owner = repo.second
		}
		v, e := m.Create(context.Background(), owner, "red", "easy")
		if e != nil {
			t.Fatal(e)
		}
		if i == 0 {
			first = v
		}
	}
	if _, e = m.Get(repo.account, first.ID); e != nil {
		t.Fatal("linked access", e)
	}
	_, e = m.Create(context.Background(), repo.account, "red", "easy")
	expectCode(t, e, "ACTIVE_GAME_LIMIT")
	v, e := m.Command(context.Background(), repo.account, first.ID, Command{RequestID: ID(), Version: first.Version, Type: "move", Move: "b0c2"})
	if e != nil || len(v.Moves) != 1 {
		t.Fatal(v, e)
	}
	saved, e := repo.FindGame(context.Background(), repo.first, first.ID)
	if e != nil || saved.Owner != repo.first {
		t.Fatal("source owner rewritten", e)
	}
}
