package pvp

import (
	"chinesechess/internal/game"
	"context"
	"time"
)

func (m *Manager) invitationRoom(code string) *room {
	if !game.ValidID(code) {
		return nil
	}
	for _, g := range m.roomList() {
		g.mu.Lock()
		_, old := g.record.PreviousInvites[code]
		ok := g.record.Invite == code || old
		g.mu.Unlock()
		if ok {
			return g
		}
	}
	return nil
}

// Pin the same canonical writer used by participant reads. Completed rooms are
// loaded only to project their invitation status and the caller's application.
func (m *Manager) findInvitation(code string) (*room, error) {
	if !game.ValidID(code) {
		return nil, game.ErrNotFound
	}
	for _, g := range m.roomList() {
		g.mu.Lock()
		_, old := g.record.PreviousInvites[code]
		ok := g.record.Invite == code || old
		id := g.record.ID
		g.mu.Unlock()
		if !ok {
			continue
		}
		m.mu.Lock()
		if m.rooms[id] == g {
			g.refs++
			m.mu.Unlock()
			return g, nil
		}
		m.mu.Unlock()
	}
	repo, ok := m.repo.(interface {
		FindPVPInvitation(context.Context, string) (Record, error)
	})
	if !ok {
		return nil, game.ErrNotFound
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	r, err := repo.FindPVPInvitation(ctx, code)
	if err != nil {
		return nil, err
	}
	if r.Rules == "" {
		r.Rules = RulesVersion
	}
	m.mu.Lock()
	g := m.rooms[r.ID]
	if g == nil {
		if r.Receipts == nil {
			r.Receipts = map[string]Receipt{}
		}
		g = &room{record: r, watchers: map[*watch]struct{}{}}
		m.rooms[r.ID] = g
	}
	g.refs++
	m.mu.Unlock()
	return g, nil
}
