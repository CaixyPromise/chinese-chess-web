package analysis

import (
	"chinesechess/internal/game"
	"context"
	"errors"
	"time"
)

func (m *Manager) Control(ctx context.Context, owner, id, action string) (Job, error) {
	if action != "cancel" && action != "resume" {
		return Job{}, problem("INVALID_REQUEST")
	}
	for attempt := 0; attempt < 3; attempt++ {
		j, err := m.repo.FindAnalysis(ctx, owner, id)
		if err != nil {
			return Job{}, err
		}
		if action == "cancel" && (j.State == "completed" || j.State == "paused") {
			return j, nil
		}
		if action == "resume" && (j.State == "queued" || j.State == "running" || j.State == "completed") {
			return j, nil
		}
		if action == "resume" {
			if _, err = m.load(ctx, owner, j.Source.Mode, j.Source.ID); err != nil {
				return Job{}, err
			}
			if j.Engine != EngineVersion {
				return m.createFrozen(ctx, j.Owner, j.Source, j.Scope, j.Profile, j.Ply)
			}
		}
		previous, gen := j.State, j.Generation
		j.Generation++
		j.Updated = time.Now().UnixMilli()
		j.ErrorCode = ""
		j.State = "paused"
		if action == "resume" {
			j.State = "queued"
		}
		if err = m.repo.SaveAnalysis(ctx, j, gen, previous); err != nil {
			var conflict *game.Error
			if errors.As(err, &conflict) && conflict.Code == "VERSION_CONFLICT" {
				continue
			}
			return Job{}, err
		}
		if action == "cancel" {
			m.mu.Lock()
			if m.activeID == j.ID && m.activeGeneration == gen && m.stop != nil {
				m.stop()
			}
			m.mu.Unlock()
		}
		return j, nil
	}
	return Job{}, problem("VERSION_CONFLICT")
}
