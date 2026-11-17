package game

import (
	"chinesechess/internal/chess"
	"context"
	"strings"
)

func (m *Manager) position(ctx context.Context, moves []string, apply string) (Snapshot, error) {
	v, e := chess.Replay(ctx, m.rules, moves, apply)
	return Snapshot{Fen: v.Fen, Legal: v.Legal, InCheck: v.InCheck, Outcome: v.Outcome}, e
}

func turn(s Snapshot) string {
	if strings.Fields(s.Fen)[1] == "r" {
		return "red"
	}
	return "black"
}

func applyView(r Record, v Snapshot) Record {
	r.Fen = v.Fen
	r.InCheck = v.InCheck
	r.Legal = v.Legal
	r.Outcome = v.Outcome
	r.ErrorCode = ""
	r.Reason = ""
	r.Phase = "playing"
	if r.Outcome != "ongoing" {
		r.Phase = "finished"
		r.Reason = "board"
	} else if turn(r.Snapshot) != r.Human {
		r.Phase = "queued"
	}
	adjudicate(&r)
	return r
}
