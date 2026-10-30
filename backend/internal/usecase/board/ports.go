// Package board implements the kanban board of a project: columns and cards.
package board

import (
	"context"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

var Module = fx.Module("board", fx.Provide(NewColumnService, NewCardService))

type Columns interface {
	List(ctx context.Context, projectID int64) ([]domain.Column, error)
	Get(ctx context.Context, columnID int64) (domain.Column, error)
	Count(ctx context.Context, projectID int64) (int, error)
	Create(ctx context.Context, projectID int64, name string) error // appended after the last column
	Rename(ctx context.Context, columnID int64, name string) error
	Reorder(ctx context.Context, columnIDs []int64) error // position = index in the slice
	Delete(ctx context.Context, columnID int64) error
	// Lock serializes ordering changes within the column until the transaction ends.
	Lock(ctx context.Context, columnID int64) error
}

type CardFilter struct {
	ColumnID   int64
	OnlyIDs    []int64 // when non-nil, only these cards (from full-text search)
	Assignees  []int64
	Unassigned bool // include unassigned cards (OR-ed with Assignees)
	After      *domain.CardCursor
	Limit      int
}

type Cards interface {
	Get(ctx context.Context, cardID int64) (domain.Card, error)
	List(ctx context.Context, f CardFilter) (domain.CardPage, error)
	Create(ctx context.Context, projectID, columnID, authorID int64, d domain.CardDraft) (int64, error)
	Update(ctx context.Context, cardID int64, d domain.CardDraft) error
	// ColumnOrder returns the card ids of the column ordered by position.
	ColumnOrder(ctx context.Context, columnID int64) ([]int64, error)
	// Place puts cardID into columnID and renumbers that column following order.
	Place(ctx context.Context, cardID, columnID int64, order []int64) error
	// MoveAll appends every card of from to the bottom of to, preserving their order.
	MoveAll(ctx context.Context, from, to int64) error
	Delete(ctx context.Context, cardID int64) error
	SearchDocs(ctx context.Context, cardIDs ...int64) ([]domain.SearchDoc, error) // no ids = all cards
	CountAll(ctx context.Context) (int, error)
}

// SearchIndex is the full-text index of cards. It only answers "which cards match";
// ordering, filtering and pagination stay in the database.
type SearchIndex interface {
	Ensure(ctx context.Context) error
	Count(ctx context.Context) (int, error)
	Upsert(ctx context.Context, docs ...domain.SearchDoc) error
	Delete(ctx context.Context, cardID int64) error
	Search(ctx context.Context, projectID int64, query string, limit int) ([]int64, error)
}

type Members interface {
	usecase.Memberships
}

type AttachmentFiles interface {
	KeysForCard(ctx context.Context, cardID int64) ([]string, error)
}

type FileStorage interface {
	DeleteMany(ctx context.Context, keys []string) error
}
