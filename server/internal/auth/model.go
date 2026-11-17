// Package auth manages revocable browser sessions and purpose-bound email proofs.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Error struct {
	Code   string
	Status int
}

func (e *Error) Error() string              { return e.Code }
func problem(code string, status int) error { return &Error{code, status} }

var ErrSession = errors.New("invalid session")

type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Nickname string `json:"nickname"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}
type Session struct {
	Hash, Raw, Owner string
	ReturnTo         string
	User             *User
	Expires          int64
}

func (s Session) CSRF() string { return Hash("csrf:" + s.Raw) }

type Config struct{ PublicURL, SMTPAddr, SMTPFrom, SMTPUser, SMTPPassword, TrustedProxyIP string }
type Service struct {
	db         *pgxpool.Pool
	config     Config
	reload     func(context.Context) error
	hashSlots  chan struct{}
	limits     *limiter
	socketsMu  sync.Mutex
	sockets    map[string]map[uint64]context.CancelFunc
	nextSocket uint64
}

func New(db *pgxpool.Pool, c Config, reload func(context.Context) error) (*Service, error) {
	if c.TrustedProxyIP != "" && net.ParseIP(c.TrustedProxyIP) == nil {
		return nil, errors.New("invalid trusted proxy IP")
	}
	u, err := url.Parse(c.PublicURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("invalid AUTH_PUBLIC_URL")
	}
	if _, err := NormalizeEmail(c.SMTPFrom); err != nil {
		return nil, errors.New("invalid SMTP_FROM")
	}
	if c.SMTPAddr == "" || c.SMTPFrom == "" {
		return nil, errors.New("SMTP_ADDR and SMTP_FROM required")
	}
	return &Service{db: db, config: c, reload: reload, hashSlots: make(chan struct{}, 2), limits: newLimiter(), sockets: map[string]map[uint64]context.CancelFunc{}}, nil
}
func randomToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}
func Hash(raw string) string { s := sha256.Sum256([]byte(raw)); return hex.EncodeToString(s[:]) }
func now() int64             { return time.Now().UnixMilli() }
func validToken(raw string) bool {
	b, e := base64.RawURLEncoding.DecodeString(raw)
	return e == nil && len(b) == 32
}
func (s *Service) PublicOrigin() string {
	u, _ := url.Parse(s.config.PublicURL)
	return u.Scheme + "://" + u.Host
}
