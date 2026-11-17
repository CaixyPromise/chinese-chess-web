-- +goose Up
-- Keep completion facts after queue acknowledgment: analysis begun during play
-- can later be related to the frozen final record without rewriting its sample.
ALTER TABLE analysis_finished_events ADD COLUMN processed_at bigint NOT NULL DEFAULT 0;
ALTER TABLE analysis_finished_events ADD COLUMN frozen_record jsonb;
DROP INDEX analysis_finished_events_ready;
CREATE INDEX analysis_finished_events_ready ON analysis_finished_events (created_at, mode, source_id, source_version) WHERE processed_at=0;
ALTER TABLE analysis_positions ADD COLUMN sample_id text NOT NULL DEFAULT '';
UPDATE analysis_positions SET sample_id=COALESCE(body->>'sampleId','');
CREATE INDEX analysis_positions_sample ON analysis_positions (sample_id) WHERE sample_id<>'';

-- +goose Down
DROP INDEX analysis_positions_sample;
ALTER TABLE analysis_positions DROP COLUMN sample_id;
DROP INDEX analysis_finished_events_ready;
CREATE INDEX analysis_finished_events_ready ON analysis_finished_events (created_at, mode, source_id, source_version);
ALTER TABLE analysis_finished_events DROP COLUMN frozen_record, DROP COLUMN processed_at;
