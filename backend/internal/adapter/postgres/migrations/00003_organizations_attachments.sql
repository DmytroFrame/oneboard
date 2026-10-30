-- +goose Up
CREATE TABLE organizations (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT        NOT NULL,
    created_by BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organization_members (
    organization_id BIGINT      NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    user_id         BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role            TEXT        NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    added_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (organization_id, user_id)
);
CREATE INDEX organization_members_user_idx ON organization_members (user_id);

-- Invitations for people who don't have an account yet; accepted automatically on sign-up.
CREATE TABLE organization_invitations (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    organization_id BIGINT      NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    email           TEXT        NOT NULL,
    role            TEXT        NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    invited_by      BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX organization_invitations_email_key ON organization_invitations (organization_id, lower(email));
CREATE INDEX organization_invitations_email_idx ON organization_invitations (lower(email));

ALTER TABLE projects ADD COLUMN organization_id BIGINT REFERENCES organizations (id) ON DELETE CASCADE;

-- Existing projects: every project owner gets a workspace with their projects and those projects' members.
INSERT INTO organizations (name, created_by)
SELECT u.name || '''s workspace', u.id FROM users u WHERE u.id IN (SELECT owner_id FROM projects);

UPDATE projects p SET organization_id = o.id FROM organizations o WHERE o.created_by = p.owner_id;

INSERT INTO organization_members (organization_id, user_id, role)
SELECT id, created_by, 'owner' FROM organizations;

INSERT INTO organization_members (organization_id, user_id, role)
SELECT DISTINCT p.organization_id, pm.user_id, 'member'
FROM project_members pm JOIN projects p ON p.id = pm.project_id
ON CONFLICT DO NOTHING;

ALTER TABLE projects ALTER COLUMN organization_id SET NOT NULL;
CREATE INDEX projects_organization_idx ON projects (organization_id);

CREATE TABLE card_attachments (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    card_id      BIGINT      NOT NULL REFERENCES cards (id) ON DELETE CASCADE,
    uploader_id  BIGINT      REFERENCES users (id) ON DELETE SET NULL,
    file_name    TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size         BIGINT      NOT NULL,
    kind         TEXT        NOT NULL CHECK (kind IN ('image', 'video', 'file')),
    storage_key  TEXT        NOT NULL,
    thumb_key    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX card_attachments_card_idx ON card_attachments (card_id, id);

-- +goose Down
DROP TABLE card_attachments;
ALTER TABLE projects DROP COLUMN organization_id;
DROP TABLE organization_invitations;
DROP TABLE organization_members;
DROP TABLE organizations;
