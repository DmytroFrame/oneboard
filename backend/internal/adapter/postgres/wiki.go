package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// --- articles ---

type WikiRepo struct{ db *DB }

func NewWikiRepo(db *DB) *WikiRepo { return &WikiRepo{db: db} }

func (r *WikiRepo) All(ctx context.Context, orgID int64) ([]domain.WikiArticle, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT a.id, a.parent_id, a.project_id, a.owner_id, COALESCE(o.name, ''), a.title,
			a.inherit_access, a.company_access, a.archived, a.created_at, a.updated_at, a.updated_by, COALESCE(e.name, '')
		FROM wiki_articles a LEFT JOIN users o ON o.id = a.owner_id LEFT JOIN users e ON e.id = a.updated_by
		WHERE a.organization_id = $1 ORDER BY lower(a.title), a.id`, orgID)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.WikiArticle, error) {
		var a domain.WikiArticle
		err := row.Scan(&a.ID, &a.ParentID, &a.ProjectID, &a.OwnerID, &a.OwnerName, &a.Title,
			&a.InheritAccess, &a.CompanyAccess, &a.Archived, &a.CreatedAt, &a.UpdatedAt, &a.UpdatedBy, &a.UpdatedByName)
		return a, err
	})
	if err != nil {
		return nil, err
	}
	grants, err := r.db.q(ctx).Query(ctx, `SELECT w.article_id, COALESCE(w.user_id, 0), COALESCE(w.group_id, 0), w.level
		FROM wiki_access w JOIN wiki_articles a ON a.id = w.article_id WHERE a.organization_id = $1`, orgID)
	if err != nil {
		return nil, err
	}
	defer grants.Close()
	byArticle := map[int64][]domain.WikiGrant{}
	for grants.Next() {
		var id int64
		var g domain.WikiGrant
		if err := grants.Scan(&id, &g.UserID, &g.GroupID, &g.Level); err != nil {
			return nil, err
		}
		byArticle[id] = append(byArticle[id], g)
	}
	if err := grants.Err(); err != nil {
		return nil, err
	}
	for i := range list {
		list[i].Grants = byArticle[list[i].ID]
	}
	return list, nil
}

func (r *WikiRepo) Content(ctx context.Context, id int64) (string, error) {
	var s string
	return s, mapErr(r.db.q(ctx).QueryRow(ctx, `SELECT content FROM wiki_articles WHERE id = $1`, id).Scan(&s))
}

func (r *WikiRepo) Create(ctx context.Context, orgID int64, a *domain.WikiArticle) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO wiki_articles (parent_id, project_id, owner_id, title, content, inherit_access,
			company_access, updated_by, organization_id) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id, created_at, updated_at`,
		a.ParentID, a.ProjectID, a.OwnerID, a.Title, a.Content, a.InheritAccess, a.CompanyAccess, a.UpdatedBy, orgID).
		Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt))
}

func (r *WikiRepo) Update(ctx context.Context, a domain.WikiArticle) error {
	return execOne(ctx, r.db, `UPDATE wiki_articles SET title = $2, content = $3, parent_id = $4, inherit_access = $5,
		updated_by = $6, updated_at = now() WHERE id = $1`, a.ID, a.Title, a.Content, a.ParentID, a.InheritAccess, a.UpdatedBy)
}

func (r *WikiRepo) SetArchived(ctx context.Context, id int64, archived bool) error {
	return execOne(ctx, r.db, `UPDATE wiki_articles SET archived = $2 WHERE id = $1`, id, archived)
}

func (r *WikiRepo) SetAccess(ctx context.Context, id int64, inherit bool, company domain.WikiLevel, grants []domain.WikiGrant) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		if err := exec(ctx, r.db, `UPDATE wiki_articles SET inherit_access = $2, company_access = $3 WHERE id = $1`, id, inherit, company); err != nil {
			return err
		}
		if err := exec(ctx, r.db, `DELETE FROM wiki_access WHERE article_id = $1`, id); err != nil {
			return err
		}
		for _, g := range grants {
			var user, group *int64
			if g.UserID != 0 {
				user = &g.UserID
			} else {
				group = &g.GroupID
			}
			if err := exec(ctx, r.db, `INSERT INTO wiki_access (article_id, user_id, group_id, level)
				SELECT $1, $2, $3, $4 FROM wiki_articles a WHERE a.id = $1
					-- only people and groups of the article's own organization
					AND ($2::bigint IS NULL OR EXISTS (SELECT 1 FROM organization_members m
						WHERE m.organization_id = a.organization_id AND m.user_id = $2))
					AND ($3::bigint IS NULL OR EXISTS (SELECT 1 FROM wiki_groups g WHERE g.id = $3 AND g.organization_id = a.organization_id))`, id, user, group, g.Level); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *WikiRepo) Search(ctx context.Context, orgID int64, q string) ([]int64, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id FROM wiki_articles WHERE organization_id = $1 AND (title ILIKE $2 OR content ILIKE $2)`,
		orgID, "%"+escapeLike(q)+"%")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func (r *WikiRepo) AddVersion(ctx context.Context, v domain.WikiVersion) error {
	return exec(ctx, r.db, `INSERT INTO wiki_versions (article_id, title, content, author_id) VALUES ($1, $2, $3, $4)`,
		v.ArticleID, v.Title, v.Content, v.AuthorID)
}

func (r *WikiRepo) Versions(ctx context.Context, articleID int64) ([]domain.WikiVersion, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT v.id, v.article_id, v.title, v.author_id, COALESCE(u.name, ''), v.created_at
		FROM wiki_versions v LEFT JOIN users u ON u.id = v.author_id WHERE v.article_id = $1 ORDER BY v.created_at DESC, v.id DESC`, articleID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.WikiVersion, error) {
		var v domain.WikiVersion
		return v, row.Scan(&v.ID, &v.ArticleID, &v.Title, &v.AuthorID, &v.AuthorName, &v.CreatedAt)
	})
}

func (r *WikiRepo) Version(ctx context.Context, id int64) (domain.WikiVersion, error) {
	var v domain.WikiVersion
	err := r.db.q(ctx).QueryRow(ctx, `SELECT v.id, v.article_id, v.title, v.content, v.author_id, COALESCE(u.name, ''), v.created_at
		FROM wiki_versions v LEFT JOIN users u ON u.id = v.author_id WHERE v.id = $1`, id).
		Scan(&v.ID, &v.ArticleID, &v.Title, &v.Content, &v.AuthorID, &v.AuthorName, &v.CreatedAt)
	return v, mapErr(err)
}

func (r *WikiRepo) Viewed(ctx context.Context, userID, articleID int64) error {
	return exec(ctx, r.db, `INSERT INTO wiki_views (user_id, article_id) VALUES ($1, $2)
		ON CONFLICT (user_id, article_id) DO UPDATE SET viewed_at = now()`, userID, articleID)
}

func (r *WikiRepo) Recent(ctx context.Context, userID int64, limit int) ([]int64, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT article_id FROM wiki_views WHERE user_id = $1 ORDER BY viewed_at DESC LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func (r *WikiRepo) ViewerContext(ctx context.Context, userID int64) ([]int64, []int64, error) {
	var groups, projects []int64
	err := r.db.q(ctx).QueryRow(ctx, `SELECT
		COALESCE((SELECT array_agg(group_id) FROM wiki_group_members WHERE user_id = $1), '{}'),
		COALESCE((SELECT array_agg(project_id) FROM project_members WHERE user_id = $1), '{}')`, userID).Scan(&groups, &projects)
	return groups, projects, err
}

// --- groups ---

type WikiGroupRepo struct{ db *DB }

func NewWikiGroupRepo(db *DB) *WikiGroupRepo { return &WikiGroupRepo{db: db} }

const wikiGroupSelect = `SELECT g.id, g.name, g.owner_id,
	COALESCE((SELECT array_agg(m.user_id ORDER BY m.user_id) FROM wiki_group_members m WHERE m.group_id = g.id), '{}')
	FROM wiki_groups g`

func scanWikiGroup(row pgx.Row) (domain.WikiGroup, error) {
	var g domain.WikiGroup
	return g, mapErr(row.Scan(&g.ID, &g.Name, &g.OwnerID, &g.MemberIDs))
}

func (r *WikiGroupRepo) List(ctx context.Context, orgID int64) ([]domain.WikiGroup, error) {
	rows, err := r.db.q(ctx).Query(ctx, wikiGroupSelect+` WHERE g.organization_id = $1 ORDER BY lower(g.name), g.id`, orgID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.WikiGroup, error) { return scanWikiGroup(row) })
}

func (r *WikiGroupRepo) Get(ctx context.Context, orgID, id int64) (domain.WikiGroup, error) {
	return scanWikiGroup(r.db.q(ctx).QueryRow(ctx, wikiGroupSelect+` WHERE g.organization_id = $1 AND g.id = $2`, orgID, id))
}

// setMembers keeps only people of the group's organization.
func (r *WikiGroupRepo) setMembers(ctx context.Context, id int64, members []int64) error {
	if err := exec(ctx, r.db, `DELETE FROM wiki_group_members WHERE group_id = $1`, id); err != nil {
		return err
	}
	return exec(ctx, r.db, `INSERT INTO wiki_group_members (group_id, user_id)
		SELECT $1, m.user_id FROM organization_members m JOIN wiki_groups g ON g.id = $1 AND g.organization_id = m.organization_id
		WHERE m.user_id = ANY($2)`, id, members)
}

func (r *WikiGroupRepo) Create(ctx context.Context, orgID int64, g *domain.WikiGroup) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		if err := r.db.q(ctx).QueryRow(ctx, `INSERT INTO wiki_groups (organization_id, name, owner_id) VALUES ($1, $2, $3) RETURNING id`,
			orgID, g.Name, g.OwnerID).Scan(&g.ID); err != nil {
			return err
		}
		return r.setMembers(ctx, g.ID, g.MemberIDs)
	})
}

func (r *WikiGroupRepo) Update(ctx context.Context, g domain.WikiGroup) error {
	return r.db.WithinTx(ctx, func(ctx context.Context) error {
		if err := execOne(ctx, r.db, `UPDATE wiki_groups SET name = $2 WHERE id = $1`, g.ID, g.Name); err != nil {
			return err
		}
		return r.setMembers(ctx, g.ID, g.MemberIDs)
	})
}

func (r *WikiGroupRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM wiki_groups WHERE id = $1`, id)
}

func (r *WikiGroupRepo) Usage(ctx context.Context, id int64) (int, error) {
	var n int
	return n, r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(DISTINCT article_id) FROM wiki_access WHERE group_id = $1`, id).Scan(&n)
}

// --- project comments ---

type ProjectCommentRepo struct{ db *DB }

func NewProjectCommentRepo(db *DB) *ProjectCommentRepo { return &ProjectCommentRepo{db: db} }

const projectCommentSelect = `SELECT c.id, c.project_id, c.author_id, COALESCE(u.name, ''), c.category, c.title, c.body, c.created_at, c.updated_at
	FROM project_comments c LEFT JOIN users u ON u.id = c.author_id`

func scanProjectComment(row pgx.Row) (domain.ProjectComment, error) {
	var c domain.ProjectComment
	return c, mapErr(row.Scan(&c.ID, &c.ProjectID, &c.AuthorID, &c.AuthorName, &c.Category, &c.Title, &c.Body, &c.CreatedAt, &c.UpdatedAt))
}

func (r *ProjectCommentRepo) List(ctx context.Context, projectID int64, category domain.ProjectCommentCategory) ([]domain.ProjectComment, error) {
	rows, err := r.db.q(ctx).Query(ctx, projectCommentSelect+` WHERE c.project_id = $1 AND ($2 = '' OR c.category = $2)
		ORDER BY c.created_at DESC, c.id DESC`, projectID, string(category))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ProjectComment, error) { return scanProjectComment(row) })
}

func (r *ProjectCommentRepo) Get(ctx context.Context, id int64) (domain.ProjectComment, error) {
	return scanProjectComment(r.db.q(ctx).QueryRow(ctx, projectCommentSelect+` WHERE c.id = $1`, id))
}

func (r *ProjectCommentRepo) Create(ctx context.Context, c *domain.ProjectComment) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO project_comments (project_id, author_id, category, title, body)
		VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`, c.ProjectID, c.AuthorID, c.Category, c.Title, c.Body).
		Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt))
}

func (r *ProjectCommentRepo) Update(ctx context.Context, c domain.ProjectComment) error {
	return execOne(ctx, r.db, `UPDATE project_comments SET category = $2, title = $3, body = $4, updated_at = now() WHERE id = $1`,
		c.ID, c.Category, c.Title, c.Body)
}

func (r *ProjectCommentRepo) ProjectOrg(ctx context.Context, projectID int64) (int64, error) {
	var org int64
	return org, mapErr(r.db.q(ctx).QueryRow(ctx, `SELECT organization_id FROM projects WHERE id = $1`, projectID).Scan(&org))
}

func (r *ProjectCommentRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM project_comments WHERE id = $1`, id)
}
