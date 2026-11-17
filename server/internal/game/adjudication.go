package game

import (
	"chinesechess/internal/chess"
	"context"
)

func (m *Manager) ruleHistory(ctx context.Context, r *Record) error {
	if r.Rules != chess.CasualRules {
		return nil
	}
	if len(r.Keys) == len(r.Moves)+1 && len(r.Checks) == len(r.Moves) && r.Keys[len(r.Keys)-1] == chess.PositionKey(r.Fen) {
		return nil
	}
	keys, checks, err := chess.RebuildRuleHistory(ctx, m.rules, r.Moves, r.Fen)
	if err != nil {
		return err
	}
	r.Keys, r.Checks = keys, checks
	return nil
}
func appendPosition(r Record, v Snapshot, move string) Record {
	r.Moves = append(append([]string{}, r.Moves...), move)
	if r.Rules == chess.CasualRules {
		r.Keys = append(append([]string{}, r.Keys...), chess.PositionKey(v.Fen))
		r.Checks = append(append([]bool{}, r.Checks...), v.InCheck)
	}
	return applyView(r, v)
}
func adjudicate(r *Record) {
	if r.Rules != chess.CasualRules || r.Phase == "finished" {
		return
	}
	if outcome, reason := chess.CasualOutcome(r.Fen, len(r.Moves), r.Keys, r.Checks); outcome != "ongoing" {
		r.Phase, r.Outcome, r.Reason = "finished", outcome, reason
		r.Legal = []string{}
	}
}
