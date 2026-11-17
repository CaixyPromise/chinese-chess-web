package analysis

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func calculator(t *testing.T) *Manager {
	t.Helper()
	binary, err := filepath.Abs("../../../build/engine/xiangqi-engine")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(binary); err != nil {
		t.Skip("build native engine first:", err)
	}
	pool := engine.NewPool(binary, 1, 2)
	rules := engine.NewPool(binary, 1, 2)
	t.Cleanup(func() { pool.Close(); rules.Close() })
	return &Manager{pool: pool, rules: rules}
}

func calculationJob(t *testing.T, m *Manager, moves []string, ply int, profile string) Job {
	t.Helper()
	p, err := chess.Replay(context.Background(), m.rules, moves, "")
	if err != nil {
		t.Fatal(err)
	}
	return Job{ID: "fixture", Generation: 1, Engine: EngineVersion, Scope: "position", NextPly: ply, Profile: profile, Budget: ProfileBudget(profile),
		Source: Source{Mode: "pve", ID: "source", Version: 9, Moves: append([]string{}, moves...), Rules: chess.CasualRules, Phase: "finished", Fen: p.Fen, Outcome: "red_win", Reason: "resigned", GroupID: "practice-origin", Origin: "practice"}}
}

func TestCalculateRealNativeProfilesAndTrainingSample(t *testing.T) {
	m := calculator(t)
	for _, profile := range []string{"standard", "deep"} {
		t.Run(profile, func(t *testing.T) {
			j := calculationJob(t, m, []string{"a0a2"}, 0, profile)
			digest := Digest(j.Source)
			result, err := m.calculate(context.Background(), &j)
			if err != nil {
				t.Fatal(err)
			}
			if result.Best.Depth != j.Budget.Depth || !result.Best.Reliable || result.Best.Kind != "evaluation" || result.Actual == nil || result.Actual.Depth != j.Budget.Depth || !result.Comparable || result.Loss <= 0 {
				t.Fatalf("analysis: %+v actual=%+v", result, result.Actual)
			}
			if result.Best.Nodes == 0 || result.Best.Evaluations == 0 || result.Best.CompletedNanos == 0 || result.Best.StopReason != "completed" || result.Best.Producer != "search" || result.Best.NativePerspective != "side_to_move" {
				t.Fatal("missing native statistics", result.Best)
			}
			if result.Best.Move == "a0a2" || result.Actual.Move != "a0a2" || result.Actual.PV[0] != "a0a2" {
				t.Fatalf("root restriction: %+v", result)
			}
			if result.Fen != chess.Initial || result.Turn != "red" || result.Teacher != "0.5.2" || result.ScorePerspective != "red" || result.ScoreUnits != "legacy-native" || result.Origin != "practice" || result.GroupID != "practice-origin" || result.SourceID != "source" || result.SourceVersion != 9 || !result.Final || result.Outcome != "red_win" || result.EndReason != "resigned" {
				t.Fatalf("metadata: %+v", result)
			}
			if len(result.History) != 0 || len(j.Source.Keys) != 2 || len(j.Source.Checks) != 1 || Digest(j.Source) != digest {
				t.Fatal("prefix/history/digest")
			}
			if len(result.Candidates) != 1 || result.Candidates[0].Role != "teacher_recommended" || result.Candidates[0].Move != result.Best.Move {
				t.Fatalf("candidates: %+v", result.Candidates)
			}
			again, err := m.calculate(context.Background(), &j)
			if err != nil {
				t.Fatal(err)
			}
			if again.PositionID != result.PositionID || again.SampleID != result.SampleID {
				t.Fatal("unstable sample identity")
			}
			j.Source.ID = "different-source"
			other, err := m.calculate(context.Background(), &j)
			if err != nil {
				t.Fatal(err)
			}
			if other.PositionID != result.PositionID || other.SampleID == result.SampleID || other.GroupID != result.GroupID {
				t.Fatal("source/group dedup identity")
			}
		})
	}
}

func TestCalculateBlackPerspectiveAndUnreliableFallback(t *testing.T) {
	m := calculator(t)
	j := calculationJob(t, m, []string{"b0c2", "a9a7"}, 1, "standard")
	result, err := m.calculate(context.Background(), &j)
	if err != nil {
		t.Fatal(err)
	}
	if result.Turn != "black" || result.Best.Score != -result.Best.NativeScore || result.Actual == nil || result.Actual.Score != -result.Actual.NativeScore || !reflect.DeepEqual(result.History, []string{"b0c2"}) {
		t.Fatalf("black score/history: %+v", result)
	}
	if result.Comparable && result.Loss != result.Actual.Score-result.Best.Score {
		t.Fatal("black loss sign")
	}
	j = calculationJob(t, m, []string{"a0a2"}, 0, "standard")
	j.Budget = Budget{Depth: 32, Millis: 1, HashMiB: 128, Nodes: 1}
	result, err = m.calculate(context.Background(), &j)
	if err != nil {
		t.Fatal(err)
	}
	if result.Best.Reliable || result.Best.Depth != 0 || result.Best.Kind != "unavailable" || result.Comparable || result.Note != "incomplete" || result.Actual.Reliable || result.Best.StopReason != "deadline" {
		t.Fatalf("fallback became evaluation: %+v", result)
	}
	if result.Best.Producer != "fallback" || len(result.Candidates) != 0 {
		t.Fatal("fallback became a training recommendation", result)
	}
}

func TestCalculateRuleDrawAndHistoryRestoration(t *testing.T) {
	m := calculator(t)
	moves := []string{"b0c2", "b9c7", "c2b0", "c7b9", "b0c2", "b9c7", "c2b0", "c7b9"}
	j := calculationJob(t, m, moves, len(moves), "standard")
	j.Source.Outcome = "draw"
	j.Source.Reason = "repetition"
	result, err := m.calculate(context.Background(), &j)
	if err != nil {
		t.Fatal(err)
	}
	if result.Best.Kind != "draw" || !result.Best.Reliable || result.Best.Move != "" || result.Best.Score != 0 || result.Best.StopReason != "terminal" || result.Comparable || result.Note != "repetition" {
		t.Fatalf("rule draw: %+v", result)
	}
	if result.Best.Producer != "rules" || result.Best.NativePerspective != "side_to_move" || result.Best.Nodes != 0 {
		t.Fatal("rule adjudication mislabeled as search", result.Best)
	}
	if len(j.Source.Keys) != 9 || len(j.Source.Checks) != 8 || !reflect.DeepEqual(result.History, moves) || result.Fen != j.Source.Fen {
		t.Fatal("missing complete prefix metadata")
	}
	// The root is the seventh ply; the played continuation reaches a Go rule
	// draw. A conflicting native evaluation cannot be exposed as a reliable mate.
	j.NextPly = 7
	result, err = m.calculate(context.Background(), &j)
	if err != nil {
		t.Fatal(err)
	}
	if result.Actual == nil || result.Actual.Move != "c7b9" || len(result.Actual.PV) != 1 || result.Comparable {
		t.Fatalf("rule endpoint: %+v actual=%+v", result, result.Actual)
	}
	if result.Actual.Kind != "draw" && (result.Note != "rule_conflict" || result.Actual.Reliable || result.Actual.Kind != "unavailable") {
		t.Fatalf("conflict disguised: %+v", result)
	}
}

func TestValidatePVTruncatesRuleConflictAndRejectsIllegal(t *testing.T) {
	m := calculator(t)
	moves := []string{"b0c2", "b9c7", "c2b0", "c7b9", "b0c2", "b9c7", "c2b0"}
	j := calculationJob(t, m, moves, 7, "standard")
	if err := m.analysisHistory(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	err := m.pool.With(context.Background(), func(w *engine.Worker) error {
		line := Line{Score: 123, NativeScore: -123, Reliable: true, Kind: "evaluation", Move: "c7b9", PV: []string{"c7b9", "b0c2"}}
		conflict, err := m.validateAnalysisPV(context.Background(), w, j, moves, &line)
		if err != nil {
			return err
		}
		if !conflict || line.Reliable || line.Kind != "unavailable" || !reflect.DeepEqual(line.PV, []string{"c7b9"}) {
			t.Fatalf("rule conflict: %+v", line)
		}
		illegal := Line{PV: []string{"a0a9"}}
		if _, err := m.validateAnalysisPV(context.Background(), w, j, moves, &illegal); err == nil {
			t.Fatal("illegal pv accepted")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCalculateCancellationAndSubsequentWorker(t *testing.T) {
	m := calculator(t)
	j := calculationJob(t, m, nil, 0, "deep")
	// Warm both leases so the cancellation exercises search rather than process
	// startup, whose readiness errors are owned by the engine pool.
	j.Budget = ProfileBudget("standard")
	if _, err := m.calculate(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	j.Budget.Depth = 32
	j.Budget.Millis = 30000
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	_, err := m.calculate(ctx, &j)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancel: %v", err)
	}
	j.Budget = ProfileBudget("standard")
	result, err := m.calculate(context.Background(), &j)
	if err != nil || !result.Best.Reliable || result.Best.Depth != 6 {
		t.Fatalf("subsequent worker: %+v %v", result, err)
	}
}

func TestCalculateRefusesOldTeacherOnBothLeases(t *testing.T) {
	m := calculator(t)
	j := calculationJob(t, m, nil, 0, "standard")
	binary, err := filepath.Abs("../../../build/phase-6-analysis/before/xiangqi-engine")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(binary); err != nil {
		t.Skip("independently frozen teacher unavailable")
	}
	pool := engine.NewPool(binary, 1, 1)
	rules := engine.NewPool(binary, 1, 1)
	defer pool.Close()
	defer rules.Close()
	old := &Manager{pool: pool, rules: rules}
	if _, err := old.calculate(context.Background(), &j); err == nil {
		t.Fatal("old rule lease accepted as current teacher")
	}
	if err := m.analysisHistory(context.Background(), &j); err != nil {
		t.Fatal(err)
	}
	if _, err := old.calculate(context.Background(), &j); err == nil {
		t.Fatal("old search lease accepted as current teacher")
	}
}

func TestComparisonRequiresCommonTargetAndStableEvaluation(t *testing.T) {
	for _, tc := range []struct {
		name, turn, kind     string
		best, actual, depth  int
		reliable, comparable bool
		loss                 int
		note                 string
	}{
		{"red loss", "red", "evaluation", 100, -200, 6, true, true, 300, ""},
		{"black loss", "black", "evaluation", -100, 200, 6, true, true, 300, ""},
		{"negative loss", "red", "evaluation", 100, 120, 6, true, false, 0, "unstable"},
		{"lower common depth", "red", "evaluation", 100, -200, 4, true, false, 0, "incomplete"},
		{"fallback", "red", "unavailable", 0, 0, 0, false, false, 0, "incomplete"},
		{"mate score", "red", "mate", 9998, 9901, 6, true, false, 0, ""},
		{"draw score", "red", "draw", 0, 0, 6, true, false, 0, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := Line{Score: tc.actual, Depth: tc.depth, Reliable: tc.reliable, Kind: tc.kind}
			p := Position{Turn: tc.turn, Best: Line{Score: tc.best, Depth: tc.depth, Reliable: tc.reliable, Kind: tc.kind}, Actual: &actual}
			compareAnalysisLines(&p, 6)
			if p.Comparable != tc.comparable || p.Loss != tc.loss || p.Note != tc.note {
				t.Fatalf("comparison: %+v", p)
			}
		})
	}
}

func TestAnalysisSampleIdentityAndActiveLabels(t *testing.T) {
	j := Job{Engine: EngineVersion, Budget: ProfileBudget("standard"), Source: Source{Mode: "pve", ID: "source", Version: 1, Rules: chess.CasualRules, Phase: "playing", Outcome: "ongoing"}}
	initial := analysisSample(j, []string{"b0c2"})
	if initial.Final || initial.GroupID != "pve:source" || initial.Origin != "game" || initial.Outcome != "ongoing" {
		t.Fatal("active training labels", initial)
	}
	j.Source.Version++
	versioned := analysisSample(j, []string{"b0c2"})
	if versioned.SampleID == initial.SampleID || versioned.PositionID != initial.PositionID {
		t.Fatal("source version missing from sample identity")
	}
	j.Budget = ProfileBudget("deep")
	deep := analysisSample(j, []string{"b0c2"})
	if deep.SampleID == versioned.SampleID || deep.PositionID != initial.PositionID {
		t.Fatal("budget missing from sample identity")
	}
	j.Source.Rules = chess.LegacyRules
	legacy := analysisSample(j, []string{"b0c2"})
	if legacy.PositionID == initial.PositionID {
		t.Fatal("rule version missing from position identity")
	}
	history := analysisSample(j, []string{"h0g2"})
	if history.PositionID == legacy.PositionID {
		t.Fatal("full history missing from position identity")
	}
}
