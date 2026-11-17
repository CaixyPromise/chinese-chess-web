package game

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
	"errors"
	"time"
)

func New(repo Repository, rules, bots *engine.Pool, parallel int) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{rooms: make(map[string]*room), repo: repo, rules: rules, bots: bots, ctx: ctx, cancel: cancel, jobs: make(chan job, 16)}
	records, err := repo.Load(ctx)
	if err != nil {
		cancel()
		return nil, err
	}
	for _, r := range records {
		if r.Rules == "" {
			r.Rules = chess.LegacyRules
		}
		if !ValidID(r.ID) || !ValidID(r.Owner) {
			cancel()
			return nil, errors.New("invalid saved game")
		}
		if r.Receipts == nil {
			r.Receipts = make(map[string]Receipt)
		}
		if chess.SupportedRules(r.Rules) && (r.Phase == "thinking" || r.Phase == "queued") {
			r.Phase = "queued"
			r.Generation++
			r.Version++
			if err = repo.Save(ctx, r, r.Version-1); err != nil {
				cancel()
				return nil, err
			}
		}
		m.rooms[r.ID] = &room{record: r, watchers: make(map[chan Snapshot]struct{})}
	}
	for i := 0; i < parallel; i++ {
		m.workers.Add(1)
		go m.work()
	}
	for _, g := range m.rooms {
		if g.record.Phase == "queued" && chess.SupportedRules(g.record.Rules) {
			g.mu.Lock()
			m.schedule(g)
			g.mu.Unlock()
		}
	}
	return m, nil
}

func (m *Manager) Close() { m.cancel(); m.workers.Wait() }

func (m *Manager) Context() context.Context { return m.ctx }

func (m *Manager) commit(g *room, r Record) error {
	r.Receipts = cloneReceipts(r.Receipts)
	r.Updated = time.Now().UnixMilli()
	for id, receipt := range r.Receipts {
		if receipt.View.Version == r.Version {
			receipt.View = r.Snapshot
			r.Receipts[id] = receipt
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := m.repo.Save(ctx, r, r.Version-1); err != nil {
		return err
	}
	g.record = r
	for ch := range g.watchers {
		select {
		case ch <- r.Snapshot:
		default:
			select {
			case <-ch:
			default:
			}
			ch <- r.Snapshot
		}
	}
	return nil
}

func (m *Manager) Create(ctx context.Context, owner, human, difficulty string) (Snapshot, error) {
	if human != "red" && human != "black" {
		return Snapshot{}, problem("INVALID_SIDE")
	}
	if _, ok := profile(difficulty); !ok {
		return Snapshot{}, problem("INVALID_DIFFICULTY")
	}
	v, err := m.position(ctx, nil, "")
	if err != nil {
		return Snapshot{}, err
	}
	r := Record{Snapshot: Snapshot{Rules: chess.CasualRules, ID: ID(), Version: 1, Moves: []string{}, Human: human, Difficulty: difficulty}, Keys: []string{chess.PositionKey(Initial)}, Checks: []bool{}, Owner: owner, Receipts: make(map[string]Receipt), Generation: 1}
	r = applyView(r, v)
	g := &room{record: r, watchers: make(map[chan Snapshot]struct{})}
	m.admission.Lock()
	defer m.admission.Unlock()
	if err = m.checkAdmission(owner, nil); err != nil {
		return Snapshot{}, err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if err = m.commit(g, r); err != nil {
		return Snapshot{}, err
	}
	m.mu.Lock()
	m.rooms[r.ID] = g
	m.mu.Unlock()
	if g.record.Phase == "queued" {
		m.schedule(g)
	}
	v = g.record.Snapshot
	return v, nil
}
