package analysis

import (
	"context"
	"time"
)

// AdminStopCommitted interrupts work only after its persisted generation has
// changed. The persisted-state watcher also reconciles a missed notification.
func (m *Manager) AdminStopCommitted(ctx context.Context, ids []string) error {
	m.mu.Lock()
	id, owner, generation := m.activeID, m.activeOwner, m.activeGeneration
	m.mu.Unlock()
	for _, candidate := range ids {
		if candidate != id {
			continue
		}
		// An idempotent replay may arrive after the user resumed this job. Check
		// the committed generation before touching the current native lease.
		current, err := m.repo.FindAnalysis(ctx, owner, id)
		if err != nil {
			return err
		}
		if current.Generation == generation && current.State == "running" {
			return nil
		}
		m.mu.Lock()
		if m.activeID == id && m.activeGeneration == generation && m.stop != nil {
			m.stop()
		}
		m.mu.Unlock()
		break
	}
	return ctx.Err()
}

func (m *Manager) watchCommittedState(ctx context.Context, j Job, stop context.CancelFunc) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check, cancel := context.WithTimeout(ctx, time.Second)
			current, err := m.repo.FindAnalysis(check, j.Owner, j.ID)
			cancel()
			if err == nil && (current.Generation != j.Generation || current.State != "running") {
				stop()
				return
			}
		}
	}
}
