package pvp

import "time"

func settle(r *Record, now int64) {
	if r.Phase != "playing" || r.TurnStarted == 0 {
		return
	}
	elapsed := max(int64(0), now-r.TurnStarted)
	if turn(r.State) == "red" {
		r.RedMillis = max(int64(0), r.RedMillis-elapsed)
	} else {
		r.BlackMillis = max(int64(0), r.BlackMillis-elapsed)
	}
	r.TurnStarted = now
}
func flag(r *Record) bool {
	if r.Phase != "playing" {
		return false
	}
	if r.RedMillis > 0 && r.BlackMillis > 0 {
		return false
	}
	r.Outcome = "red_win"
	if r.RedMillis == 0 {
		r.Outcome = "black_win"
	}
	r.Phase = "finished"
	r.Reason = "timeout"
	r.TurnStarted = 0
	r.Proposal = nil
	r.Legal = []string{}
	return true
}
func (m *Manager) expire(g *room) error {
	if g.record.InviteExpiresAt != 0 {
		if err := m.prepareInvitation(g); err != nil {
			return err
		}
	}
	r := g.record
	settle(&r, m.now())
	if flag(&r) {
		return m.commit(g, r)
	}
	return nil
}
func (m *Manager) tick() {
	defer close(m.done)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			list := make([]*room, 0, len(m.rooms))
			for _, g := range m.rooms {
				list = append(list, g)
			}
			m.mu.RUnlock()
			for _, g := range list {
				g.mu.Lock()
				_ = m.expire(g)
				g.mu.Unlock()
				m.prune(g)
			}
		}
	}
}
