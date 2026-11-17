-- +goose Up
CREATE TABLE pvp_rooms (
    id text PRIMARY KEY,
    version bigint NOT NULL CHECK(version > 0),
    updated_at bigint NOT NULL,
    body jsonb NOT NULL
);
-- +goose Down
DROP TABLE pvp_rooms;
