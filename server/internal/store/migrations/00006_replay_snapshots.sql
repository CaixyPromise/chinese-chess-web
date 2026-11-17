-- +goose Up
-- A replay frame is immutable even if the live game appends or undoes moves.
-- Store only the authorized chess source, without game/room credentials.
CREATE TABLE analysis_replay_snapshots (
    id text PRIMARY KEY,
    owner_key text NOT NULL,
    digest text NOT NULL,
    source_mode text NOT NULL,
    source_id text NOT NULL,
    source_version bigint NOT NULL,
    body jsonb NOT NULL,
    UNIQUE (owner_key, digest)
);
CREATE INDEX analysis_replay_snapshots_source ON analysis_replay_snapshots (owner_key, source_mode, source_id, source_version);

-- +goose Down
DROP TABLE analysis_replay_snapshots;
