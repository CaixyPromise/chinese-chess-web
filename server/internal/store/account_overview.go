package store

import (
	"chinesechess/internal/account"
	"chinesechess/internal/history"
	"context"
	"encoding/json"
)

func (p *Postgres) AccountOverview(ctx context.Context, owner string) (account.Overview, error) {
	out := account.Overview{Active: []account.Active{}, Recent: []history.Entry{}, Analyses: []account.Analysis{}}
	rows, err := p.pool.Query(ctx, `WITH sources AS (SELECT owner_key FROM auth_owner_links WHERE account_id=$1 UNION SELECT $1::text)
 SELECT id,mode,side,phase,move_count,updated_at,difficulty,practice FROM (
 SELECT id,'pve' AS mode,COALESCE(body->>'human','') AS side,phase,
 CASE WHEN jsonb_typeof(body->'moves')='array' THEN jsonb_array_length(body->'moves') ELSE 0 END AS move_count,updated_at,
 COALESCE(body->>'difficulty','') AS difficulty,body->'practice' AS practice
 FROM games WHERE owner_key IN(SELECT owner_key FROM sources) AND phase<>'finished'
 UNION ALL
 SELECT id,'pvp',CASE WHEN red_owner IN(SELECT owner_key FROM sources) THEN 'red' ELSE 'black' END,phase,
 CASE WHEN jsonb_typeof(body->'moves')='array' THEN jsonb_array_length(body->'moves') ELSE 0 END,updated_at,'' AS difficulty,NULL::jsonb
 FROM pvp_rooms WHERE (red_owner IN(SELECT owner_key FROM sources) OR black_owner IN(SELECT owner_key FROM sources)) AND phase<>'finished'
 ) active ORDER BY updated_at DESC,id DESC,mode DESC`, owner)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var entry account.Active
		var practice []byte
		if err = rows.Scan(&entry.ID, &entry.Mode, &entry.Side, &entry.Phase, &entry.MoveCount, &entry.UpdatedAt, &entry.Difficulty, &practice); err != nil {
			rows.Close()
			return out, err
		}
		if len(practice) > 0 && string(practice) != "null" {
			if err = json.Unmarshal(practice, &entry.Practice); err != nil {
				rows.Close()
				return out, err
			}
		}
		out.Active = append(out.Active, entry)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	page, err := p.History(ctx, owner, history.Query{Mode: "all", Limit: 6})
	if err != nil {
		return out, err
	}
	out.Recent = page.Items
	rows, err = p.pool.Query(ctx, `WITH sources AS (SELECT owner_key FROM auth_owner_links WHERE account_id=$1 UNION SELECT $1::text)
 SELECT id,source_mode,source_id,source_version,scope,state,completed,total,updated_at FROM analysis_jobs WHERE owner_key IN(SELECT owner_key FROM sources) ORDER BY updated_at DESC,id DESC LIMIT 6`, owner)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var entry account.Analysis
		if err = rows.Scan(&entry.ID, &entry.SourceMode, &entry.SourceID, &entry.SourceVersion, &entry.Scope, &entry.State, &entry.Completed, &entry.Total, &entry.UpdatedAt); err != nil {
			return out, err
		}
		out.Analyses = append(out.Analyses, entry)
	}
	return out, rows.Err()
}
