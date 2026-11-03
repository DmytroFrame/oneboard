-- +goose Up
-- Database backups made from the admin panel ("Backwoods") or on a schedule (author_id NULL = CRON).
-- The dumps live in the private bucket and are never deleted.
CREATE TABLE backups (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    object_key TEXT   NOT NULL,
    size_bytes BIGINT NOT NULL,
    tables     INT    NOT NULL,
    author_id  BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX backups_created_idx ON backups (created_at DESC);

-- +goose Down
DROP TABLE backups;
