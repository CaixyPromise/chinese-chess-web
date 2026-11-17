package game

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

func (m *Manager) schedule(g *room) {
	ctx, cancel := context.WithCancel(m.ctx)
	g.cancel = cancel
	select {
	case m.jobs <- job{g, g.record.Generation, ctx}:
	default:
		cancel()
		g.cancel = nil
		r := g.record
		r.Phase = "error"
		r.ErrorCode = "BOT_BUSY"
		r.Version++
		if m.commit(g, r) != nil {
			m.transientError(g)
		}
	}
}

func (m *Manager) work() {
	defer m.workers.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case task := <-m.jobs:
			m.compute(task)
		}
	}
}

func (m *Manager) compute(task job) {
	g := task.room
	defer m.prune(g)
	g.mu.Lock()
	if g.record.Generation != task.generation || g.record.Phase != "queued" || task.ctx.Err() != nil || !chess.SupportedRules(g.record.Rules) {
		g.mu.Unlock()
		return
	}
	r := g.record
	r.Phase = "thinking"
	r.Version++
	if m.commit(g, r) != nil {
		m.transientError(g)
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()
	ctx, cancel := context.WithTimeout(task.ctx, 8*time.Second)
	defer cancel()
	p, _ := profile(r.Difficulty)
	var best string
	var elapsed int
	err := m.bots.With(ctx, func(w *engine.Worker) error {
		if _, e := w.Call(ctx, "position\tfen="+Initial+"\trules=legacy-java-v1\tmoves="+strings.Join(r.Moves, ",")); e != nil {
			return e
		}
		searchID := ID()
		fields, e := w.Call(ctx, fmt.Sprintf("search\tgame_id=%s\tposition_version=%d\tsearch_id=%s\tdepth=%d\ttime_ms=%d\tnodes=%d\thash_mib=128", r.ID, r.Generation, searchID, p.Depth, p.Millis, p.Nodes))
		if e != nil {
			return e
		}
		if fields["game_id"] != r.ID || fields["position_version"] != strconv.FormatInt(r.Generation, 10) || fields["search_id"] != searchID {
			return errors.New("stale native result")
		}
		best = fields["move"]
		elapsed, _ = strconv.Atoi(fields["elapsed_ms"])
		if !ValidMove(best) {
			return errors.New("native search returned no legal move")
		}
		return nil
	})
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.record.Generation != task.generation || g.record.Phase != "thinking" || task.ctx.Err() != nil {
		return
	}
	r = g.record
	if err == nil {
		err = m.ruleHistory(ctx, &r)
	}
	if err == nil {
		var v Snapshot
		v, err = m.position(ctx, r.Moves, best)
		if err == nil {
			r = appendPosition(r, v, best)
			r.LastThink = elapsed
		}
	}
	if err != nil {
		r.Phase = "error"
		r.ErrorCode = "BOT_UNAVAILABLE"
	}
	r.Version++
	r.Generation++
	if m.commit(g, r) != nil {
		m.transientError(g)
	}
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
}

// Keep the last committed board/version when storage fails; allow a safe retry.
func (m *Manager) transientError(g *room) {
	g.record.Phase = "error"
	g.record.ErrorCode = "SAVE_UNAVAILABLE"
	for ch := range g.watchers {
		select {
		case ch <- g.record.Snapshot:
		default:
		}
	}
}
