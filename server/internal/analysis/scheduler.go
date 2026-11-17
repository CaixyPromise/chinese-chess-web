package analysis

import (
	"context"
	"time"
)

func (m *Manager) work() {
	defer close(m.done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		jobs, err := m.repo.ReadyAnalyses(ctx, 8)
		cancel()
		if err != nil || len(jobs) == 0 {
			continue
		}
		j := jobs[0]
		for _, candidate := range jobs {
			if (candidate.Scope == "position") != (j.Scope == "position") {
				continue
			}
			if candidate.Owner != m.lastOwner {
				j = candidate
				break
			}
		}
		m.lastOwner = j.Owner
		m.run(j)
	}
}

// Event ingestion cannot wait behind a deep position search. It only persists
// jobs; the single search scheduler still controls all native analysis work.
func (m *Manager) materialize() {
	defer close(m.outboxDone)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
		}
		ctx, cancel := context.WithTimeout(m.ctx, 3*time.Second)
		_ = m.repo.MaterializeAnalyses(ctx, EngineVersion)
		cancel()
	}
}

func (m *Manager) run(j Job) {
	claim, cancelClaim := context.WithTimeout(m.ctx, 3*time.Second)
	j.State = "running"
	j.Updated = time.Now().UnixMilli()
	err := m.repo.SaveAnalysis(claim, j, j.Generation, "queued")
	cancelClaim()
	if err != nil {
		return
	}
	ctx, cancel := context.WithCancel(m.ctx)
	m.mu.Lock()
	m.activeID = j.ID
	m.activeOwner = j.Owner
	m.activeGeneration = j.Generation
	m.stop = cancel
	m.mu.Unlock()
	defer func() {
		cancel()
		m.mu.Lock()
		m.stop = nil
		m.activeID = ""
		m.activeOwner = ""
		m.mu.Unlock()
	}()
	current, err := m.repo.FindAnalysis(ctx, j.Owner, j.ID)
	if err != nil || current.Generation != j.Generation || current.State != "running" {
		return
	}
	go m.watchCommittedState(ctx, j, cancel)
	position, cached, err := m.repo.CachedAnalysisPosition(ctx, CacheKey(j), j.NextPly)
	if err == nil && !cached {
		position, err = m.calculate(ctx, &j)
	}
	if m.ctx.Err() != nil {
		return
	} // Startup recovery retains the committed checkpoint.
	write, stopWrite := context.WithTimeout(context.Background(), 3*time.Second)
	defer stopWrite()
	if err != nil {
		j.State = "failed"
		j.ErrorCode = "ANALYSIS_UNAVAILABLE"
		j.Updated = time.Now().UnixMilli()
		_ = m.repo.SaveAnalysis(write, j, j.Generation, "running")
		return
	}
	checkpoint := j
	j.Completed++
	j.NextPly++
	j.Updated = time.Now().UnixMilli()
	j.State = "queued"
	if j.Scope == "position" || j.NextPly > len(j.Source.Moves) {
		j.State = "completed"
	}
	if err = m.repo.CompletePosition(write, j, j.Generation, position, CacheKey(j)); err != nil {
		checkpoint.State = "queued"
		checkpoint.Updated = time.Now().UnixMilli()
		retry, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = m.repo.SaveAnalysis(retry, checkpoint, checkpoint.Generation, "running")
	}
}
