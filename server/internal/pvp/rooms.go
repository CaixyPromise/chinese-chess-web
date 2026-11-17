package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"context"
)

func (m *Manager) Create(ctx context.Context, owner, human string, nicknames ...string) (View, error) {
	if human != "red" && human != "black" {
		return View{}, problem("INVALID_SIDE")
	}
	nickname := "房主"
	if len(nicknames) > 0 && nicknames[0] != "" {
		nickname = nicknames[0]
	}
	if !validNickname(nickname) {
		return View{}, problem("INVALID_NICKNAME")
	}
	p, e := m.position(ctx, nil, "")
	if e != nil {
		return View{}, e
	}
	r := Record{State: State{Rules: RulesVersion, ID: game.ID(), Fen: p.Fen, Moves: []string{}, Legal: p.Legal, Phase: "waiting", Outcome: "ongoing", RedMillis: InitialMillis, BlackMillis: InitialMillis}, Keys: []string{positionKey(chess.Initial)}, Checks: []bool{}, Host: owner, HostNickname: nickname, InviteExpiresAt: m.now() + InviteLifetime, Invite: game.ID(), Receipts: map[string]Receipt{}}
	if human == "red" {
		r.RedOwner = owner
		r.Red.Occupied = true
	} else {
		r.BlackOwner = owner
		r.Black.Occupied = true
	}
	g := &room{record: r, watchers: map[*watch]struct{}{}}
	m.admission.Lock()
	defer m.admission.Unlock()
	if e = m.checkAdmission(owner); e != nil {
		return View{}, e
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if e = m.commit(g, r); e != nil {
		return View{}, e
	}
	m.mu.Lock()
	m.rooms[r.ID] = g
	m.mu.Unlock()
	return m.view(g.record, owner, m.now()), nil
}
