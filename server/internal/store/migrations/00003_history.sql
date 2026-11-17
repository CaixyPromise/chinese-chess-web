-- +goose Up
ALTER TABLE games ADD COLUMN phase text NOT NULL DEFAULT '';
UPDATE games SET phase = COALESCE(body->>'phase', '');
ALTER TABLE pvp_rooms ADD COLUMN phase text NOT NULL DEFAULT '';
ALTER TABLE pvp_rooms ADD COLUMN red_owner text NOT NULL DEFAULT '';
ALTER TABLE pvp_rooms ADD COLUMN black_owner text NOT NULL DEFAULT '';
UPDATE pvp_rooms SET phase = COALESCE(body->>'phase', ''),
    red_owner = COALESCE(body->>'RedOwner', ''),
    black_owner = COALESCE(body->>'BlackOwner', '');
CREATE INDEX games_owner_phase_history_idx ON games(owner_key, phase, updated_at DESC, id DESC);
CREATE INDEX games_active_idx ON games(updated_at DESC) WHERE phase <> 'finished';
CREATE INDEX pvp_red_phase_history_idx ON pvp_rooms(red_owner, phase, updated_at DESC, id DESC);
CREATE INDEX pvp_black_phase_history_idx ON pvp_rooms(black_owner, phase, updated_at DESC, id DESC);
CREATE INDEX pvp_active_idx ON pvp_rooms(updated_at DESC) WHERE phase <> 'finished';

-- +goose Down
DROP INDEX pvp_active_idx;
DROP INDEX pvp_black_phase_history_idx;
DROP INDEX pvp_red_phase_history_idx;
DROP INDEX games_active_idx;
DROP INDEX games_owner_phase_history_idx;
ALTER TABLE pvp_rooms DROP COLUMN black_owner, DROP COLUMN red_owner, DROP COLUMN phase;
ALTER TABLE games DROP COLUMN phase;
