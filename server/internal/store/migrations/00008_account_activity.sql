-- +goose Up
CREATE TABLE account_activity (
    event_key text NOT NULL,
    owner_key text NOT NULL,
    kind text NOT NULL CHECK (kind IN ('game','practice','analysis')),
    source_mode text NOT NULL,
    source_id text NOT NULL,
    occurred_at bigint NOT NULL,
    origin text NOT NULL DEFAULT '',
    PRIMARY KEY (event_key,owner_key)
);
CREATE INDEX account_activity_owner_time ON account_activity(owner_key,occurred_at,event_key);
CREATE TABLE account_tracking (
    id boolean PRIMARY KEY DEFAULT true CHECK (id),
    started_at bigint NOT NULL
);
INSERT INTO account_tracking(id,started_at) VALUES(true,(extract(epoch FROM clock_timestamp())*1000)::bigint);

-- Existing immutable job timestamps and committed finished records are usable
-- evidence. Practice creation has no historical timestamp and starts tracking now.
INSERT INTO account_activity(event_key,owner_key,kind,source_mode,source_id,occurred_at,origin)
 SELECT 'analysis:'||id,owner_key,'analysis',source_mode,id,created_at,
 CASE WHEN automatic THEN 'automatic' ELSE 'manual' END FROM analysis_jobs
 ON CONFLICT DO NOTHING;
INSERT INTO account_activity(event_key,owner_key,kind,source_mode,source_id,occurred_at)
 SELECT 'game:pve:'||id,owner_key,'game',CASE WHEN body->'practice' IS NOT NULL AND body->'practice'<>'null'::jsonb THEN 'practice' ELSE 'pve' END,id,updated_at
 FROM games WHERE phase='finished' AND body->>'outcome' IN ('red_win','black_win','draw')
 AND COALESCE(body->>'reason','') NOT IN ('cancelled','system_error','engine_error') ON CONFLICT DO NOTHING;
INSERT INTO account_activity(event_key,owner_key,kind,source_mode,source_id,occurred_at)
 SELECT 'game:pvp:'||r.id,s.owner_key,'game','pvp',r.id,r.updated_at
 FROM pvp_rooms r CROSS JOIN LATERAL (VALUES(r.red_owner),(r.black_owner)) s(owner_key)
 WHERE r.phase='finished' AND s.owner_key<>'' AND r.red_owner<>'' AND r.black_owner<>''
 AND r.body->>'outcome' IN ('red_win','black_win','draw')
 AND COALESCE(r.body->>'reason','') NOT IN ('cancelled','system_error','engine_error') ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE account_tracking;
DROP TABLE account_activity;
