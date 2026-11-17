package store

import (
	"context"
	"encoding/json"
	"errors"

	"chinesechess/internal/game"
	"chinesechess/internal/history"
	"chinesechess/internal/pvp"
	"github.com/jackc/pgx/v5"
)

func decodeGame(data []byte) (game.Record, error) {
	var r game.Record
	err := json.Unmarshal(data, &r)
	if r.Rules == "" {
		r.Rules = "legacy-java-v1"
	}
	return r, err
}
func decodePVP(data []byte) (pvp.Record, error) {
	var r pvp.Record
	err := json.Unmarshal(data, &r)
	if r.Rules == "" {
		r.Rules = "casual-xiangqi-v1"
	}
	return r, err
}

func (p *Postgres) FindGame(ctx context.Context, owner, id string) (game.Record, error) {
	var data []byte
	err := p.pool.QueryRow(ctx, `SELECT body FROM games WHERE id=$1 AND `+ownedBy2+` AND $2<>''`, id, owner).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return game.Record{}, game.ErrNotFound
	}
	if err != nil {
		return game.Record{}, err
	}
	return decodeGame(data)
}
func (p *Postgres) RecentGames(ctx context.Context, owner string) ([]game.Record, error) {
	rows, err := p.pool.Query(ctx, `SELECT body FROM games WHERE `+ownedBy1+` AND $1<>'' ORDER BY updated_at DESC,id DESC LIMIT 8`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []game.Record{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		r, err := decodeGame(data)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (p *Postgres) FindPVP(ctx context.Context, owner, id string) (pvp.Record, error) {
	var data []byte
	err := p.pool.QueryRow(ctx, `SELECT body FROM pvp_rooms WHERE id=$1 AND (red_owner=$2 OR black_owner=$2 OR red_owner IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$2) OR black_owner IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$2)) AND $2<>''`, id, owner).Scan(&data)
	if errors.Is(err, pgx.ErrNoRows) {
		return pvp.Record{}, game.ErrNotFound
	}
	if err != nil {
		return pvp.Record{}, err
	}
	return decodePVP(data)
}
func (p *Postgres) RecentPVP(ctx context.Context, owner string) ([]pvp.Record, error) {
	rows, err := p.pool.Query(ctx, `SELECT body FROM pvp_rooms WHERE (red_owner=$1 OR black_owner=$1 OR red_owner IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$1) OR black_owner IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND $1<>'' ORDER BY updated_at DESC,id DESC LIMIT 8`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []pvp.Record{}
	for rows.Next() {
		var data []byte
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		r, err := decodePVP(data)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// History projects summaries in PostgreSQL so large moves and receipt bodies never
// travel to the application for listing. Authorization applies to both UNION arms.
func (p *Postgres) History(ctx context.Context, owner string, q history.Query) (history.Page, error) {
	page := history.Page{Items: []history.Entry{}}
	if err := q.Validate(); err != nil {
		return page, err
	}
	var updated int64
	var id, mode, cursorSide string
	if q.Before != nil {
		updated, id, mode, cursorSide = q.Before.UpdatedAt, q.Before.ID, q.Before.Mode, q.Before.Side
	}
	rows, err := p.pool.Query(ctx, `SELECT id,mode,side,rules_version,outcome,reason,move_count,updated_at,practice FROM (
 SELECT id,'pve'::text AS mode,COALESCE(body->>'human','') AS side,
 COALESCE(NULLIF(body->>'rulesVersion',''),'legacy-java-v1') AS rules_version,
 COALESCE(body->>'outcome','') AS outcome,COALESCE(body->>'reason','') AS reason,
 CASE WHEN jsonb_typeof(body->'moves')='array' THEN jsonb_array_length(body->'moves') ELSE 0 END AS move_count,updated_at,body->'practice' AS practice
 FROM games WHERE `+ownedBy1+` AND $1<>'' AND phase='finished' AND $2 IN ('all','pve')
 UNION ALL
 SELECT id,'pvp'::text AS mode,seat.side AS side,
 COALESCE(NULLIF(body->>'rulesVersion',''),'casual-xiangqi-v1') AS rules_version,
 COALESCE(body->>'outcome','') AS outcome,COALESCE(body->>'reason','') AS reason,
 CASE WHEN jsonb_typeof(body->'moves')='array' THEN jsonb_array_length(body->'moves') ELSE 0 END AS move_count,updated_at,NULL::jsonb AS practice
 FROM pvp_rooms CROSS JOIN LATERAL (VALUES ('red'::text,red_owner),('black'::text,black_owner)) seat(side,source_owner) WHERE (seat.source_owner=$1 OR seat.source_owner IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$1)) AND $1<>'' AND phase='finished' AND $2 IN ('all','pvp')
 ) entries WHERE (NOT $3::boolean OR (updated_at,id,mode,side)<($4,$5,$6,$8))
 ORDER BY updated_at DESC,id DESC,mode DESC,side DESC LIMIT $7`, owner, q.Mode, q.Before != nil, updated, id, mode, q.Limit+1, cursorSide)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var e history.Entry
		var practice []byte
		if err = rows.Scan(&e.ID, &e.Mode, &e.Side, &e.RulesVersion, &e.Outcome, &e.Reason, &e.MoveCount, &e.UpdatedAt, &practice); err != nil {
			return page, err
		}
		if len(practice) > 0 && string(practice) != "null" {
			if err = json.Unmarshal(practice, &e.Practice); err != nil {
				return page, err
			}
		}
		page.Items = append(page.Items, e)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > q.Limit {
		page.Items = page.Items[:q.Limit]
		page.NextCursor = history.Cursor(q.Mode, page.Items[len(page.Items)-1])
	}
	return page, nil
}

// Invitation possession grants only the manager's restricted preview projection.
// This is separate from FindPVP, which always requires a seated principal.
func (p *Postgres) FindPVPInvitation(ctx context.Context, code string) (pvp.Record, error) {
	if !game.ValidID(code) {
		return pvp.Record{}, game.ErrNotFound
	}
	var body []byte
	err := p.pool.QueryRow(ctx, `SELECT body FROM pvp_rooms WHERE body->>'Invite'=$1 OR (body->'PreviousInvites') ? $1 LIMIT 1`, code).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return pvp.Record{}, game.ErrNotFound
	}
	if err != nil {
		return pvp.Record{}, err
	}
	return decodePVP(body)
}
