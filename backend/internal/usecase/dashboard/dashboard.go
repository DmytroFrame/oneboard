// Package dashboard builds the signed-in user's home page overview.
package dashboard

import (
	"context"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
)

var Module = fx.Module("dashboard", fx.Provide(NewService))

const assignedLimit = 8

type Reader interface {
	Stats(ctx context.Context, userID int64) (domain.DashboardStats, error)
	// AssignedCards returns the user's cards, most urgent and recently changed first.
	AssignedCards(ctx context.Context, userID int64, limit int) ([]domain.AssignedCard, error)
}

type Service struct{ reader Reader }

func NewService(r Reader) *Service { return &Service{reader: r} }

type Dashboard struct {
	Stats    domain.DashboardStats
	Assigned []domain.AssignedCard
}

func (s *Service) Get(ctx context.Context, userID int64) (Dashboard, error) {
	stats, err := s.reader.Stats(ctx, userID)
	if err != nil {
		return Dashboard{}, err
	}
	assigned, err := s.reader.AssignedCards(ctx, userID, assignedLimit)
	return Dashboard{Stats: stats, Assigned: assigned}, err
}
