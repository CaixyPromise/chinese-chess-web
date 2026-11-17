package pvp

import (
	"chinesechess/internal/game"
	"context"
)

func (m *Manager) Get(owner, id string) (View, error) {
	return m.GetSide(owner, id, "")
}

func (m *Manager) GetSide(owner, id, chosenSide string) (View, error) {
	g, err := m.get(owner, id)
	if err != nil {
		return View{}, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	if m.side(g.record, owner) == "" {
		return View{}, game.ErrNotFound
	}
	if err = m.prepareInvitation(g); err != nil {
		return View{}, err
	}
	if err = m.expire(g); err != nil {
		return View{}, err
	}
	v := m.view(g.record, owner, m.now())
	if chosenSide != "" {
		if chosenSide != "red" && chosenSide != "black" {
			return View{}, problem("INVALID_SIDE")
		}
		if !m.ownsSide(g.record, owner, chosenSide) {
			return View{}, game.ErrNotFound
		}
		v.You = chosenSide
	}
	return v, nil
}

func (m *Manager) Recent(ctx context.Context, owner string) ([]View, error) {
	rows, err := m.repo.RecentPVP(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(rows))
	for _, r := range rows {
		if r.Rules == "" {
			r.Rules = RulesVersion
		}
		out = append(out, m.view(r, owner, m.now()))
	}
	return out, nil
}
