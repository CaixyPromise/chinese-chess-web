-- +goose Up
ALTER TABLE auth_accounts ADD COLUMN role text NOT NULL DEFAULT 'user' CHECK(role IN ('user','admin'));
ALTER TABLE auth_accounts ADD COLUMN status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','banned'));
ALTER TABLE auth_accounts ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE auth_sessions ADD COLUMN display_id text NOT NULL DEFAULT md5(random()::text || clock_timestamp()::text);
CREATE UNIQUE INDEX auth_sessions_display_id ON auth_sessions(display_id);
ALTER TABLE auth_sessions ADD COLUMN version bigint NOT NULL DEFAULT 1;
ALTER TABLE auth_sessions ADD COLUMN admin_confirmed_at bigint NOT NULL DEFAULT 0;
CREATE TABLE admin_audit (
 id text PRIMARY KEY, actor_id text NOT NULL, action text NOT NULL, target_id text NOT NULL,
 reason text NOT NULL, request_id text NOT NULL, created_at bigint NOT NULL,
 before_state jsonb NOT NULL, after_state jsonb NOT NULL, result jsonb NOT NULL,
 UNIQUE(actor_id,request_id)
);
CREATE INDEX admin_audit_order ON admin_audit(created_at DESC,id DESC);
CREATE TABLE admin_security_events(id bigserial PRIMARY KEY, actor_id text NOT NULL, code text NOT NULL, created_at bigint NOT NULL);
CREATE INDEX auth_accounts_admin_order ON auth_accounts(created_at DESC,id DESC);
CREATE INDEX auth_sessions_admin_order ON auth_sessions(created_at DESC,display_id DESC);
CREATE INDEX analysis_jobs_admin_order ON analysis_jobs(created_at DESC,id DESC);
-- +goose Down
DROP INDEX analysis_jobs_admin_order;
DROP INDEX auth_sessions_admin_order;
DROP INDEX auth_accounts_admin_order;
DROP TABLE admin_security_events;
DROP TABLE admin_audit;
ALTER TABLE auth_sessions DROP COLUMN admin_confirmed_at, DROP COLUMN version, DROP COLUMN display_id;
ALTER TABLE auth_accounts DROP COLUMN version, DROP COLUMN status, DROP COLUMN role;
