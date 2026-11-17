package game

import (
	"chinesechess/internal/chess"
	"context"
)

// CreatePractice rebuilds the source prefix against the native rules and saves
// an independent game. The caller is responsible for authorizing the source.
func (m *Manager) CreatePractice(ctx context.Context, owner, human, difficulty, sourceMode, sourceID string, sourceVersion int64, moves []string, rules string, sourceGroups ...string) (Snapshot, error) {
	if human != "red" && human != "black" {
		return Snapshot{}, problem("INVALID_SIDE")
	}
	if _, ok := profile(difficulty); !ok {
		return Snapshot{}, problem("INVALID_DIFFICULTY")
	}
	if !ValidID(owner) || !ValidID(sourceID) || sourceVersion < 1 || (sourceMode != "pve" && sourceMode != "pvp") {
		return Snapshot{}, problem("INVALID_REQUEST")
	}
	if rules == "" {
		rules = chess.LegacyRules
	}
	if !chess.SupportedRules(rules) {
		return Snapshot{}, problem("UNSUPPORTED_RULES")
	}
	v, err := m.position(ctx, nil, "")
	if err != nil {
		return Snapshot{}, err
	}
	r := Record{Snapshot: Snapshot{Rules: rules, ID: ID(), Version: 1, Moves: []string{}, Human: human, Difficulty: difficulty,
		Practice: &Practice{SourceMode: sourceMode, SourceID: sourceID, SourceVersion: sourceVersion, Ply: len(moves)}},
		Keys: []string{chess.PositionKey(Initial)}, Checks: []bool{}, Owner: owner, Receipts: make(map[string]Receipt), Generation: 1}
	r.Practice.SourceGroupID = sourceMode + ":" + sourceID
	if len(sourceGroups) > 0 && sourceGroups[0] != "" {
		r.Practice.SourceGroupID = sourceGroups[0]
	}
	r = applyView(r, v)
	for _, move := range moves {
		if r.Phase == "finished" {
			return Snapshot{}, problem("PRACTICE_TERMINAL")
		}
		legal := false
		if ValidMove(move) {
			for _, candidate := range r.Legal {
				if candidate == move {
					legal = true
					break
				}
			}
		}
		if !legal {
			return Snapshot{}, problem("INVALID_REQUEST")
		}
		v, err = m.position(ctx, r.Moves, move)
		if err != nil {
			return Snapshot{}, err
		}
		r = appendPosition(r, v, move)
	}
	if r.Phase == "finished" {
		return Snapshot{}, problem("PRACTICE_TERMINAL")
	}
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
	if r.Phase == "queued" {
		m.schedule(g)
	}
	return g.record.Snapshot, nil
}
