// Package pvp owns invitation rooms, two-player commands, clocks and consent.
package pvp

import (
	"chinesechess/internal/chess"
	"chinesechess/internal/engine"
	"chinesechess/internal/game"
	"context"
	"sync"
)

const InitialMillis int64 = 600000
const IncrementMillis int64 = 5000

type Seat struct {
	Occupied bool `json:"occupied"`
	Ready    bool `json:"ready"`
}
type Proposal struct {
	Kind string `json:"kind"`
	By   string `json:"by"`
}
type State struct {
	Rules       string    `json:"rulesVersion"`
	ID          string    `json:"id"`
	Version     int64     `json:"version"`
	Fen         string    `json:"fen"`
	Moves       []string  `json:"moves"`
	Legal       []string  `json:"legalMoves"`
	Phase       string    `json:"phase"`
	Outcome     string    `json:"outcome"`
	Reason      string    `json:"reason,omitempty"`
	InCheck     bool      `json:"inCheck"`
	Red         Seat      `json:"red"`
	Black       Seat      `json:"black"`
	RedMillis   int64     `json:"redMillis"`
	BlackMillis int64     `json:"blackMillis"`
	TurnStarted int64     `json:"turnStartedAt"`
	Updated     int64     `json:"updatedAt"`
	Proposal    *Proposal `json:"proposal,omitempty"`
}
type View struct {
	State
	You                 string           `json:"you"`
	Invite              string           `json:"inviteCode,omitempty"`
	InvitationStatus    string           `json:"invitationStatus,omitempty"`
	InviteExpiresAt     int64            `json:"inviteExpiresAt,omitempty"`
	ServerNow           int64            `json:"serverNow"`
	HostNickname        string           `json:"hostNickname"`
	ParticipantNickname string           `json:"participantNickname,omitempty"`
	PendingApplication  *ApplicationView `json:"pendingApplication,omitempty"`
	ViewChoices         []string         `json:"viewChoices,omitempty"`
}
type Receipt struct {
	Hash  string
	State State
}
type Record struct {
	Keys   []string
	Checks []bool
	State
	Host                string
	RedOwner            string
	BlackOwner          string
	Invite              string
	Receipts            map[string]Receipt
	Order               []string
	HostNickname        string
	ParticipantNickname string
	InviteExpiresAt     int64
	InviteRevoked       bool
	InviteExpired       bool
	PreviousInvites     map[string]string
	Applications        map[string]Application
	CurrentApplication  string
}
type Command struct {
	RequestID     string `json:"requestId"`
	Version       int64  `json:"expectedVersion"`
	Type          string `json:"type"`
	Move          string `json:"move,omitempty"`
	ApplicationID string `json:"applicationId,omitempty"`
}
type Repository interface {
	LoadPVP(context.Context) ([]Record, error)
	SavePVP(context.Context, Record, int64) error
	FindPVP(context.Context, string, string) (Record, error)
	RecentPVP(context.Context, string) ([]Record, error)
}
type watch struct {
	owner string
	side  string
	ch    chan View
}
type room struct {
	refs int // protected by Manager.mu

	mu       sync.Mutex
	record   Record
	watchers map[*watch]struct{}
}
type Manager struct {
	admission           sync.Mutex
	applicationLimitsMu sync.Mutex
	applicationLimits   map[string][]int64

	position func(context.Context, []string, string) (chess.Position, error)
	mu       sync.RWMutex
	rooms    map[string]*room
	repo     Repository
	rules    *engine.Pool
	now      func() int64
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

func problem(code string) error { return &game.Error{Code: code} }
func turn(s State) string {
	if len(s.Moves)%2 == 0 {
		return "red"
	}
	return "black"
}
func applyPosition(r *Record, p chess.Position) {
	r.Fen = p.Fen
	r.Legal = p.Legal
	r.InCheck = p.InCheck
	r.Outcome = p.Outcome
	if p.Outcome != "ongoing" {
		r.Phase = "finished"
		r.Reason = "board"
		r.TurnStarted = 0
		r.Proposal = nil
	}
}
