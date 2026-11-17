package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
	"strings"
	"testing"
)

func TestRealLegalThreefoldDraw(t *testing.T) {
	m, _, _ := setup(t)
	v, red, black := started(t, m)
	for _, move := range []string{"b0c2", "b9c7", "c2b0", "c7b9", "b0c2", "b9c7", "c2b0", "c7b9"} {
		actor := red
		if len(v.Moves)%2 == 1 {
			actor = black
		}
		v = command(t, m, actor, v.ID, "move", move)
	}
	if v.Phase != "finished" || v.Outcome != "draw" || v.Reason != "repetition" || v.Rules != RulesVersion {
		t.Fatal("threefold", v.Phase, v.Outcome, v.Reason)
	}
}
func TestNativeChecksDrivePerpetualCheckLoss(t *testing.T) {
	m, _, _ := setup(t)
	initial := "3k5/9/4R4/9/9/9/9/9/9/5K3 r - - 0 1"
	r := Record{State: State{Phase: "playing", Fen: initial, Moves: []string{}}, Keys: []string{positionKey(initial)}}
	e := m.rules.With(context.Background(), func(w *engine.Worker) error {
		if _, e := w.Call(context.Background(), "position\tfen="+initial+"\trules=legacy-java-v1"); e != nil {
			return e
		}
		for _, move := range []string{"e7d7", "d9e9", "d7e7", "e9d9", "e7d7", "d9e9", "d7e7", "e9d9"} {
			p, e := w.Call(context.Background(), "apply\tmove="+move)
			if e != nil {
				return e
			}
			r.Moves = append(r.Moves, move)
			r.Fen = p["fen"]
			r.Keys = append(r.Keys, positionKey(r.Fen))
			r.Checks = append(r.Checks, p["in_check"] == "true")
			adjudicate(&r)
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	if r.Reason != "perpetual_check" || r.Outcome != "black_win" {
		t.Fatal("native perpetual check not adjudicated", r.Checks, r.Reason, r.Outcome)
	}
}
func TestRuleHistoryRebuildAndNoAttackDraw(t *testing.T) {
	m, _, _ := setup(t)
	moves := []string{"b0c2", "b9c7"}
	p, e := chess.Replay(context.Background(), m.rules, moves, "")
	if e != nil {
		t.Fatal(e)
	}
	r := Record{State: State{Fen: p.Fen, Moves: moves, Phase: "playing"}}
	if e = ruleHistory(context.Background(), m.rules, &r); e != nil {
		t.Fatal(e)
	}
	if len(r.Keys) != 3 || len(r.Checks) != 2 || r.Keys[2] != positionKey(p.Fen) {
		t.Fatal("incomplete reconstructed history")
	}
	r.Fen = "3k5/9/9/9/9/9/9/9/9/5K3 r - - 0 1"
	adjudicate(&r)
	if r.Reason != "insufficient_material" || r.Outcome != "draw" {
		t.Fatal("no attack draw")
	}
	if strings.Contains(positionKey(p.Fen), " - ") {
		t.Fatal("position identity contains clocks")
	}
}
