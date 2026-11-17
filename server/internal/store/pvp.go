package store

import (
	"chinesechess/internal/analysis"
	"chinesechess/internal/chess"
	"chinesechess/internal/pvp"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (p *Postgres) LoadPVP(ctx context.Context) ([]pvp.Record, error) {
	rows, e := p.pool.Query(ctx, "SELECT body FROM pvp_rooms WHERE phase <> 'finished' ORDER BY updated_at DESC")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []pvp.Record{}
	for rows.Next() {
		var b []byte
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		r, decodeErr := decodePVP(b)
		if decodeErr != nil {
			return nil, decodeErr
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (p *Postgres) SavePVP(ctx context.Context, r pvp.Record, expected int64) error {
	b, e := json.Marshal(r)
	if e != nil {
		return e
	}
	tx, e := p.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var oldPhase string
	e = tx.QueryRow(ctx, `SELECT phase FROM pvp_rooms WHERE id=$1 FOR UPDATE`, r.ID).Scan(&oldPhase)
	if e != nil && !errors.Is(e, pgx.ErrNoRows) {
		return e
	}
	tag, e := tx.Exec(ctx, `INSERT INTO pvp_rooms(id,version,updated_at,body,phase,red_owner,black_owner) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(id) DO UPDATE SET version=EXCLUDED.version,updated_at=EXCLUDED.updated_at,body=EXCLUDED.body,phase=EXCLUDED.phase,red_owner=EXCLUDED.red_owner,black_owner=EXCLUDED.black_owner WHERE pvp_rooms.version=$8`, r.ID, r.Version, r.Updated, b, r.Phase, r.RedOwner, r.BlackOwner, expected)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("saved PVP version conflict")
	}
	if r.Phase == "finished" && oldPhase != "finished" && r.RedOwner != "" && r.BlackOwner != "" && validResult(r.Outcome, r.Reason) {
		for _, owner := range []string{r.RedOwner, r.BlackOwner} {
			if e = saveActivity(ctx, tx, "game:pvp:"+r.ID, owner, "game", "pvp", r.ID, r.Updated, ""); e != nil {
				return e
			}
		}
	}
	if r.Phase == "finished" && oldPhase != "finished" {
		rules := r.Rules
		if rules == "" {
			rules = "casual-xiangqi-v1"
		}
		source := analysis.Source{Mode: "pvp", ID: r.ID, Version: r.Version, Moves: r.Moves, Rules: rules, Phase: "finished", Fen: r.Fen, Outcome: r.Outcome, Reason: r.Reason, Origin: "game", GroupID: "pvp:" + r.ID}
		if len(r.Keys) == len(r.Moves)+1 && len(r.Checks) == len(r.Moves) && r.Keys[len(r.Keys)-1] == chess.PositionKey(r.Fen) {
			source.Keys = r.Keys
			source.Checks = r.Checks
		}
		if e = saveFinishedEvent(ctx, tx, source, []string{r.RedOwner, r.BlackOwner}, b); e != nil {
			return e
		}
	}
	return tx.Commit(ctx)
}
