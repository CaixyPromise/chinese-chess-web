package game

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func archivedRecord(t *testing.T, m *Manager, owner string) Record {
	t.Helper()
	v, err := m.position(context.Background(), []string{"b0c2"}, "")
	if err != nil {
		t.Fatal(err)
	}
	return Record{Snapshot: Snapshot{ID: ID(), Version: 1, Fen: v.Fen, Moves: []string{"b0c2"}, Legal: []string{}, Human: "red", Difficulty: "easy", Phase: "finished", Outcome: "black_win", Reason: "resign"}, Owner: owner, Generation: 1, Receipts: map[string]Receipt{}}
}

type blockedArchive struct {
	*memoryRepo
	entered, resume chan struct{}
	finds           atomic.Int32
}

func (r *blockedArchive) FindGame(ctx context.Context, owner, id string) (Record, error) {
	if r.finds.Add(1) == 1 {
		close(r.entered)
		select {
		case <-r.resume:
		case <-ctx.Done():
			return Record{}, ctx.Err()
		}
	}
	return r.memoryRepo.FindGame(ctx, owner, id)
}

func TestConcurrentArchiveWatchAndUndoShareCanonicalRoom(t *testing.T) {
	base := &memoryRepo{rows: map[string][]byte{}}
	seed := testManager(t, base, 0)
	owner := ID()
	archived := archivedRecord(t, seed, owner)
	if err := base.Save(context.Background(), archived, 0); err != nil {
		t.Fatal(err)
	}
	repo := &blockedArchive{memoryRepo: base, entered: make(chan struct{}), resume: make(chan struct{})}
	m, err := New(repo, seed.rules, seed.bots, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(m.Close)
	type subscription struct {
		ch   <-chan Snapshot
		stop func()
		err  error
	}
	subs := make(chan subscription, 2)
	watch := func() { _, ch, stop, e := m.Watch(owner, archived.ID); subs <- subscription{ch, stop, e} }
	go watch()
	select {
	case <-repo.entered:
	case <-time.After(time.Second):
		t.Fatal("archive fetch never started")
	}
	go watch()
	result := make(chan error, 1)
	go func() {
		_, e := m.Command(context.Background(), owner, archived.ID, Command{ID(), 1, "undo", ""})
		result <- e
	}()
	close(repo.resume)
	stops := []func(){}
	for i := 0; i < 2; i++ {
		select {
		case sub := <-subs:
			if sub.err != nil {
				t.Fatal(sub.err)
			}
			stops = append(stops, sub.stop)
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent archive watch deadlocked")
		}
	}
	select {
	case e := <-result:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("concurrent archive undo deadlocked")
	}
	if repo.finds.Load() != 1 || len(m.roomList()) != 1 {
		t.Fatal("duplicate archive writer", repo.finds.Load(), len(m.roomList()))
	}
	for _, stop := range stops {
		stop()
	}
	v, err := m.Get(owner, archived.ID)
	if err != nil || v.Phase != "playing" || v.Version != 2 {
		t.Fatal("lost undo", v, err)
	}
}

func TestHistoryDoesNotOccupyActiveRegistryAndArchivedWatchCanReopen(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	seed := testManager(t, repo, 0)
	owner := ID()
	var archived Record
	for i := 0; i < 300; i++ {
		archived = archivedRecord(t, seed, owner)
		if err := repo.Save(context.Background(), archived, 0); err != nil {
			t.Fatal(err)
		}
	}
	m := testManager(t, repo, 0)
	if len(m.roomList()) != 0 {
		t.Fatal("historical records loaded as rooms")
	}
	if _, err := m.Get(ID(), archived.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal("foreign history", err)
	}
	if _, err := m.Get(owner, archived.ID); err != nil {
		t.Fatal(err)
	}
	if len(m.roomList()) != 0 {
		t.Fatal("read pinned completed room")
	}
	v, ch, unwatch, err := m.Watch(owner, archived.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer unwatch()
	if len(m.roomList()) != 1 {
		t.Fatal("subscription lost canonical room")
	}
	v, err = m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "undo", ""})
	if err != nil || v.Phase != "playing" || len(v.Moves) != 0 {
		t.Fatal("reopen", v, err)
	}
	if received := <-ch; received.Version != v.Version || received.Phase != "playing" {
		t.Fatal("archived subscriber not updated", received)
	}
	unwatch()
	v, err = m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "resign", ""})
	if err != nil || len(m.roomList()) != 0 {
		t.Fatal("completed room retained", err)
	}
	if _, err = m.Get(owner, v.ID); err != nil || len(m.roomList()) != 0 {
		t.Fatal("archived re-read", err)
	}
}

func TestFinishedUndoMustReacquireActiveCapacity(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	m := testManager(t, repo, 0)
	owner := ID()
	archived := archivedRecord(t, m, owner)
	if err := repo.Save(context.Background(), archived, 0); err != nil {
		t.Fatal(err)
	}
	var active Snapshot
	for i := 0; i < 8; i++ {
		var err error
		active, err = m.Create(context.Background(), owner, "red", "easy")
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := m.Command(context.Background(), owner, archived.ID, Command{ID(), 1, "undo", ""})
	expectCode(t, err, "ACTIVE_GAME_LIMIT")
	if len(m.roomList()) != 8 {
		t.Fatal("rejected undo leaked archive")
	}
	_, err = m.Command(context.Background(), owner, active.ID, Command{ID(), active.Version, "resign", ""})
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.Command(context.Background(), owner, archived.ID, Command{ID(), 1, "undo", ""})
	if err != nil || v.Phase != "playing" || len(m.roomList()) != 8 {
		t.Fatal("capacity reopen", v, err)
	}
	_, err = m.Create(context.Background(), owner, "red", "easy")
	expectCode(t, err, "ACTIVE_GAME_LIMIT")
	repo.mu.Lock()
	var saved Record
	_ = json.Unmarshal(repo.rows[archived.ID], &saved)
	repo.mu.Unlock()
	if saved.Phase != "playing" {
		t.Fatal("reopened record not persisted")
	}
}
