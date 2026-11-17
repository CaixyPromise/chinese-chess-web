package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

func (m *Manager) Command(ctx context.Context, owner, id string, c Command) (View, error) {
	if len(c.RequestID) < 8 || len(c.RequestID) > 80 || strings.ContainsAny(c.RequestID, "\t\r\n") {
		return View{}, problem("INVALID_REQUEST")
	}
	if c.Type == "approve_join" {
		m.admission.Lock()
		defer m.admission.Unlock()
	}
	g, e := m.get(owner, id)
	if e != nil {
		return View{}, e
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.record
	if m.side(r, owner) == "" {
		return View{}, game.ErrNotFound
	}
	b, _ := json.Marshal(c)
	hash := fmt.Sprintf("%x", sha256.Sum256(b))
	receiptOwner := owner
	if m.side(r, owner) == "red" {
		receiptOwner = r.RedOwner
	} else if m.side(r, owner) == "black" {
		receiptOwner = r.BlackOwner
	}
	key := receiptOwner + ":" + c.RequestID
	if receipt, ok := r.Receipts[key]; ok {
		if receipt.Hash != hash {
			return View{}, problem("REQUEST_CONFLICT")
		}
		v := m.view(r, owner, m.now())
		v.State = receipt.State
		if v.Rules == "" {
			v.Rules = RulesVersion
		}
		return v, nil
	}
	if e = m.expire(g); e != nil {
		return View{}, e
	}
	r = g.record
	if c.Version != r.Version {
		return View{}, problem("VERSION_CONFLICT")
	}
	if r.Phase == "finished" {
		return View{}, problem("GAME_FINISHED")
	}
	if r.Rules != "" && !chess.SupportedRules(r.Rules) {
		return View{}, problem("UNSUPPORTED_RULES")
	}
	actor := m.side(r, owner)
	settle(&r, m.now())
	if flag(&r) {
		if err := m.commit(g, r); err != nil {
			return View{}, err
		}
		return m.view(g.record, owner, m.now()), nil
	}
	switch c.Type {
	case "approve_join", "reject_join", "revoke_invite", "rotate_invite":
		if e = m.invitationCommand(g, &r, owner, c); e != nil {
			return View{}, e
		}
	case "ready", "unready":
		if r.Phase != "waiting" {
			return View{}, problem("ALREADY_STARTED")
		}
		if actor == "red" {
			r.Red.Ready = c.Type == "ready"
		} else {
			r.Black.Ready = c.Type == "ready"
		}
		if r.Red.Occupied && r.Black.Occupied && r.Red.Ready && r.Black.Ready {
			r.Phase = "playing"
			r.TurnStarted = m.now()
		}
	case "move":
		if r.Phase != "playing" {
			return View{}, problem("NOT_STARTED")
		}
		if actor != turn(r.State) {
			return View{}, problem("NOT_YOUR_TURN")
		}
		if !game.ValidMove(c.Move) {
			return View{}, problem("ILLEGAL_MOVE")
		}
		allowed := false
		for _, move := range r.Legal {
			if move == c.Move {
				allowed = true
			}
		}
		if !allowed {
			return View{}, problem("ILLEGAL_MOVE")
		}
		if err := ruleHistory(ctx, m.rules, &r); err != nil {
			return View{}, err
		}
		p, err := m.position(ctx, r.Moves, c.Move)
		if err != nil {
			return View{}, err
		}
		settle(&r, m.now())
		if flag(&r) {
			if err = m.commit(g, r); err != nil {
				return View{}, err
			}
			return m.view(g.record, owner, m.now()), nil
		}
		r.Moves = append(append([]string{}, r.Moves...), c.Move)
		if actor == "red" {
			r.RedMillis += IncrementMillis
		} else {
			r.BlackMillis += IncrementMillis
		}
		r.Proposal = nil
		applyPosition(&r, p)
		r.Keys = append(append([]string{}, r.Keys...), positionKey(p.Fen))
		r.Checks = append(append([]bool{}, r.Checks...), p.InCheck)
		adjudicate(&r)
	case "resign":
		if r.Phase != "playing" {
			return View{}, problem("NOT_STARTED")
		}
		r.Phase = "finished"
		r.Reason = "resign"
		r.Outcome = "red_win"
		if actor == "red" {
			r.Outcome = "black_win"
		}
		r.TurnStarted = 0
		r.Proposal = nil
		r.Legal = []string{}
	case "draw", "undo":
		if r.Phase != "playing" {
			return View{}, problem("NOT_STARTED")
		}
		if r.Proposal != nil {
			return View{}, problem("PROPOSAL_PENDING")
		}
		if c.Type == "undo" && !canUndo(r.State, actor) {
			return View{}, problem("NO_HISTORY")
		}
		r.Proposal = &Proposal{Kind: c.Type, By: actor}
	case "accept", "decline", "withdraw":
		if r.Phase != "playing" || r.Proposal == nil {
			return View{}, problem("NO_PROPOSAL")
		}
		own := r.Proposal.By == actor
		if c.Type == "withdraw" && !own || c.Type != "withdraw" && own {
			return View{}, problem("NOT_ALLOWED")
		}
		if c.Type == "accept" {
			if r.Proposal.Kind == "draw" {
				r.Phase = "finished"
				r.Outcome = "draw"
				r.Reason = "agreement"
				r.TurnStarted = 0
				r.Legal = []string{}
			} else {
				if err := ruleHistory(ctx, m.rules, &r); err != nil {
					return View{}, err
				}
				n := undoPly(r.State, r.Proposal.By)
				history := append([]string{}, r.Moves[:n]...)
				p, err := m.position(ctx, history, "")
				if err != nil {
					return View{}, err
				}
				settle(&r, m.now())
				if flag(&r) {
					if err = m.commit(g, r); err != nil {
						return View{}, err
					}
					return m.view(g.record, owner, m.now()), nil
				}
				r.Moves = history
				if len(r.Keys) > n && len(r.Checks) >= n {
					r.Keys = append([]string{}, r.Keys[:n+1]...)
					r.Checks = append([]bool{}, r.Checks[:n]...)
				}
				applyPosition(&r, p)
			}
		}
		r.Proposal = nil
	case "leave":
		if r.Phase != "waiting" {
			return View{}, problem("ALREADY_STARTED")
		}
		if m.sameOwner(owner, r.Host) {
			r.Phase = "finished"
			r.Outcome = "draw"
			r.Reason = "cancelled"
			r.InviteRevoked = true
			invalidateApplication(&r, "cancelled")
			r.Legal = []string{}
		} else {
			for _, a := range r.Applications {
				if a.State == "approved" && m.sameOwner(a.Owner, owner) {
					a.State = "left"
					setApplication(&r, a)
				}
			}
			r.ParticipantNickname = ""
			if actor == "red" {
				r.Red = Seat{}
				r.RedOwner = ""
				r.Black.Ready = false
			} else {
				r.Black = Seat{}
				r.BlackOwner = ""
				r.Red.Ready = false
			}
		}
	default:
		return View{}, problem("INVALID_REQUEST")
	}
	next := make(map[string]Receipt, len(r.Receipts)+1)
	for k, v := range r.Receipts {
		next[k] = v
	}
	r.Receipts = next
	r.Order = append(append([]string{}, r.Order...), key)
	receiptState := r.State
	receiptState.Version++
	receiptState.Updated = m.now()
	r.Receipts[key] = Receipt{hash, receiptState}
	if len(r.Order) > 128 {
		delete(r.Receipts, r.Order[0])
		r.Order = r.Order[1:]
	}
	if e = m.commit(g, r); e != nil {
		return View{}, e
	}
	return m.view(g.record, owner, m.now()), nil
}
func undoPly(s State, actor string) int {
	n := len(s.Moves) - 1
	for n >= 0 {
		if (n%2 == 0 && actor == "red") || (n%2 == 1 && actor == "black") {
			return n
		}
		n--
	}
	return -1
}
func canUndo(s State, actor string) bool { return undoPly(s, actor) >= 0 }
