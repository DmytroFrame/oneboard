package board

import (
	"context"
	"strings"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

const (
	projectNotFound = "проєкт не знайдено"
	columnNotFound  = "колонку не знайдено"
	ownerOnly       = "керувати колонками може лише власник проєкту"
)

type ColumnService struct {
	columns Columns
	cards   Cards
	members Members
	tx      usecase.Transactor
}

func NewColumnService(columns Columns, cards Cards, members Members, tx usecase.Transactor) *ColumnService {
	return &ColumnService{columns: columns, cards: cards, members: members, tx: tx}
}

func validColumnName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 60 {
		return "", domain.Validation("назва колонки обов'язкова (до 60 символів)")
	}
	return name, nil
}

func (s *ColumnService) List(ctx context.Context, userID, projectID int64) ([]domain.Column, error) {
	if _, err := usecase.RequireMember(ctx, s.members, projectID, userID, projectNotFound); err != nil {
		return nil, err
	}
	return s.columns.List(ctx, projectID)
}

func (s *ColumnService) Create(ctx context.Context, userID, projectID int64, name string) ([]domain.Column, error) {
	if err := usecase.RequireOwner(ctx, s.members, projectID, userID, projectNotFound, ownerOnly); err != nil {
		return nil, err
	}
	name, err := validColumnName(name)
	if err != nil {
		return nil, err
	}
	n, err := s.columns.Count(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if n >= domain.MaxColumnsPerProject {
		return nil, domain.Validation("максимум 20 колонок у проєкті")
	}
	if err := s.columns.Create(ctx, projectID, name); err != nil {
		return nil, err
	}
	return s.columns.List(ctx, projectID)
}

// ownedColumn loads a column and requires userID to own its project.
func (s *ColumnService) ownedColumn(ctx context.Context, userID, columnID int64) (domain.Column, error) {
	col, err := s.columns.Get(ctx, columnID)
	if err != nil {
		return col, usecase.NotFoundAs(err, columnNotFound)
	}
	return col, usecase.RequireOwner(ctx, s.members, col.ProjectID, userID, columnNotFound, ownerOnly)
}

func (s *ColumnService) Rename(ctx context.Context, userID, columnID int64, name string) ([]domain.Column, error) {
	col, err := s.ownedColumn(ctx, userID, columnID)
	if err != nil {
		return nil, err
	}
	if name, err = validColumnName(name); err != nil {
		return nil, err
	}
	if err := s.columns.Rename(ctx, col.ID, name); err != nil {
		return nil, err
	}
	return s.columns.List(ctx, col.ProjectID)
}

// Move puts the column at index among the project's columns.
func (s *ColumnService) Move(ctx context.Context, userID, columnID int64, index int) ([]domain.Column, error) {
	col, err := s.ownedColumn(ctx, userID, columnID)
	if err != nil {
		return nil, err
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		cols, err := s.columns.List(ctx, col.ProjectID)
		if err != nil {
			return err
		}
		order := make([]int64, 0, len(cols))
		for _, c := range cols {
			if c.ID != col.ID {
				order = append(order, c.ID)
			}
		}
		i := min(max(index, 0), len(order))
		order = append(order[:i], append([]int64{col.ID}, order[i:]...)...)
		return s.columns.Reorder(ctx, order)
	})
	if err != nil {
		return nil, err
	}
	return s.columns.List(ctx, col.ProjectID)
}

// Delete removes a column. If it has cards, moveTo is required and they are appended to that column.
func (s *ColumnService) Delete(ctx context.Context, userID, columnID int64, moveTo *int64) ([]domain.Column, error) {
	col, err := s.ownedColumn(ctx, userID, columnID)
	if err != nil {
		return nil, err
	}
	n, err := s.columns.Count(ctx, col.ProjectID)
	if err != nil {
		return nil, err
	}
	if n <= 1 {
		return nil, domain.Validation("у проєкті має лишитися хоча б одна колонка")
	}

	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.columns.Lock(ctx, col.ID); err != nil {
			return err
		}
		// Re-read inside the lock: cards may have been added since the check above.
		col, err := s.columns.Get(ctx, col.ID)
		if err != nil {
			return err
		}
		if col.CardCount > 0 {
			if moveTo == nil || *moveTo == col.ID {
				return domain.Conflict("у колонці є картки — вкажіть, куди їх перенести")
			}
			target, err := s.columns.Get(ctx, *moveTo)
			if err != nil || target.ProjectID != col.ProjectID {
				return domain.Validation("колонку для перенесення не знайдено")
			}
			if err := s.columns.Lock(ctx, target.ID); err != nil {
				return err
			}
			if err := s.cards.MoveAll(ctx, col.ID, target.ID); err != nil {
				return err
			}
		}
		return s.columns.Delete(ctx, col.ID)
	})
	if err != nil {
		return nil, err
	}
	return s.columns.List(ctx, col.ProjectID)
}
