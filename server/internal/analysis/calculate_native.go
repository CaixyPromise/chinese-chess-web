package analysis

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func analysisPosition(history []string) string {
	return "position\tfen=" + chess.Initial + "\trules=" + chess.LegacyRules + "\tmoves=" + strings.Join(history, ",")
}

func parseAnalysisLine(fields map[string]string, turn string, info bool) (Line, error) {
	line := Line{NativePerspective: "side_to_move", Producer: "search"}
	var err error
	line.NativeScore, err = strconv.Atoi(fields["score"])
	if err != nil {
		return line, err
	}
	line.Depth, err = strconv.Atoi(fields["depth"])
	if err != nil {
		return line, err
	}
	line.Millis, err = strconv.Atoi(fields["elapsed_ms"])
	if err != nil {
		return line, err
	}
	for key, target := range map[string]*uint64{
		"completed_ns": &line.CompletedNanos,
		"nodes":        &line.Nodes,
		"evaluations":  &line.Evaluations,
		"cache_hits":   &line.CacheHits,
		"cutoffs":      &line.Cutoffs,
	} {
		*target, err = strconv.ParseUint(fields[key], 10, 64)
		if err != nil {
			return line, err
		}
	}
	line.Score = line.NativeScore
	if turn == "black" {
		line.Score = -line.Score
	}
	line.Reliable = (info && line.Depth > 0) || fields["score_reliable"] == "true"
	line.Kind = "unavailable"
	if !line.Reliable {
		line.Producer = "fallback"
	}
	if line.Reliable {
		line.Kind = "evaluation"
		if line.NativeScore >= 9900 || line.NativeScore <= -9900 {
			line.Kind = "mate"
		}
	}
	line.StopReason = fields["stop_reason"]
	line.PV = []string{}
	if fields["pv"] != "" {
		line.PV = strings.Split(fields["pv"], ",")
		line.Move = line.PV[0]
	}
	if !info && fields["move"] != "none" && fields["move"] != "" {
		if line.Move != "" && line.Move != fields["move"] {
			return line, errors.New("native bestmove differs from pv")
		}
		line.Move = fields["move"]
	}
	return line, nil
}

func searchAnalysis(ctx context.Context, w *engine.Worker, j Job, history []string, root, turn string) (Line, map[int]Line, error) {
	var empty Line
	iterations := map[int]Line{}
	setup, stopSetup := context.WithTimeout(ctx, 5*time.Second)
	_, err := w.Call(setup, analysisPosition(history))
	stopSetup()
	if err != nil {
		return empty, nil, err
	}
	budget := j.Budget
	task := "recommended"
	if root != "" {
		task = "played"
	}
	command := fmt.Sprintf("search\tgame_id=analysis\tposition_version=%d\tsearch_id=%s\tdepth=%d\ttime_ms=%d\thash_mib=%d\tnodes=%d", j.Generation, task, budget.Depth, budget.Millis, budget.HashMiB, budget.Nodes)
	if root != "" {
		command += "\troot_move=" + root
	}
	searchCtx, cancel := context.WithTimeout(ctx, time.Duration(budget.Millis+500)*time.Millisecond)
	defer cancel()
	var parseErr error
	fields, err := w.CallWithInfo(searchCtx, command, func(fields map[string]string) {
		line, e := parseAnalysisLine(fields, turn, true)
		if e != nil {
			parseErr = e
			return
		}
		iterations[line.Depth] = line
	})
	if err != nil {
		return empty, nil, err
	}
	if parseErr != nil {
		return empty, nil, parseErr
	}
	line, err := parseAnalysisLine(fields, turn, false)
	if err != nil {
		return empty, nil, err
	}
	if line.Reliable && line.Depth > 0 {
		iterations[line.Depth] = line
	}
	if root != "" && line.Move != root {
		return empty, nil, errors.New("native constrained root mismatch")
	}
	return line, iterations, nil
}

// Every retained PV is replayed through native legal/apply. Casual rule endings
// truncate the displayed variation before any following moves can be presented.
func (m *Manager) validateAnalysisPV(ctx context.Context, w *engine.Worker, j Job, history []string, line *Line) (bool, error) {
	work, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := w.Call(work, analysisPosition(history)); err != nil {
		return false, err
	}
	keys := append([]string{}, j.Source.Keys[:len(history)+1]...)
	checks := append([]bool{}, j.Source.Checks[:len(history)]...)
	original := append([]string{}, line.PV...)
	for i, move := range original {
		legal, err := w.Call(work, "legal")
		if err != nil {
			return false, err
		}
		found := false
		for _, candidate := range strings.Split(legal["moves"], ",") {
			if candidate == move {
				found = true
				break
			}
		}
		if !found {
			return false, errors.New("illegal native analysis pv")
		}
		applied, err := w.Call(work, "apply\tmove="+move)
		if err != nil {
			return false, err
		}
		keys = append(keys, chess.PositionKey(applied["fen"]))
		checks = append(checks, applied["in_check"] == "true")
		outcome, reason := applied["outcome"], ""
		if outcome == "ongoing" && j.Source.Rules == chess.CasualRules {
			outcome, reason = chess.CasualOutcome(applied["fen"], len(history)+i+1, keys, checks)
		}
		if outcome != "ongoing" {
			line.PV = append([]string{}, original[:i+1]...)
			if reason != "" {
				// A rule draw requires a real zero score; a rule win requires the same
				// winning side. Never turn a conflicting native score into a mate label.
				agrees := line.Reliable && ((outcome == "draw" && line.Score == 0 && line.Kind == "evaluation") || (outcome == "red_win" && line.Kind == "mate" && line.Score > 0) || (outcome == "black_win" && line.Kind == "mate" && line.Score < 0))
				if !agrees {
					line.Reliable = false
					line.Kind = "unavailable"
					return true, nil
				}
				if outcome == "draw" {
					line.Kind = "draw"
				}
			}
			break
		}
	}
	return false, nil
}
