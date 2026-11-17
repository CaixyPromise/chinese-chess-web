// Package chess is the shared native rule gateway for PVE and PVP.
package chess

import (
	"chinesechess/internal/engine"
	"context"
	"strings"
)

const Initial = "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1"

type Position struct {
	Fen     string
	Legal   []string
	InCheck bool
	Outcome string
}

func Replay(ctx context.Context, pool *engine.Pool, moves []string, apply string) (Position, error) {
	var view Position
	err := pool.With(ctx, func(w *engine.Worker) error {
		if _, e := w.Call(ctx, "position\tfen="+Initial+"\trules=legacy-java-v1\tmoves="+strings.Join(moves, ",")); e != nil {
			return e
		}
		if apply != "" {
			r, e := w.Call(ctx, "apply\tmove="+apply)
			if e != nil {
				return e
			}
			view.Outcome = r["outcome"]
		}
		state, e := w.Call(ctx, "inspect")
		if e != nil {
			return e
		}
		legal, e := w.Call(ctx, "legal")
		if e != nil {
			return e
		}
		view.Fen = state["fen"]
		view.InCheck = legal["in_check"] == "true"
		view.Legal = []string{}
		if legal["moves"] != "" {
			view.Legal = strings.Split(legal["moves"], ",")
		}
		if view.Outcome == "" {
			view.Outcome = "ongoing"
			if len(view.Legal) == 0 {
				if strings.Fields(view.Fen)[1] == "r" {
					view.Outcome = "black_win"
				} else {
					view.Outcome = "red_win"
				}
			}
		}
		return nil
	})
	return view, err
}
