-- +goose Up
-- Time logs (OmiTeam time tracking): minutes worked on a task on a day.
CREATE TABLE timelogs (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    card_id    BIGINT  NOT NULL REFERENCES cards (id), -- a task with logged time can't be deleted
    project_id BIGINT  NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    day        DATE    NOT NULL,
    minutes    INT     NOT NULL CHECK (minutes > 0 AND minutes <= 24 * 60),
    comment    TEXT    NOT NULL DEFAULT '',
    created_by BIGINT  REFERENCES users (id) ON DELETE SET NULL, -- someone may log time for someone else
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX timelogs_user_day_idx ON timelogs (user_id, day);
CREATE INDEX timelogs_card_idx ON timelogs (card_id);
CREATE INDEX timelogs_project_day_idx ON timelogs (project_id, day);

-- project_id cascades, so deleting a project also deletes its logs before its cards.

-- +goose Down
DROP TABLE timelogs;
