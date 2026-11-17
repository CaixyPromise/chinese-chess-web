package game

func (m *Manager) Watch(owner, id string) (Snapshot, <-chan Snapshot, func(), error) {
	g, err := m.get(owner, id)
	if err != nil {
		return Snapshot{}, nil, nil, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.watchers) >= 8 {
		return Snapshot{}, nil, nil, problem("TOO_MANY_CONNECTIONS")
	}
	ch := make(chan Snapshot, 1)
	g.watchers[ch] = struct{}{}
	return g.record.Snapshot, ch, func() { g.mu.Lock(); delete(g.watchers, ch); g.mu.Unlock(); m.prune(g) }, nil
}
