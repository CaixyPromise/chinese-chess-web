package auth

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"time"
)

// ConfirmAdmin binds the confirmation to this exact live browser session.
func (s *Service) ConfirmAdmin(ctx context.Context, session Session, password, ip string) (int64, error) {
	if !s.limits.allow("adminreauth:"+session.Owner+":"+ip, 5, 15*time.Minute, 0) {
		return 0, problem("RATE_LIMITED", 429)
	}
	var encoded string
	err := s.db.QueryRow(ctx, `SELECT a.password_hash FROM auth_accounts a JOIN auth_sessions s ON s.owner_key=a.id WHERE a.id=$1 AND a.role='admin' AND a.status='active' AND s.token_hash=$2 AND s.revoked_at=0 AND s.expires_at>$3`, session.Owner, session.Hash, now()).Scan(&encoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, problem("FORBIDDEN", 403)
	}
	if err != nil {
		return 0, err
	}
	ok, err := s.checkPassword(ctx, password, encoded)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, problem("INVALID_CREDENTIALS", 401)
	}
	stamp := now()
	tag, err := s.db.Exec(ctx, `UPDATE auth_sessions SET admin_confirmed_at=$3 WHERE token_hash=$1 AND owner_key=$2 AND revoked_at=0 AND expires_at>$3 AND EXISTS(SELECT 1 FROM auth_accounts WHERE id=$2 AND role='admin' AND status='active' AND password_hash=$4)`, session.Hash, session.Owner, stamp, encoded)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() != 1 {
		return 0, problem("FORBIDDEN", 403)
	}
	return stamp + int64((10*time.Minute)/time.Millisecond), nil
}
