package postgres

import (
	"context"
	"fmt"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
)

// Seeder fills an empty database with demo users, projects and cards so the app isn't blank on first run.
type Seeder struct {
	db          *DB
	users       *UserRepo
	orgs        *OrgRepo
	orgMembers  *OrgMemberRepo
	invitations *InvitationRepo
	projects    *ProjectRepo
	columns     *ColumnRepo
	cards       *CardRepo
	hasher      auth.PasswordHasher
}

func NewSeeder(db *DB, users *UserRepo, orgs *OrgRepo, orgMembers *OrgMemberRepo, invitations *InvitationRepo,
	projects *ProjectRepo, columns *ColumnRepo, cards *CardRepo, hasher auth.PasswordHasher) *Seeder {
	return &Seeder{db, users, orgs, orgMembers, invitations, projects, columns, cards, hasher}
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
	// Accounts; their roles are per organization, below.
	people := []struct{ name, email string }{
		{"Demo User", "demo@example.com"},           // 0
		{"Olena Koval", "olena@example.com"},        // 1
		{"Andrii Shevchenko", "andrii@example.com"}, // 2
		{"Maria Bondar", "maria@example.com"},       // 3
		{"Taras Melnyk", "taras@example.com"},       // 4
		{"Iryna Lysenko", "iryna@example.com"},      // 5 candidate
		{"Petro Hnatiuk", "petro@example.com"},      // 6 candidate
		{"Oksana Tkachenko", "oksana@example.com"},  // 7 candidate
		{"Sofia Romanenko", "sofia@example.com"},    // 8 only in Mobile Studio
	}
	uid := make([]int64, len(people))
	for i, p := range people {
		u := domain.User{Name: p.name, Email: p.email, PasswordHash: hash, Color: domain.AvatarColors[i%len(domain.AvatarColors)]}
		if err := s.users.Create(ctx, &u); err != nil {
			return err
		}
		uid[i] = u.ID
	}

	// Two isolated organizations. The demo user owns one and administers the other, so switching shows
	// both views; Sofia exists only in Mobile Studio and is invisible from OneBoard Inc.
	orgs := []struct {
		name    string
		owner   int
		members map[int]domain.CompanyRole // index into uid → role
	}{
		{"OneBoard Inc.", 0, map[int]domain.CompanyRole{1: domain.CompanyHR, 2: domain.CompanyManager, 3: domain.CompanyEmployee,
			4: domain.CompanyContractor, 5: domain.CompanyCandidate, 6: domain.CompanyCandidate, 7: domain.CompanyCandidate}},
		{"Mobile Studio", 1, map[int]domain.CompanyRole{0: domain.CompanyAdmin, 8: domain.CompanyEmployee}},
	}
	orgID := make([]int64, len(orgs))
	for i, o := range orgs {
		if orgID[i], err = s.orgs.Found(ctx, o.name, uid[o.owner]); err != nil {
			return err
		}
		for m, role := range o.members {
			if err := s.orgMembers.Add(ctx, orgID[i], uid[m], role); err != nil {
				return err
			}
		}
	}
	if err := s.invitations.Create(ctx, orgID[0], "new.hire@example.com", domain.CompanyEmployee, uid[0]); err != nil {
		return err
	}

	// Each organization's HR profiles, relative to today so the dashboard has upcoming birthdays,
	// anniversaries and a trial. Columns: birthday and start date as (years ago, days from today), trial
	// end in days from today (NULL when absent).
	profiles := []struct {
		org                  int
		user, by, bd, sy, sd int
		trial                *int
		billable             bool
		specialization       string
	}{
		{0, 0, 34, 3, 6, 9, nil, false, "Project management"},
		{0, 1, 29, 12, 3, 25, nil, false, ""},
		{0, 2, 41, 47, 2, 70, nil, true, "Backend"},
		{0, 3, 26, 20, 0, -60, ptr(30), true, "Frontend"},
		{0, 4, 31, 160, 1, 18, nil, true, "Mobile"},
		{0, 5, 24, 200, 0, 0, nil, false, "Frontend"},
		{0, 6, 27, 210, 0, 0, nil, false, "QA"},
		{0, 7, 30, 220, 0, 0, nil, false, "Backend"},
		{1, 1, 29, 12, 4, 5, nil, false, ""},
		{1, 0, 34, 3, 1, 40, nil, true, "Mobile"},
		{1, 8, 28, 6, 0, -20, ptr(70), true, "Design"},
	}
	for _, p := range profiles {
		if err := exec(ctx, s.db, `INSERT INTO profiles (organization_id, user_id, birthday, start_date, trial_until, billable, details)
			VALUES ($1, $2, CURRENT_DATE - make_interval(years => $3::int, days => -($4::int)),
				CASE WHEN $5::int = 0 AND $6::int = 0 THEN NULL ELSE CURRENT_DATE - make_interval(years => $5::int, days => -($6::int)) END,
				CASE WHEN $7::int IS NULL THEN NULL ELSE CURRENT_DATE + $7::int END, $8,
				CASE WHEN $9::text = '' THEN '{}'::jsonb ELSE jsonb_build_object('specialization', $9::text) END)`,
			orgID[p.org], uid[p.user], p.by, p.bd, p.sy, p.sd, p.trial, p.billable, p.specialization); err != nil {
			return err
		}
	}

	// Public holidays differ by organization (each keeps its own calendar).
	if err := exec(ctx, s.db, `INSERT INTO holidays (organization_id, day, name_latin, name_cyrillic) VALUES
		($1, make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 12, 25), 'Christmas', 'Різдво'),
		($1, make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int + 1, 1, 1), 'New Year', 'Новий рік'),
		($2, make_date(EXTRACT(YEAR FROM CURRENT_DATE)::int, 12, 31), 'Studio day off', 'Вихідний студії')`, orgID[0], orgID[1]); err != nil {
		return err
	}

	// OneBoard Inc.'s hiring funnel, hires and dismissals.
	candidates := []struct {
		user            int
		status, channel string
		attrs           []string
		min, max        float64
		reason          string
	}{
		{5, "hr_interview", "Djinni", []string{"promising", "full_time"}, 1500, 2200, ""},
		{6, "client_interview", "DOU", []string{"test_task_done", "english_check"}, 1000, 1400, ""},
		{7, "new", "LinkedIn", []string{"part_time"}, 2500, 3200, "salary_too_high"},
	}
	for _, c := range candidates {
		if err := exec(ctx, s.db, `INSERT INTO candidates (organization_id, user_id, status, attributes, channel, min_salary, max_salary,
			recruiter_id, rejection_reason) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			orgID[0], uid[c.user], c.status, c.attrs, c.channel, c.min, c.max, uid[1], c.reason); err != nil {
			return err
		}
		if c.reason != "" {
			if err := exec(ctx, s.db, `INSERT INTO hr_events (organization_id, user_id, kind, day, reason) VALUES ($1, $2, 'rejected', CURRENT_DATE - 3, $3)`,
				orgID[0], uid[c.user], c.reason); err != nil {
				return err
			}
		}
	}
	if err := exec(ctx, s.db, `INSERT INTO hr_events (organization_id, user_id, kind, day, from_candidate)
		SELECT o, u, 'hired', CURRENT_DATE - d, fc FROM unnest($1::bigint[], $2::bigint[], $3::int[], $4::bool[]) AS t(o, u, d, fc)`,
		[]int64{orgID[0], orgID[0], orgID[1]}, []int64{uid[3], uid[4], uid[8]}, []int{60, 400, 20}, []bool{true, false, false}); err != nil {
		return err
	}

	// OneBoard Inc.'s wiki: a company-wide onboarding guide with a sub-article, a private draft and a group.
	wiki := []struct {
		title, content, access string
		parent                 int // index into ids; -1 = root
	}{
		{"Developer onboarding", "# Welcome\n\nStart with the **code style** and set up the dev stack:\n\n- `make infra-up`\n- `make dev-backend`\n- `make dev-frontend`", "read", -1},
		{"Code style", "Go: `gofmt`, small packages, ports in the use case.\n\nTypeScript: CSS modules, no barrel files.", "none", 0},
		{"Release checklist (draft)", "- [ ] Changelog\n- [ ] Migrations reviewed\n- [ ] Backup taken", "none", -1},
	}
	var ids []int64
	for _, w := range wiki {
		var parent *int64
		if w.parent >= 0 {
			parent = &ids[w.parent]
		}
		var id int64
		if err := s.db.q(ctx).QueryRow(ctx, `INSERT INTO wiki_articles (organization_id, parent_id, owner_id, title, content, inherit_access,
			company_access, updated_by) VALUES ($1, $2, $3, $4, $5, $6, $7, $3) RETURNING id`,
			orgID[0], parent, uid[0], w.title, w.content, parent != nil, w.access).Scan(&id); err != nil {
			return err
		}
		if err := exec(ctx, s.db, `INSERT INTO wiki_versions (article_id, title, content, author_id) SELECT id, title, content, owner_id FROM wiki_articles WHERE id = $1`, id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := exec(ctx, s.db, `WITH g AS (INSERT INTO wiki_groups (organization_id, name, owner_id) VALUES ($1, 'Team leads', $2) RETURNING id)
		INSERT INTO wiki_group_members (group_id, user_id) SELECT g.id, u FROM g, unnest($3::bigint[]) AS u`, orgID[0], uid[0], []int64{uid[0], uid[2]}); err != nil {
		return err
	}

	type card struct {
		title, desc      string
		priority         domain.Priority
		column           int // index into the project's default columns
		author, assignee int // indexes into uid; assignee -1 = unassigned
	}
	projects := []struct {
		org       int // index into orgID
		name, key string
		members   []int
		cards     []card
		backlog   int // extra generated cards in the first column, to demo infinite scroll
	}{
		{0, "OneBoard", "KAN", []int{1, 2, 3}, []card{
			{"Set up CI/CD", "GitHub Actions: lint, tests, Docker image build.", "high", 0, 0, 2},
			{"Profile page design", "Figma mockups for desktop and mobile.", "medium", 0, 1, 3},
			{"Add task search", "Full-text search with OpenSearch.", "low", 0, 2, -1},
			{"Comments API", "CRUD endpoints with pagination.", "high", 1, 0, 0},
			{"Migrate to PostgreSQL", "Move data off SQLite, sessions to Redis.", "medium", 1, 2, 2},
			{"New user onboarding", "Step-by-step board tour.", "medium", 2, 3, 1},
			{"Fix login bug", "Session is lost after a page reload.", "critical", 3, 1, 0},
			{"Landing page", "", "low", 3, 3, 3},
		}, 45},
		{1, "Mobile App", "MOB", []int{1, 8}, []card{
			{"Sign-in screen", "", "high", 0, 0, 1},
			{"Push notifications", "Firebase Cloud Messaging.", "medium", 1, 1, 0},
			{"Dark theme", "", "low", 3, 0, 1},
			{"App icon", "Three options for the store listing.", "medium", 2, 8, 8},
		}, 0},
	}

	priorities := []domain.Priority{domain.PriorityLow, domain.PriorityMedium, domain.PriorityHigh, domain.PriorityCritical}
	for _, p := range projects {
		pid, err := s.projects.Create(ctx, orgID[p.org], p.name, p.key, uid[0])
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

func ptr[T any](v T) *T { return &v }
