package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"context"
	"time"
)

func New(repo Repository, rules *engine.Pool, clocks ...func() int64) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{rooms: map[string]*room{}, repo: repo, rules: rules, now: func() int64 { return time.Now().UnixMilli() }, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	m.position = func(ctx context.Context, moves []string, apply string) (chess.Position, error) {
		return chess.Replay(ctx, rules, moves, apply)
	}
	if len(clocks) > 0 {
		m.now = clocks[0]
	}
	records, e := repo.LoadPVP(ctx)
	if e != nil {
		cancel()
		return nil, e
	}
	for _, r := range records {
		if !game.ValidID(r.ID) || !game.ValidID(r.Host) || !game.ValidID(r.Invite) {
			cancel()
			return nil, problem("INVALID_SAVED_ROOM")
		}
		if r.Rules == "" {
			r.Rules = RulesVersion
		}
		if r.Receipts == nil {
			r.Receipts = map[string]Receipt{}
		}
		m.rooms[r.ID] = &room{record: r, watchers: map[*watch]struct{}{}}
	}
	go m.tick()
	return m, nil
}
func (m *Manager) Close()                   { m.cancel(); <-m.done }
func (m *Manager) Context() context.Context { return m.ctx }
func (m *Manager) commit(g *room, r Record) error {
	r.Version++
	r.Updated = m.now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if e := m.repo.SavePVP(ctx, r, r.Version-1); e != nil {
		return e
	}
	g.record = r
	for w := range g.watchers {
		v := m.view(r, w.owner, m.now())
		if w.side != "" && m.ownsSide(r, w.owner, w.side) {
			v.You = w.side
		}
		select {
		case w.ch <- v:
		default:
			select {
			case <-w.ch:
			default:
			}
			w.ch <- v
		}
	}
	return nil
}
