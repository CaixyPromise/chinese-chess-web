package game

import (
	"chinesechess/internal/engine"
	"chinesechess/internal/history"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
)

const Initial = "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR r - - 0 1"

var ErrNotFound = errors.New("not found")

type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

func problem(code string) error { return &Error{code} }

type Profile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Depth  int    `json:"-"`
	Millis int    `json:"-"`
	Nodes  int    `json:"-"`
}

// Resource tiers share the complete search; time is the user-facing budget boundary.
var Profiles = []Profile{{"easy", "轻松", 32, 250, 0}, {"normal", "标准", 32, 750, 0}, {"hard", "挑战", 32, 1500, 0}}

func profile(id string) (Profile, bool) {
	for _, p := range Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return Profile{}, false
}

func ID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func ValidID(s string) bool {
	if len(s) != 32 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

func ValidMove(s string) bool {
	return len(s) == 4 && s[0] >= 'a' && s[0] <= 'i' && s[1] >= '0' && s[1] <= '9' && s[2] >= 'a' && s[2] <= 'i' && s[3] >= '0' && s[3] <= '9' && s[:2] != s[2:]
}

type Snapshot struct {
	Practice   *Practice `json:"practice,omitempty"`
	Rules      string    `json:"rulesVersion"`
	Reason     string    `json:"reason,omitempty"`
	ID         string    `json:"id"`
	Version    int64     `json:"version"`
	Fen        string    `json:"fen"`
	Moves      []string  `json:"moves"`
	Legal      []string  `json:"legalMoves"`
	Human      string    `json:"human"`
	Difficulty string    `json:"difficulty"`
	Phase      string    `json:"phase"`
	Outcome    string    `json:"outcome"`
	InCheck    bool      `json:"inCheck"`
	ErrorCode  string    `json:"errorCode,omitempty"`
	Updated    int64     `json:"updatedAt"`
	LastThink  int       `json:"lastThinkMs"`
}

// Practice identifies the immutable source and the boundary of its copied prefix.
type Practice struct {
	SourceGroupID string `json:"sourceGroupId,omitempty"`
	SourceMode    string `json:"sourceMode"`
	SourceID      string `json:"sourceId"`
	SourceVersion int64  `json:"sourceVersion"`
	Ply           int    `json:"ply"`
}

type Receipt struct {
	Hash string
	View Snapshot
}

type Record struct {
	Snapshot
	Keys       []string
	Checks     []bool
	Owner      string
	Generation int64
	Receipts   map[string]Receipt
	Order      []string
}

type room struct {
	refs int // protected by Manager.mu; operations pin archived rooms until release

	mu       sync.Mutex
	record   Record
	watchers map[chan Snapshot]struct{}
	cancel   context.CancelFunc
}

type Command struct {
	RequestID string `json:"requestId"`
	Version   int64  `json:"expectedVersion"`
	Type      string `json:"type"`
	Move      string `json:"move,omitempty"`
}

type job struct {
	room       *room
	generation int64
	ctx        context.Context
}

type Manager struct {
	admission sync.Mutex // serializes operations that add active rooms

	mu          sync.RWMutex
	rooms       map[string]*room
	repo        Repository
	rules, bots *engine.Pool
	ctx         context.Context
	cancel      context.CancelFunc
	jobs        chan job
	workers     sync.WaitGroup
}

type Repository interface {
	Load(context.Context) ([]Record, error)
	Save(context.Context, Record, int64) error
	FindGame(context.Context, string, string) (Record, error)
	RecentGames(context.Context, string) ([]Record, error)
	History(context.Context, string, history.Query) (history.Page, error)
}

func cloneReceipts(src map[string]Receipt) map[string]Receipt {
	dst := make(map[string]Receipt, len(src)+1)
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
