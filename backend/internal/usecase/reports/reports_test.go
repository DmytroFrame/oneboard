package reports

import (
	"context"
	"testing"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

type fakeReader struct {
	Reader
	moves  []domain.ColumnMove
	starts []domain.CardStart
}

func (f fakeReader) ColumnMoves(context.Context, int64) ([]domain.ColumnMove, error) {
	return f.moves, nil
}
func (f fakeReader) CardStarts(context.Context, int64) ([]domain.CardStart, error) {
	return f.starts, nil
}

type fakeMembers struct{}

func (fakeMembers) Role(context.Context, int64, int64) (domain.Role, error) {
	return domain.RoleOwner, nil
}

type fakeColumns []domain.Column

func (f fakeColumns) List(context.Context, int64) ([]domain.Column, error) { return f, nil }

func TestTaskStats(t *testing.T) {
	now := time.Now()
	day := func(n int) time.Time { return now.Add(time.Duration(n) * 24 * time.Hour) }
	cols := fakeColumns{{ID: 1, Status: domain.StatusTodo}, {ID: 2, Status: domain.StatusInProgress}, {ID: 3, Status: domain.StatusDone}}
	r := fakeReader{
		starts: []domain.CardStart{{CardID: 10, ColumnID: 3, CreatedAt: day(-3)}, {CardID: 11, ColumnID: 2, CreatedAt: day(-1)}},
		moves: []domain.ColumnMove{
			{CardID: 10, From: 1, To: 2, At: day(-2)}, // 1 day in To Do
			{CardID: 10, From: 2, To: 3, At: day(-1)}, // 1 day in progress, then done
		},
	}
	s := NewService(r, nil, cols, fakeMembers{})
	viewer := domain.User{Role: domain.CompanyOwner}

	// Default columns: everything but "done" — card 10 spent two days there; card 11 is still in progress.
	stats, err := s.TaskStats(context.Background(), viewer, 1, domain.StatsYear, nil)
	if err != nil {
		t.Fatal(err)
	}
	last := stats.Buckets[len(stats.Buckets)-1]
	if len(stats.Buckets) != 5 || stats.Stays != 1 || stats.AvgMinutes != 2*24*60 {
		t.Fatalf("stats = %+v", stats)
	}
	if day(-3).Year() == now.Year() && (last.Created != 2 || last.Done != 1) {
		t.Fatalf("this year = %+v", last)
	}

	// Only "in progress": one day.
	stats, _ = s.TaskStats(context.Background(), viewer, 1, domain.StatsYear, []int64{2})
	if stats.Stays != 1 || stats.AvgMinutes != 24*60 {
		t.Fatalf("in progress only = %+v", stats)
	}
}
