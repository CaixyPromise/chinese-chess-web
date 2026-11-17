package engine

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestCancellationReplacesProcessAndPoolIsBounded(t *testing.T) {
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	p := NewPool(binary, 1, 0)
	defer p.Close()
	entered := make(chan struct{})
	done := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		done <- p.With(ctx, func(w *Worker) error {
			if _, e := w.Call(ctx, "position\tfen=rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1\trules=legacy-java-v1"); e != nil {
				return e
			}
			close(entered)
			_, e := w.Call(ctx, "search\tgame_id=test\tposition_version=1\tsearch_id=cancel\tdepth=30\ttime_ms=30000\tnodes=100000000\thash_mib=16")
			return e
		})
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not start")
	}
	if e := p.With(context.Background(), func(*Worker) error { return nil }); !errors.Is(e, ErrBusy) {
		t.Fatal("unbounded pool", e)
	}
	cancel()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation did not reap worker")
	}
	if e := p.With(context.Background(), func(w *Worker) error {
		h, e := w.Call(context.Background(), "hello")
		if h["search_ready"] != "true" {
			t.Fatal(h)
		}
		return e
	}); e != nil {
		t.Fatal("replacement", e)
	}
	p.Close()
	if e := p.With(context.Background(), func(*Worker) error { return nil }); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

func TestCrashedWorkerIsReplaced(t *testing.T) {
	binary, _ := filepath.Abs("../../../build/engine/xiangqi-engine")
	p := NewPool(binary, 1, 0)
	defer p.Close()
	e := p.With(context.Background(), func(w *Worker) error {
		_ = w.cmd.Process.Kill()
		_, err := w.Call(context.Background(), "ping")
		return err
	})
	if e == nil {
		t.Fatal("crash was not reported")
	}
	if e = p.With(context.Background(), func(w *Worker) error { _, e := w.Call(context.Background(), "ping"); return e }); e != nil {
		t.Fatal("crashed worker was not replaced", e)
	}
}
