package game

import (
	"chinesechess/internal/engine"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type memoryRepo struct {
	mu   sync.Mutex
	rows map[string][]byte
	fail bool
}

func (r *memoryRepo) Load(context.Context) ([]Record, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := []Record{}
	for _, b := range r.rows {
		var row Record
		_ = json.Unmarshal(b, &row)
		if row.Phase != "finished" {
			out = append(out, row)
		}
	}
	return out, nil
}
func (r *memoryRepo) Save(_ context.Context, row Record, version int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("storage unavailable")
	}
	if old, ok := r.rows[row.ID]; ok {
		var saved Record
		_ = json.Unmarshal(old, &saved)
		if saved.Version != version {
			return errors.New("CAS conflict")
		}
	}
	b, _ := json.Marshal(row)
	r.rows[row.ID] = b
	return nil
}
func testManager(t *testing.T, r *memoryRepo, workers int) *Manager {
	t.Helper()
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	rules, bots := engine.NewPool(binary, 2, 16), engine.NewPool(binary, 2, 0)
	m, e := New(r, rules, bots, workers)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close(); rules.Close(); bots.Close() })
	return m
}
func waitView(t *testing.T, m *Manager, owner, id string, match func(Snapshot) bool) Snapshot {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		v, e := m.Get(owner, id)
		if e != nil {
			t.Fatal(e)
		}
		if match(v) {
			return v
		}
		time.Sleep(10 * time.Millisecond)
	}
	v, _ := m.Get(owner, id)
	t.Fatalf("timed out: %+v", v)
	return Snapshot{}
}
func expectCode(t *testing.T, e error, code string) {
	t.Helper()
	var p *Error
	if !errors.As(e, &p) || p.Code != code {
		t.Fatalf("wanted %s, got %v", code, e)
	}
}

func TestNativePVEAndCommandConsistency(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	m := testManager(t, repo, 2)
	owner := ID()
	ctx := context.Background()
	v, e := m.Create(ctx, owner, "red", "easy")
	if e != nil {
		t.Fatal(e)
	}
	if len(v.Legal) != 44 || v.Phase != "playing" {
		t.Fatalf("bad initial: %+v", v)
	}
	if _, e = m.Get(ID(), v.ID); !errors.Is(e, ErrNotFound) {
		t.Fatal("owner isolation", e)
	}
	c := Command{ID(), v.Version, "move", "b0c2"}
	first, e := m.Command(ctx, owner, v.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	replay, e := m.Command(ctx, owner, v.ID, c)
	if e != nil || replay.Version != first.Version {
		t.Fatal("idempotency", e)
	}
	c.Move = "h0g2"
	_, e = m.Command(ctx, owner, v.ID, c)
	expectCode(t, e, "REQUEST_CONFLICT")
	_, e = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "move", "h0g2"})
	expectCode(t, e, "VERSION_CONFLICT")
	v = waitView(t, m, owner, v.ID, func(v Snapshot) bool { return len(v.Moves) == 2 && v.Phase == "playing" })
	if v.Moves[0] != "b0c2" {
		t.Fatal(v.Moves)
	}
	v, e = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "undo", ""})
	if e != nil || len(v.Moves) != 0 || v.Fen != Initial {
		t.Fatal("undo", v, e)
	}
	_, e = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "move", "a0a9"})
	expectCode(t, e, "ILLEGAL_MOVE")
	repo.mu.Lock()
	repo.fail = true
	repo.mu.Unlock()
	_, e = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "move", "b0c2"})
	if e == nil {
		t.Fatal("storage error not returned")
	}
	unchanged, _ := m.Get(owner, v.ID)
	if unchanged.Version != v.Version || len(unchanged.Moves) != 0 {
		t.Fatal("uncommitted move published")
	}
	repo.mu.Lock()
	repo.fail = false
	repo.mu.Unlock()
	v, e = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "resign", ""})
	if e != nil || v.Outcome != "black_win" {
		t.Fatal(v, e)
	}
	_, e = m.Command(ctx, owner, v.ID, Command{ID(), v.Version, "move", "b0c2"})
	expectCode(t, e, "GAME_FINISHED")
}
func TestRecoveryAndCancelledResult(t *testing.T) {
	repo := &memoryRepo{rows: map[string][]byte{}}
	owner := ID()
	paused := testManager(t, repo, 0)
	v, e := paused.Create(context.Background(), owner, "black", "easy")
	if e != nil {
		t.Fatal(e)
	}
	paused.Close()
	m := testManager(t, repo, 2)
	v = waitView(t, m, owner, v.ID, func(v Snapshot) bool { return v.Phase == "playing" && len(v.Moves) == 1 })
	if v.Human != "black" {
		t.Fatal(v)
	}
	v, e = m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "move", v.Legal[0]})
	if e != nil {
		t.Fatal(e)
	}
	// Cancel either the queued or running computation; a late result must not append.
	gameID := v.ID
	for i := 0; i < 10; i++ {
		current, _ := m.Get(owner, gameID)
		v, e = m.Command(context.Background(), owner, gameID, Command{ID(), current.Version, "resign", ""})
		if e == nil {
			break
		}
	}
	if e != nil {
		t.Fatal(e)
	}
	count := len(v.Moves)
	time.Sleep(350 * time.Millisecond)
	after, _ := m.Get(owner, v.ID)
	if after.Phase != "finished" || len(after.Moves) != count {
		t.Fatalf("late search mutated resigned game: %+v", after)
	}
}
func TestConcurrentCommandHasOneWinner(t *testing.T) {
	m := testManager(t, &memoryRepo{rows: map[string][]byte{}}, 0)
	owner := ID()
	v, e := m.Create(context.Background(), owner, "red", "easy")
	if e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "move", "b0c2"})
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		} else {
			expectCode(t, e, "VERSION_CONFLICT")
		}
	}
	if wins != 1 {
		t.Fatal("winners", wins)
	}
}

func TestBotFailureCanRecoverWithoutLosingPlayerMove(t *testing.T) {
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	link := filepath.Join(t.TempDir(), "native-worker")
	rules, bots := engine.NewPool(binary, 1, 4), engine.NewPool(link, 1, 0)
	defer rules.Close()
	defer bots.Close()
	m, e := New(&memoryRepo{rows: map[string][]byte{}}, rules, bots, 1)
	if e != nil {
		t.Fatal(e)
	}
	defer m.Close()
	owner := ID()
	v, e := m.Create(context.Background(), owner, "red", "easy")
	if e != nil {
		t.Fatal(e)
	}
	v, e = m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "move", "b0c2"})
	if e != nil {
		t.Fatal(e)
	}
	v = waitView(t, m, owner, v.ID, func(v Snapshot) bool { return v.Phase == "error" })
	if len(v.Moves) != 1 || v.ErrorCode != "BOT_UNAVAILABLE" {
		t.Fatal("lost accepted move", v)
	}
	if e = os.Symlink(binary, link); e != nil {
		t.Fatal(e)
	}
	v, e = m.Command(context.Background(), owner, v.ID, Command{ID(), v.Version, "retry", ""})
	if e != nil {
		t.Fatal(e)
	}
	v = waitView(t, m, owner, v.ID, func(v Snapshot) bool { return v.Phase == "playing" && len(v.Moves) == 2 })
	if v.Moves[0] != "b0c2" {
		t.Fatal(v)
	}
}
