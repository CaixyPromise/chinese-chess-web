package pvp

import (
	"chinesechess/internal/game"
	"context"
	"time"
)

// A room has one canonical writer while operations or subscribers pin it.
// Completed rooms otherwise remain in PostgreSQL, not in the active registry.
// Lock ordering: room.mu before Manager.mu; never wait on a room under Manager.mu.
func (m *Manager) get(owner, id string) (*room, error) {
	if !game.ValidID(owner) || !game.ValidID(id) {
		return nil, game.ErrNotFound
	}
	m.mu.Lock()
	g := m.rooms[id]
	if g == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		r, err := m.repo.FindPVP(ctx, owner, id)
		cancel()
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		if r.Rules == "" {
			r.Rules = RulesVersion
		}
		if r.Receipts == nil {
			r.Receipts = map[string]Receipt{}
		}
		g = &room{record: r, watchers: map[*watch]struct{}{}}
		m.rooms[id] = g
	}
	g.refs++
	m.mu.Unlock()
	g.mu.Lock()
	allowed := m.side(g.record, owner) != ""
	g.mu.Unlock()
	if !allowed {
		m.release(g)
		return nil, game.ErrNotFound
	}
	return g, nil
}

func (m *Manager) release(g *room) { m.unpin(g, true) }
func (m *Manager) prune(g *room)   { m.unpin(g, false) }
func (m *Manager) unpin(g *room, decrement bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	if decrement {
		g.refs--
	}
	if g.record.Phase == "finished" && g.refs == 0 && len(g.watchers) == 0 && m.rooms[g.record.ID] == g {
		delete(m.rooms, g.record.ID)
	}
}

func (m *Manager) roomList() []*room {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*room, 0, len(m.rooms))
	for _, g := range m.rooms {
		list = append(list, g)
	}
	return list
}

// Called under admission; only Create increases the total active-room count.
func (m *Manager) checkAdmission(owner string) error {
	total, own := 0, 0
	for _, g := range m.roomList() {
		g.mu.Lock()
		if g.record.Phase != "finished" {
			total++
			if m.side(g.record, owner) != "" {
				own++
			}
		}
		g.mu.Unlock()
	}
	if total >= 256 {
		return problem("GAME_LIMIT")
	}
	if own >= 8 {
		return problem("ACTIVE_GAME_LIMIT")
	}
	return nil
}
