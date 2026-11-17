package game

import (
	"chinesechess/internal/chess"
	"context"
	"time"
)

// get pins one canonical room for every operation. A completed room is loaded
// only on demand; it stays cached while an operation or live subscriber uses it.
// No code may hold Manager.mu while waiting on room.mu.
func (m *Manager) get(owner, id string) (*room, error) {
	if !ValidID(owner) || !ValidID(id) {
		return nil, ErrNotFound
	}
	m.mu.Lock()
	g := m.rooms[id]
	if g == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		r, err := m.repo.FindGame(ctx, owner, id)
		cancel()
		if err != nil {
			m.mu.Unlock()
			return nil, err
		}
		if r.Rules == "" {
			r.Rules = chess.LegacyRules
		}
		if r.Receipts == nil {
			r.Receipts = map[string]Receipt{}
		}
		g = &room{record: r, watchers: map[chan Snapshot]struct{}{}}
		m.rooms[id] = g
	}
	g.refs++
	m.mu.Unlock()
	g.mu.Lock()
	allowed := m.sameOwner(g.record.Owner, owner)
	g.mu.Unlock()
	if !allowed {
		m.release(g)
		return nil, ErrNotFound
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

// Caller owns admission. except allows a finished room to reopen without locking
// its already-held mutex or counting it twice.
func (m *Manager) checkAdmission(owner string, except *room) error {
	total, own := 0, 0
	for _, g := range m.roomList() {
		if g == except {
			continue
		}
		g.mu.Lock()
		if g.record.Phase != "finished" {
			total++
			if m.sameOwner(g.record.Owner, owner) {
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

func (m *Manager) sameOwner(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if repo, ok := m.repo.(interface{ SameOwner(string, string) bool }); ok {
		return repo.SameOwner(a, b)
	}
	return a == b
}
