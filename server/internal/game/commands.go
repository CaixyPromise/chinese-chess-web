package game

import (
	"chinesechess/internal/chess"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
)

func (m *Manager) Command(ctx context.Context, owner, id string, c Command) (Snapshot, error) {
	if len(c.RequestID) < 8 || len(c.RequestID) > 80 || strings.ContainsAny(c.RequestID, "\t\r\n") {
		return Snapshot{}, problem("INVALID_REQUEST")
	}
	if c.Type == "undo" {
		m.admission.Lock()
		defer m.admission.Unlock()
	}
	g, err := m.get(owner, id)
	if err != nil {
		return Snapshot{}, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	r := g.record
	payload, _ := json.Marshal(c)
	hash := fmt.Sprintf("%x", sha256.Sum256(payload))
	if receipt, ok := r.Receipts[c.RequestID]; ok {
		if receipt.Hash != hash {
			return Snapshot{}, problem("REQUEST_CONFLICT")
		}
		v := receipt.View
		if v.Rules == "" {
			v.Rules = chess.LegacyRules
		}
		return v, nil
	}
	if r.Version != c.Version {
		return Snapshot{}, problem("VERSION_CONFLICT")
	}
	if r.Phase == "finished" && c.Type != "undo" {
		return Snapshot{}, problem("GAME_FINISHED")
	}
	if !chess.SupportedRules(r.Rules) {
		return Snapshot{}, problem("UNSUPPORTED_RULES")
	}
	boundary := 0
	if r.Practice != nil {
		boundary = r.Practice.Ply
	}
	if c.Type == "undo" && len(r.Moves) <= boundary {
		return Snapshot{}, problem("NO_HISTORY")
	}
	if r.Phase == "finished" && c.Type == "undo" {
		if err := m.checkAdmission(owner, g); err != nil {
			return Snapshot{}, err
		}
	}
	if c.Type == "move" || c.Type == "undo" {
		if err := m.ruleHistory(ctx, &r); err != nil {
			return Snapshot{}, err
		}
	}
	switch c.Type {
	case "move":
		if r.Phase != "playing" || turn(r.Snapshot) != r.Human {
			return Snapshot{}, problem("NOT_YOUR_TURN")
		}
		if !ValidMove(c.Move) {
			return Snapshot{}, problem("ILLEGAL_MOVE")
		}
		legal := false
		for _, move := range r.Legal {
			if c.Move == move {
				legal = true
			}
		}
		if !legal {
			return Snapshot{}, problem("ILLEGAL_MOVE")
		}
		v, e := m.position(ctx, r.Moves, c.Move)
		if e != nil {
			return Snapshot{}, e
		}
		r = appendPosition(r, v, c.Move)
	case "undo":
		n := len(r.Moves) - 1
		for n > boundary {
			side := "red"
			if n%2 == 1 {
				side = "black"
			}
			if side == r.Human {
				break
			}
			n--
		}
		history := append([]string{}, r.Moves[:n]...)
		v, e := m.position(ctx, history, "")
		if e != nil {
			return Snapshot{}, e
		}
		r.Moves = history
		if r.Rules == chess.CasualRules {
			r.Keys = append([]string{}, r.Keys[:n+1]...)
			r.Checks = append([]bool{}, r.Checks[:n]...)
		}
		r = applyView(r, v)
	case "resign":
		r.Phase = "finished"
		r.Reason = "resign"
		if r.Human == "red" {
			r.Outcome = "black_win"
		} else {
			r.Outcome = "red_win"
		}
		r.Legal = []string{}
	case "retry":
		if r.Phase != "error" {
			return Snapshot{}, problem("NOT_RETRYABLE")
		}
		r.Phase = "queued"
		r.ErrorCode = ""
	default:
		return Snapshot{}, problem("INVALID_REQUEST")
	}
	r.Version++
	r.Generation++
	r.Receipts = cloneReceipts(r.Receipts)
	r.Order = append(append([]string{}, r.Order...), c.RequestID)
	r.Receipts[c.RequestID] = Receipt{hash, r.Snapshot}
	if len(r.Order) > 128 {
		delete(r.Receipts, r.Order[0])
		r.Order = r.Order[1:]
	}
	if err = m.commit(g, r); err != nil {
		return Snapshot{}, err
	}
	if g.cancel != nil {
		g.cancel()
		g.cancel = nil
	}
	if r.Phase == "queued" {
		m.schedule(g)
	}
	return g.record.Snapshot, nil
}
