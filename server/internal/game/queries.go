package game

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/history"
	"context"
)

func (m *Manager) Get(owner, id string) (Snapshot, error) {
	g, err := m.get(owner, id)
	if err != nil {
		return Snapshot{}, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.record.Snapshot, nil
}

func (m *Manager) Recent(ctx context.Context, owner string) ([]Snapshot, error) {
	rows, err := m.repo.RecentGames(ctx, owner)
	if err != nil {
		return nil, err
	}
	out := make([]Snapshot, 0, len(rows))
	for _, r := range rows {
		if r.Rules == "" {
			r.Rules = chess.LegacyRules
		}
		out = append(out, r.Snapshot)
	}
	return out, nil
}

func (m *Manager) History(ctx context.Context, owner string, q history.Query) (history.Page, error) {
	return m.repo.History(ctx, owner, q)
}
