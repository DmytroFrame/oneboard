package httpapi

import (
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the columns module.

type columnDTO struct {
	ID        int64               `json:"id"`
	ProjectID int64               `json:"projectId"`
	Name      string              `json:"name"`
	Position  int                 `json:"position"`
	CardCount int                 `json:"cardCount"`
	Status    domain.ColumnStatus `json:"status"`
}

func toColumns(cs []domain.Column) []columnDTO {
	out := make([]columnDTO, len(cs))
	for i, c := range cs {
		out[i] = columnDTO{c.ID, c.ProjectID, c.Name, c.Position, c.CardCount, c.Status}
	}
	return out
}
