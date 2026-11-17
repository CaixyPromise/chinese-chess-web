package chess

import (
	"chinesechess/internal/engine"
	"context"
	"errors"
)

// RebuildRuleHistory repairs missing metadata, never the rule version or saved result.
// New records maintain these arrays incrementally.
func RebuildRuleHistory(ctx context.Context, pool *engine.Pool, moves []string, fen string) ([]string, []bool, error) {
	keys, checks := []string{PositionKey(Initial)}, []bool{}
	for n := 1; n <= len(moves); n++ {
		p, err := Replay(ctx, pool, moves[:n], "")
		if err != nil {
			return nil, nil, err
		}
		keys = append(keys, PositionKey(p.Fen))
		checks = append(checks, p.InCheck)
		if n == len(moves) && p.Fen != fen {
			return nil, nil, errors.New("saved history differs from board")
		}
	}
	if len(moves) == 0 && PositionKey(fen) != PositionKey(Initial) {
		return nil, nil, errors.New("saved initial board differs")
	}
	return keys, checks, nil
}
