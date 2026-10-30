package board

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 100
	// maxSearchHits caps how many matches full-text search feeds into the database filter.
	maxSearchHits = 10000
)

var cardNotFound = domain.ErrCardNotFound

type CardService struct {
	cards   Cards
	columns Columns
	members Members
	index   SearchIndex
	files   AttachmentFiles
	storage FileStorage
	tx      usecase.Transactor
	log     *slog.Logger
}

type CardDeps struct {
	fx.In
	Cards   Cards
	Columns Columns
	Members Members
	Index   SearchIndex
	Files   AttachmentFiles
	Storage FileStorage
	Tx      usecase.Transactor
	Log     *slog.Logger
}

func NewCardService(d CardDeps) *CardService {
	return &CardService{cards: d.Cards, columns: d.Columns, members: d.Members, index: d.Index, files: d.Files, storage: d.Storage, tx: d.Tx, log: d.Log}
}

type ListQuery struct {
	ColumnID   int64
	Search     string
	Assignees  []int64
	Unassigned bool
	Cursor     *domain.CardCursor
	Limit      int
}

// List returns one page of a column's cards, ordered by position.
func (s *CardService) List(ctx context.Context, userID, projectID int64, q ListQuery) (domain.CardPage, error) {
	if _, err := usecase.RequireMember(ctx, s.members, projectID, userID, projectNotFound); err != nil {
		return domain.CardPage{}, err
	}
	if err := s.columnInProject(ctx, q.ColumnID, projectID, columnNotFound); err != nil {
		return domain.CardPage{}, err
	}

	f := CardFilter{
		ColumnID:   q.ColumnID,
		Assignees:  q.Assignees,
		Unassigned: q.Unassigned,
		After:      q.Cursor,
		Limit:      DefaultPageSize,
	}
	if q.Limit > 0 {
		f.Limit = min(q.Limit, MaxPageSize)
	}
	if search := strings.TrimSpace(q.Search); search != "" {
		ids, err := s.index.Search(ctx, projectID, search, maxSearchHits)
		if err != nil {
			return domain.CardPage{}, err
		}
		if len(ids) == 0 {
			return domain.CardPage{Cards: []domain.Card{}}, nil
		}
		f.OnlyIDs = ids
	}
	return s.cards.List(ctx, f)
}

func (s *CardService) columnInProject(ctx context.Context, columnID, projectID int64, notOK error) error {
	col, err := s.columns.Get(ctx, columnID)
	if errors.Is(err, domain.ErrNoRows) || (err == nil && col.ProjectID != projectID) {
		return notOK
	}
	return err
}

func (s *CardService) checkAssignee(ctx context.Context, projectID int64, assignee *int64) error {
	if assignee == nil {
		return nil
	}
	role, err := s.members.Role(ctx, projectID, *assignee)
	if err != nil {
		return err
	}
	if role == domain.RoleNone {
		return domain.ErrAssigneeNotMember
	}
	return nil
}

func (s *CardService) Create(ctx context.Context, userID, projectID, columnID int64, d domain.CardDraft) (domain.Card, error) {
	if _, err := usecase.RequireMember(ctx, s.members, projectID, userID, projectNotFound); err != nil {
		return domain.Card{}, err
	}
	if err := d.Normalize(); err != nil {
		return domain.Card{}, err
	}
	if err := s.columnInProject(ctx, columnID, projectID, domain.ErrColumnInvalid); err != nil {
		return domain.Card{}, err
	}
	if err := s.checkAssignee(ctx, projectID, d.AssigneeID); err != nil {
		return domain.Card{}, err
	}

	var id int64
	err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.columns.Lock(ctx, columnID); err != nil {
			return err
		}
		var err error
		id, err = s.cards.Create(ctx, projectID, columnID, userID, d)
		return err
	})
	if err != nil {
		return domain.Card{}, err
	}
	s.reindex(ctx, id)
	return s.cards.Get(ctx, id)
}

// accessibleCard loads a card and the user's role in its project.
func (s *CardService) accessibleCard(ctx context.Context, userID, cardID int64) (domain.Card, domain.Role, error) {
	c, err := s.cards.Get(ctx, cardID)
	if err != nil {
		return c, "", usecase.NotFoundAs(err, cardNotFound)
	}
	role, err := usecase.RequireMember(ctx, s.members, c.ProjectID, userID, cardNotFound)
	return c, role, err
}

func (s *CardService) Update(ctx context.Context, userID, cardID int64, d domain.CardDraft) (domain.Card, error) {
	c, _, err := s.accessibleCard(ctx, userID, cardID)
	if err != nil {
		return c, err
	}
	if err := d.Normalize(); err != nil {
		return c, err
	}
	if err := s.checkAssignee(ctx, c.ProjectID, d.AssigneeID); err != nil {
		return c, err
	}
	if err := s.cards.Update(ctx, c.ID, d); err != nil {
		return c, err
	}
	s.reindex(ctx, c.ID)
	return s.cards.Get(ctx, c.ID)
}

// Placement says where a moved card goes. A paginated client may drop below the last
// *loaded* card, which is not the end of the column — hence AfterID.
type Placement struct {
	BeforeID *int64
	AfterID  *int64 // neither = end of column
}

func (s *CardService) Move(ctx context.Context, userID, cardID, columnID int64, p Placement) (domain.Card, error) {
	c, _, err := s.accessibleCard(ctx, userID, cardID)
	if err != nil {
		return c, err
	}
	if err := s.columnInProject(ctx, columnID, c.ProjectID, domain.ErrColumnInvalid); err != nil {
		return c, err
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.columns.Lock(ctx, columnID); err != nil {
			return err
		}
		current, err := s.cards.ColumnOrder(ctx, columnID)
		if err != nil {
			return err
		}
		return s.cards.Place(ctx, c.ID, columnID, insertInOrder(current, c.ID, p))
	})
	if err != nil {
		return c, err
	}
	return s.cards.Get(ctx, c.ID)
}

// insertInOrder returns order without cardID, with cardID re-inserted according to p.
func insertInOrder(order []int64, cardID int64, p Placement) []int64 {
	out := make([]int64, 0, len(order)+1)
	placed := false
	for _, id := range order {
		if id == cardID {
			continue
		}
		if !placed && p.BeforeID != nil && id == *p.BeforeID {
			out = append(out, cardID)
			placed = true
		}
		out = append(out, id)
		if !placed && p.AfterID != nil && id == *p.AfterID {
			out = append(out, cardID)
			placed = true
		}
	}
	if !placed {
		out = append(out, cardID)
	}
	return out
}

func (s *CardService) Delete(ctx context.Context, userID, cardID int64) error {
	c, role, err := s.accessibleCard(ctx, userID, cardID)
	if err != nil {
		return err
	}
	if c.AuthorID != userID && role != domain.RoleOwner {
		return domain.ErrCardDeleteDenied
	}
	keys, err := s.files.KeysForCard(ctx, c.ID)
	if err != nil {
		return err
	}
	if err := s.cards.Delete(ctx, c.ID); err != nil {
		return err
	}
	if err := s.index.Delete(ctx, c.ID); err != nil {
		s.log.Error("search index: delete card", "card", c.ID, "err", err)
	}
	if err := s.storage.DeleteMany(ctx, keys); err != nil {
		s.log.Error("storage: delete card files", "card", c.ID, "err", err)
	}
	return nil
}

// reindex pushes a card to the search index. The database is the source of truth, so a failure
// is logged rather than failing the request; SyncIndex repairs drift on the next start.
func (s *CardService) reindex(ctx context.Context, cardID int64) {
	docs, err := s.cards.SearchDocs(ctx, cardID)
	if err == nil {
		err = s.index.Upsert(ctx, docs...)
	}
	if err != nil {
		s.log.Error("search index: upsert card", "card", cardID, "err", err)
	}
}

// SyncIndex creates the search index if needed and rebuilds it when it disagrees with the database.
func (s *CardService) SyncIndex(ctx context.Context) error {
	if err := s.index.Ensure(ctx); err != nil {
		return err
	}
	indexed, err := s.index.Count(ctx)
	if err != nil {
		return err
	}
	total, err := s.cards.CountAll(ctx)
	if err != nil || indexed == total {
		return err
	}
	docs, err := s.cards.SearchDocs(ctx)
	if err != nil {
		return err
	}
	s.log.Info("search index: rebuilding", "indexed", indexed, "cards", total)
	return s.index.Upsert(ctx, docs...)
}
