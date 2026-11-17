-- +goose Up
CREATE TABLE analysis_finished_events (
    mode text NOT NULL,
    source_id text NOT NULL,
    source_version bigint NOT NULL,
    source jsonb NOT NULL,
    owners jsonb NOT NULL,
    created_at bigint NOT NULL,
    PRIMARY KEY (mode, source_id, source_version)
);
CREATE INDEX analysis_finished_events_ready ON analysis_finished_events (created_at, mode, source_id, source_version);

CREATE TABLE analysis_jobs (
    id text PRIMARY KEY,
    owner_key text NOT NULL,
    source_mode text NOT NULL,
    source_id text NOT NULL,
    source_version bigint NOT NULL,
    digest text NOT NULL,
    scope text NOT NULL,
    profile text NOT NULL,
    ply integer NOT NULL,
    engine_version text NOT NULL,
    state text NOT NULL,
    generation bigint NOT NULL,
    next_ply integer NOT NULL,
    completed integer NOT NULL,
    total integer NOT NULL,
    automatic boolean NOT NULL,
    created_at bigint NOT NULL,
    updated_at bigint NOT NULL,
    body jsonb NOT NULL,
    UNIQUE (owner_key, digest, scope, profile, ply, engine_version)
);
CREATE INDEX analysis_jobs_source ON analysis_jobs (owner_key, source_mode, source_id, source_version, updated_at DESC, id);
CREATE INDEX analysis_jobs_ready ON analysis_jobs (automatic, updated_at, id) WHERE state='queued';
CREATE INDEX analysis_jobs_recovery ON analysis_jobs (engine_version, state) WHERE state NOT IN ('completed','failed');
CREATE TABLE analysis_positions (
    job_id text NOT NULL,
    ply integer NOT NULL,
    body jsonb NOT NULL,
    PRIMARY KEY (job_id, ply)
);
CREATE TABLE analysis_cache (
    cache_key text NOT NULL,
    ply integer NOT NULL,
    body jsonb NOT NULL,
    PRIMARY KEY (cache_key, ply)
);

-- +goose Down
DROP TABLE analysis_cache;
DROP TABLE analysis_positions;
DROP TABLE analysis_jobs;
DROP TABLE analysis_finished_events;
