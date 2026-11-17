package store

import (
	"chinesechess/internal/account"
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type activityWriter interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

func saveActivity(ctx context.Context, db activityWriter, key, owner, kind, mode, id string, at int64, origin string) error {
	if owner == "" {
		return nil
	}
	_, err := db.Exec(ctx, `INSERT INTO account_activity(event_key,owner_key,kind,source_mode,source_id,occurred_at,origin)
 VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(event_key,owner_key) DO NOTHING`, key, owner, kind, mode, id, at, origin)
	return err
}

func validResult(outcome, reason string) bool {
	return (outcome == "red_win" || outcome == "black_win" || outcome == "draw") && reason != "cancelled" && reason != "system_error" && reason != "engine_error"
}

func (p *Postgres) AccountActivity(ctx context.Context, owner, from, to string) (account.Activity, error) {
	start, end, err := account.Range(from, to, time.Now())
	if err != nil {
		return account.Activity{}, err
	}
	out := account.Activity{From: start.Format(time.DateOnly), To: end.AddDate(0, 0, -1).Format(time.DateOnly), TimeZone: account.TimeZone, Days: []account.Day{}}
	if err = p.pool.QueryRow(ctx, `SELECT started_at FROM account_tracking WHERE id=true`).Scan(&out.TrackedSince); err != nil {
		return out, err
	}
	rows, err := p.pool.Query(ctx, `WITH sources AS (SELECT owner_key FROM auth_owner_links WHERE account_id=$1 UNION SELECT $1::text),
 events AS (SELECT DISTINCT ON(event_key) event_key,kind,occurred_at,origin FROM account_activity WHERE owner_key IN(SELECT owner_key FROM sources) AND occurred_at >= $2 AND occurred_at < $3 ORDER BY event_key,occurred_at)
 SELECT to_char(to_timestamp(occurred_at/1000.0) AT TIME ZONE 'Asia/Singapore','YYYY-MM-DD'),
 count(*) FILTER(WHERE kind='game'),count(*) FILTER(WHERE kind='practice'),
 count(*) FILTER(WHERE kind='analysis' AND origin='manual'),count(*) FILTER(WHERE kind='analysis' AND origin='automatic') FROM events GROUP BY 1`, owner, start.UnixMilli(), end.UnixMilli())
	if err != nil {
		return out, err
	}
	defer rows.Close()
	byDate := map[string]account.Day{}
	for rows.Next() {
		var day account.Day
		if err = rows.Scan(&day.Date, &day.Game, &day.Practice, &day.Manual, &day.Automatic); err != nil {
			return out, err
		}
		byDate[day.Date] = day
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	for d := start; d.Before(end); d = d.AddDate(0, 0, 1) {
		day := byDate[d.Format(time.DateOnly)]
		day.Date = d.Format(time.DateOnly)
		day.Partial = d.UnixMilli() < out.TrackedSince
		out.Totals.Game += day.Game
		out.Totals.Practice += day.Practice
		out.Totals.Manual += day.Manual
		out.Totals.Automatic += day.Automatic
		out.Days = append(out.Days, day)
	}
	return out, nil
}

var _ activityWriter = pgx.Tx(nil)
