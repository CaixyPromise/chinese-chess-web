package pvp

import (
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"
)

type linkedRepository struct {
	*repository
	linksMu sync.RWMutex
	links   map[string]string
}

func (r *linkedRepository) SameOwner(a, b string) bool {
	r.linksMu.RLock()
	defer r.linksMu.RUnlock()
	if v := r.links[a]; v != "" {
		a = v
	}
	if v := r.links[b]; v != "" {
		b = v
	}
	return a != "" && b != "" && a == b
}
func (r *linkedRepository) link(source, account string) {
	r.linksMu.Lock()
	r.links[source] = account
	r.linksMu.Unlock()
}
func linkedManager(t *testing.T) (*Manager, *linkedRepository) {
	t.Helper()
	repo := &linkedRepository{repository: &repository{rows: map[string][]byte{}}, links: map[string]string{}}
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	pool := engine.NewPool(binary, 1, 8)
	m, e := New(repo, pool)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close(); pool.Close() })
	return m, repo
}
func TestApprovalRechecksCurrentIdentityAndQuota(t *testing.T) {
	m, repo := linkedManager(t)
	host, guest, account := game.ID(), game.ID(), game.ID()
	v, _ := m.Create(context.Background(), host, "red")
	a, e := m.ApplyJoin(guest, v.Invite, "来宾", game.ID())
	if e != nil {
		t.Fatal(e)
	}
	// Claiming the applicant after the application must not allow both seats.
	repo.link(host, account)
	repo.link(guest, account)
	current, e := m.Get(account, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, e = m.Command(context.Background(), account, v.ID, Command{RequestID: game.ID(), Version: current.Version, Type: "approve_join", ApplicationID: a.ApplicationID})
	code(t, e, "SAME_ACCOUNT_SEATS")
	current, _ = m.Get(account, v.ID)
	if current.Black.Occupied || current.PendingApplication == nil {
		t.Fatal("failed approval mutated room", current)
	}
	// Host privileges and source identity survive an account transition.
	current = command(t, m, account, v.ID, "leave", "")
	if current.Reason != "cancelled" {
		t.Fatal(current)
	}
	for i := 0; i < 8; i++ {
		source := guest
		if i%2 == 0 {
			source = host
		}
		if _, e = m.Create(context.Background(), source, "red"); e != nil {
			t.Fatal(e)
		}
	}
	_, e = m.Create(context.Background(), account, "red")
	code(t, e, "ACTIVE_GAME_LIMIT")
	other := game.ID()
	waiting, e := m.Create(context.Background(), other, "red")
	if e != nil {
		t.Fatal(e)
	}
	a, e = m.ApplyJoin(account, waiting.Invite, "来宾", game.ID())
	if e != nil {
		t.Fatal(e)
	}
	_, e = m.Command(context.Background(), other, waiting.ID, Command{RequestID: game.ID(), Version: a.Version, Type: "approve_join", ApplicationID: a.ApplicationID})
	code(t, e, "ACTIVE_GAME_LIMIT")
}
func TestLinkedCommandsRetainSeatAndIdempotentIdentity(t *testing.T) {
	m, repo := linkedManager(t)
	host, guest, account := game.ID(), game.ID(), game.ID()
	v, _ := m.Create(context.Background(), host, "red")
	if _, e := joinApproved(m, guest, v.Invite); e != nil {
		t.Fatal(e)
	}
	v, _ = m.Get(host, v.ID)
	c := Command{RequestID: game.ID(), Version: v.Version, Type: "ready"}
	first, e := m.Command(context.Background(), host, v.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	repo.link(host, account)
	repeated, e := m.Command(context.Background(), account, v.ID, c)
	if e != nil || repeated.Version != first.Version || repeated.You != "red" {
		t.Fatal("claim changed receipt identity", repeated, e)
	}
	repo.mu.Lock()
	var saved Record
	_ = json.Unmarshal(repo.rows[v.ID], &saved)
	repo.mu.Unlock()
	if saved.Host != host || saved.RedOwner != host || saved.BlackOwner != guest {
		t.Fatal("source rewritten")
	}
	_, updates, stop, e := m.Watch(account, v.ID)
	if e != nil {
		t.Fatal(e)
	}
	defer stop()
	command(t, m, account, v.ID, "unready", "")
	if update := <-updates; update.You != "red" || update.Red.Ready {
		t.Fatal("linked watcher", update)
	}
}
