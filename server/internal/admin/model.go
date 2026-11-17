// Package admin exposes restricted account metadata and audited state changes.
package admin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"chinesechess/internal/auth"
	"chinesechess/internal/game"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Service      { return &Service{db: db} }
func fail(code string, status int) error { return &auth.Error{Code: code, Status: status} }
func stamp() int64                       { return time.Now().UnixMilli() }

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   string `json:"status"`
	Version  int64  `json:"version"`
	Created  int64  `json:"createdAt"`
	Sessions int64  `json:"sessionCount"`
}
type Session struct {
	ID       string `json:"id"`
	UserID   string `json:"userId"`
	Created  int64  `json:"createdAt"`
	LastSeen int64  `json:"lastSeen"`
	Expires  int64  `json:"expiresAt"`
	Revoked  int64  `json:"revokedAt"`
	Version  int64  `json:"version"`
}
type Task struct {
	ID        string          `json:"id"`
	UserID    string          `json:"userId"`
	Status    string          `json:"status"`
	Version   int64           `json:"version"`
	Profile   string          `json:"profile"`
	Engine    string          `json:"engineVersion"`
	Completed int             `json:"completed"`
	Total     int             `json:"total"`
	Created   int64           `json:"createdAt"`
	Updated   int64           `json:"updatedAt"`
	Error     string          `json:"errorCode"`
	Budget    json.RawMessage `json:"budget"`
}
type Audit struct {
	ID        string          `json:"id"`
	Actor     string          `json:"actorId"`
	Action    string          `json:"action"`
	Target    string          `json:"targetId"`
	Reason    string          `json:"reason"`
	RequestID string          `json:"requestId"`
	Created   int64           `json:"createdAt"`
	Before    json.RawMessage `json:"before"`
	After     json.RawMessage `json:"after"`
}
type Page[T any] struct {
	Items []T    `json:"items"`
	Next  string `json:"nextCursor"`
}
type Query struct {
	Q, Status, UserID, Cursor string
	Limit                     int
}
type Action struct {
	Reason    string `json:"reason"`
	RequestID string `json:"requestId"`
	Version   int64  `json:"version"`
}
type Result struct {
	RequestID string   `json:"requestId"`
	Version   int64    `json:"version"`
	Status    string   `json:"status"`
	TaskIDs   []string `json:"taskIds,omitempty"`
	Hashes    []string `json:"-"`
}

func (q Query) bounds() (int64, string, int, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	if len(q.Q) > 254 || len(q.Cursor) > 512 {
		return 0, "", 0, fail("INVALID_REQUEST", 400)
	}
	if q.Cursor == "" {
		return 9223372036854775807, "~", limit, nil
	}
	b, e := base64.RawURLEncoding.DecodeString(q.Cursor)
	if e != nil {
		return 0, "", 0, fail("INVALID_REQUEST", 400)
	}
	parts := strings.SplitN(string(b), "|", 2)
	if len(parts) != 2 {
		return 0, "", 0, fail("INVALID_REQUEST", 400)
	}
	n, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil {
		return 0, "", 0, fail("INVALID_REQUEST", 400)
	}
	return n, parts[1], limit, nil
}
func cursor(t int64, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(t, 10) + "|" + id))
}
func (s *Service) Allowed(ctx context.Context, owner, hash string) error {
	var ok bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_accounts a JOIN auth_sessions s ON s.owner_key=a.id WHERE a.id=$1 AND a.role='admin' AND a.status='active' AND s.token_hash=$2 AND s.revoked_at=0 AND s.expires_at>$3)`, owner, hash, stamp()).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return fail("FORBIDDEN", 403)
	}
	return nil
}
func (s *Service) Security(ctx context.Context, actor, code string) {
	_, _ = s.db.Exec(ctx, `INSERT INTO admin_security_events(actor_id,code,created_at) VALUES($1,$2,$3)`, actor, code, stamp())
}
func insertAudit(ctx context.Context, tx pgx.Tx, actor, action, target, reason, request string, before, after, result any) error {
	b, e := json.Marshal(before)
	if e != nil {
		return e
	}
	a, e := json.Marshal(after)
	if e != nil {
		return e
	}
	r, e := json.Marshal(result)
	if e != nil {
		return e
	}
	_, e = tx.Exec(ctx, `INSERT INTO admin_audit(id,actor_id,action,target_id,reason,request_id,created_at,before_state,after_state,result) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, game.ID(), actor, action, target, reason, request, stamp(), b, a, r)
	return e
}
func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return fail("NOT_FOUND", 404)
	}
	return err
}
