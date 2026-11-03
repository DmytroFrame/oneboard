-- +goose Up
-- Deactivation and dismissal: such accounts can't sign in but keep their history.
ALTER TABLE users
    ADD COLUMN active           BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN dismissed_at     TIMESTAMPTZ,
    ADD COLUMN dismissal_reason TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN active, DROP COLUMN dismissed_at, DROP COLUMN dismissal_reason;
