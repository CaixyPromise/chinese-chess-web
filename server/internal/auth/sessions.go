package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (s *Service) GuestAvailable(ctx context.Context, owner string) (bool, error) {
	if len(owner) != 32 {
		return false, nil
	}
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_owner_links WHERE owner_key=$1)`, owner).Scan(&exists)
	return !exists, err
}
func (s *Service) NewSession(ctx context.Context, owner string) (Session, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	out, err := s.newSessionTx(ctx, tx, owner)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return out, nil
}
func (s *Service) newSessionTx(ctx context.Context, tx pgx.Tx, owner string) (Session, error) {
	var blocked bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_accounts WHERE id=$1 AND status<>'active') OR EXISTS(SELECT 1 FROM auth_owner_links WHERE owner_key=$1 AND account_id<>$1)`, owner).Scan(&blocked); err != nil {
		return Session{}, err
	}
	if blocked {
		return Session{}, ErrSession
	}
	raw := randomToken()
	out := Session{Raw: raw, Hash: Hash(raw), Owner: owner, Expires: time.Now().Add(30 * 24 * time.Hour).UnixMilli()}
	_, err := tx.Exec(ctx, `INSERT INTO auth_sessions(token_hash,owner_key,created_at,last_seen,expires_at) VALUES($1,$2,$3,$3,$4)`, out.Hash, owner, now(), out.Expires)
	if err != nil {
		return Session{}, err
	}
	var user User
	err = tx.QueryRow(ctx, `SELECT id,email,nickname,role,status FROM auth_accounts WHERE id=$1 AND status='active'`, owner).Scan(&user.ID, &user.Email, &user.Nickname, &user.Role, &user.Status)
	if err == nil {
		out.User = &user
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, err
	}
	return out, nil
}
func (s *Service) Resolve(ctx context.Context, raw string) (Session, error) {
	if !validToken(raw) {
		return Session{}, ErrSession
	}
	out := Session{Raw: raw, Hash: Hash(raw)}
	var email, nick, id, role, status *string
	err := s.db.QueryRow(ctx, `UPDATE auth_sessions s SET last_seen=$2 FROM (SELECT 1) x
 WHERE s.token_hash=$1 AND revoked_at=0 AND expires_at>$2 AND last_seen>$3
 AND (EXISTS(SELECT 1 FROM auth_accounts a WHERE a.id=s.owner_key AND a.status='active') OR (NOT EXISTS(SELECT 1 FROM auth_accounts a WHERE a.id=s.owner_key) AND NOT EXISTS(SELECT 1 FROM auth_owner_links l WHERE l.owner_key=s.owner_key)))
 RETURNING s.owner_key,s.expires_at,(SELECT id FROM auth_accounts WHERE id=s.owner_key),(SELECT email FROM auth_accounts WHERE id=s.owner_key),(SELECT nickname FROM auth_accounts WHERE id=s.owner_key),(SELECT role FROM auth_accounts WHERE id=s.owner_key),(SELECT status FROM auth_accounts WHERE id=s.owner_key)`, out.Hash, now(), time.Now().Add(-7*24*time.Hour).UnixMilli()).Scan(&out.Owner, &out.Expires, &id, &email, &nick, &role, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSession
	}
	if err != nil {
		return Session{}, err
	}
	if id != nil {
		out.User = &User{ID: *id, Email: *email, Nickname: *nick, Role: *role, Status: *status}
	}
	return out, nil
}
func (s *Service) Revoke(ctx context.Context, hash string) error {
	_, err := s.db.Exec(ctx, `UPDATE auth_sessions SET revoked_at=$2 WHERE token_hash=$1 AND revoked_at=0`, hash, now())
	if err == nil {
		s.closeSessions([]string{hash})
	}
	return err
}
func (s *Service) revokeOwnerTx(ctx context.Context, tx pgx.Tx, owner string) ([]string, error) {
	rows, err := tx.Query(ctx, `UPDATE auth_sessions SET revoked_at=$2 WHERE owner_key=$1 AND revoked_at=0 RETURNING token_hash`, owner, now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hashes := []string{}
	for rows.Next() {
		var h string
		if err = rows.Scan(&h); err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}
	return hashes, rows.Err()
}

// SocketContext is called under the HTTP identity read gate. Identity-changing
// writes hold its write gate before revoking sessions and cancelling sockets.
func (s *Service) SocketContext(parent context.Context, hash string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	s.socketsMu.Lock()
	s.nextSocket++
	id := s.nextSocket
	if s.sockets[hash] == nil {
		s.sockets[hash] = map[uint64]context.CancelFunc{}
	}
	s.sockets[hash][id] = cancel
	s.socketsMu.Unlock()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				work, stop := context.WithTimeout(ctx, 2*time.Second)
				var valid bool
				err := s.db.QueryRow(work, `SELECT EXISTS(SELECT 1 FROM auth_sessions WHERE token_hash=$1 AND revoked_at=0 AND expires_at>$2 AND last_seen>$3)`, hash, now(), time.Now().Add(-7*24*time.Hour).UnixMilli()).Scan(&valid)
				stop()
				if err != nil || !valid {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, func() {
		cancel()
		s.socketsMu.Lock()
		delete(s.sockets[hash], id)
		if len(s.sockets[hash]) == 0 {
			delete(s.sockets, hash)
		}
		s.socketsMu.Unlock()
	}
}
func (s *Service) closeSessions(hashes []string) {
	s.socketsMu.Lock()
	defer s.socketsMu.Unlock()
	for _, hash := range hashes {
		for _, cancel := range s.sockets[hash] {
			cancel()
		}
		delete(s.sockets, hash)
	}
}
func (s *Service) CloseSocketSession(hash string) { s.closeSessions([]string{hash}) }

func (s *Service) DB() *pgxpool.Pool             { return s.db }
func (s *Service) CloseSessions(hashes []string) { s.closeSessions(hashes) }
