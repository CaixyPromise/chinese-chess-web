package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
)

const RulesVersion = chess.CasualRules

func positionKey(fen string) string { return chess.PositionKey(fen) }

func ruleHistory(ctx context.Context, pool *engine.Pool, r *Record) error {
	if r.Rules != "" && r.Rules != RulesVersion {
		return nil
	}
	if len(r.Keys) == len(r.Moves)+1 && len(r.Checks) == len(r.Moves) && r.Keys[len(r.Keys)-1] == positionKey(r.Fen) {
		return nil
	}
	keys, checks, err := chess.RebuildRuleHistory(ctx, pool, r.Moves, r.Fen)
	if err != nil {
		return err
	}
	r.Keys, r.Checks = keys, checks
	return nil
}
func adjudicate(r *Record) {
	// PVP has always used casual rules; only missing versions inherit this default.
	if r.Rules == "" {
		r.Rules = RulesVersion
	}
	if r.Rules != RulesVersion || r.Phase != "playing" {
		return
	}
	if outcome, reason := chess.CasualOutcome(r.Fen, len(r.Moves), r.Keys, r.Checks); outcome != "ongoing" {
		finish(r, outcome, reason)
	}
}
func finish(r *Record, outcome, reason string) {
	r.Phase = "finished"
	r.Outcome = outcome
	r.Reason = reason
	r.TurnStarted = 0
	r.Proposal = nil
	r.Legal = []string{}
}
