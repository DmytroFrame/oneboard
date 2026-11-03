-- +goose Up
-- People (OmiTeam team & profiles). A missing profile row means "all defaults".
CREATE TABLE profiles (
    user_id       BIGINT PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    name_cyrillic TEXT     NOT NULL DEFAULT '',
    birthday      DATE,
    start_date    DATE,
    trial_until   DATE,
    working_hours SMALLINT NOT NULL DEFAULT 8 CHECK (working_hours BETWEEN 0 AND 24),
    billable      BOOLEAN  NOT NULL DEFAULT FALSE,
    -- contacts, skills, links, emergency contact, HR notes (see domain.ProfileDetails)
    details       JSONB    NOT NULL DEFAULT '{}',
    -- salary, rate, bank details (domain.ProfileFinance): only for viewSalary and the person
    finance       JSONB    NOT NULL DEFAULT '{}',
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Reference lists editable by HR.
CREATE TABLE directory_items (
    id   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind TEXT NOT NULL CHECK (kind IN ('specialization', 'tech_stack', 'candidate_channel')),
    name TEXT NOT NULL
);
CREATE UNIQUE INDEX directory_items_name_key ON directory_items (kind, lower(name));

INSERT INTO directory_items (kind, name) VALUES
    ('specialization', 'Frontend'), ('specialization', 'Backend'), ('specialization', 'Fullstack'),
    ('specialization', 'Mobile'), ('specialization', 'QA'), ('specialization', 'Design'),
    ('specialization', 'DevOps'), ('specialization', 'Project management'),
    ('tech_stack', 'React'), ('tech_stack', 'TypeScript'), ('tech_stack', 'Go'), ('tech_stack', 'Node.js'),
    ('tech_stack', 'Python'), ('tech_stack', 'Flutter'), ('tech_stack', 'PostgreSQL'), ('tech_stack', 'AWS'),
    ('candidate_channel', 'Djinni'), ('candidate_channel', 'DOU'), ('candidate_channel', 'LinkedIn'),
    ('candidate_channel', 'Referral');

CREATE TABLE user_documents (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id      BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    uploader_id  BIGINT  REFERENCES users (id) ON DELETE SET NULL,
    name         TEXT    NOT NULL,
    description  TEXT    NOT NULL DEFAULT '',
    type         TEXT    NOT NULL,
    private      BOOLEAN NOT NULL DEFAULT FALSE,
    link         TEXT    NOT NULL DEFAULT '', -- a document is a link or a file
    storage_key  TEXT    NOT NULL DEFAULT '',
    file_name    TEXT    NOT NULL DEFAULT '',
    content_type TEXT    NOT NULL DEFAULT '',
    size         BIGINT  NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX user_documents_user_idx ON user_documents (user_id);

-- Internal notes about a person (interview feedback, English check, dismissal…).
CREATE TABLE user_comments (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    author_id  BIGINT  REFERENCES users (id) ON DELETE SET NULL,
    kind       TEXT    NOT NULL,
    body       TEXT    NOT NULL,
    anonymous  BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX user_comments_user_idx ON user_comments (user_id, created_at DESC);

-- 360 feedback: start / stop / keep doing.
CREATE TABLE user_feedback (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id     BIGINT  NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    author_id   BIGINT  REFERENCES users (id) ON DELETE SET NULL,
    start_doing TEXT    NOT NULL DEFAULT '',
    stop_doing  TEXT    NOT NULL DEFAULT '',
    keep_doing  TEXT    NOT NULL DEFAULT '',
    anonymous   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX user_feedback_user_idx ON user_feedback (user_id, created_at DESC);

-- +goose Down
DROP TABLE user_feedback, user_comments, user_documents, directory_items, profiles;
