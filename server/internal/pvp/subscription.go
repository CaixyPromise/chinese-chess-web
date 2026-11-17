package pvp

import "chinesechess/internal/game"

func (m *Manager) Watch(owner, id string) (View, <-chan View, func(), error) {
	return m.WatchSide(owner, id, "")
}

func (m *Manager) WatchSide(owner, id, chosenSide string) (View, <-chan View, func(), error) {
	g, e := m.get(owner, id)
	if e != nil {
		return View{}, nil, nil, e
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	if m.side(g.record, owner) == "" {
		return View{}, nil, nil, game.ErrNotFound
	}
	if chosenSide != "" && (chosenSide != "red" && chosenSide != "black" || !m.ownsSide(g.record, owner, chosenSide)) {
		return View{}, nil, nil, game.ErrNotFound
	}
	if len(g.watchers) >= 8 {
		return View{}, nil, nil, problem("TOO_MANY_CONNECTIONS")
	}
	if e = m.prepareInvitation(g); e != nil {
		return View{}, nil, nil, e
	}
	if e = m.expire(g); e != nil {
		return View{}, nil, nil, e
	}
	w := &watch{owner: owner, side: chosenSide, ch: make(chan View, 1)}
	g.watchers[w] = struct{}{}
	v := m.view(g.record, owner, m.now())
	if chosenSide != "" {
		v.You = chosenSide
	}
	return v, w.ch, func() { g.mu.Lock(); delete(g.watchers, w); g.mu.Unlock(); m.prune(g) }, nil
}
