package domain

import (
	"slices"
	"strings"
	"time"
)

// WikiLevel is what someone may do with an article.
type WikiLevel string

const (
	WikiNone WikiLevel = "none"
	WikiRead WikiLevel = "read"
	WikiEdit WikiLevel = "edit"
)

func (l WikiLevel) rank() int { return map[WikiLevel]int{WikiRead: 1, WikiEdit: 2}[l] }

// AtLeast reports whether l grants at least min.
func (l WikiLevel) AtLeast(min WikiLevel) bool { return l.rank() >= min.rank() }

func maxLevel(a, b WikiLevel) WikiLevel {
	if b.rank() > a.rank() {
		return b
	}
	return a
}

// WikiGrant gives a person or a group access to an article.
type WikiGrant struct {
	UserID  int64 // one of UserID / GroupID
	GroupID int64
	Level   WikiLevel
}

// WikiArticle is an article without its content (trees and lists) or with it (Content set).
type WikiArticle struct {
	ID            int64
	ParentID      *int64
	ProjectID     *int64
	OwnerID       *int64
	OwnerName     string
	Title         string
	Content       string
	InheritAccess bool
	CompanyAccess WikiLevel
	Archived      bool
	Grants        []WikiGrant
	CreatedAt     time.Time
	UpdatedAt     time.Time
	UpdatedBy     *int64
	UpdatedByName string
}

const (
	MaxWikiTitle   = 200
	MaxWikiContent = 500_000
)

func NormalizeWikiText(title, content string) (string, string, error) {
	title = strings.TrimSpace(title)
	if title == "" || len(title) > MaxWikiTitle || len(content) > MaxWikiContent {
		return "", "", ErrWikiArticleInvalid
	}
	return title, content, nil
}

// WikiViewer is who is asking, with what the access rules need to know about them.
type WikiViewer struct {
	User     User
	Groups   []int64 // wiki groups they are in
	Projects []int64 // projects they are a member of
}

// WikiTree resolves access over a set of articles (all of them: inheritance walks up the parents).
type WikiTree map[int64]*WikiArticle

func NewWikiTree(list []WikiArticle) WikiTree {
	t := WikiTree{}
	for i := range list {
		t[list[i].ID] = &list[i]
	}
	return t
}

// AccessSource is the article whose access list applies to a (itself, or the nearest ancestor that
// doesn't inherit).
func (t WikiTree) AccessSource(a *WikiArticle) *WikiArticle {
	seen := map[int64]bool{}
	for a.InheritAccess && a.ParentID != nil && !seen[a.ID] {
		seen[a.ID] = true
		p, ok := t[*a.ParentID]
		if !ok {
			break
		}
		a = p
	}
	return a
}

// Level is what v may do with article a: its owner and editors edit, readers read, a project
// article is open to the project's members, and "everyone in the company" covers staff.
func (t WikiTree) Level(a *WikiArticle, v WikiViewer) WikiLevel {
	if !v.User.Can(PermViewCrewBlock) { // contractors and candidates have no wiki
		return WikiNone
	}
	level := WikiNone
	if a.OwnerID != nil && *a.OwnerID == v.User.ID {
		return WikiEdit
	}
	if a.ProjectID != nil && slices.Contains(v.Projects, *a.ProjectID) {
		level = WikiEdit
	}
	src := t.AccessSource(a)
	if src.OwnerID != nil && *src.OwnerID == v.User.ID {
		return WikiEdit
	}
	if src.CompanyAccess.AtLeast(WikiRead) {
		level = maxLevel(level, src.CompanyAccess)
	}
	for _, g := range src.Grants {
		if (g.UserID != 0 && g.UserID == v.User.ID) || (g.GroupID != 0 && slices.Contains(v.Groups, g.GroupID)) {
			level = maxLevel(level, g.Level)
		}
	}
	return level
}

// WouldCycle reports whether making parentID the parent of id would put id under itself.
func (t WikiTree) WouldCycle(id, parentID int64) bool {
	for cur, ok := t[parentID]; ok; {
		if cur.ID == id {
			return true
		}
		if cur.ParentID == nil {
			return false
		}
		cur, ok = t[*cur.ParentID]
	}
	return false
}

// Path is the chain of ancestors from the root down to a's parent.
func (t WikiTree) Path(a *WikiArticle) []*WikiArticle {
	var out []*WikiArticle
	seen := map[int64]bool{a.ID: true}
	for p := a.ParentID; p != nil; {
		parent, ok := t[*p]
		if !ok || seen[parent.ID] {
			break
		}
		seen[parent.ID] = true
		out = append([]*WikiArticle{parent}, out...)
		p = parent.ParentID
	}
	return out
}

type WikiGroup struct {
	ID        int64
	Name      string
	OwnerID   *int64
	MemberIDs []int64
}

type WikiVersion struct {
	ID         int64
	ArticleID  int64
	Title      string
	Content    string // empty in lists
	AuthorID   *int64
	AuthorName string
	CreatedAt  time.Time
}

// ProjectCommentCategory: OmiTeam's project comment categories.
type ProjectCommentCategory string

var ProjectCommentCategories = []ProjectCommentCategory{"call", "requirements", "design", "estimate", "task_management", "other"}

type ProjectComment struct {
	ID         int64
	ProjectID  int64
	AuthorID   *int64
	AuthorName string
	Category   ProjectCommentCategory
	Title      string
	Body       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (c *ProjectComment) Normalize() error {
	c.Title, c.Body = strings.TrimSpace(c.Title), strings.TrimSpace(c.Body)
	if !slices.Contains(ProjectCommentCategories, c.Category) || c.Title == "" || len(c.Title) > 200 || len(c.Body) > 20000 {
		return ErrProjectCommentInvalid
	}
	return nil
}
