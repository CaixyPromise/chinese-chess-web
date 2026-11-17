package admin

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"chinesechess/internal/auth"
	"github.com/jackc/pgx/v5"
)

func validAction(b Action) bool {
	if strings.TrimSpace(b.Reason) == "" || utf8.RuneCountInString(b.Reason) > 500 || len(b.RequestID) < 8 || len(b.RequestID) > 128 || b.Version < 1 {
		return false
	}
	for _, r := range b.RequestID {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_' {
			return false
		}
	}
	for _, r := range b.Reason {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}
func (s *Service) Act(ctx context.Context, actor auth.Session, action, target string, b Action) (Result, error) {
	out := Result{RequestID: b.RequestID}
	if !validAction(b) {
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
	var confirmed int64
	e = tx.QueryRow(ctx, `SELECT s.admin_confirmed_at FROM auth_accounts a JOIN auth_sessions s ON s.owner_key=a.id WHERE a.id=$1 AND a.role='admin' AND a.status='active' AND s.token_hash=$2 AND s.revoked_at=0 AND s.expires_at>$3 FOR UPDATE OF a,s`, actor.Owner, actor.Hash, stamp()).Scan(&confirmed)
	if errors.Is(e, pgx.ErrNoRows) {
		return out, fail("FORBIDDEN", 403)
	}
	if e != nil {
		return out, e
	}
	if confirmed < stamp()-600000 || confirmed > stamp() {
		return out, fail("REAUTH_REQUIRED", 428)
	}
	var oldAction, oldTarget, oldReason string
	var result, beforeRaw []byte
	e = tx.QueryRow(ctx, `SELECT action,target_id,reason,result,before_state FROM admin_audit WHERE actor_id=$1 AND request_id=$2`, actor.Owner, b.RequestID).Scan(&oldAction, &oldTarget, &oldReason, &result, &beforeRaw)
	if e == nil {
		var oldBefore struct {
			Version int64 `json:"version"`
		}
		_ = json.Unmarshal(beforeRaw, &oldBefore)
		if oldAction != action || oldTarget != target || oldReason != b.Reason || oldBefore.Version != b.Version {
			return out, fail("REQUEST_CONFLICT", 409)
		}
		if e = json.Unmarshal(result, &out); e != nil {
			return out, e
		}
		out.Hashes, e = revokedHashes(ctx, tx, action, target)
		return out, e
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return out, e
	}
	before := map[string]any{"version": b.Version}
	after := map[string]any{}
	switch action {
	case "user.ban", "user.unban", "user.revoke-sessions":
		var role, status string
		var version int64
		e = tx.QueryRow(ctx, `SELECT role,status,version FROM auth_accounts WHERE id=$1 FOR UPDATE`, target).Scan(&role, &status, &version)
		if e != nil {
			return out, notFound(e)
		}
		if action == "user.ban" && (role == "admin" || target == actor.Owner) {
			return out, fail("ADMIN_PROTECTED", 409)
		}
		if version != b.Version {
			return out, fail("VERSION_CONFLICT", 409)
		}
		before["status"] = status
		before["role"] = role
		if action == "user.ban" {
			status = "banned"
		}
		if action == "user.unban" {
			status = "active"
		}
		out.Status = status
		out.Version = version + 1
		if _, e = tx.Exec(ctx, `UPDATE auth_accounts SET status=$2,version=version+1 WHERE id=$1`, target, status); e != nil {
			return out, e
		}
		if action != "user.unban" {
			out.Hashes, e = revokeAccount(ctx, tx, target)
			if e != nil {
				return out, e
			}
		}
		if action == "user.ban" {
			if _, e = tx.Exec(ctx, `UPDATE auth_tokens SET used_at=$2 WHERE (account_id=$1 OR email=(SELECT email FROM auth_accounts WHERE id=$1) OR guest_owner IN(SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND used_at=0`, target, stamp()); e != nil {
				return out, e
			}
			if _, e = tx.Exec(ctx, `UPDATE auth_registration_grants SET used_at=$2 WHERE (email=(SELECT email FROM auth_accounts WHERE id=$1) OR guest_owner IN(SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND used_at=0`, target, stamp()); e != nil {
				return out, e
			}
			rows, err := tx.Query(ctx, `UPDATE analysis_jobs SET state='paused',generation=generation+1,updated_at=$2,body=body||jsonb_build_object('state','paused','generation',generation+1,'updatedAt',$2::bigint,'errorCode','ADMIN_PAUSED') WHERE (owner_key=$1 OR owner_key IN(SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND state IN('queued','running') RETURNING id`, target, stamp())
			if err != nil {
				return out, err
			}
			for rows.Next() {
				var id string
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return out, err
				}
				out.TaskIDs = append(out.TaskIDs, id)
			}
			e = rows.Err()
			rows.Close()
			if e != nil {
				return out, e
			}
		}
		after = map[string]any{"status": status, "version": out.Version, "pausedTasks": out.TaskIDs}
	case "session.revoke":
		var hash string
		var version, revoked int64
		e = tx.QueryRow(ctx, `SELECT token_hash,version,revoked_at FROM auth_sessions WHERE display_id=$1 FOR UPDATE`, target).Scan(&hash, &version, &revoked)
		if e != nil {
			return out, notFound(e)
		}
		if version != b.Version {
			return out, fail("VERSION_CONFLICT", 409)
		}
		before["revoked"] = revoked != 0
		out.Version = version + 1
		out.Status = "revoked"
		out.Hashes = []string{hash}
		if _, e = tx.Exec(ctx, `UPDATE auth_sessions SET revoked_at=CASE WHEN revoked_at=0 THEN $2 ELSE revoked_at END,version=version+1,admin_confirmed_at=0 WHERE display_id=$1`, target, stamp()); e != nil {
			return out, e
		}
		after = map[string]any{"revoked": true, "version": out.Version}
	case "task.pause":
		var state string
		var generation int64
		e = tx.QueryRow(ctx, `SELECT state,generation FROM analysis_jobs WHERE id=$1 FOR UPDATE`, target).Scan(&state, &generation)
		if e != nil {
			return out, notFound(e)
		}
		before["status"] = state
		if generation != b.Version && state != "completed" {
			return out, fail("VERSION_CONFLICT", 409)
		}
		out.Status = state
		out.Version = generation
		if state == "queued" || state == "running" {
			out.Status = "paused"
			out.Version++
			out.TaskIDs = []string{target}
			_, e = tx.Exec(ctx, `UPDATE analysis_jobs SET state='paused',generation=generation+1,updated_at=$2,body=body||jsonb_build_object('state','paused','generation',generation+1,'updatedAt',$2::bigint,'errorCode','ADMIN_PAUSED') WHERE id=$1`, target, stamp())
			if e != nil {
				return out, e
			}
		}
		after = map[string]any{"status": out.Status, "version": out.Version}
	default:
		return out, fail("INVALID_REQUEST", 400)
	}
	if e = insertAudit(ctx, tx, actor.Owner, action, target, b.Reason, b.RequestID, before, after, out); e != nil {
		return Result{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return Result{}, e
	}
	return out, nil
}
func revokeAccount(ctx context.Context, tx pgx.Tx, target string) ([]string, error) {
	rows, e := tx.Query(ctx, `UPDATE auth_sessions SET revoked_at=$2,version=version+1,admin_confirmed_at=0 WHERE (owner_key=$1 OR owner_key IN(SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND revoked_at=0 RETURNING token_hash`, target, stamp())
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var h string
		if e = rows.Scan(&h); e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
func revokedHashes(ctx context.Context, tx pgx.Tx, action, target string) ([]string, error) {
	query := `SELECT token_hash FROM auth_sessions WHERE revoked_at<>0 AND (owner_key=$1 OR owner_key IN(SELECT owner_key FROM auth_owner_links WHERE account_id=$1))`
	if action == "session.revoke" {
		query = `SELECT token_hash FROM auth_sessions WHERE display_id=$1 AND revoked_at<>0`
	}
	if action != "session.revoke" && action != "user.ban" && action != "user.revoke-sessions" {
		return nil, nil
	}
	rows, e := tx.Query(ctx, query, target)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var h string
		if e = rows.Scan(&h); e != nil {
			return nil, e
		}
		out = append(out, h)
	}
	return out, rows.Err()
}
