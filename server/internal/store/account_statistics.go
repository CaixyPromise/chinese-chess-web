package store

import (
	"chinesechess/internal/account"
	"context"
	"time"
)

func (p *Postgres) AccountStatistics(ctx context.Context, owner, mode string) (account.Statistics, error) {
	if mode == "" {
		mode = "pvp"
	}
	if mode != "pvp" && mode != "pve" && mode != "practice" {
		return account.Statistics{}, account.ErrInvalidQuery
	}
	_, end, _ := account.Range("", "", time.Now())
	start := end.AddDate(0, 0, -30)
	out := account.Statistics{Mode: mode, From: start.Format(time.DateOnly), To: end.AddDate(0, 0, -1).Format(time.DateOnly), TimeZone: account.TimeZone}
	rows, err := p.pool.Query(ctx, `WITH sources AS (SELECT owner_key FROM auth_owner_links WHERE account_id=$1 UNION SELECT $1::text),
 results AS (
 SELECT body->>'outcome' AS outcome,body->>'human' AS side,false AS self_game,updated_at,
 CASE WHEN body->'practice' IS NOT NULL AND body->'practice'<>'null'::jsonb THEN 'practice' ELSE 'pve' END AS mode,
 COALESCE(body->>'reason','') AS reason FROM games WHERE owner_key IN(SELECT owner_key FROM sources) AND phase='finished'
 UNION ALL
 SELECT body->>'outcome',CASE WHEN red_owner IN(SELECT owner_key FROM sources) THEN 'red' ELSE 'black' END,
 red_owner IN(SELECT owner_key FROM sources) AND black_owner IN(SELECT owner_key FROM sources),updated_at,'pvp',COALESCE(body->>'reason','')
 FROM pvp_rooms WHERE phase='finished' AND red_owner<>'' AND black_owner<>'' AND (red_owner IN(SELECT owner_key FROM sources) OR black_owner IN(SELECT owner_key FROM sources)))
 SELECT outcome,side,self_game FROM results WHERE mode=$2 AND updated_at >= $3 AND updated_at < $4
 AND outcome IN('red_win','black_win','draw') AND reason NOT IN('cancelled','system_error','engine_error')`, owner, mode, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var outcome, side string
		var self bool
		if err = rows.Scan(&outcome, &side, &self); err != nil {
			rows.Close()
			return out, err
		}
		if self {
			out.ExcludedSelfGames++
			continue
		}
		account.Add(&out.Results, outcome, side)
		if side == "red" {
			account.Add(&out.Sides.Red, outcome, side)
		} else {
			account.Add(&out.Sides.Black, outcome, side)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	annual := end.AddDate(0, 0, -365)
	out.Analysis.From = annual.Format(time.DateOnly)
	out.Analysis.To = out.To
	rows, err = p.pool.Query(ctx, `WITH sources AS (SELECT owner_key FROM auth_owner_links WHERE account_id=$1 UNION SELECT $1::text)
 SELECT state,count(*) FROM analysis_jobs WHERE owner_key IN(SELECT owner_key FROM sources) AND created_at >= $2 AND created_at < $3 GROUP BY state`, owner, annual.UnixMilli(), end.UnixMilli())
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var state string
		var n int
		if err = rows.Scan(&state, &n); err != nil {
			return out, err
		}
		out.Analysis.Total += n
		switch state {
		case "completed":
			out.Analysis.Completed = n
		case "queued":
			out.Analysis.Queued = n
		case "running":
			out.Analysis.Running = n
		case "paused":
			out.Analysis.Paused = n
		case "failed":
			out.Analysis.Failed = n
		}
	}
	return out, rows.Err()
}
