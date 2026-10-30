package postgres

import (
	"context"
	"fmt"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/auth"
)

// Seeder fills an empty database with demo users, projects and cards so the app isn't blank on first run.
type Seeder struct {
	db       *DB
	users    *UserRepo
	projects *ProjectRepo
	columns  *ColumnRepo
	cards    *CardRepo
	hasher   auth.PasswordHasher
}

func NewSeeder(db *DB, users *UserRepo, projects *ProjectRepo, columns *ColumnRepo, cards *CardRepo, hasher auth.PasswordHasher) *Seeder {
	return &Seeder{db, users, projects, columns, cards, hasher}
}

func (s *Seeder) Seed(ctx context.Context) (bool, error) {
	n, err := s.users.Count(ctx)
	if err != nil || n > 0 {
		return false, err
	}
	return true, s.db.WithinTx(ctx, s.seed)
}

func (s *Seeder) seed(ctx context.Context) error {
	hash, err := s.hasher.Hash("demo1234")
	if err != nil {
		return err
	}
	people := []struct{ name, email string }{
		{"Demo User", "demo@example.com"},
		{"Olena Koval", "olena@example.com"},
		{"Andrii Shevchenko", "andrii@example.com"},
		{"Maria Bondar", "maria@example.com"},
		{"Taras Melnyk", "taras@example.com"},
	}
	uid := make([]int64, len(people))
	for i, p := range people {
		u := domain.User{Name: p.name, Email: p.email, PasswordHash: hash, Color: domain.AvatarColors[i%len(domain.AvatarColors)]}
		if err := s.users.Create(ctx, &u); err != nil {
			return err
		}
		uid[i] = u.ID
	}

	type card struct {
		title, desc      string
		priority         domain.Priority
		column           int // index into the project's default columns
		author, assignee int // indexes into uid; assignee -1 = unassigned
	}
	projects := []struct {
		name, key string
		members   []int
		cards     []card
		backlog   int // extra generated cards in the first column, to demo infinite scroll
	}{
		{"Flowboard", "KAN", []int{1, 2, 3}, []card{
			{"Set up CI/CD", "GitHub Actions: lint, tests, Docker image build.", "high", 0, 0, 2},
			{"Profile page design", "Figma mockups for desktop and mobile.", "medium", 0, 1, 3},
			{"Add task search", "Full-text search with OpenSearch.", "low", 0, 2, -1},
			{"Comments API", "CRUD endpoints with pagination.", "high", 1, 0, 0},
			{"Migrate to PostgreSQL", "Move data off SQLite, sessions to Redis.", "medium", 1, 2, 2},
			{"New user onboarding", "Step-by-step board tour.", "medium", 2, 3, 1},
			{"Fix login bug", "Session is lost after a page reload.", "critical", 3, 1, 0},
			{"Landing page", "", "low", 3, 3, 3},
		}, 45},
		{"Mobile App", "MOB", []int{1}, []card{
			{"Sign-in screen", "", "high", 0, 0, 1},
			{"Push notifications", "Firebase Cloud Messaging.", "medium", 1, 1, 0},
			{"Dark theme", "", "low", 3, 0, 1},
		}, 0},
	}

	priorities := []domain.Priority{domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical}
	for _, p := range projects {
		pid, err := s.projects.Create(ctx, p.name, p.key, uid[0])
		if err != nil {
			return err
		}
		if err := s.projects.Add(ctx, pid, uid[0], domain.RoleOwner); err != nil {
			return err
		}
		if err := s.columns.CreateMany(ctx, pid, domain.DefaultColumns); err != nil {
			return err
		}
		cols, err := s.columns.List(ctx, pid)
		if err != nil {
			return err
		}
		team := []int{0}
		for _, m := range p.members {
			if err := s.projects.Add(ctx, pid, uid[m], domain.RoleMember); err != nil {
				return err
			}
			team = append(team, m)
		}
		for i := range p.backlog {
			p.cards = append(p.cards, card{fmt.Sprintf("Backlog task #%d", i+1), "", priorities[i%4], 0, team[i%len(team)], team[(i+1)%len(team)]})
		}
		for _, c := range p.cards {
			d := domain.CardDraft{Title: c.title, Description: c.desc, Priority: c.priority}
			if c.assignee >= 0 {
				d.AssigneeID = &uid[c.assignee]
			}
			if _, err := s.cards.Create(ctx, pid, cols[c.column].ID, uid[c.author], d); err != nil {
				return err
			}
		}
	}
	return nil
}
