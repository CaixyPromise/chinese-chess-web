package analysis

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const calculationVersion = "restricted-root-v1"

func sampleHash(value any) string {
	data, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

// Rule metadata is rebuilt in one native lease, preserving the complete frozen
// prefix and never replacing it with a board-only identity.
func (m *Manager) analysisHistory(ctx context.Context, j *Job) error {
	s := &j.Source
	if len(s.Keys) == len(s.Moves)+1 && len(s.Checks) == len(s.Moves) && s.Keys[0] == chess.PositionKey(chess.Initial) && (s.Fen == "" || s.Keys[len(s.Keys)-1] == chess.PositionKey(s.Fen)) {
		return nil
	}
	keys, checks := []string{chess.PositionKey(chess.Initial)}, []bool{}
	work, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := m.rules.With(work, func(w *engine.Worker) error {
		if err := verifyAnalysisTeacher(work, w, j.Engine); err != nil {
			return err
		}
		if _, err := w.Call(work, analysisPosition(nil)); err != nil {
			return err
		}
		lastFen := chess.Initial
		for _, move := range s.Moves {
			applied, err := w.Call(work, "apply\tmove="+move)
			if err != nil {
				return err
			}
			lastFen = applied["fen"]
			keys = append(keys, chess.PositionKey(lastFen))
			checks = append(checks, applied["in_check"] == "true")
		}
		if s.Fen != "" && chess.PositionKey(s.Fen) != chess.PositionKey(lastFen) {
			return errors.New("analysis frozen board differs from complete history")
		}
		return nil
	})
	if err == nil {
		s.Keys = keys
		s.Checks = checks
	}
	return err
}

func verifyAnalysisTeacher(ctx context.Context, w *engine.Worker, version string) error {
	hello, err := w.Call(ctx, "hello")
	if err != nil {
		return err
	}
	if version != EngineVersion || hello["engine_version"] != version || hello["root_move_ready"] != "true" {
		return errors.New("analysis teacher version or restricted-root capability mismatch")
	}
	return nil
}

func analysisSample(j Job, history []string) Position {
	ply := j.NextPly
	positionID := sampleHash(struct {
		Rules   string
		History []string
	}{j.Source.Rules, history})
	group := j.Source.GroupID
	if group == "" {
		group = j.Source.Mode + ":" + j.Source.ID
	}
	origin := j.Source.Origin
	if origin == "" {
		origin = "game"
	}
	result := Position{SchemaVersion: 1, AnalysisVersion: calculationVersion, PositionID: positionID, GroupID: group, SourceMode: j.Source.Mode, SourceID: j.Source.ID, SourceVersion: j.Source.Version,
		Origin: origin, History: history, Rules: j.Source.Rules, Teacher: EngineVersion, Budget: j.Budget, ScorePerspective: "red", ScoreUnits: "legacy-native",
		Outcome: j.Source.Outcome, EndReason: j.Source.Reason, Final: j.Source.Phase == "finished", Candidates: []Candidate{}, Ply: ply}
	result.SampleID = sampleHash(struct {
		PositionID, Mode, ID string
		Version              int64
		Engine               string
		Budget               Budget
		Algorithm            string
	}{positionID, j.Source.Mode, j.Source.ID, j.Source.Version, EngineVersion, j.Budget, calculationVersion})
	return result
}
