package auth

import (
	"context"
	"errors"
	"time"

	"chinesechess/internal/game"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// VerifyRegister consumes only an email proof. It cannot create an account or
// claim the original requesting browser's data. The grant binds this browser's
// currently held guest session, independently of where the email was requested.
func (s *Service) VerifyRegister(ctx context.Context, token, guest string) (string, string, error) {
	out, err := s.VerifyRegistration(ctx, token, guest)
	return out.Grant, out.Email, err
}
func (s *Service) VerifyRegistration(ctx context.Context, token, guest string) (Registration, error) {
	if !validToken(token) {
		return Registration{}, problem("INVALID_TOKEN", 400)
	}
	available, err := s.GuestAvailable(ctx, guest)
	if err != nil {
		return Registration{}, err
	}
	if !available {
		return Registration{}, problem("GUEST_ALREADY_CLAIMED", 409)
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Registration{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return Registration{}, err
	}
	var email, requestGuest, returnPath string
	var expiry int64
	err = tx.QueryRow(ctx, `SELECT email,expires_at,guest_owner,return_path FROM auth_tokens WHERE token_hash=$1 AND purpose='register' AND used_at=0 FOR UPDATE`, Hash(token)).Scan(&email, &expiry, &requestGuest, &returnPath)
	if errors.Is(err, pgx.ErrNoRows) {
		return Registration{}, problem("INVALID_TOKEN", 400)
	}
	if err != nil {
		return Registration{}, err
	}
	if expiry <= now() {
		return Registration{}, problem("TOKEN_EXPIRED", 400)
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_accounts WHERE email=$1)`, email).Scan(&exists); err != nil {
		return Registration{}, err
	}
	if exists {
		return Registration{}, problem("INVALID_TOKEN", 400)
	}
	if requestGuest != guest {
		returnPath = ""
	}
	raw := randomToken()
	if _, err = tx.Exec(ctx, `UPDATE auth_tokens SET used_at=$2 WHERE token_hash=$1`, Hash(token), now()); err != nil {
		return Registration{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_registration_grants(token_hash,email,guest_owner,expires_at,return_path) VALUES($1,$2,$3,$4,$5)`, Hash(raw), email, guest, time.Now().Add(15*time.Minute).UnixMilli(), returnPath); err != nil {
		return Registration{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Registration{}, err
	}
	return Registration{Grant: raw, Email: email, ReturnTo: returnPath}, nil
}
func (s *Service) CompleteRegister(ctx context.Context, grant, guest, password, nickname string) (Session, error) {
	if !validToken(grant) {
		return Session{}, problem("INVALID_TOKEN", 400)
	}
	if err := validateNickname(nickname); err != nil {
		return Session{}, err
	}
	encoded, err := s.hashPassword(ctx, password)
	if err != nil {
		return Session{}, err
	}
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return Session{}, err
	}
	var email, boundGuest, returnPath string
	var expiry int64
	err = tx.QueryRow(ctx, `SELECT email,guest_owner,expires_at,return_path FROM auth_registration_grants WHERE token_hash=$1 AND used_at=0 FOR UPDATE`, Hash(grant)).Scan(&email, &boundGuest, &expiry, &returnPath)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, problem("INVALID_TOKEN", 400)
	}
	if err != nil {
		return Session{}, err
	}
	if expiry <= now() {
		return Session{}, problem("TOKEN_EXPIRED", 400)
	}
	if boundGuest != guest {
		return Session{}, problem("INVALID_TOKEN", 400)
	}
	id := game.ID()
	_, err = tx.Exec(ctx, `INSERT INTO auth_accounts(id,email,password_hash,nickname,created_at) VALUES($1,$2,$3,$4,$5)`, id, email, encoded, nickname, now())
	if err != nil {
		var pg *pgconn.PgError
		if errors.As(err, &pg) && pg.Code == "23505" {
			return Session{}, problem("INVALID_TOKEN", 400)
		}
		return Session{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_owner_links(owner_key,account_id,created_at) VALUES($1,$1,$2)`, id, now()); err != nil {
		return Session{}, err
	}
	revoked, err := s.claim(ctx, tx, id, guest)
	if err != nil {
		return Session{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_registration_grants SET used_at=$2 WHERE token_hash=$1`, Hash(grant), now()); err != nil {
		return Session{}, err
	}
	session, err := s.newSessionTx(ctx, tx, id)
	if err != nil {
		return Session{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	s.closeSessions(revoked)
	if s.reload != nil {
		if err = s.reload(ctx); err != nil {
			return Session{}, err
		}
	}
	session.ReturnTo = returnPath
	return session, nil
}

// HTTP serializes claims against all seat/admission mutations. The SQL checks
// also preserve atomic rollback of claims, including a concurrent second claim.
func (s *Service) claim(ctx context.Context, tx pgx.Tx, account, guest string) ([]string, error) {
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM auth_accounts WHERE id=$1 AND status='active')`, account).Scan(&active); err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrSession
	}
	if guest == "" || guest == account {
		return nil, nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return nil, err
	}
	var existing string
	err := tx.QueryRow(ctx, `SELECT account_id FROM auth_owner_links WHERE owner_key=$1`, guest).Scan(&existing)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if existing != "" {
		if existing != account {
			return nil, problem("GUEST_ALREADY_CLAIMED", 409)
		}
		return nil, nil
	}
	var conflict bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pvp_rooms r WHERE phase<>'finished' AND red_owner<>'' AND black_owner<>'' AND
 (CASE WHEN red_owner=$2 THEN $1 ELSE COALESCE((SELECT account_id FROM auth_owner_links WHERE owner_key=r.red_owner),red_owner) END)=$1 AND
 (CASE WHEN black_owner=$2 THEN $1 ELSE COALESCE((SELECT account_id FROM auth_owner_links WHERE owner_key=r.black_owner),black_owner) END)=$1)`, account, guest).Scan(&conflict)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, problem("MERGE_CONFLICT", 409)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO auth_owner_links(owner_key,account_id,created_at) VALUES($1,$2,$3)`, guest, account, now()); err != nil {
		return nil, err
	}
	return s.revokeOwnerTx(ctx, tx, guest)
}
