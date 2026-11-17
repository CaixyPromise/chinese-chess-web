package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
)

type repository struct {
	mu   sync.Mutex
	rows map[string][]byte
	fail bool
}

func (r *repository) LoadPVP(context.Context) ([]Record, error) {
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
func (r *repository) SavePVP(_ context.Context, row Record, expected int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.fail {
		return errors.New("save failed")
	}
	if b, ok := r.rows[row.ID]; ok {
		var old Record
		_ = json.Unmarshal(b, &old)
		if old.Version != expected {
			return errors.New("CAS")
		}
	}
	b, _ := json.Marshal(row)
	r.rows[row.ID] = b
	return nil
}
func setup(t *testing.T) (*Manager, *atomic.Int64, *repository) {
	t.Helper()
	var clock atomic.Int64
	clock.Store(1000000)
	repo := &repository{rows: map[string][]byte{}}
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	pool := engine.NewPool(binary, 2, 8)
	m, e := New(repo, pool, clock.Load)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { m.Close(); pool.Close() })
	return m, &clock, repo
}
func command(t *testing.T, m *Manager, owner, id, kind, move string) View {
	t.Helper()
	v, e := m.Get(owner, id)
	if e != nil {
		t.Fatal(e)
	}
	next, e := m.Command(context.Background(), owner, id, Command{game.ID(), v.Version, kind, move, ""})
	if e != nil {
		t.Fatalf("%s: %v", kind, e)
	}
	return next
}
func started(t *testing.T, m *Manager) (View, string, string) {
	t.Helper()
	red, black := game.ID(), game.ID()
	v, e := m.Create(context.Background(), red, "red")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = joinApproved(m, black, v.Invite); e != nil {
		t.Fatal(e)
	}
	command(t, m, red, v.ID, "ready", "")
	v = command(t, m, black, v.ID, "ready", "")
	if v.Phase != "playing" || v.You != "black" {
		t.Fatal(v)
	}
	return v, red, black
}
func code(t *testing.T, e error, want string) {
	t.Helper()
	var p *game.Error
	if !errors.As(e, &p) || p.Code != want {
		t.Fatalf("wanted %s got %v", want, e)
	}
}
func TestTwoPlayersConsentAndAuthoritativeClock(t *testing.T) {
	m, clock, _ := setup(t)
	v, red, black := started(t, m)
	clock.Add(500)
	v = command(t, m, red, v.ID, "move", "b0c2")
	if v.RedMillis != 604500 || v.BlackMillis != 600000 {
		t.Fatal("red clock", v)
	}
	_, e := m.Command(context.Background(), red, v.ID, Command{game.ID(), v.Version, "move", "h0g2", ""})
	code(t, e, "NOT_YOUR_TURN")
	clock.Add(1000)
	v = command(t, m, black, v.ID, "move", "b9c7")
	if v.BlackMillis != 604000 {
		t.Fatal("black clock", v)
	}
	clock.Add(200)
	v = command(t, m, red, v.ID, "undo", "")
	_, e = m.Command(context.Background(), red, v.ID, Command{game.ID(), v.Version, "accept", "", ""})
	code(t, e, "NOT_ALLOWED")
	clock.Add(400)
	v = command(t, m, black, v.ID, "accept", "")
	if len(v.Moves) != 0 || v.RedMillis != 603900 || v.BlackMillis != 604000 {
		t.Fatal("consensual undo clock", v)
	}
	v = command(t, m, red, v.ID, "draw", "")
	v = command(t, m, black, v.ID, "decline", "")
	if v.Proposal != nil || v.Phase != "playing" {
		t.Fatal(v)
	}
	v = command(t, m, black, v.ID, "draw", "")
	v = command(t, m, red, v.ID, "accept", "")
	if v.Phase != "finished" || v.Outcome != "draw" || v.Reason != "agreement" {
		t.Fatal(v)
	}
}
func TestTimeoutBeforeMoveAndIdempotentReceipt(t *testing.T) {
	m, clock, _ := setup(t)
	v, red, black := started(t, m)
	c := Command{game.ID(), v.Version, "move", "b0c2", ""}
	first, e := m.Command(context.Background(), red, v.ID, c)
	if e != nil {
		t.Fatal(e)
	}
	again, e := m.Command(context.Background(), red, v.ID, c)
	if e != nil || again.Version != first.Version || len(again.Moves) != 1 {
		t.Fatal("receipt", e)
	}
	c.Move = "h0g2"
	_, e = m.Command(context.Background(), red, v.ID, c)
	code(t, e, "REQUEST_CONFLICT")
	clock.Add(600001)
	after, e := m.Get(black, v.ID)
	if e != nil || after.Outcome != "red_win" || after.Reason != "timeout" || after.BlackMillis != 0 {
		t.Fatal("timeout", after, e)
	}
	_, e = m.Command(context.Background(), black, v.ID, Command{game.ID(), after.Version, "move", "b9c7", ""})
	code(t, e, "GAME_FINISHED")
}
func TestRoomMembershipLeaveAndConcurrentJoin(t *testing.T) {
	m, _, _ := setup(t)
	host := game.ID()
	v, e := m.Create(context.Background(), host, "black")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Get(game.ID(), v.ID); !errors.Is(e, game.ErrNotFound) {
		t.Fatal("room leaked", e)
	}
	var wg sync.WaitGroup
	results := make(chan View, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, e := joinApproved(m, game.ID(), v.Invite)
			if e == nil {
				results <- v
			}
		}()
	}
	wg.Wait()
	close(results)
	if len(results) != 1 {
		t.Fatal("concurrent join winners", len(results))
	}
	guestView := <-results
	g, _ := m.get(host, v.ID)
	defer m.release(g)
	g.mu.Lock()
	guest := g.record.RedOwner
	g.mu.Unlock()
	if guestView.You != "red" {
		t.Fatal(guestView)
	}
	before, e := joinApproved(m, guest, v.Invite)
	if e != nil {
		t.Fatal(e)
	}
	if before.Version != guestView.Version {
		t.Fatal("repeat join changed version")
	}
	command(t, m, guest, v.ID, "leave", "")
	if _, e = m.Get(guest, v.ID); !errors.Is(e, game.ErrNotFound) {
		t.Fatal("departed guest retained access")
	}
	if _, e = joinApproved(m, game.ID(), v.Invite); e != nil {
		t.Fatal("seat was not released", e)
	}
	v = command(t, m, host, v.ID, "leave", "")
	if v.Phase != "finished" || v.Reason != "cancelled" {
		t.Fatal(v)
	}
}
func TestStorageFailureDoesNotPublishMove(t *testing.T) {
	m, _, repo := setup(t)
	v, red, _ := started(t, m)
	repo.mu.Lock()
	repo.fail = true
	repo.mu.Unlock()
	_, e := m.Command(context.Background(), red, v.ID, Command{game.ID(), v.Version, "move", "b0c2", ""})
	if e == nil {
		t.Fatal("save failure was hidden")
	}
	after, _ := m.Get(red, v.ID)
	if after.Version != v.Version || len(after.Moves) != 0 {
		t.Fatal("uncommitted move", after)
	}
}

func TestMoveAtDeadlineCannotBeatTimeout(t *testing.T) {
	m, clock, _ := setup(t)
	v, red, _ := started(t, m)
	clock.Add(InitialMillis)
	_, e := m.Command(context.Background(), red, v.ID, Command{game.ID(), v.Version, "move", "b0c2", ""})
	if e == nil {
		t.Fatal("expired move accepted")
	}
	after, e := m.Get(red, v.ID)
	if e != nil || len(after.Moves) != 0 || after.Outcome != "black_win" || after.Reason != "timeout" {
		t.Fatal("deadline race", after.Phase, after.Reason, e)
	}
}
func TestResignAndDuplicateMoveRaceHasOneWinner(t *testing.T) {
	m, _, _ := setup(t)
	v, red, _ := started(t, m)
	var wg sync.WaitGroup
	success := atomic.Int64{}
	for _, kind := range []string{"resign", "move"} {
		wg.Add(1)
		go func(kind string) {
			defer wg.Done()
			move := ""
			if kind == "move" {
				move = "b0c2"
			}
			if _, e := m.Command(context.Background(), red, v.ID, Command{game.ID(), v.Version, kind, move, ""}); e == nil {
				success.Add(1)
			}
		}(kind)
	}
	wg.Wait()
	if success.Load() != 1 {
		t.Fatal("concurrent commands accepted", success.Load())
	}
}

func TestUndoReplayLatencyIsChargedBeforeChangingTurn(t *testing.T) {
	for _, delay := range []int64{2000, 600001} {
		t.Run(fmt.Sprint(delay), func(t *testing.T) {
			m, clock, _ := setup(t)
			v, red, black := started(t, m)
			v = command(t, m, red, v.ID, "move", "b0c2")
			v = command(t, m, red, v.ID, "undo", "")
			native := m.position
			m.position = func(ctx context.Context, moves []string, apply string) (chess.Position, error) {
				clock.Add(delay)
				return native(ctx, moves, apply)
			}
			v = command(t, m, black, v.ID, "accept", "")
			if delay < InitialMillis {
				if len(v.Moves) != 0 || v.RedMillis != 605000 || v.BlackMillis != 598000 || v.TurnStarted != clock.Load() {
					t.Fatal("undo replay charged new turn", v.RedMillis, v.BlackMillis, v.Moves)
				}
			} else {
				if len(v.Moves) != 1 || v.Outcome != "red_win" || v.Reason != "timeout" {
					t.Fatal("undo accepted after timeout", v.Outcome, v.Reason)
				}
			}
		})
	}
}

// A standalone state machine and sequential clock make the two samples exact;
// no wall-time sleeps or background ticker can mask the boundary regression.
func TestConsentAndResignCannotOverwriteTimeoutBetweenSamples(t *testing.T) {
	for _, kind := range []string{"accept", "resign"} {
		t.Run(kind, func(t *testing.T) {
			red, black, id := game.ID(), game.ID(), game.ID()
			r := Record{State: State{ID: id, Version: 1, Phase: "playing", Outcome: "ongoing", Moves: []string{"b0c2"}, RedMillis: 605000, BlackMillis: InitialMillis, TurnStarted: 1000000, Proposal: &Proposal{Kind: "draw", By: "red"}}, RedOwner: red, BlackOwner: black, Receipts: map[string]Receipt{}}
			b, _ := json.Marshal(r)
			repo := &repository{rows: map[string][]byte{id: b}}
			samples := 0
			m := &Manager{rooms: map[string]*room{id: {record: r, watchers: map[*watch]struct{}{}}}, repo: repo, now: func() int64 {
				samples++
				if samples == 1 {
					return 1599999
				}
				return 1600000
			}}
			_, e := m.Command(context.Background(), black, id, Command{game.ID(), 1, kind, "", ""})
			if e != nil {
				t.Fatal(e)
			}
			repo.mu.Lock()
			var after Record
			_ = json.Unmarshal(repo.rows[id], &after)
			repo.mu.Unlock()
			if after.Phase != "finished" || after.Outcome != "red_win" || after.Reason != "timeout" || after.BlackMillis != 0 {
				t.Fatal("terminal command overwrote deadline", after.Outcome, after.Reason)
			}
		})
	}
}

// Regression setup follows the production application and host approval flow.
func joinApproved(m *Manager, owner, invite string) (View, error) {
	a, err := m.ApplyJoin(owner, invite, "受邀者", game.ID())
	if err != nil {
		return View{}, err
	}
	if a.State == "approved" {
		return m.Get(owner, a.RoomID)
	}
	g := m.invitationRoom(invite)
	g.mu.Lock()
	host := g.record.Host
	g.mu.Unlock()
	_, err = m.Command(context.Background(), host, a.RoomID, Command{RequestID: game.ID(), Version: a.Version, Type: "approve_join", ApplicationID: a.ApplicationID})
	if err != nil {
		return View{}, err
	}
	return m.Get(owner, a.RoomID)
}
