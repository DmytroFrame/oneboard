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
		{"Демо Користувач", "demo@example.com"},
		{"Олена Коваль", "olena@example.com"},
		{"Андрій Шевченко", "andrii@example.com"},
		{"Марія Бондар", "maria@example.com"},
		{"Тарас Мельник", "taras@example.com"},
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
			{"Налаштувати CI/CD", "GitHub Actions: lint, тести, збірка Docker-образу.", "high", 0, 0, 2},
			{"Дизайн сторінки профілю", "Макети у Figma для десктопу та мобілки.", "medium", 0, 1, 3},
			{"Додати пошук по задачах", "Повнотекстовий пошук через OpenSearch.", "low", 0, 2, -1},
			{"API для коментарів", "CRUD ендпоінти + пагінація.", "high", 1, 0, 0},
			{"Міграція на PostgreSQL", "Перенести дані зі SQLite, сесії — у Redis.", "medium", 1, 2, 2},
			{"Онбординг нових користувачів", "Покроковий тур по дошці.", "medium", 2, 3, 1},
			{"Виправити баг з логіном", "Сесія злітає після перезавантаження сторінки.", "critical", 3, 1, 0},
			{"Лендінг сторінка", "", "low", 3, 3, 3},
		}, 45},
		{"Мобільний застосунок", "MOB", []int{1}, []card{
			{"Екран входу", "", "high", 0, 0, 1},
			{"Push-сповіщення", "Firebase Cloud Messaging.", "medium", 1, 1, 0},
			{"Темна тема", "", "low", 3, 0, 1},
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
			p.cards = append(p.cards, card{fmt.Sprintf("Задача з беклогу #%d", i+1), "", priorities[i%4], 0, team[i%len(team)], team[(i+1)%len(team)]})
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
