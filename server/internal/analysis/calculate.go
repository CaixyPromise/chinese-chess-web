package analysis

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
	"errors"
	"strings"
	"time"
)

func (m *Manager) calculate(ctx context.Context, j *Job) (Position, error) {
	var result Position
	ply := j.NextPly
	if ply < 0 || ply > len(j.Source.Moves) || !chess.SupportedRules(j.Source.Rules) {
		return result, errors.New("invalid frozen analysis position")
	}
	if err := m.analysisHistory(ctx, j); err != nil {
		return result, err
	}
	history := append([]string{}, j.Source.Moves[:ply]...)
	result = analysisSample(*j, history)
	err := m.pool.With(ctx, func(w *engine.Worker) error {
		setup, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := verifyAnalysisTeacher(setup, w, j.Engine); err != nil {
			cancel()
			return err
		}
		if _, err := w.Call(setup, analysisPosition(history)); err != nil {
			cancel()
			return err
		}
		state, err := w.Call(setup, "inspect")
		if err != nil {
			cancel()
			return err
		}
		legal, err := w.Call(setup, "legal")
		cancel()
		if err != nil {
			return err
		}
		result.Fen = state["fen"]
		fields := strings.Fields(result.Fen)
		if len(fields) < 2 {
			return errors.New("invalid native root fen")
		}
		result.Turn = "red"
		if fields[1] == "b" {
			result.Turn = "black"
		}
		outcome, reason := analysisOutcome(result.Fen, legal["moves"] == "", j.Source.Rules, ply, j.Source.Keys[:ply+1], j.Source.Checks[:ply])
		if outcome != "ongoing" {
			result.Best = terminalAnalysisLine(outcome, result.Turn)
			result.Note = reason
			return nil
		}
		best, bestIterations, err := searchAnalysis(ctx, w, *j, history, "", result.Turn)
		if err != nil {
			return err
		}
		result.Best = best
		var actualIterations map[int]Line
		if ply < len(j.Source.Moves) {
			result.ActualMove = j.Source.Moves[ply]
			if result.ActualMove == best.Move {
				copy := best
				copy.PV = append([]string{}, best.PV...)
				result.Actual = &copy
				actualIterations = bestIterations
			} else {
				actual, iterations, err := searchAnalysis(ctx, w, *j, history, result.ActualMove, result.Turn)
				if err != nil {
					return err
				}
				result.Actual = &actual
				actualIterations = iterations
			}
			// Publish a common complete iteration when both calls reached it. Ranking
			// additionally requires the requested target, never an opportunistic depth.
			common := 0
			for depth, line := range bestIterations {
				if actual, ok := actualIterations[depth]; ok && line.Reliable && actual.Reliable && depth > common {
					common = depth
				}
			}
			if common > 0 {
				result.Best = iterationWithFinal(bestIterations[common], best)
				selected := iterationWithFinal(actualIterations[common], *result.Actual)
				result.Actual = &selected
			}
		}
		conflict, err := m.validateAnalysisPV(ctx, w, *j, history, &result.Best)
		if err != nil {
			return err
		}
		if result.Actual != nil {
			other, err := m.validateAnalysisPV(ctx, w, *j, history, result.Actual)
			if err != nil {
				return err
			}
			conflict = conflict || other
		}
		if conflict {
			result.Note = "rule_conflict"
		} else {
			compareAnalysisLines(&result, j.Budget.Depth)
		}
		if result.Best.Move != "" && result.Best.Reliable {
			result.Candidates = []Candidate{{Move: result.Best.Move, Role: "teacher_recommended", Line: result.Best}}
		}
		return nil
	})
	if err != nil {
		return Position{}, err
	}
	return result, nil
}

// Only a common target-depth regular evaluation supports a ranked loss.
func compareAnalysisLines(result *Position, targetDepth int) {
	result.Comparable = false
	result.Loss = 0
	if !result.Best.Reliable || result.Best.Depth != targetDepth {
		result.Note = "incomplete"
		return
	}
	if result.Actual == nil {
		return
	}
	actual := *result.Actual
	if !actual.Reliable || actual.Depth != targetDepth {
		result.Note = "incomplete"
		return
	}
	if result.Best.Kind != "evaluation" || actual.Kind != "evaluation" {
		return
	}
	loss := result.Best.Score - actual.Score
	if result.Turn == "black" {
		loss = -loss
	}
	if loss < 0 {
		result.Note = "unstable"
		return
	}
	result.Comparable = true
	result.Loss = loss
}

func iterationWithFinal(iteration, final Line) Line {
	iteration.StopReason = final.StopReason
	iteration.Millis = final.Millis
	return iteration
}

func analysisOutcome(fen string, noMoves bool, rules string, ply int, keys []string, checks []bool) (string, string) {
	if noMoves {
		if strings.Fields(fen)[1] == "r" {
			return "black_win", "no_legal_moves"
		}
		return "red_win", "no_legal_moves"
	}
	if rules == chess.CasualRules {
		return chess.CasualOutcome(fen, ply, keys, checks)
	}
	return "ongoing", ""
}

func terminalAnalysisLine(outcome, turn string) Line {
	line := Line{Reliable: true, Kind: "draw", PV: []string{}, StopReason: "terminal", NativePerspective: "side_to_move", Producer: "rules"}
	if outcome != "draw" {
		line.Kind = "mate"
		line.Score = 9999
		if outcome == "black_win" {
			line.Score = -9999
		}
		line.NativeScore = line.Score
		if turn == "black" {
			line.NativeScore = -line.NativeScore
		}
	}
	return line
}
