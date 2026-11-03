-- +goose Up
-- Wiki (OmiTeam knowledge base): articles in a tree, shared with people, groups or the whole company,
-- with every saved version kept.
CREATE TABLE wiki_groups (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       TEXT NOT NULL,
    owner_id   BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE wiki_group_members (
    group_id BIGINT NOT NULL REFERENCES wiki_groups (id) ON DELETE CASCADE,
    user_id  BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, user_id)
);

CREATE TABLE wiki_articles (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_id      BIGINT REFERENCES wiki_articles (id) ON DELETE SET NULL,
    project_id     BIGINT REFERENCES projects (id) ON DELETE CASCADE,
    owner_id       BIGINT REFERENCES users (id) ON DELETE SET NULL,
    title          TEXT    NOT NULL,
    content        TEXT    NOT NULL DEFAULT '',
    -- use the parent's access list instead of its own
    inherit_access BOOLEAN NOT NULL DEFAULT FALSE,
    -- everyone in the company: none, read or edit
    company_access TEXT    NOT NULL DEFAULT 'none' CHECK (company_access IN ('none', 'read', 'edit')),
    archived       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     BIGINT REFERENCES users (id) ON DELETE SET NULL
);
CREATE INDEX wiki_articles_parent_idx ON wiki_articles (parent_id);
CREATE INDEX wiki_articles_project_idx ON wiki_articles (project_id);

-- Exactly one of user_id / group_id per row.
CREATE TABLE wiki_access (
    article_id BIGINT NOT NULL REFERENCES wiki_articles (id) ON DELETE CASCADE,
    user_id    BIGINT REFERENCES users (id) ON DELETE CASCADE,
    group_id   BIGINT REFERENCES wiki_groups (id) ON DELETE CASCADE,
    level      TEXT NOT NULL CHECK (level IN ('read', 'edit')),
    CHECK ((user_id IS NULL) <> (group_id IS NULL))
);
CREATE UNIQUE INDEX wiki_access_user_key ON wiki_access (article_id, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX wiki_access_group_key ON wiki_access (article_id, group_id) WHERE group_id IS NOT NULL;

CREATE TABLE wiki_versions (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    article_id BIGINT NOT NULL REFERENCES wiki_articles (id) ON DELETE CASCADE,
    title      TEXT   NOT NULL,
    content    TEXT   NOT NULL,
    author_id  BIGINT REFERENCES users (id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX wiki_versions_article_idx ON wiki_versions (article_id, created_at DESC);

-- "Recent" on the wiki home: what each person opened last.
CREATE TABLE wiki_views (
    user_id    BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    article_id BIGINT NOT NULL REFERENCES wiki_articles (id) ON DELETE CASCADE,
    viewed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, article_id)
);

-- Project comments (OmiTeam), by category; being phased out in favour of the project wiki.
CREATE TABLE project_comments (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id  BIGINT NOT NULL REFERENCES projects (id) ON DELETE CASCADE,
    author_id   BIGINT REFERENCES users (id) ON DELETE SET NULL,
    category    TEXT   NOT NULL CHECK (category IN ('call', 'requirements', 'design', 'estimate', 'task_management', 'other')),
    title       TEXT   NOT NULL,
    body        TEXT   NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX project_comments_project_idx ON project_comments (project_id, created_at DESC);

-- +goose Down
DROP TABLE project_comments, wiki_views, wiki_versions, wiki_access, wiki_articles, wiki_group_members, wiki_groups;
