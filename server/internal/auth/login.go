package auth

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *Service) Login(ctx context.Context, email, password, guest, ip string, saveGuest bool) (Session, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return Session{}, problem("INVALID_CREDENTIALS", 401)
	}
	if !s.limits.allow("loginip:"+ip, 30, 15*time.Minute, 0) || !s.limits.loginAllowed(email) {
		return Session{}, problem("RATE_LIMITED", 429)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return Session{}, err
	}
	var id, encoded, status string
	err = tx.QueryRow(ctx, `SELECT id,password_hash,status FROM auth_accounts WHERE email=$1 FOR UPDATE`, email).Scan(&id, &encoded, &status)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, err
	}
	ok, err := s.checkPassword(ctx, password, encoded)
	if err != nil {
		return Session{}, err
	}
	s.limits.loginResult(email, ok)
	if !ok || status != "active" {
		return Session{}, problem("INVALID_CREDENTIALS", 401)
	}
	var hashes []string
	if saveGuest && guest != "" {
		hashes, err = s.claim(ctx, tx, id, guest)
		if err != nil {
			return Session{}, err
		}
	}
	session, err := s.newSessionTx(ctx, tx, id)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	s.closeSessions(hashes)
	if s.reload != nil {
		if err = s.reload(ctx); err != nil {
			return Session{}, err
		}
	}
	return session, nil
}

func (s *Service) ChangePassword(ctx context.Context, account, current, password string) error {
	encoded, err := s.hashPassword(ctx, password)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return err
	}
	var old string
	if err = tx.QueryRow(ctx, `SELECT password_hash FROM auth_accounts WHERE id=$1 FOR UPDATE`, account).Scan(&old); err != nil {
		return err
	}
	ok, err := s.checkPassword(ctx, current, old)
	if err != nil {
		return err
	}
	if !ok {
		return problem("INVALID_CREDENTIALS", 401)
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_accounts SET password_hash=$2 WHERE id=$1`, account, encoded); err != nil {
		return err
	}
	hashes, err := s.revokeOwnerTx(ctx, tx, account)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET used_at=$2 WHERE account_id=$1 AND used_at=0`, account, now()); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.closeSessions(hashes)
	return nil
}
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if !validToken(token) {
		return problem("INVALID_TOKEN", 400)
	}
	encoded, err := s.hashPassword(ctx, password)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return err
	}
	var account string
	var expiry int64
	// Lock the account before all its reset tokens to prevent lock-order inversion
	// when two different outstanding tokens are consumed at once.
	err = tx.QueryRow(ctx, `SELECT account_id FROM auth_tokens WHERE token_hash=$1 AND purpose='reset'`, Hash(token)).Scan(&account)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem("INVALID_TOKEN", 400)
	}
	if err != nil {
		return err
	}
	var id string
	if err = tx.QueryRow(ctx, `SELECT id FROM auth_accounts WHERE id=$1 AND status='active' FOR UPDATE`, account).Scan(&id); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return problem("INVALID_TOKEN", 400)
		}
		return err
	}
	err = tx.QueryRow(ctx, `SELECT expires_at FROM auth_tokens WHERE token_hash=$1 AND purpose='reset' AND used_at=0 FOR UPDATE`, Hash(token)).Scan(&expiry)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem("INVALID_TOKEN", 400)
	}
	if err != nil {
		return err
	}
	if expiry <= now() {
		return problem("TOKEN_EXPIRED", 400)
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_accounts SET password_hash=$2 WHERE id=$1`, account, encoded); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET used_at=$2 WHERE account_id=$1 AND purpose='reset' AND used_at=0`, account, now()); err != nil {
		return err
	}
	hashes, err := s.revokeOwnerTx(ctx, tx, account)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	s.closeSessions(hashes)
	return nil
}
