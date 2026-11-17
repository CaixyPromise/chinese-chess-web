// Package store implements PostgreSQL persistence independently of HTTP and engine execution.
package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"sync"

	"chinesechess/internal/analysis"
	"chinesechess/internal/chess"
	"chinesechess/internal/game"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

type Postgres struct {
	pool       *pgxpool.Pool
	identityMu sync.RWMutex
	identities map[string]string
}

func Open(ctx context.Context, dsn string) (*Postgres, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, mustSub())
	if err != nil {
		return nil, err
	}
	if _, err = provider.Up(ctx); err != nil {
		return nil, err
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	if err = pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	p := &Postgres{pool: pool}
	if err = p.ReloadIdentity(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return p, nil
}
func (p *Postgres) Close() { p.pool.Close() }
func (p *Postgres) Load(ctx context.Context) ([]game.Record, error) {
	rows, err := p.pool.Query(ctx, "SELECT body FROM games WHERE phase <> 'finished' ORDER BY updated_at DESC")
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
		r, decodeErr := decodeGame(data)
		if decodeErr != nil {
			return nil, decodeErr
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func (p *Postgres) Save(ctx context.Context, r game.Record, expected int64) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var oldPhase string
	err = tx.QueryRow(ctx, `SELECT phase FROM games WHERE id=$1 FOR UPDATE`, r.ID).Scan(&oldPhase)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO games(id,owner_key,version,updated_at,body,phase) VALUES($1,$2,$3,$4,$5,$6)
ON CONFLICT(id) DO UPDATE SET version=EXCLUDED.version,updated_at=EXCLUDED.updated_at,body=EXCLUDED.body,phase=EXCLUDED.phase
WHERE games.version=$7 AND games.owner_key=EXCLUDED.owner_key`, r.ID, r.Owner, r.Version, r.Updated, data, r.Phase, expected)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("saved game version conflict")
	}
	mode := "pve"
	if r.Practice != nil {
		mode = "practice"
		if expected == 0 {
			if err = saveActivity(ctx, tx, "practice:"+r.ID, r.Owner, "practice", mode, r.ID, r.Updated, ""); err != nil {
				return err
			}
		}
	}
	if r.Phase == "finished" && oldPhase != "finished" && validResult(r.Outcome, r.Reason) {
		if err = saveActivity(ctx, tx, "game:pve:"+r.ID, r.Owner, "game", mode, r.ID, r.Updated, ""); err != nil {
			return err
		}
	}
	if r.Phase == "finished" && oldPhase != "finished" {
		rules := r.Rules
		if rules == "" {
			rules = "legacy-java-v1"
		}
		source := analysis.Source{Mode: "pve", ID: r.ID, Version: r.Version, Moves: r.Moves, Rules: rules, Phase: "finished", Fen: r.Fen, Outcome: r.Outcome, Reason: r.Reason, Origin: "game", GroupID: "pve:" + r.ID}
		if r.Practice != nil {
			source.Origin = "practice"
			source.GroupID = r.Practice.SourceGroupID
			if source.GroupID == "" {
				source.GroupID = r.Practice.SourceMode + ":" + r.Practice.SourceID
			}
		}
		if len(r.Keys) == len(r.Moves)+1 && len(r.Checks) == len(r.Moves) && r.Keys[len(r.Keys)-1] == chess.PositionKey(r.Fen) {
			source.Keys = r.Keys
			source.Checks = r.Checks
		}
		if err = saveFinishedEvent(ctx, tx, source, []string{r.Owner}, data); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
