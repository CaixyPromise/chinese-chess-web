-- +goose Up
-- Browser-local task context is separate from the email ownership proof. Old
-- outstanding proofs have no requester context and receive a safe default.
ALTER TABLE auth_tokens ADD COLUMN guest_owner text NOT NULL DEFAULT '';
ALTER TABLE auth_tokens ADD COLUMN return_path text NOT NULL DEFAULT '';
ALTER TABLE auth_registration_grants ADD COLUMN return_path text NOT NULL DEFAULT '';
-- +goose Down
ALTER TABLE auth_registration_grants DROP COLUMN return_path;
ALTER TABLE auth_tokens DROP COLUMN return_path, DROP COLUMN guest_owner;
