-- +goose Up
CREATE TABLE auth_accounts (
 id text PRIMARY KEY, email text NOT NULL UNIQUE, password_hash text NOT NULL,
 nickname text NOT NULL, created_at bigint NOT NULL
);
CREATE TABLE auth_owner_links (
 owner_key text PRIMARY KEY, account_id text NOT NULL, created_at bigint NOT NULL
);
CREATE INDEX auth_owner_links_account ON auth_owner_links(account_id);
CREATE TABLE auth_sessions (
 token_hash text PRIMARY KEY, owner_key text NOT NULL, created_at bigint NOT NULL,
 last_seen bigint NOT NULL, expires_at bigint NOT NULL, revoked_at bigint NOT NULL DEFAULT 0
);
CREATE INDEX auth_sessions_owner ON auth_sessions(owner_key) WHERE revoked_at=0;
CREATE TABLE auth_tokens (
 token_hash text PRIMARY KEY, purpose text NOT NULL, email text NOT NULL,
 account_id text NOT NULL DEFAULT '', expires_at bigint NOT NULL, used_at bigint NOT NULL DEFAULT 0
);
CREATE INDEX auth_tokens_email ON auth_tokens(email,purpose);
CREATE TABLE auth_registration_grants (
 token_hash text PRIMARY KEY, email text NOT NULL, guest_owner text NOT NULL,
 expires_at bigint NOT NULL, used_at bigint NOT NULL DEFAULT 0
);
-- +goose Down
DROP TABLE auth_registration_grants;
DROP TABLE auth_tokens;
DROP TABLE auth_sessions;
DROP TABLE auth_owner_links;
DROP TABLE auth_accounts;
