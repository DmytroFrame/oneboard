// Package tasks adds OmiTeam's task management to OneBoard's boards: task details (story points,
// budget, special type, planning, tags), checklists, comments, history, templates, column statuses,
// bulk moves, milestones, sprints and versions. The board itself (cards, columns, moving, the shared
// editor) stays in package board.
package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("tasks", fx.Provide(NewService)))

type Cards interface {
	Get(ctx context.Context, cardID int64) (domain.Card, error)
	SetDetails(ctx context.Context, cardID int64, d domain.TaskDetails) error
	SetTemplate(ctx context.Context, cardID int64, template bool) error
	Templates(ctx context.Context, projectID int64) ([]domain.Card, error)
	// CopyFrom creates a card in columnID copying the template's text, fields, tags and checklists.
	CopyFrom(ctx context.Context, templateID, columnID, authorID int64) (int64, error)
	// TopOfColumn returns up to limit card ids from the top of a column (templates excluded).
	TopOfColumn(ctx context.Context, columnID int64, limit int) ([]int64, error)
	// SetPlanning sets one planning field (milestone_id, sprint_id or version_id) on cards.
	SetPlanning(ctx context.Context, cardIDs []int64, field string, id *int64) error
	MoveTo(ctx context.Context, cardIDs []int64, columnID int64) error
	// Find matches a title fragment or the task number across the whole project (templates excluded).
	Find(ctx context.Context, projectID int64, query string, number *int64, limit int) ([]domain.Card, error)
}

type Columns interface {
	Get(ctx context.Context, columnID int64) (domain.Column, error)
	SetStatus(ctx context.Context, columnID int64, status domain.ColumnStatus) error
	FirstWithStatus(ctx context.Context, projectID int64, status domain.ColumnStatus) (int64, error) // domain.ErrNoRows if none
}

type Tags interface {
	List(ctx context.Context, projectID int64) ([]domain.Tag, error)
	Get(ctx context.Context, tagID int64) (domain.Tag, error)
	Create(ctx context.Context, t *domain.Tag) error // domain.ErrDuplicate on a taken name
	Update(ctx context.Context, t domain.Tag) error
	Delete(ctx context.Context, tagID int64) error
	CountInProject(ctx context.Context, projectID int64, ids []int64) (int, error)
}

type Checklists interface {
	List(ctx context.Context, cardID int64) ([]domain.Checklist, error)
	Get(ctx context.Context, id int64) (domain.Checklist, error)
	Create(ctx context.Context, c *domain.Checklist) error
	Update(ctx context.Context, c domain.Checklist) error
	Delete(ctx context.Context, id int64) error
	Item(ctx context.Context, id int64) (domain.ChecklistItem, error)
	AddItem(ctx context.Context, it *domain.ChecklistItem) error
	UpdateItem(ctx context.Context, it domain.ChecklistItem) error
	DeleteItem(ctx context.Context, id int64) error
}

type Comments interface {
	List(ctx context.Context, cardID int64) ([]domain.CardComment, error)
	Get(ctx context.Context, id int64) (domain.CardComment, error)
	Create(ctx context.Context, c *domain.CardComment) error
	Update(ctx context.Context, id int64, body string) error
	Delete(ctx context.Context, id int64) error
}

// History records task changes; board writes to it as well.
type History interface {
	Add(ctx context.Context, entries ...domain.HistoryEntry) error
	List(ctx context.Context, cardID int64, offset, limit int) ([]domain.HistoryEntry, error)
}

type Planning interface {
	Milestones(ctx context.Context, projectID int64) ([]domain.Milestone, error)
	SaveMilestone(ctx context.Context, m *domain.Milestone) error // insert when ID is 0
	Sprints(ctx context.Context, projectID int64) ([]domain.Sprint, error)
	SaveSprint(ctx context.Context, s *domain.Sprint) error
	Versions(ctx context.Context, projectID int64) ([]domain.Version, error)
	SaveVersion(ctx context.Context, v *domain.Version) error
	// ProjectOf returns the project of a planning item ("milestones", "sprints", "versions").
	ProjectOf(ctx context.Context, table string, id int64) (int64, error)
	Delete(ctx context.Context, table string, id int64) error
	// InProject reports whether the planning item belongs to the project.
	InProject(ctx context.Context, table string, id, projectID int64) (bool, error)
}

type Service struct {
	cards      Cards
	columns    Columns
	tags       Tags
	checklists Checklists
	comments   Comments
	history    History
	planning   Planning
	members    usecase.Memberships
	events     usecase.BoardEvents
	tx         usecase.Transactor
	log        *slog.Logger
}

type Deps struct {
	fx.In
	Cards      Cards
	Columns    Columns
	Tags       Tags
	Checklists Checklists
	Comments   Comments
	History    History
	Planning   Planning
	Members    usecase.Memberships
	Events     usecase.BoardEvents
	Tx         usecase.Transactor
	Log        *slog.Logger
}

func NewService(d Deps) *Service {
	return &Service{d.Cards, d.Columns, d.Tags, d.Checklists, d.Comments, d.History, d.Planning, d.Members, d.Events, d.Tx, d.Log}
}

var projectNotFound = domain.ErrProjectNotFound

// member checks the user is in the project and returns their role.
func (s *Service) member(ctx context.Context, actor domain.User, projectID int64) (domain.Role, error) {
	return usecase.RequireMember(ctx, s.members, projectID, actor.ID, projectNotFound)
}

// manager: the project's owner or admin, or a company manager (editProject) in the project.
func (s *Service) manager(ctx context.Context, actor domain.User, projectID int64) error {
	role, err := s.member(ctx, actor, projectID)
	if err != nil {
		return err
	}
	if role == domain.RoleOwner || role == domain.RoleAdmin || actor.Can(domain.PermEditProject) {
		return nil
	}
	return domain.ErrPermissionDenied
}

// card loads a card the actor can see.
func (s *Service) card(ctx context.Context, actor domain.User, cardID int64) (domain.Card, error) {
	c, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return c, usecase.NotFoundAs(err, domain.ErrCardNotFound)
	}
	if _, err := s.member(ctx, actor, c.ProjectID); err != nil {
		return c, domain.ErrCardNotFound
	}
	return c, nil
}

func (s *Service) changed(ctx context.Context, actorID, cardID int64) (domain.Card, error) {
	c, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return c, err
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventCardUpdated, ProjectID: c.ProjectID, ActorID: actorID, Card: &c})
	return c, nil
}

// --- details ---

// SetDetails changes a task's OmiTeam fields and records what changed.
func (s *Service) SetDetails(ctx context.Context, actor domain.User, cardID int64, d domain.TaskDetails) (domain.Card, error) {
	c, err := s.card(ctx, actor, cardID)
	if err != nil {
		return c, err
	}
	if err := d.Validate(); err != nil {
		return c, err
	}
	slices.Sort(d.TagIDs)
	d.TagIDs = slices.Compact(d.TagIDs)
	if n, err := s.tags.CountInProject(ctx, c.ProjectID, d.TagIDs); err != nil {
		return c, err
	} else if n != len(d.TagIDs) {
		return c, domain.ErrTagNotFound
	}
	for _, p := range []struct {
		table string
		id    *int64
	}{{"milestones", d.MilestoneID}, {"sprints", d.SprintID}, {"versions", d.VersionID}} {
		if p.id == nil {
			continue
		}
		if ok, err := s.planning.InProject(ctx, p.table, *p.id, c.ProjectID); err != nil {
			return c, err
		} else if !ok {
			return c, domain.ErrPlanningNotFound
		}
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.cards.SetDetails(ctx, cardID, d); err != nil {
			return err
		}
		return s.history.Add(ctx, diffDetails(cardID, actor.ID, c.TaskDetails, d)...)
	})
	if err != nil {
		return c, err
	}
	return s.changed(ctx, actor.ID, cardID)
}

func diffDetails(cardID, actorID int64, old, next domain.TaskDetails) []domain.HistoryEntry {
	var out []domain.HistoryEntry
	add := func(field, a, b string) {
		if a != b {
			out = append(out, domain.HistoryEntry{CardID: cardID, ActorID: &actorID, Field: field, Old: a, New: b})
		}
	}
	add("storyPoints", ptrString(old.StoryPoints), ptrString(next.StoryPoints))
	add("budget", ptrString(old.BudgetHours), ptrString(next.BudgetHours))
	add("billable", strconv.FormatBool(old.Billable), strconv.FormatBool(next.Billable))
	add("specialType", string(old.SpecialType), string(next.SpecialType))
	add("milestone", ptrString(old.MilestoneID), ptrString(next.MilestoneID))
	add("sprint", ptrString(old.SprintID), ptrString(next.SprintID))
	add("version", ptrString(old.VersionID), ptrString(next.VersionID))
	add("tags", idList(old.TagIDs), idList(next.TagIDs))
	return out
}

func ptrString[T int | int64 | float64](p *T) string {
	if p == nil {
		return ""
	}
	return fmt.Sprint(*p)
}

func idList(ids []int64) string {
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	parts := make([]string, len(sorted))
	for i, id := range sorted {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func (s *Service) History(ctx context.Context, actor domain.User, cardID int64, offset int) ([]domain.HistoryEntry, error) {
	if _, err := s.card(ctx, actor, cardID); err != nil {
		return nil, err
	}
	return s.history.List(ctx, cardID, max(0, offset), 30)
}

// --- templates ---

// SetTemplate turns a task into a template (hidden from the board) or back.
func (s *Service) SetTemplate(ctx context.Context, actor domain.User, cardID int64, template bool) (domain.Card, error) {
	c, err := s.card(ctx, actor, cardID)
	if err != nil {
		return c, err
	}
	if err := s.cards.SetTemplate(ctx, cardID, template); err != nil {
		return c, err
	}
	if template {
		s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventCardDeleted, ProjectID: c.ProjectID, ActorID: actor.ID, CardID: c.ID, ColumnID: c.ColumnID})
		c.IsTemplate = true
		return c, nil
	}
	c, err = s.cards.Get(ctx, cardID)
	if err == nil {
		s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventCardCreated, ProjectID: c.ProjectID, ActorID: actor.ID, Card: &c})
	}
	return c, err
}

func (s *Service) Templates(ctx context.Context, actor domain.User, projectID int64) ([]domain.Card, error) {
	if _, err := s.member(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.cards.Templates(ctx, projectID)
}

// FromTemplate creates a task from a template in the given column.
func (s *Service) FromTemplate(ctx context.Context, actor domain.User, templateID, columnID int64) (domain.Card, error) {
	tpl, err := s.card(ctx, actor, templateID)
	if err != nil || !tpl.IsTemplate {
		return domain.Card{}, domain.ErrTemplateNotFound
	}
	col, err := s.columns.Get(ctx, columnID)
	if err != nil || col.ProjectID != tpl.ProjectID {
		return domain.Card{}, domain.ErrColumnInvalid
	}
	var id int64
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		var err error
		id, err = s.cards.CopyFrom(ctx, templateID, columnID, actor.ID)
		return err
	})
	if err != nil {
		return domain.Card{}, err
	}
	c, err := s.cards.Get(ctx, id)
	if err == nil {
		s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventCardCreated, ProjectID: c.ProjectID, ActorID: actor.ID, Card: &c})
	}
	return c, err
}

// --- columns ---

func (s *Service) SetColumnStatus(ctx context.Context, actor domain.User, columnID int64, status domain.ColumnStatus) error {
	col, err := s.columns.Get(ctx, columnID)
	if err != nil {
		return usecase.NotFoundAs(err, domain.ErrColumnNotFound)
	}
	if err := s.manager(ctx, actor, col.ProjectID); err != nil {
		return err
	}
	if !status.Valid() {
		return domain.ErrColumnStatus
	}
	if err := s.columns.SetStatus(ctx, columnID, status); err != nil {
		return err
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventColumnsChanged, ProjectID: col.ProjectID, ActorID: actor.ID})
	return nil
}

// BulkLimit caps a bulk move; the rest of the column is moved by repeating it.
const BulkLimit = 100

// BulkTarget is where a bulk move sends a column's top tasks.
type BulkTarget struct {
	Kind string // "column", "backlog", "milestone", "sprint", "version"
	ID   *int64 // nil for "backlog", or to clear a planning field
}

// BulkMove moves up to BulkLimit tasks from the top of a column; it returns how many moved.
func (s *Service) BulkMove(ctx context.Context, actor domain.User, columnID int64, target BulkTarget) (int, error) {
	col, err := s.columns.Get(ctx, columnID)
	if err != nil {
		return 0, usecase.NotFoundAs(err, domain.ErrColumnNotFound)
	}
	if err := s.manager(ctx, actor, col.ProjectID); err != nil {
		return 0, err
	}
	ids, err := s.cards.TopOfColumn(ctx, columnID, BulkLimit)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	switch target.Kind {
	case "column", "backlog":
		var to int64
		if target.Kind == "backlog" {
			to, err = s.columns.FirstWithStatus(ctx, col.ProjectID, domain.StatusBacklog)
			if errors.Is(err, domain.ErrNoRows) {
				return 0, domain.ErrBulkTarget
			}
		} else if target.ID != nil {
			var dest domain.Column
			dest, err = s.columns.Get(ctx, *target.ID)
			if err == nil && dest.ProjectID != col.ProjectID {
				return 0, domain.ErrBulkTarget
			}
			to = dest.ID
		}
		if err != nil || to == 0 || to == columnID {
			return 0, domain.ErrBulkTarget
		}
		if err := s.tx.WithinTx(ctx, func(ctx context.Context) error { return s.cards.MoveTo(ctx, ids, to) }); err != nil {
			return 0, err
		}
	case "milestone", "sprint", "version":
		table := target.Kind + "s"
		if target.ID != nil {
			if ok, err := s.planning.InProject(ctx, table, *target.ID, col.ProjectID); err != nil {
				return 0, err
			} else if !ok {
				return 0, domain.ErrBulkTarget
			}
		}
		if err := s.cards.SetPlanning(ctx, ids, target.Kind+"_id", target.ID); err != nil {
			return 0, err
		}
	default:
		return 0, domain.ErrBulkTarget
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventColumnsChanged, ProjectID: col.ProjectID, ActorID: actor.ID})
	return len(ids), nil
}

// FindTasks is the task picker (time tracking): "#12", "KAN-12", or words of the title.
func (s *Service) FindTasks(ctx context.Context, actor domain.User, projectID int64, query string) ([]domain.Card, error) {
	if _, err := s.member(ctx, actor, projectID); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	var number *int64
	if m := numberRe.FindStringSubmatch(query); m != nil {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		number = &n
	}
	return s.cards.Find(ctx, projectID, query, number, 20)
}

var numberRe = regexp.MustCompile(`^(?:#|[A-Za-z][A-Za-z0-9]*-)?(\d{1,15})$`)
