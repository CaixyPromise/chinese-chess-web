package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"chinesechess/internal/analysis"
	"chinesechess/internal/game"
	"github.com/jackc/pgx/v5"
)

var _ analysis.Repository = (*Postgres)(nil)

type analysisQuerier interface {
	activityWriter
	QueryRow(context.Context, string, ...any) pgx.Row
}

func createAnalysis(ctx context.Context, db analysisQuerier, j analysis.Job) (analysis.Job, error) {
	blocked, err := analysisOwnerBanned(ctx, db, j.Owner)
	if err != nil {
		return analysis.Job{}, err
	}
	if blocked {
		return analysis.Job{}, &game.Error{Code: "ACCOUNT_BANNED"}
	}
	body, err := json.Marshal(j)
	if err != nil {
		return analysis.Job{}, err
	}
	var saved []byte
	// The no-op update returns the original immutable job even when a repeated
	// request supplies a different ID or mutable progress.
	err = db.QueryRow(ctx, `INSERT INTO analysis_jobs(id,owner_key,source_mode,source_id,source_version,digest,scope,profile,ply,engine_version,state,generation,next_ply,completed,total,automatic,created_at,updated_at,body)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)
 ON CONFLICT(owner_key,digest,scope,profile,ply,engine_version) DO UPDATE SET id=analysis_jobs.id RETURNING body`, j.ID, j.Owner, j.Source.Mode, j.Source.ID, j.Source.Version, j.Digest, j.Scope, j.Profile, j.Ply, j.Engine, j.State, j.Generation, j.NextPly, j.Completed, j.Total, j.Automatic, j.Created, j.Updated, body).Scan(&saved)
	if err != nil {
		return analysis.Job{}, err
	}
	// Owner deliberately does not appear in the API JSON body.
	var out analysis.Job
	err = json.Unmarshal(saved, &out)
	out.Owner = j.Owner
	if err != nil {
		return out, err
	}
	origin := "manual"
	if out.Automatic {
		origin = "automatic"
	}
	err = saveActivity(ctx, db, "analysis:"+out.ID, out.Owner, "analysis", out.Source.Mode, out.ID, out.Created, origin)
	return out, err
}
func (p *Postgres) CreateAnalysis(ctx context.Context, j analysis.Job) (analysis.Job, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return analysis.Job{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return analysis.Job{}, err
	}
	out, err := createAnalysis(ctx, tx, j)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (p *Postgres) FindAnalysis(ctx context.Context, owner, id string) (analysis.Job, error) {
	var body []byte
	var originalOwner string
	err := p.pool.QueryRow(ctx, `SELECT body,owner_key FROM analysis_jobs WHERE id=$1 AND `+ownedBy2+` AND $2<>''`, id, owner).Scan(&body, &originalOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return analysis.Job{}, game.ErrNotFound
	}
	if err != nil {
		return analysis.Job{}, err
	}
	var j analysis.Job
	err = json.Unmarshal(body, &j)
	j.Owner = originalOwner
	if err != nil {
		return j, err
	}
	jobs := []analysis.Job{j}
	err = p.analysisSides(ctx, jobs)
	return jobs[0], err
}
func decodeAnalysisRows(rows pgx.Rows) ([]analysis.Job, error) {
	defer rows.Close()
	out := []analysis.Job{}
	for rows.Next() {
		var b []byte
		var owner string
		if err := rows.Scan(&b, &owner); err != nil {
			return nil, err
		}
		var j analysis.Job
		if err := json.Unmarshal(b, &j); err != nil {
			return nil, err
		}
		j.Owner = owner
		out = append(out, j)
	}
	return out, rows.Err()
}
func (p *Postgres) ListAnalyses(ctx context.Context, owner, mode, id string, version int64) ([]analysis.Job, error) {
	rows, err := p.pool.Query(ctx, `SELECT body,owner_key FROM analysis_jobs WHERE `+ownedBy1+` AND $1<>'' AND source_mode=$2 AND source_id=$3 AND source_version=$4 ORDER BY updated_at DESC,id DESC LIMIT 100`, owner, mode, id, version)
	if err != nil {
		return nil, err
	}
	jobs, err := decodeAnalysisRows(rows)
	if err != nil {
		return nil, err
	}
	return jobs, p.analysisSides(ctx, jobs)
}
func (p *Postgres) ReadyAnalyses(ctx context.Context, limit int) ([]analysis.Job, error) {
	if limit < 1 || limit > 8 {
		limit = 8
	}
	rows, err := p.pool.Query(ctx, `SELECT body,owner_key FROM (SELECT body,owner_key,(scope='position') AS position_priority,updated_at,id,row_number() OVER (PARTITION BY COALESCE((SELECT account_id FROM auth_owner_links WHERE auth_owner_links.owner_key=analysis_jobs.owner_key),owner_key) ORDER BY (scope='position') DESC,updated_at,id) AS owner_rank FROM analysis_jobs WHERE state='queued' AND NOT EXISTS(SELECT 1 FROM auth_accounts a WHERE a.status='banned' AND (a.id=analysis_jobs.owner_key OR a.id=(SELECT account_id FROM auth_owner_links WHERE owner_key=analysis_jobs.owner_key)))) ready WHERE owner_rank=1 ORDER BY position_priority DESC,updated_at,id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	return decodeAnalysisRows(rows)
}
func versionConflict() error { return &game.Error{Code: "VERSION_CONFLICT"} }
func (p *Postgres) SaveAnalysis(ctx context.Context, j analysis.Job, generation int64, state string) error {
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}
	tag, err := p.pool.Exec(ctx, `UPDATE analysis_jobs SET state=$1,generation=$2,next_ply=$3,completed=$4,total=$5,updated_at=$6,body=$7 WHERE id=$8 AND owner_key=$9 AND owner_key<>'' AND generation=$10 AND state=$11`, j.State, j.Generation, j.NextPly, j.Completed, j.Total, j.Updated, b, j.ID, j.Owner, generation, state)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return versionConflict()
	}
	return nil
}
func (p *Postgres) CompletePosition(ctx context.Context, j analysis.Job, generation int64, pos analysis.Position, cacheKey string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var currentState string
	var currentGen int64
	err = tx.QueryRow(ctx, `SELECT state,generation FROM analysis_jobs WHERE id=$1 AND owner_key=$2 AND $2<>'' FOR UPDATE`, j.ID, j.Owner).Scan(&currentState, &currentGen)
	if errors.Is(err, pgx.ErrNoRows) {
		return versionConflict()
	}
	if err != nil {
		return err
	}
	if currentState != "running" || currentGen != generation {
		return versionConflict()
	}
	b, err := json.Marshal(pos)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO analysis_positions(job_id,ply,body,sample_id) VALUES($1,$2,$3,$4) ON CONFLICT(job_id,ply) DO NOTHING`, j.ID, pos.Ply, b, pos.SampleID)
	if err != nil {
		return err
	}
	// A duplicate delivery cannot advance progress a second time or overwrite the
	// original result with a different completed search.
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	if cacheKey != "" {
		if _, err = tx.Exec(ctx, `INSERT INTO analysis_cache(cache_key,ply,body) VALUES($1,$2,$3) ON CONFLICT(cache_key,ply) DO NOTHING`, cacheKey, pos.Ply, b); err != nil {
			return err
		}
	}
	jobBody, err := json.Marshal(j)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE analysis_jobs SET state=$1,generation=$2,next_ply=$3,completed=$4,total=$5,updated_at=$6,body=$7 WHERE id=$8`, j.State, j.Generation, j.NextPly, j.Completed, j.Total, j.Updated, jobBody, j.ID)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (p *Postgres) AnalysisPositions(ctx context.Context, owner, id string, cursor, limit int) (analysis.Page, error) {
	page := analysis.Page{Items: []analysis.Position{}}
	if _, err := p.FindAnalysis(ctx, owner, id); err != nil {
		return page, err
	}
	if cursor < 0 {
		cursor = 0
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := p.pool.Query(ctx, `SELECT p.body FROM analysis_positions p JOIN analysis_jobs j ON j.id=p.job_id WHERE j.id=$1 AND (j.owner_key=$2 OR j.owner_key IN (SELECT owner_key FROM auth_owner_links WHERE account_id=$2)) AND $2<>'' AND p.ply >= $3 ORDER BY p.ply LIMIT $4`, id, owner, cursor, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return page, err
		}
		var pos analysis.Position
		if err = json.Unmarshal(b, &pos); err != nil {
			return page, err
		}
		page.Items = append(page.Items, pos)
	}
	if err = rows.Err(); err != nil {
		return page, err
	}
	if len(page.Items) > limit {
		page.Next = page.Items[limit].Ply
		page.Items = page.Items[:limit]
	}
	return page, nil
}
func (p *Postgres) CachedAnalysisPosition(ctx context.Context, key string, ply int) (analysis.Position, bool, error) {
	var b []byte
	var pos analysis.Position
	err := p.pool.QueryRow(ctx, `SELECT body FROM analysis_cache WHERE cache_key=$1 AND ply=$2`, key, ply).Scan(&b)
	if errors.Is(err, pgx.ErrNoRows) {
		return pos, false, nil
	}
	if err != nil {
		return pos, false, err
	}
	err = json.Unmarshal(b, &pos)
	return pos, err == nil, err
}
func (p *Postgres) RecoverAnalyses(ctx context.Context, engine string) error {
	now := time.Now().UnixMilli()
	_, err := p.pool.Exec(ctx, `UPDATE analysis_jobs SET state=CASE WHEN engine_version=$1 THEN 'queued' ELSE 'paused' END,generation=generation+1,updated_at=$2,
 body=body || jsonb_build_object('state',CASE WHEN engine_version=$1 THEN 'queued' ELSE 'paused' END,'generation',generation+1,'updatedAt',$2::bigint,'errorCode',CASE WHEN engine_version=$1 THEN '' ELSE 'ENGINE_CHANGED' END)
 WHERE (engine_version=$1 AND state='running') OR (engine_version<>$1 AND state NOT IN ('completed','failed') AND NOT(state='paused' AND body->>'errorCode'='ENGINE_CHANGED'))`, engine, now)
	return err
}

// The event carries source and current seat owners frozen in the same transaction
// as game completion. Later undo or seat changes cannot change its input.
func saveFinishedEvent(ctx context.Context, tx pgx.Tx, source analysis.Source, owners []string, record []byte) error {
	if source.Moves == nil {
		source.Moves = []string{}
	}
	src, err := json.Marshal(source)
	if err != nil {
		return err
	}
	os, err := json.Marshal(owners)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO analysis_finished_events(mode,source_id,source_version,source,owners,created_at,frozen_record) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, source.Mode, source.ID, source.Version, src, os, time.Now().UnixMilli(), record)
	return err
}
func (p *Postgres) MaterializeAnalyses(ctx context.Context, engine string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(71824109)`); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT mode,source_id,source_version,source,owners FROM analysis_finished_events WHERE processed_at=0 ORDER BY created_at,mode,source_id,source_version LIMIT 32 FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
	}
	type event struct {
		mode, id string
		version  int64
		source   analysis.Source
		owners   []string
	}
	events := []event{}
	for rows.Next() {
		var e event
		var src, owners []byte
		if err = rows.Scan(&e.mode, &e.id, &e.version, &src, &owners); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(src, &e.source); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(owners, &e.owners); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		seen := map[string]bool{}
		for _, owner := range e.owners {
			if owner == "" || seen[owner] {
				continue
			}
			seen[owner] = true
			blocked, checkErr := analysisOwnerBanned(ctx, tx, owner)
			if checkErr != nil {
				return checkErr
			}
			if blocked {
				continue
			}
			now := time.Now().UnixMilli()
			j := analysis.Job{ID: game.ID(), Owner: owner, Source: e.source, Digest: analysis.Digest(e.source), Engine: engine, Scope: "game", Profile: "standard", Budget: analysis.ProfileBudget("standard"), State: "queued", Generation: 1, Total: len(e.source.Moves) + 1, Automatic: true, Created: now, Updated: now}
			if len(e.source.Moves) == 0 {
				j.Total = 0
				j.State = "completed"
			}
			if _, err = createAnalysis(ctx, tx, j); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `UPDATE analysis_finished_events SET processed_at=$4 WHERE mode=$1 AND source_id=$2 AND source_version=$3 AND processed_at=0`, e.mode, e.id, e.version, time.Now().UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
