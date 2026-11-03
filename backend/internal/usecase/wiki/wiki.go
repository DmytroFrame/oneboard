// Package wiki implements OmiTeam's knowledge base: an article tree with per-article access (owner,
// editors, readers, groups, the whole company, or the parent's list), versions, search and groups.
package wiki

import (
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("wiki", fx.Provide(NewService, NewCommentService)))

type Articles interface {
	// All lists every article of the organization without content, with its grants (access needs the
	// whole tree). Each organization has its own wiki.
	All(ctx context.Context, orgID int64) ([]domain.WikiArticle, error)
	Content(ctx context.Context, id int64) (string, error)
	Create(ctx context.Context, orgID int64, a *domain.WikiArticle) error
	Update(ctx context.Context, a domain.WikiArticle) error
	SetArchived(ctx context.Context, id int64, archived bool) error
	SetAccess(ctx context.Context, id int64, inherit bool, company domain.WikiLevel, grants []domain.WikiGrant) error
	// Search returns ids of the organization's articles whose title or text contains q.
	Search(ctx context.Context, orgID int64, q string) ([]int64, error)
	AddVersion(ctx context.Context, v domain.WikiVersion) error
	Versions(ctx context.Context, articleID int64) ([]domain.WikiVersion, error)
	Version(ctx context.Context, id int64) (domain.WikiVersion, error)
	Viewed(ctx context.Context, userID, articleID int64) error
	Recent(ctx context.Context, userID int64, limit int) ([]int64, error)
	// ViewerContext: the wiki groups and projects a user is in.
	ViewerContext(ctx context.Context, userID int64) (groups, projects []int64, err error)
}

type Groups interface {
	List(ctx context.Context, orgID int64) ([]domain.WikiGroup, error)
	Get(ctx context.Context, orgID, id int64) (domain.WikiGroup, error)
	Create(ctx context.Context, orgID int64, g *domain.WikiGroup) error
	Update(ctx context.Context, g domain.WikiGroup) error
	Delete(ctx context.Context, id int64) error
	// Usage counts the articles whose access list has the group.
	Usage(ctx context.Context, id int64) (int, error)
}

type Service struct {
	articles Articles
	groups   Groups
	members  usecase.Memberships
	tx       usecase.Transactor
}

func NewService(a Articles, g Groups, m usecase.Memberships, tx usecase.Transactor) *Service {
	return &Service{a, g, m, tx}
}

// Item is an article as one viewer sees it in a list.
type Item struct {
	Article domain.WikiArticle
	Level   domain.WikiLevel
}

func (s *Service) viewer(ctx context.Context, u domain.User) (domain.WikiViewer, error) {
	if !u.Can(domain.PermViewCrewBlock) {
		return domain.WikiViewer{}, domain.ErrPermissionDenied
	}
	groups, projects, err := s.articles.ViewerContext(ctx, u.ID)
	return domain.WikiViewer{User: u, Groups: groups, Projects: projects}, err
}

// visible loads the tree and what the viewer may open in it.
func (s *Service) visible(ctx context.Context, u domain.User) (domain.WikiTree, domain.WikiViewer, []Item, error) {
	v, err := s.viewer(ctx, u)
	if err != nil {
		return nil, v, nil, err
	}
	all, err := s.articles.All(ctx, u.OrgID)
	if err != nil {
		return nil, v, nil, err
	}
	tree := domain.NewWikiTree(all)
	var items []Item
	for i := range all {
		if l := tree.Level(&all[i], v); l != domain.WikiNone {
			items = append(items, Item{all[i], l})
		}
	}
	return tree, v, items, nil
}

// List is every article the viewer can open (the client builds "mine", "shared" and the archive).
func (s *Service) List(ctx context.Context, u domain.User) ([]Item, error) {
	_, _, items, err := s.visible(ctx, u)
	return items, err
}

// ProjectArticles is a project's wiki tab.
func (s *Service) ProjectArticles(ctx context.Context, u domain.User, projectID int64) ([]Item, error) {
	if _, err := usecase.RequireMember(ctx, s.members, projectID, u.ID, domain.ErrProjectNotFound); err != nil && !u.Can(domain.PermViewAllProjects) {
		return nil, err
	}
	_, _, items, err := s.visible(ctx, u)
	return slices.DeleteFunc(items, func(it Item) bool { return it.Article.ProjectID == nil || *it.Article.ProjectID != projectID }), err
}

// Home: what the viewer opened lately and the latest changes they can see.
func (s *Service) Home(ctx context.Context, u domain.User) (recent, changed []Item, err error) {
	_, _, items, err := s.visible(ctx, u)
	if err != nil {
		return nil, nil, err
	}
	byID := map[int64]Item{}
	for _, it := range items {
		byID[it.Article.ID] = it
	}
	ids, err := s.articles.Recent(ctx, u.ID, 20)
	if err != nil {
		return nil, nil, err
	}
	recent = []Item{}
	for _, id := range ids {
		if it, ok := byID[id]; ok && !it.Article.Archived && len(recent) < 10 {
			recent = append(recent, it)
		}
	}
	changed = slices.DeleteFunc(slices.Clone(items), func(it Item) bool { return it.Article.Archived })
	slices.SortFunc(changed, func(a, b Item) int { return b.Article.UpdatedAt.Compare(a.Article.UpdatedAt) })
	return recent, changed[:min(10, len(changed))], nil
}

func (s *Service) Search(ctx context.Context, u domain.User, q string) ([]Item, error) {
	q = strings.TrimSpace(q)
	if utf8.RuneCountInString(q) < 2 {
		return nil, domain.ErrWikiSearchShort
	}
	_, _, items, err := s.visible(ctx, u)
	if err != nil {
		return nil, err
	}
	ids, err := s.articles.Search(ctx, u.OrgID, q)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(items, func(it Item) bool { return !slices.Contains(ids, it.Article.ID) }), nil
}

// Page is an opened article: its content, the viewer's level and where it sits.
type Page struct {
	Item
	Path         []domain.WikiArticle
	AccessSource domain.WikiArticle
}

// article finds an article and checks the viewer's level (invisible ones are "not found").
func (s *Service) article(ctx context.Context, u domain.User, id int64, need domain.WikiLevel) (domain.WikiTree, domain.WikiViewer, *domain.WikiArticle, domain.WikiLevel, error) {
	tree, v, _, err := s.visible(ctx, u)
	if err != nil {
		return nil, v, nil, "", err
	}
	a, ok := tree[id]
	if !ok {
		return nil, v, nil, "", domain.ErrWikiNotFound
	}
	level := tree.Level(a, v)
	if level == domain.WikiNone {
		return nil, v, nil, "", domain.ErrWikiNotFound
	}
	if !level.AtLeast(need) {
		return nil, v, nil, "", domain.ErrPermissionDenied
	}
	return tree, v, a, level, nil
}

func (s *Service) Get(ctx context.Context, u domain.User, id int64) (Page, error) {
	tree, _, a, level, err := s.article(ctx, u, id, domain.WikiRead)
	if err != nil {
		return Page{}, err
	}
	if a.Content, err = s.articles.Content(ctx, id); err != nil {
		return Page{}, err
	}
	_ = s.articles.Viewed(ctx, u.ID, id) // "recent" is a convenience
	p := Page{Item: Item{*a, level}, AccessSource: *tree.AccessSource(a)}
	for _, anc := range tree.Path(a) {
		p.Path = append(p.Path, *anc)
	}
	return p, nil
}

// Draft is the article form.
type Draft struct {
	Title     string
	Content   string
	ParentID  *int64
	ProjectID *int64
}

// checkParent: a new or moved article can only go under an article the author may edit, never under itself.
func (s *Service) checkParent(tree domain.WikiTree, v domain.WikiViewer, id int64, parentID *int64) error {
	if parentID == nil {
		return nil
	}
	p, ok := tree[*parentID]
	if !ok || !tree.Level(p, v).AtLeast(domain.WikiEdit) || (id != 0 && tree.WouldCycle(id, *parentID)) {
		return domain.ErrWikiParent
	}
	return nil
}

func (s *Service) Create(ctx context.Context, u domain.User, d Draft) (Page, error) {
	if !u.Can(domain.PermCreateWikiArticle) {
		return Page{}, domain.ErrPermissionDenied
	}
	title, content, err := domain.NormalizeWikiText(d.Title, d.Content)
	if err != nil {
		return Page{}, err
	}
	tree, v, _, err := s.visible(ctx, u)
	if err != nil {
		return Page{}, err
	}
	if err := s.checkParent(tree, v, 0, d.ParentID); err != nil {
		return Page{}, err
	}
	if d.ProjectID != nil && !slices.Contains(v.Projects, *d.ProjectID) {
		return Page{}, domain.ErrProjectNotFound
	}
	a := domain.WikiArticle{
		ParentID: d.ParentID, ProjectID: d.ProjectID, OwnerID: &u.ID, Title: title, Content: content,
		InheritAccess: d.ParentID != nil, CompanyAccess: domain.WikiNone, UpdatedBy: &u.ID,
	}
	if d.ParentID != nil && a.ProjectID == nil {
		a.ProjectID = tree[*d.ParentID].ProjectID // sub-articles stay in their project's wiki
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.articles.Create(ctx, u.OrgID, &a); err != nil {
			return err
		}
		return s.articles.AddVersion(ctx, domain.WikiVersion{ArticleID: a.ID, Title: title, Content: content, AuthorID: &u.ID})
	})
	if err != nil {
		return Page{}, err
	}
	return s.Get(ctx, u, a.ID)
}

func (s *Service) Update(ctx context.Context, u domain.User, id int64, d Draft) (Page, error) {
	tree, v, a, _, err := s.article(ctx, u, id, domain.WikiEdit)
	if err != nil {
		return Page{}, err
	}
	title, content, err := domain.NormalizeWikiText(d.Title, d.Content)
	if err != nil {
		return Page{}, err
	}
	moved := (d.ParentID == nil) != (a.ParentID == nil) || (d.ParentID != nil && *d.ParentID != *a.ParentID)
	if moved {
		if err := s.checkParent(tree, v, id, d.ParentID); err != nil {
			return Page{}, err
		}
	}
	current, err := s.articles.Content(ctx, id)
	if err != nil {
		return Page{}, err
	}
	next := *a
	next.Title, next.Content, next.ParentID, next.UpdatedBy = title, content, d.ParentID, &u.ID
	if next.ParentID == nil {
		next.InheritAccess = false // a root has nothing to inherit
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.articles.Update(ctx, next); err != nil {
			return err
		}
		if title == a.Title && content == current {
			return nil
		}
		return s.articles.AddVersion(ctx, domain.WikiVersion{ArticleID: id, Title: title, Content: content, AuthorID: &u.ID})
	})
	if err != nil {
		return Page{}, err
	}
	return s.Get(ctx, u, id)
}

func (s *Service) SetArchived(ctx context.Context, u domain.User, id int64, archived bool) (Page, error) {
	if _, _, _, _, err := s.article(ctx, u, id, domain.WikiEdit); err != nil {
		return Page{}, err
	}
	if err := s.articles.SetArchived(ctx, id, archived); err != nil {
		return Page{}, err
	}
	return s.Get(ctx, u, id)
}

// Access is an article's sharing settings.
type Access struct {
	Inherit bool
	Company domain.WikiLevel
	Grants  []domain.WikiGrant
}

func (s *Service) Access(ctx context.Context, u domain.User, id int64) (Access, error) {
	_, _, a, _, err := s.article(ctx, u, id, domain.WikiRead)
	if err != nil {
		return Access{}, err
	}
	return Access{Inherit: a.InheritAccess, Company: a.CompanyAccess, Grants: a.Grants}, nil
}

// SetAccess changes who sees an article; editors share it. Inheriting drops the article's own list.
func (s *Service) SetAccess(ctx context.Context, u domain.User, id int64, in Access) (Page, error) {
	_, _, a, _, err := s.article(ctx, u, id, domain.WikiEdit)
	if err != nil {
		return Page{}, err
	}
	if in.Inherit && a.ParentID == nil {
		return Page{}, domain.ErrWikiAccessInvalid
	}
	if in.Company == "" {
		in.Company = domain.WikiNone
	}
	if !slices.Contains([]domain.WikiLevel{domain.WikiNone, domain.WikiRead, domain.WikiEdit}, in.Company) {
		return Page{}, domain.ErrWikiAccessInvalid
	}
	seen := map[[2]int64]bool{}
	grants := []domain.WikiGrant{}
	for _, g := range in.Grants {
		key := [2]int64{g.UserID, g.GroupID}
		if (g.UserID == 0) == (g.GroupID == 0) || (g.Level != domain.WikiRead && g.Level != domain.WikiEdit) || seen[key] {
			return Page{}, domain.ErrWikiAccessInvalid
		}
		seen[key] = true
		grants = append(grants, g)
	}
	if in.Inherit {
		in.Company, grants = domain.WikiNone, []domain.WikiGrant{}
	}
	if err := s.articles.SetAccess(ctx, id, in.Inherit, in.Company, grants); err != nil {
		return Page{}, err
	}
	return s.Get(ctx, u, id)
}

func (s *Service) Versions(ctx context.Context, u domain.User, id int64) ([]domain.WikiVersion, error) {
	if _, _, _, _, err := s.article(ctx, u, id, domain.WikiRead); err != nil {
		return nil, err
	}
	return s.articles.Versions(ctx, id)
}

func (s *Service) Version(ctx context.Context, u domain.User, versionID int64) (domain.WikiVersion, error) {
	ver, err := s.articles.Version(ctx, versionID)
	if err != nil {
		return ver, usecase.NotFoundAs(err, domain.ErrWikiNotFound)
	}
	if _, _, _, _, err := s.article(ctx, u, ver.ArticleID, domain.WikiRead); err != nil {
		return domain.WikiVersion{}, err
	}
	return ver, nil
}

// --- groups ---

func (s *Service) Groups(ctx context.Context, u domain.User) ([]domain.WikiGroup, error) {
	if !u.Can(domain.PermViewCrewBlock) {
		return nil, domain.ErrPermissionDenied
	}
	return s.groups.List(ctx, u.OrgID)
}

func normalizeGroup(g *domain.WikiGroup) error {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" || len(g.Name) > 100 {
		return domain.ErrWikiGroupInvalid
	}
	slices.Sort(g.MemberIDs)
	g.MemberIDs = slices.Compact(g.MemberIDs)
	return nil
}

func (s *Service) CreateGroup(ctx context.Context, u domain.User, g domain.WikiGroup) (domain.WikiGroup, error) {
	if !u.Can(domain.PermCreateWikiArticle) {
		return g, domain.ErrPermissionDenied
	}
	if err := normalizeGroup(&g); err != nil {
		return g, err
	}
	g.OwnerID = &u.ID
	err := s.groups.Create(ctx, u.OrgID, &g)
	return g, err
}

// canManage: a group's creator, or company owners and admins.
func canManage(u domain.User, g domain.WikiGroup) bool {
	return (g.OwnerID != nil && *g.OwnerID == u.ID) || u.Role == domain.CompanyOwner || u.Role == domain.CompanyAdmin
}

func (s *Service) UpdateGroup(ctx context.Context, u domain.User, id int64, in domain.WikiGroup) (domain.WikiGroup, error) {
	g, err := s.groups.Get(ctx, u.OrgID, id)
	if err != nil {
		return g, usecase.NotFoundAs(err, domain.ErrWikiGroupNotFound)
	}
	if !canManage(u, g) {
		return g, domain.ErrPermissionDenied
	}
	g.Name, g.MemberIDs = in.Name, in.MemberIDs
	if err := normalizeGroup(&g); err != nil {
		return g, err
	}
	return g, s.groups.Update(ctx, g)
}

func (s *Service) GroupUsage(ctx context.Context, u domain.User, id int64) (int, error) {
	if !u.Can(domain.PermViewCrewBlock) {
		return 0, domain.ErrPermissionDenied
	}
	return s.groups.Usage(ctx, id)
}

// DeleteGroup also removes it from every article's access list (by the foreign key).
func (s *Service) DeleteGroup(ctx context.Context, u domain.User, id int64) error {
	g, err := s.groups.Get(ctx, u.OrgID, id)
	if err != nil {
		return usecase.NotFoundAs(err, domain.ErrWikiGroupNotFound)
	}
	if !canManage(u, g) {
		return domain.ErrPermissionDenied
	}
	return s.groups.Delete(ctx, id)
}
