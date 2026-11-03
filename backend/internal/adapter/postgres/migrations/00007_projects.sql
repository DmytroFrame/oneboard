-- +goose Up
-- OmiTeam project fields. "sections" turns project tabs on and off; is_main marks the company's own
-- project, the only one meant to hold special tasks (vacation, sick day, bench…).
ALTER TABLE projects
    ADD COLUMN client       TEXT    NOT NULL DEFAULT '',
    ADD COLUMN description  TEXT    NOT NULL DEFAULT '',
    ADD COLUMN start_date   DATE,
    ADD COLUMN active       BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN billable     BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN budget_hours NUMERIC(10, 2),
    ADD COLUMN is_main      BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN sections     JSONB   NOT NULL DEFAULT '{"backlog": true, "planning": true, "comments": true, "statistics": true, "wiki": true}';

-- Project admins (OmiTeam): may log time for others and delete tasks, with the LEAD role.
ALTER TABLE project_members DROP CONSTRAINT project_members_role_check;
ALTER TABLE project_members ADD CONSTRAINT project_members_role_check CHECK (role IN ('owner', 'admin', 'member'));

-- Pinned projects, per user.
CREATE TABLE project_pins (
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    project_id BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, project_id)
);

-- +goose Down
DROP TABLE project_pins;
ALTER TABLE project_members DROP CONSTRAINT project_members_role_check;
ALTER TABLE project_members ADD CONSTRAINT project_members_role_check CHECK (role IN ('owner', 'member'));
ALTER TABLE projects DROP COLUMN client, DROP COLUMN description, DROP COLUMN start_date, DROP COLUMN active,
    DROP COLUMN billable, DROP COLUMN budget_hours, DROP COLUMN is_main, DROP COLUMN sections;
