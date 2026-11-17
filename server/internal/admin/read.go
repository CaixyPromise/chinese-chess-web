package admin

import (
	"chinesechess/internal/game"
	"context"
	"strings"
)

const userCols = `a.id,a.email,a.nickname,a.role,a.status,a.version,a.created_at,(SELECT count(*) FROM auth_sessions s WHERE (s.owner_key=a.id OR s.owner_key IN(SELECT owner_key FROM auth_owner_links WHERE account_id=a.id)) AND s.revoked_at=0 AND s.expires_at>(extract(epoch FROM clock_timestamp())*1000)::bigint AND s.last_seen>(extract(epoch FROM clock_timestamp())*1000)::bigint-604800000)`

func mask(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return "***"
	}
	return string([]rune(parts[0])[:1]) + "***@" + parts[1]
}
func (s *Service) Users(ctx context.Context, q Query) (Page[User], error) {
	out := Page[User]{Items: []User{}}
	t, id, limit, e := q.bounds()
	if e != nil {
		return out, e
	}
	rows, e := s.db.Query(ctx, `SELECT `+userCols+` FROM auth_accounts a WHERE ($1='' OR a.id ILIKE '%'||$1||'%' OR a.email ILIKE '%'||$1||'%' OR a.nickname ILIKE '%'||$1||'%') AND ($2='' OR a.status=$2) AND (a.created_at,a.id)<($3,$4) ORDER BY a.created_at DESC,a.id DESC LIMIT $5`, q.Q, q.Status, t, id, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x User
		if e = rows.Scan(&x.ID, &x.Email, &x.Nickname, &x.Role, &x.Status, &x.Version, &x.Created, &x.Sessions); e != nil {
			return out, e
		}
		x.Email = mask(x.Email)
		out.Items = append(out.Items, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[limit-1]
		out.Next = cursor(last.Created, last.ID)
	}
	return out, nil
}
func (s *Service) User(ctx context.Context, actor, id string) (User, error) {
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return User{}, e
	}
	defer tx.Rollback(ctx)
	var x User
	e = tx.QueryRow(ctx, `SELECT `+userCols+` FROM auth_accounts a WHERE a.id=$1`, id).Scan(&x.ID, &x.Email, &x.Nickname, &x.Role, &x.Status, &x.Version, &x.Created, &x.Sessions)
	if e != nil {
		return x, notFound(e)
	}
	e = insertAudit(ctx, tx, actor, "user.view", id, "查看用户详情", game.ID(), map[string]any{}, map[string]any{"viewed": true}, map[string]any{})
	if e != nil {
		return User{}, e
	}
	return x, tx.Commit(ctx)
}
func (s *Service) Sessions(ctx context.Context, q Query) (Page[Session], error) {
	out := Page[Session]{Items: []Session{}}
	t, id, limit, e := q.bounds()
	if e != nil {
		return out, e
	}
	rows, e := s.db.Query(ctx, `SELECT s.display_id,COALESCE(l.account_id,s.owner_key),s.created_at,s.last_seen,s.expires_at,s.revoked_at,s.version FROM auth_sessions s LEFT JOIN auth_owner_links l ON l.owner_key=s.owner_key WHERE ($1='' OR COALESCE(l.account_id,s.owner_key)=$1) AND (s.created_at,s.display_id)<($2,$3) ORDER BY s.created_at DESC,s.display_id DESC LIMIT $4`, q.UserID, t, id, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x Session
		if e = rows.Scan(&x.ID, &x.UserID, &x.Created, &x.LastSeen, &x.Expires, &x.Revoked, &x.Version); e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[limit-1]
		out.Next = cursor(last.Created, last.ID)
	}
	return out, nil
}

// Only selected operational fields are projected. Source, positions and samples
// are deliberately absent from this query and every administration response.
const taskCols = `j.id,COALESCE((SELECT account_id FROM auth_owner_links WHERE owner_key=j.owner_key),j.owner_key),j.state,j.generation,j.profile,j.engine_version,j.completed,j.total,j.created_at,j.updated_at,CASE WHEN j.body->>'errorCode' IN ('ADMIN_PAUSED','ENGINE_CHANGED','ANALYSIS_UNAVAILABLE') THEN j.body->>'errorCode' ELSE '' END,jsonb_build_object('depth',COALESCE((j.body#>>'{budget,depth}')::int,0),'timeMs',COALESCE((j.body#>>'{budget,timeMs}')::int,0),'hashMiB',COALESCE((j.body#>>'{budget,hashMiB}')::int,0),'nodes',COALESCE((j.body#>>'{budget,nodes}')::numeric,0))`

func (s *Service) Tasks(ctx context.Context, q Query) (Page[Task], error) {
	out := Page[Task]{Items: []Task{}}
	t, id, limit, e := q.bounds()
	if e != nil {
		return out, e
	}
	rows, e := s.db.Query(ctx, `SELECT `+taskCols+` FROM analysis_jobs j WHERE ($1='' OR j.id=$1 OR j.owner_key=$1 OR j.owner_key IN(SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND ($2='' OR j.state=$2) AND (j.created_at,j.id)<($3,$4) ORDER BY j.created_at DESC,j.id DESC LIMIT $5`, q.Q, q.Status, t, id, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x Task
		if e = rows.Scan(&x.ID, &x.UserID, &x.Status, &x.Version, &x.Profile, &x.Engine, &x.Completed, &x.Total, &x.Created, &x.Updated, &x.Error, &x.Budget); e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[limit-1]
		out.Next = cursor(last.Created, last.ID)
	}
	return out, nil
}
func (s *Service) Task(ctx context.Context, id string) (Task, error) {
	var x Task
	e := s.db.QueryRow(ctx, `SELECT `+taskCols+` FROM analysis_jobs j WHERE j.id=$1`, id).Scan(&x.ID, &x.UserID, &x.Status, &x.Version, &x.Profile, &x.Engine, &x.Completed, &x.Total, &x.Created, &x.Updated, &x.Error, &x.Budget)
	return x, notFound(e)
}

const auditCols = `id,actor_id,action,target_id,reason,request_id,created_at,before_state,after_state`

func (s *Service) Audits(ctx context.Context, q Query) (Page[Audit], error) {
	out := Page[Audit]{Items: []Audit{}}
	t, id, limit, e := q.bounds()
	if e != nil {
		return out, e
	}
	rows, e := s.db.Query(ctx, `SELECT `+auditCols+` FROM admin_audit WHERE ($1='' OR actor_id ILIKE '%'||$1||'%' OR target_id ILIKE '%'||$1||'%' OR action ILIKE '%'||$1||'%' OR request_id=$1 OR reason ILIKE '%'||$1||'%') AND(created_at,id)<($2,$3) ORDER BY created_at DESC,id DESC LIMIT $4`, q.Q, t, id, limit+1)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var x Audit
		if e = rows.Scan(&x.ID, &x.Actor, &x.Action, &x.Target, &x.Reason, &x.RequestID, &x.Created, &x.Before, &x.After); e != nil {
			return out, e
		}
		out.Items = append(out.Items, x)
	}
	if e = rows.Err(); e != nil {
		return out, e
	}
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
		last := out.Items[limit-1]
		out.Next = cursor(last.Created, last.ID)
	}
	return out, nil
}
func (s *Service) Audit(ctx context.Context, id string) (Audit, error) {
	var x Audit
	e := s.db.QueryRow(ctx, `SELECT `+auditCols+` FROM admin_audit WHERE id=$1`, id).Scan(&x.ID, &x.Actor, &x.Action, &x.Target, &x.Reason, &x.RequestID, &x.Created, &x.Before, &x.After)
	return x, notFound(e)
}
