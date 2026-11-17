package store

import (
	"context"
	"encoding/json"
	"errors"

	"chinesechess/internal/analysis"
	"chinesechess/internal/game"
	"github.com/jackc/pgx/v5"
)

// SaveReplaySnapshot freezes the exact source already read by the authorized
// replay handler. Repeated reads of that source reuse its owner-scoped identity
// without replacing the original body or changing analysis/cache identities.
func (p *Postgres) SaveReplaySnapshot(ctx context.Context, owner string, source analysis.Source) (string, error) {
	if owner == "" {
		return "", game.ErrNotFound
	}
	body, err := json.Marshal(source)
	if err != nil {
		return "", err
	}
	var id string
	err = p.pool.QueryRow(ctx, `INSERT INTO analysis_replay_snapshots(id,owner_key,digest,source_mode,source_id,source_version,body)
 VALUES($1,$2,$3,$4,$5,$6,$7)
 ON CONFLICT(owner_key,digest) DO UPDATE SET id=analysis_replay_snapshots.id RETURNING id`, game.ID(), owner, analysis.Digest(source), source.Mode, source.ID, source.Version, body).Scan(&id)
	return id, err
}

func (p *Postgres) FindReplaySnapshot(ctx context.Context, owner, id string) (analysis.Source, error) {
	var body []byte
	err := p.pool.QueryRow(ctx, `SELECT body FROM analysis_replay_snapshots WHERE id=$1 AND `+ownedBy2+` AND $2<>''`, id, owner).Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return analysis.Source{}, game.ErrNotFound
	}
	if err != nil {
		return analysis.Source{}, err
	}
	var source analysis.Source
	err = json.Unmarshal(body, &source)
	return source, err
}
