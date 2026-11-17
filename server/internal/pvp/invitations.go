package pvp

import (
	"chinesechess/internal/game"
	"strings"
	"unicode"
	"unicode/utf8"
)

const InviteLifetime int64 = 24 * 60 * 60 * 1000
const ApplicationLifetime int64 = 5 * 60 * 1000

type Application struct {
	ID         string
	Owner      string
	Nickname   string
	RequestID  string
	InviteCode string
	State      string
	ExpiresAt  int64
}
type ApplicationView struct {
	ApplicationID string `json:"applicationId,omitempty"`
	State         string `json:"state"`
	RoomID        string `json:"roomId"`
	Version       int64  `json:"version"`
	ExpiresAt     int64  `json:"expiresAt"`
	Nickname      string `json:"nickname,omitempty"`
	You           string `json:"you,omitempty"`
}
type InvitationPreview struct {
	RoomID          string           `json:"roomId"`
	HostNickname    string           `json:"hostNickname"`
	HostSide        string           `json:"hostSide"`
	GuestSide       string           `json:"guestSide"`
	RulesVersion    string           `json:"rulesVersion"`
	InitialMillis   int64            `json:"initialMillis"`
	IncrementMillis int64            `json:"incrementMillis"`
	Status          string           `json:"status"`
	ExpiresAt       int64            `json:"expiresAt"`
	Application     *ApplicationView `json:"application,omitempty"`
}

func validNickname(s string) bool {
	if strings.TrimSpace(s) == "" || !utf8.ValidString(s) || utf8.RuneCountInString(s) > 40 {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validRequest(s string) bool {
	return len(s) >= 8 && len(s) <= 80 && !strings.ContainsAny(s, "\t\r\n")
}
func applicationView(r Record, a Application) ApplicationView {
	return ApplicationView{ApplicationID: a.ID, State: a.State, RoomID: r.ID, Version: r.Version, ExpiresAt: a.ExpiresAt, Nickname: a.Nickname}
}
func invalidateApplication(r *Record, state string) {
	if a, ok := r.Applications[r.CurrentApplication]; ok && a.State == "pending" {
		a.State = state
		setApplication(r, a)
	}
	r.CurrentApplication = ""
}

// Copy on write keeps a failed CAS from publishing application mutations.
func setApplication(r *Record, a Application) {
	next := make(map[string]Application, len(r.Applications)+1)
	for k, v := range r.Applications {
		next[k] = v
	}
	next[a.ID] = a
	r.Applications = next
}
func (m *Manager) invitationStatus(r Record, code string) string {
	if code != r.Invite {
		return "revoked"
	}
	if r.Reason == "cancelled" {
		return "cancelled"
	}
	if r.Phase != "waiting" {
		return "started"
	}
	if r.InviteRevoked {
		return "revoked"
	}
	if r.InviteExpired || r.InviteExpiresAt != 0 && m.now() >= r.InviteExpiresAt {
		return "expired"
	}
	if r.Red.Occupied && r.Black.Occupied {
		return "full"
	}
	return "available"
}

// A legacy invitation is given its lifetime once, durably, at first use.
func (m *Manager) prepareInvitation(g *room) error {
	r := g.record
	changed := false
	if r.InviteExpiresAt == 0 && r.Phase == "waiting" {
		r.InviteExpiresAt = m.now() + InviteLifetime
		changed = true
	}
	if r.Phase == "waiting" && r.InviteExpiresAt != 0 && m.now() >= r.InviteExpiresAt && !r.InviteExpired {
		r.InviteExpired = true
		changed = true
	}
	if a, ok := r.Applications[r.CurrentApplication]; ok && a.State == "pending" && (m.now() >= a.ExpiresAt || (r.InviteExpiresAt != 0 && m.now() >= r.InviteExpiresAt)) {
		a.State = "expired"
		setApplication(&r, a)
		r.CurrentApplication = ""
		changed = true
	}
	if changed {
		return m.commit(g, r)
	}
	return nil
}
func (m *Manager) Preview(owner, code string) (InvitationPreview, error) {
	g, err := m.findInvitation(code)
	if err != nil {
		return InvitationPreview{}, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := m.prepareInvitation(g); err != nil {
		return InvitationPreview{}, err
	}
	r := g.record
	hostSide := "red"
	guestSide := "black"
	if r.Host == r.BlackOwner {
		hostSide, guestSide = "black", "red"
	}
	v := InvitationPreview{RoomID: r.ID, HostNickname: hostNickname(r), HostSide: hostSide, GuestSide: guestSide, RulesVersion: r.Rules, InitialMillis: InitialMillis, IncrementMillis: IncrementMillis, Status: m.invitationStatus(r, code), ExpiresAt: r.InviteExpiresAt}
	for _, a := range r.Applications {
		if a.InviteCode == code && m.sameOwner(a.Owner, owner) {
			av := applicationView(r, a)
			if v.Application == nil || av.ExpiresAt > v.Application.ExpiresAt {
				v.Application = &av
			}
		}
	}
	return v, nil
}
func (m *Manager) ApplyJoin(owner, code, nickname, requestID string) (ApplicationView, error) {
	return m.ApplyJoinFrom(owner, code, nickname, requestID, "")
}

func (m *Manager) ApplyJoinFrom(owner, code, nickname, requestID, ip string) (ApplicationView, error) {
	if !game.ValidID(owner) || !validRequest(requestID) {
		return ApplicationView{}, problem("INVALID_REQUEST")
	}
	if !validNickname(nickname) {
		return ApplicationView{}, problem("INVALID_NICKNAME")
	}
	g, err := m.findInvitation(code)
	if err != nil {
		return ApplicationView{}, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := m.prepareInvitation(g); err != nil {
		return ApplicationView{}, err
	}
	r := g.record
	if you := m.side(r, owner); you != "" {
		return ApplicationView{RoomID: r.ID, Version: r.Version, State: "approved", You: you}, nil
	}
	for _, a := range r.Applications {
		if m.sameOwner(a.Owner, owner) && a.RequestID == requestID {
			if a.InviteCode != code || a.Nickname != nickname {
				return ApplicationView{}, problem("REQUEST_CONFLICT")
			}
			return applicationView(r, a), nil
		}
	}
	if status := m.invitationStatus(r, code); status != "available" {
		return ApplicationView{}, problem("INVITATION_" + strings.ToUpper(status))
	}
	if a, ok := r.Applications[r.CurrentApplication]; ok && a.State == "pending" {
		if m.sameOwner(a.Owner, owner) {
			return applicationView(r, a), nil
		}
		return ApplicationView{}, problem("APPLICATION_PENDING")
	}
	if !m.allowApplication(owner, ip) {
		return ApplicationView{}, problem("RATE_LIMITED")
	}
	// Keep a bounded per-room retry history; pending and approved entries survive.
	if len(r.Applications) >= 128 {
		next := map[string]Application{}
		for k, a := range r.Applications {
			if a.State == "approved" || a.State == "pending" {
				next[k] = a
			}
		}
		r.Applications = next
	}
	a := Application{ID: game.ID(), Owner: owner, Nickname: nickname, RequestID: requestID, InviteCode: code, State: "pending", ExpiresAt: m.now() + ApplicationLifetime}
	setApplication(&r, a)
	r.CurrentApplication = a.ID
	if err := m.commit(g, r); err != nil {
		return ApplicationView{}, err
	}
	return applicationView(g.record, a), nil
}
func (m *Manager) WithdrawApplication(owner, code, id, requestID string) (ApplicationView, error) {
	if !validRequest(requestID) {
		return ApplicationView{}, problem("INVALID_REQUEST")
	}
	g, err := m.findInvitation(code)
	if err != nil {
		return ApplicationView{}, err
	}
	defer m.release(g)
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := m.prepareInvitation(g); err != nil {
		return ApplicationView{}, err
	}
	r := g.record
	a, ok := r.Applications[id]
	if !ok || !m.sameOwner(a.Owner, owner) || a.InviteCode != code {
		return ApplicationView{}, game.ErrNotFound
	}
	if a.State == "withdrawn" {
		return applicationView(r, a), nil
	}
	if a.State != "pending" {
		return ApplicationView{}, problem("APPLICATION_" + strings.ToUpper(a.State))
	}
	a.State = "withdrawn"
	setApplication(&r, a)
	r.CurrentApplication = ""
	if err := m.commit(g, r); err != nil {
		return ApplicationView{}, err
	}
	return applicationView(g.record, a), nil
}
func (m *Manager) invitationCommand(g *room, r *Record, owner string, c Command) error {
	if !m.sameOwner(owner, r.Host) {
		return problem("NOT_ALLOWED")
	}
	if r.Phase != "waiting" {
		return problem("ALREADY_STARTED")
	}
	switch c.Type {
	case "revoke_invite", "rotate_invite":
		invalidateApplication(r, "revoked")
		next := map[string]string{}
		if len(r.PreviousInvites) < 32 {
			for k, v := range r.PreviousInvites {
				next[k] = v
			}
		}
		next[r.Invite] = "revoked"
		r.PreviousInvites = next
		r.InviteRevoked = true
		if c.Type == "rotate_invite" {
			r.Invite = game.ID()
			r.InviteExpiresAt = m.now() + InviteLifetime
			r.InviteRevoked = false
			r.InviteExpired = false
		}
	case "approve_join", "reject_join":
		a, ok := r.Applications[c.ApplicationID]
		if !ok || a.ID != r.CurrentApplication || a.State != "pending" {
			return problem("APPLICATION_NOT_PENDING")
		}
		if m.now() >= a.ExpiresAt {
			a.State = "expired"
			setApplication(r, a)
			r.CurrentApplication = ""
			if err := m.commit(g, *r); err != nil {
				return err
			}
			return problem("APPLICATION_EXPIRED")
		}
		if c.Type == "reject_join" {
			a.State = "rejected"
			setApplication(r, a)
			r.CurrentApplication = ""
			return nil
		}
		if m.invitationStatus(*r, a.InviteCode) != "available" {
			return problem("INVITATION_UNAVAILABLE")
		}
		if m.sameOwner(a.Owner, r.Host) || m.side(*r, a.Owner) != "" {
			return problem("SAME_ACCOUNT_SEATS")
		}
		active := 0
		for _, other := range m.roomList() {
			if other == g {
				continue
			}
			other.mu.Lock()
			if other.record.Phase != "finished" && m.side(other.record, a.Owner) != "" {
				active++
			}
			other.mu.Unlock()
		}
		if active >= 8 {
			return problem("ACTIVE_GAME_LIMIT")
		}
		if !r.Red.Occupied {
			r.RedOwner = a.Owner
			r.Red = Seat{Occupied: true}
		} else {
			r.BlackOwner = a.Owner
			r.Black = Seat{Occupied: true}
		}
		r.ParticipantNickname = a.Nickname
		a.State = "approved"
		setApplication(r, a)
		r.CurrentApplication = ""
		r.Proposal = nil
	}
	return nil
}
