-- +goose Up
ALTER TABLE users
    ADD COLUMN language TEXT NOT NULL DEFAULT 'auto' CHECK (language IN ('auto', 'en', 'uk', 'de')),
    ADD COLUMN theme    TEXT NOT NULL DEFAULT 'auto' CHECK (theme IN ('auto', 'light', 'dark'));

-- +goose Down
ALTER TABLE users DROP COLUMN language, DROP COLUMN theme;
