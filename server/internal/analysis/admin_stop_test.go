package analysis

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type committedStateRepo struct {
	Repository
	reads atomic.Int32
}

func (r *committedStateRepo) FindAnalysis(context.Context, string, string) (Job, error) {
	if r.reads.Add(1) == 1 {
		return Job{}, errors.New("temporary read failure")
	}
	return Job{ID: "task", Generation: 2, State: "paused"}, nil
}
func TestCommittedPauseReconcilesMissedNotificationAfterReadFailure(t *testing.T) {
	repo := &committedStateRepo{}
	m := &Manager{repo: repo}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.watchCommittedState(ctx, Job{ID: "task", Generation: 1, State: "running"}, func() { close(stopped); cancel() })
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("committed pause failed to cancel after transient read failure")
	}
	<-done
	if repo.reads.Load() < 2 {
		t.Fatal("read failure was treated as a pause")
	}
}

type fixedStateRepo struct {
	Repository
	job Job
}

func (r fixedStateRepo) FindAnalysis(context.Context, string, string) (Job, error) { return r.job, nil }
func TestReplayPauseNotificationDoesNotCancelResumedGeneration(t *testing.T) {
	for _, tc := range []struct {
		name      string
		persisted Job
		wantStop  bool
	}{
		{"resumed current generation", Job{State: "running", Generation: 3}, false},
		{"committed new pause", Job{State: "paused", Generation: 4}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stopped := false
			m := &Manager{repo: fixedStateRepo{job: tc.persisted}, activeID: "task", activeOwner: "owner", activeGeneration: 3, stop: func() { stopped = true }}
			if err := m.AdminStopCommitted(context.Background(), []string{"task"}); err != nil {
				t.Fatal(err)
			}
			if stopped != tc.wantStop {
				t.Fatalf("stop=%v want=%v", stopped, tc.wantStop)
			}
		})
	}
}
