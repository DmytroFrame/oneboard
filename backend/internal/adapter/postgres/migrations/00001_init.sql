-- +goose Up
CREATE TABLE users (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name              TEXT        NOT NULL,
    email             TEXT        NOT NULL,
    password_hash     TEXT        NOT NULL,
    color             TEXT        NOT NULL,
    avatar_key        TEXT,
    avatar_updated_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE projects (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    key        TEXT        NOT NULL UNIQUE,
    owner_id   BIGINT      NOT NULL REFERENCES users (id),
    card_seq   BIGINT      NOT NULL DEFAULT 0, -- last issued card number, so keys are never reused
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE project_members (
    project_id BIGINT      NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       TEXT        NOT NULL CHECK (role IN ('owner', 'member')),
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, user_id)
);
CREATE INDEX project_members_user_idx ON project_members (user_id);

CREATE TABLE board_columns (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT      NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       TEXT        NOT NULL,
    position   INT         NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX board_columns_project_idx ON board_columns (project_id, position);

CREATE TABLE cards (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id  BIGINT      NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    column_id   BIGINT      NOT NULL REFERENCES board_columns (id),
    number      BIGINT      NOT NULL,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    priority    TEXT        NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'critical')),
    position    INT         NOT NULL,
    author_id   BIGINT      NOT NULL REFERENCES users (id),
    assignee_id BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, number)
);
CREATE INDEX cards_column_idx ON cards (column_id, position, id);
CREATE INDEX cards_assignee_idx ON cards (project_id, assignee_id);

-- +goose Down
DROP TABLE cards;
DROP TABLE board_columns;
DROP TABLE project_members;
DROP TABLE projects;
DROP TABLE users;
