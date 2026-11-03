package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// --- task fields on CardRepo ---

func (r *CardRepo) SetDetails(ctx context.Context, cardID int64, d domain.TaskDetails) error {
	if err := exec(ctx, r.db, `UPDATE cards SET story_points = $2, budget_hours = $3, billable = $4, special_type = $5,
		milestone_id = $6, sprint_id = $7, version_id = $8, updated_at = now() WHERE id = $1`,
		cardID, d.StoryPoints, d.BudgetHours, d.Billable, d.SpecialType, d.MilestoneID, d.SprintID, d.VersionID); err != nil {
		return err
	}
	if err := exec(ctx, r.db, `DELETE FROM card_tags WHERE card_id = $1`, cardID); err != nil {
		return err
	}
	if len(d.TagIDs) == 0 {
		return nil
	}
	return exec(ctx, r.db, `INSERT INTO card_tags (card_id, tag_id) SELECT $1, unnest($2::bigint[])`, cardID, d.TagIDs)
}

func (r *CardRepo) SetTemplate(ctx context.Context, cardID int64, template bool) error {
	return exec(ctx, r.db, `UPDATE cards SET is_template = $2, updated_at = now() WHERE id = $1`, cardID, template)
}

func (r *CardRepo) Templates(ctx context.Context, projectID int64) ([]domain.Card, error) {
	rows, err := r.db.q(ctx).Query(ctx, cardSelect+` WHERE c.project_id = $1 AND c.is_template ORDER BY lower(c.title)`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Card, error) { return scanCard(row) })
}

// CopyFrom must run in a transaction: the new number, the card, its tags and checklists go together.
func (r *CardRepo) CopyFrom(ctx context.Context, templateID, columnID, authorID int64) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx, `
		WITH t AS (SELECT * FROM cards WHERE id = $1),
		seq AS (UPDATE projects SET card_seq = card_seq + 1 WHERE id = (SELECT project_id FROM t) RETURNING card_seq)
		INSERT INTO cards (project_id, column_id, number, title, description, priority, position, author_id, assignee_id,
			story_points, budget_hours, billable, special_type, milestone_id, sprint_id, version_id)
		SELECT t.project_id, $2, seq.card_seq, t.title, t.description, t.priority,
			(SELECT COALESCE(MAX(position), -1) + 1 FROM cards WHERE column_id = $2), $3, t.assignee_id,
			t.story_points, t.budget_hours, t.billable, t.special_type, t.milestone_id, t.sprint_id, t.version_id
		FROM t, seq
		RETURNING id`, templateID, columnID, authorID).Scan(&id)
	if err != nil {
		return 0, mapErr(err)
	}
	if err := exec(ctx, r.db, `INSERT INTO card_tags (card_id, tag_id) SELECT $2, tag_id FROM card_tags WHERE card_id = $1`, templateID, id); err != nil {
		return 0, err
	}
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id, title, position FROM card_checklists WHERE card_id = $1 ORDER BY position, id`, templateID)
	if err != nil {
		return 0, err
	}
	type list struct {
		id       int64
		title    string
		position int
	}
	lists, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (list, error) {
		var l list
		return l, row.Scan(&l.id, &l.title, &l.position)
	})
	if err != nil {
		return 0, err
	}
	for _, l := range lists {
		var newID int64
		if err := r.db.q(ctx).QueryRow(ctx, `INSERT INTO card_checklists (card_id, title, position) VALUES ($1, $2, $3) RETURNING id`,
			id, l.title, l.position).Scan(&newID); err != nil {
			return 0, err
		}
		if err := exec(ctx, r.db, `INSERT INTO checklist_items (checklist_id, text, position)
			SELECT $2, text, position FROM checklist_items WHERE checklist_id = $1`, l.id, newID); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *CardRepo) TopOfColumn(ctx context.Context, columnID int64, limit int) ([]int64, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id FROM cards WHERE column_id = $1 AND NOT is_template ORDER BY position, id LIMIT $2`, columnID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

// SetPlanning sets milestone_id, sprint_id or version_id; field is chosen by the use case, never by input.
func (r *CardRepo) SetPlanning(ctx context.Context, cardIDs []int64, field string, id *int64) error {
	switch field {
	case "milestone_id", "sprint_id", "version_id":
	default:
		return fmt.Errorf("unknown planning field %q", field)
	}
	return exec(ctx, r.db, `UPDATE cards SET `+field+` = $2, updated_at = now() WHERE id = ANY($1)`, cardIDs, id)
}

// MoveTo appends the cards, in their current order, to the bottom of a column.
func (r *CardRepo) MoveTo(ctx context.Context, cardIDs []int64, columnID int64) error {
	return exec(ctx, r.db, `
		WITH base AS (SELECT COALESCE(MAX(position), -1) AS top FROM cards WHERE column_id = $2),
		ordered AS (
			SELECT id, row_number() OVER (ORDER BY position, id) AS n FROM cards WHERE id = ANY($1)
		)
		UPDATE cards c SET column_id = $2, position = base.top + ordered.n, updated_at = now()
		FROM ordered, base WHERE c.id = ordered.id`, cardIDs, columnID)
}

func (r *CardRepo) HasTime(ctx context.Context, cardID int64) (bool, error) {
	var has bool
	err := r.db.q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM timelogs WHERE card_id = $1)`, cardID).Scan(&has)
	return has, err
}

// --- columns ---

func (r *ColumnRepo) SetStatus(ctx context.Context, columnID int64, status domain.ColumnStatus) error {
	return exec(ctx, r.db, `UPDATE board_columns SET status = $2 WHERE id = $1`, columnID, status)
}

func (r *ColumnRepo) FirstWithStatus(ctx context.Context, projectID int64, status domain.ColumnStatus) (int64, error) {
	var id int64
	err := r.db.q(ctx).QueryRow(ctx, `SELECT id FROM board_columns WHERE project_id = $1 AND status = $2 ORDER BY position LIMIT 1`,
		projectID, status).Scan(&id)
	return id, mapErr(err)
}

// --- tags ---

type TagRepo struct{ db *DB }

func NewTagRepo(db *DB) *TagRepo { return &TagRepo{db: db} }

func (r *TagRepo) List(ctx context.Context, projectID int64) ([]domain.Tag, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id, project_id, name, color FROM project_tags WHERE project_id = $1 ORDER BY lower(name)`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Tag, error) {
		var t domain.Tag
		return t, row.Scan(&t.ID, &t.ProjectID, &t.Name, &t.Color)
	})
}

func (r *TagRepo) Get(ctx context.Context, tagID int64) (domain.Tag, error) {
	var t domain.Tag
	err := r.db.q(ctx).QueryRow(ctx, `SELECT id, project_id, name, color FROM project_tags WHERE id = $1`, tagID).
		Scan(&t.ID, &t.ProjectID, &t.Name, &t.Color)
	return t, mapErr(err)
}

func (r *TagRepo) Create(ctx context.Context, t *domain.Tag) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO project_tags (project_id, name, color) VALUES ($1, $2, $3) RETURNING id`,
		t.ProjectID, t.Name, t.Color).Scan(&t.ID))
}

func (r *TagRepo) Update(ctx context.Context, t domain.Tag) error {
	return exec(ctx, r.db, `UPDATE project_tags SET name = $2, color = $3 WHERE id = $1`, t.ID, t.Name, t.Color)
}

func (r *TagRepo) Delete(ctx context.Context, tagID int64) error {
	return execOne(ctx, r.db, `DELETE FROM project_tags WHERE id = $1`, tagID)
}

func (r *TagRepo) CountInProject(ctx context.Context, projectID int64, ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var n int
	err := r.db.q(ctx).QueryRow(ctx, `SELECT COUNT(*) FROM project_tags WHERE project_id = $1 AND id = ANY($2)`, projectID, ids).Scan(&n)
	return n, err
}

// --- checklists ---

type ChecklistRepo struct{ db *DB }

func NewChecklistRepo(db *DB) *ChecklistRepo { return &ChecklistRepo{db: db} }

func (r *ChecklistRepo) List(ctx context.Context, cardID int64) ([]domain.Checklist, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT id, card_id, title, position, collapsed FROM card_checklists WHERE card_id = $1 ORDER BY position, id`, cardID)
	if err != nil {
		return nil, err
	}
	lists, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Checklist, error) {
		var c domain.Checklist
		c.Items = []domain.ChecklistItem{}
		return c, row.Scan(&c.ID, &c.CardID, &c.Title, &c.Position, &c.Collapsed)
	})
	if err != nil || len(lists) == 0 {
		return lists, err
	}
	rows, err = r.db.q(ctx).Query(ctx, `SELECT i.id, i.checklist_id, i.text, i.done, i.position FROM checklist_items i
		JOIN card_checklists c ON c.id = i.checklist_id WHERE c.card_id = $1 ORDER BY i.position, i.id`, cardID)
	if err != nil {
		return nil, err
	}
	items, err := pgx.CollectRows(rows, scanItem)
	if err != nil {
		return nil, err
	}
	byID := map[int64]*domain.Checklist{}
	for i := range lists {
		byID[lists[i].ID] = &lists[i]
	}
	for _, it := range items {
		byID[it.ChecklistID].Items = append(byID[it.ChecklistID].Items, it)
	}
	return lists, nil
}

func scanItem(row pgx.CollectableRow) (domain.ChecklistItem, error) {
	var it domain.ChecklistItem
	return it, row.Scan(&it.ID, &it.ChecklistID, &it.Text, &it.Done, &it.Position)
}

func (r *ChecklistRepo) Get(ctx context.Context, id int64) (domain.Checklist, error) {
	var c domain.Checklist
	err := r.db.q(ctx).QueryRow(ctx, `SELECT id, card_id, title, position, collapsed FROM card_checklists WHERE id = $1`, id).
		Scan(&c.ID, &c.CardID, &c.Title, &c.Position, &c.Collapsed)
	return c, mapErr(err)
}

func (r *ChecklistRepo) Create(ctx context.Context, c *domain.Checklist) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO card_checklists (card_id, title, position)
		VALUES ($1, $2, (SELECT COALESCE(MAX(position), -1) + 1 FROM card_checklists WHERE card_id = $1)) RETURNING id, position`,
		c.CardID, c.Title).Scan(&c.ID, &c.Position))
}

func (r *ChecklistRepo) Update(ctx context.Context, c domain.Checklist) error {
	return exec(ctx, r.db, `UPDATE card_checklists SET title = $2, collapsed = $3 WHERE id = $1`, c.ID, c.Title, c.Collapsed)
}

func (r *ChecklistRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM card_checklists WHERE id = $1`, id)
}

func (r *ChecklistRepo) Item(ctx context.Context, id int64) (domain.ChecklistItem, error) {
	var it domain.ChecklistItem
	err := r.db.q(ctx).QueryRow(ctx, `SELECT id, checklist_id, text, done, position FROM checklist_items WHERE id = $1`, id).
		Scan(&it.ID, &it.ChecklistID, &it.Text, &it.Done, &it.Position)
	return it, mapErr(err)
}

func (r *ChecklistRepo) AddItem(ctx context.Context, it *domain.ChecklistItem) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO checklist_items (checklist_id, text, position)
		VALUES ($1, $2, (SELECT COALESCE(MAX(position), -1) + 1 FROM checklist_items WHERE checklist_id = $1)) RETURNING id, position`,
		it.ChecklistID, it.Text).Scan(&it.ID, &it.Position))
}

func (r *ChecklistRepo) UpdateItem(ctx context.Context, it domain.ChecklistItem) error {
	return exec(ctx, r.db, `UPDATE checklist_items SET text = $2, done = $3 WHERE id = $1`, it.ID, it.Text, it.Done)
}

func (r *ChecklistRepo) DeleteItem(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM checklist_items WHERE id = $1`, id)
}

// --- comments ---

type CardCommentRepo struct{ db *DB }

func NewCardCommentRepo(db *DB) *CardCommentRepo { return &CardCommentRepo{db: db} }

const cardCommentSelect = `SELECT c.id, c.card_id, c.author_id, COALESCE(u.name, ''), c.body, c.created_at, c.updated_at
	FROM card_comments c LEFT JOIN users u ON u.id = c.author_id`

func scanCardComment(row pgx.Row) (domain.CardComment, error) {
	var c domain.CardComment
	err := row.Scan(&c.ID, &c.CardID, &c.AuthorID, &c.AuthorName, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	return c, mapErr(err)
}

func (r *CardCommentRepo) List(ctx context.Context, cardID int64) ([]domain.CardComment, error) {
	rows, err := r.db.q(ctx).Query(ctx, cardCommentSelect+` WHERE c.card_id = $1 ORDER BY c.created_at`, cardID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.CardComment, error) { return scanCardComment(row) })
}

func (r *CardCommentRepo) Get(ctx context.Context, id int64) (domain.CardComment, error) {
	return scanCardComment(r.db.q(ctx).QueryRow(ctx, cardCommentSelect+` WHERE c.id = $1`, id))
}

func (r *CardCommentRepo) Create(ctx context.Context, c *domain.CardComment) error {
	return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO card_comments (card_id, author_id, body) VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at`, c.CardID, c.AuthorID, c.Body).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt))
}

func (r *CardCommentRepo) Update(ctx context.Context, id int64, body string) error {
	return exec(ctx, r.db, `UPDATE card_comments SET body = $2, updated_at = now() WHERE id = $1`, id, body)
}

func (r *CardCommentRepo) Delete(ctx context.Context, id int64) error {
	return execOne(ctx, r.db, `DELETE FROM card_comments WHERE id = $1`, id)
}

// --- history ---

type HistoryRepo struct{ db *DB }

func NewHistoryRepo(db *DB) *HistoryRepo { return &HistoryRepo{db: db} }

func (r *HistoryRepo) Add(ctx context.Context, entries ...domain.HistoryEntry) error {
	for _, e := range entries {
		if err := exec(ctx, r.db, `INSERT INTO card_history (card_id, actor_id, field, old_value, new_value) VALUES ($1, $2, $3, $4, $5)`,
			e.CardID, e.ActorID, e.Field, e.Old, e.New); err != nil {
			return err
		}
	}
	return nil
}

func (r *HistoryRepo) List(ctx context.Context, cardID int64, offset, limit int) ([]domain.HistoryEntry, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT h.id, h.card_id, h.actor_id, COALESCE(u.name, ''), h.field, h.old_value, h.new_value, h.created_at
		FROM card_history h LEFT JOIN users u ON u.id = h.actor_id
		WHERE h.card_id = $1 ORDER BY h.created_at DESC, h.id DESC OFFSET $2 LIMIT $3`, cardID, offset, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.HistoryEntry, error) {
		var e domain.HistoryEntry
		return e, row.Scan(&e.ID, &e.CardID, &e.ActorID, &e.ActorName, &e.Field, &e.Old, &e.New, &e.CreatedAt)
	})
}

// --- planning ---

type PlanningRepo struct{ db *DB }

func NewPlanningRepo(db *DB) *PlanningRepo { return &PlanningRepo{db: db} }

// planningTable guards the table names that are spliced into SQL.
func planningTable(table string) error {
	switch table {
	case "milestones", "sprints", "versions":
		return nil
	}
	return fmt.Errorf("unknown planning table %q", table)
}

// planningCounts: how many tasks point at the item and how many of them are in a done column.
func planningCounts(field string) string {
	return `(SELECT COUNT(*) FROM cards c WHERE c.` + field + ` = x.id AND NOT c.is_template),
		(SELECT COUNT(*) FROM cards c JOIN board_columns bc ON bc.id = c.column_id
			WHERE c.` + field + ` = x.id AND NOT c.is_template AND bc.status = 'done')`
}

func (r *PlanningRepo) Milestones(ctx context.Context, projectID int64) ([]domain.Milestone, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT x.id, x.project_id, x.name, x.description, x.start_date, x.end_date, `+
		planningCounts("milestone_id")+` FROM milestones x WHERE x.project_id = $1 ORDER BY x.start_date NULLS LAST, x.id`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Milestone, error) {
		var m domain.Milestone
		var start, end *time.Time
		err := row.Scan(&m.ID, &m.ProjectID, &m.Name, &m.Description, &start, &end, &m.CardCount, &m.DoneCount)
		m.Start, m.End = toDate(start), toDate(end)
		return m, err
	})
}

func (r *PlanningRepo) SaveMilestone(ctx context.Context, m *domain.Milestone) error {
	if m.ID == 0 {
		return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO milestones (project_id, name, description, start_date, end_date)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, m.ProjectID, m.Name, m.Description, fromDate(m.Start), fromDate(m.End)).Scan(&m.ID))
	}
	return exec(ctx, r.db, `UPDATE milestones SET name = $2, description = $3, start_date = $4, end_date = $5 WHERE id = $1`,
		m.ID, m.Name, m.Description, fromDate(m.Start), fromDate(m.End))
}

func (r *PlanningRepo) Sprints(ctx context.Context, projectID int64) ([]domain.Sprint, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT x.id, x.project_id, x.name, x.goal, x.start_date, x.end_date, x.status, `+
		planningCounts("sprint_id")+` FROM sprints x WHERE x.project_id = $1 ORDER BY x.start_date NULLS LAST, x.id`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Sprint, error) {
		var s domain.Sprint
		var start, end *time.Time
		err := row.Scan(&s.ID, &s.ProjectID, &s.Name, &s.Goal, &start, &end, &s.Status, &s.CardCount, &s.DoneCount)
		s.Start, s.End = toDate(start), toDate(end)
		return s, err
	})
}

func (r *PlanningRepo) SaveSprint(ctx context.Context, s *domain.Sprint) error {
	if s.ID == 0 {
		return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO sprints (project_id, name, goal, start_date, end_date, status)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, s.ProjectID, s.Name, s.Goal, fromDate(s.Start), fromDate(s.End), s.Status).Scan(&s.ID))
	}
	return exec(ctx, r.db, `UPDATE sprints SET name = $2, goal = $3, start_date = $4, end_date = $5, status = $6 WHERE id = $1`,
		s.ID, s.Name, s.Goal, fromDate(s.Start), fromDate(s.End), s.Status)
}

func (r *PlanningRepo) Versions(ctx context.Context, projectID int64) ([]domain.Version, error) {
	rows, err := r.db.q(ctx).Query(ctx, `SELECT x.id, x.project_id, x.name, x.description, x.release_date, x.status, `+
		planningCounts("version_id")+` FROM versions x WHERE x.project_id = $1 ORDER BY x.release_date NULLS LAST, x.id`, projectID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Version, error) {
		var v domain.Version
		var release *time.Time
		err := row.Scan(&v.ID, &v.ProjectID, &v.Name, &v.Description, &release, &v.Status, &v.CardCount, &v.DoneCount)
		v.ReleaseDate = toDate(release)
		return v, err
	})
}

func (r *PlanningRepo) SaveVersion(ctx context.Context, v *domain.Version) error {
	if v.ID == 0 {
		return mapErr(r.db.q(ctx).QueryRow(ctx, `INSERT INTO versions (project_id, name, description, release_date, status)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`, v.ProjectID, v.Name, v.Description, fromDate(v.ReleaseDate), v.Status).Scan(&v.ID))
	}
	return exec(ctx, r.db, `UPDATE versions SET name = $2, description = $3, release_date = $4, status = $5 WHERE id = $1`,
		v.ID, v.Name, v.Description, fromDate(v.ReleaseDate), v.Status)
}

func (r *PlanningRepo) ProjectOf(ctx context.Context, table string, id int64) (int64, error) {
	if err := planningTable(table); err != nil {
		return 0, err
	}
	var projectID int64
	err := r.db.q(ctx).QueryRow(ctx, `SELECT project_id FROM `+table+` WHERE id = $1`, id).Scan(&projectID)
	return projectID, mapErr(err)
}

func (r *PlanningRepo) InProject(ctx context.Context, table string, id, projectID int64) (bool, error) {
	if err := planningTable(table); err != nil {
		return false, err
	}
	var ok bool
	err := r.db.q(ctx).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM `+table+` WHERE id = $1 AND project_id = $2)`, id, projectID).Scan(&ok)
	return ok, err
}

func (r *PlanningRepo) Delete(ctx context.Context, table string, id int64) error {
	if err := planningTable(table); err != nil {
		return err
	}
	return execOne(ctx, r.db, `DELETE FROM `+table+` WHERE id = $1`, id)
}

func (r *CardRepo) Find(ctx context.Context, projectID int64, query string, number *int64, limit int) ([]domain.Card, error) {
	rows, err := r.db.q(ctx).Query(ctx, cardSelect+` WHERE c.project_id = $1 AND NOT c.is_template
		AND ($2 = '' OR c.title ILIKE '%' || $3 || '%' OR c.number = $4)
		ORDER BY c.number = $4 DESC, c.updated_at DESC LIMIT $5`, projectID, query, escapeLike(query), number, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Card, error) { return scanCard(row) })
}
