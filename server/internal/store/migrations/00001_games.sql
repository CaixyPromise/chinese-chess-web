-- +goose Up
CREATE TABLE games (
    id text PRIMARY KEY,
    owner_key text NOT NULL,
    version bigint NOT NULL CHECK (version > 0),
    updated_at bigint NOT NULL,
    body jsonb NOT NULL
);
CREATE INDEX games_owner_updated_idx ON games (owner_key, updated_at DESC);

-- +goose Down
DROP TABLE games;
