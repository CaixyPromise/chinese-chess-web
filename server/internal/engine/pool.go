// Package engine manages bounded, isolated native C++ workers.
package engine

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
)

var ErrBusy = errors.New("engine queue is full")

type Error struct{ Code string }

func (e *Error) Error() string { return "native engine: " + e.Code }

type Worker struct {
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     *bufio.Scanner
	id      uint64
	writeMu sync.Mutex
}

func (w *Worker) close() {
	if w.cmd != nil {
		_ = w.in.Close()
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
		w.cmd = nil
	}
}

type Pool struct {
	binary    string
	idle      chan *Worker
	admission chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	closed    bool
	active    sync.WaitGroup
}

func NewPool(binary string, parallel, waiting int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{binary: binary, idle: make(chan *Worker, parallel), admission: make(chan struct{}, parallel+waiting), ctx: ctx, cancel: cancel}
	for i := 0; i < parallel; i++ {
		p.idle <- &Worker{}
	}
	return p
}

func (p *Pool) With(ctx context.Context, fn func(*Worker) error) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return context.Canceled
	}
	p.active.Add(1)
	p.mu.Unlock()
	defer p.active.Done()
	ctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(p.ctx, cancel)
	defer func() { stop(); cancel() }()
	select {
	case p.admission <- struct{}{}:
	default:
		return ErrBusy
	}
	defer func() { <-p.admission }()
	var w *Worker
	select {
	case w = <-p.idle:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { p.idle <- w }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if w.cmd == nil {
		cmd := exec.Command(p.binary)
		in, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		out, err := cmd.StdoutPipe()
		if err != nil {
			_ = in.Close()
			return err
		}
		cmd.Stderr = io.Discard
		if err = cmd.Start(); err != nil {
			_ = in.Close()
			_ = out.Close()
			return err
		}
		w.cmd, w.in, w.out = cmd, in, bufio.NewScanner(out)
		w.out.Buffer(make([]byte, 8192), 1<<20)
		hello, err := w.Call(ctx, "hello")
		if err != nil || hello["rules_ready"] != "true" || hello["search_ready"] != "true" {
			w.close()
			return errors.New("native engine is not ready")
		}
	}
	err := fn(w)
	if err != nil {
		w.close()
	}
	return err
}

func (p *Pool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.cancel()
	p.mu.Unlock()
	p.active.Wait()
	for i := 0; i < cap(p.idle); i++ {
		(<-p.idle).close()
	}
}
