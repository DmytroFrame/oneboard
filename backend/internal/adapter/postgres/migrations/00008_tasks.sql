-- +goose Up
-- OmiTeam tasks: column statuses, planning (milestones, sprints, versions), task fields, tags,
-- checklists, comments, history and templates.

ALTER TABLE board_columns
    ADD COLUMN status TEXT NOT NULL DEFAULT 'in_progress'
        CHECK (status IN ('backlog', 'todo', 'in_progress', 'testing', 'done', 'rejected'));
UPDATE board_columns SET status = CASE lower(name)
    WHEN 'backlog' THEN 'backlog'
    WHEN 'to do' THEN 'todo'
    WHEN 'review' THEN 'testing'
    WHEN 'testing' THEN 'testing'
    WHEN 'done' THEN 'done'
    ELSE 'in_progress' END;

CREATE TABLE milestones (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name        TEXT   NOT NULL,
    description TEXT   NOT NULL DEFAULT '',
    start_date  DATE,
    end_date    DATE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);
CREATE INDEX milestones_project_idx ON milestones (project_id);

CREATE TABLE sprints (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       TEXT   NOT NULL,
    goal       TEXT   NOT NULL DEFAULT '',
    start_date DATE,
    end_date   DATE,
    status     TEXT   NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'current', 'past')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (end_date IS NULL OR start_date IS NULL OR end_date >= start_date)
);
CREATE INDEX sprints_project_idx ON sprints (project_id);

CREATE TABLE versions (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id   BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name         TEXT   NOT NULL,
    description  TEXT   NOT NULL DEFAULT '',
    release_date DATE,
    status       TEXT   NOT NULL DEFAULT 'in_development'
        CHECK (status IN ('in_development', 'upcoming', 'released', 'archived')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX versions_project_idx ON versions (project_id);

ALTER TABLE cards
    ADD COLUMN story_points SMALLINT CHECK (story_points BETWEEN 0 AND 1000),
    ADD COLUMN budget_hours NUMERIC(10, 2) CHECK (budget_hours > 0),
    ADD COLUMN billable     BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN special_type TEXT    NOT NULL DEFAULT ''
        CHECK (special_type IN ('', 'sick_day', 'day_off', 'vacation', 'pre_sale', 'bench', 'interview')),
    ADD COLUMN milestone_id BIGINT REFERENCES milestones (id) ON DELETE SET NULL,
    ADD COLUMN sprint_id    BIGINT REFERENCES sprints (id) ON DELETE SET NULL,
    ADD COLUMN version_id   BIGINT REFERENCES versions (id) ON DELETE SET NULL,
    ADD COLUMN is_template  BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE project_tags (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    name       TEXT   NOT NULL,
    color      TEXT   NOT NULL DEFAULT '#8e8e93'
);
CREATE UNIQUE INDEX project_tags_name_key ON project_tags (project_id, lower(name));

CREATE TABLE card_tags (
    card_id BIGINT NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    tag_id  BIGINT NOT NULL REFERENCES project_tags (id) ON DELETE CASCADE,
    PRIMARY KEY (card_id, tag_id)
);
CREATE INDEX card_tags_tag_idx ON card_tags (tag_id);

CREATE TABLE card_checklists (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id   BIGINT  NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    title     TEXT    NOT NULL,
    position  INT     NOT NULL DEFAULT 0,
    collapsed BOOLEAN NOT NULL DEFAULT FALSE
);
CREATE INDEX card_checklists_card_idx ON card_checklists (card_id, position);

CREATE TABLE checklist_items (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    checklist_id BIGINT  NOT NULL REFERENCES card_checklists (id) ON DELETE CASCADE,
    text         TEXT    NOT NULL,
    done         BOOLEAN NOT NULL DEFAULT FALSE,
    position     INT     NOT NULL DEFAULT 0
);
CREATE INDEX checklist_items_list_idx ON checklist_items (checklist_id, position);

CREATE TABLE card_comments (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id    BIGINT NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    author_id  BIGINT REFERENCES users (id) ON DELETE SET NULL,
    body       TEXT   NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX card_comments_card_idx ON card_comments (card_id, created_at);

-- What changed on a task, for its "History" tab.
CREATE TABLE card_history (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id    BIGINT NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    actor_id   BIGINT REFERENCES users (id) ON DELETE SET NULL,
    field      TEXT   NOT NULL,
    old_value  TEXT   NOT NULL DEFAULT '',
    new_value  TEXT   NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX card_history_card_idx ON card_history (card_id, created_at DESC);

-- +goose Down
DROP TABLE card_history, card_comments, checklist_items, card_checklists, card_tags, project_tags;
ALTER TABLE cards DROP COLUMN story_points, DROP COLUMN budget_hours, DROP COLUMN billable, DROP COLUMN special_type,
    DROP COLUMN milestone_id, DROP COLUMN sprint_id, DROP COLUMN version_id, DROP COLUMN is_template;
DROP TABLE versions, sprints, milestones;
ALTER TABLE board_columns DROP COLUMN status;
