package admin

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (s *Service) RoleTarget(ctx context.Context, email string) (User, error) {
	var x User
	e := s.db.QueryRow(ctx, `SELECT id,email,nickname,role,status,version,created_at FROM auth_accounts WHERE email=$1`, strings.ToLower(strings.TrimSpace(email))).Scan(&x.ID, &x.Email, &x.Nickname, &x.Role, &x.Status, &x.Version, &x.Created)
	return x, notFound(e)
}

// SetRole is local-server administration only; it has no HTTP route.
func (s *Service) SetRole(ctx context.Context, targetID, role string, b Action) (Result, error) {
	out := Result{RequestID: b.RequestID}
	if (role != "user" && role != "admin") || !validAction(b) {
		return out, fail("INVALID_REQUEST", 400)
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return out, e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); e != nil {
		return out, e
	}
	var previous, status string
	var version int64
	e = tx.QueryRow(ctx, `SELECT role,status,version FROM auth_accounts WHERE id=$1 FOR UPDATE`, targetID).Scan(&previous, &status, &version)
	if e != nil {
		return out, notFound(e)
	}
	action := "role.grant"
	if role == "user" {
		action = "role.revoke"
	}
	var priorAction, priorTarget, priorReason string
	var priorResult, priorBefore []byte
	e = tx.QueryRow(ctx, `SELECT action,target_id,reason,result,before_state FROM admin_audit WHERE actor_id='local-cli' AND request_id=$1`, b.RequestID).Scan(&priorAction, &priorTarget, &priorReason, &priorResult, &priorBefore)
	if e == nil {
		if priorAction != action || priorTarget != targetID || priorReason != b.Reason {
			return out, fail("REQUEST_CONFLICT", 409)
		}
		if e = json.Unmarshal(priorResult, &out); e != nil {
			return out, e
		}
		out.Hashes, e = revokedHashes(ctx, tx, "user.revoke-sessions", targetID)
		return out, e
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	if version != b.Version {
		return out, fail("VERSION_CONFLICT", 409)
	}
	if status != "active" {
		return out, fail("ACCOUNT_INACTIVE", 409)
	}
	if previous == "admin" && role == "user" {
		var count int
		e = tx.QueryRow(ctx, `SELECT count(*) FROM auth_accounts WHERE role='admin' AND status='active'`).Scan(&count)
		if e != nil {
			return out, e
		}
		if count <= 1 {
			return out, fail("LAST_ADMIN", 409)
		}
	}
	out.Version = version + 1
	out.Status = role
	if _, e = tx.Exec(ctx, `UPDATE auth_accounts SET role=$2,version=version+1 WHERE id=$1`, targetID, role); e != nil {
		return out, e
	}
	if role == "user" {
		out.Hashes, e = revokeAccount(ctx, tx, targetID)
		if e != nil {
			return out, e
		}
	}
	if e = insertAudit(ctx, tx, "local-cli", action, targetID, b.Reason, b.RequestID, map[string]any{"role": previous, "version": version}, map[string]any{"role": role, "version": out.Version}, out); e != nil {
		return Result{}, e
	}
	return out, tx.Commit(ctx)
}
