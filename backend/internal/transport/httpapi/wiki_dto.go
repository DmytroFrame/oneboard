package httpapi

import (
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/wiki"
)

type wikiItemDTO struct {
	ID            int64            `json:"id"`
	ParentID      *int64           `json:"parentId"`
	ProjectID     *int64           `json:"projectId"`
	OwnerID       *int64           `json:"ownerId"`
	OwnerName     string           `json:"ownerName"`
	Title         string           `json:"title"`
	Archived      bool             `json:"archived"`
	Inherit       bool             `json:"inheritAccess"`
	Company       domain.WikiLevel `json:"companyAccess"`
	SharedWith    int              `json:"sharedWith"` // people and groups on its own list
	Level         domain.WikiLevel `json:"level"`      // the viewer's
	UpdatedAt     time.Time        `json:"updatedAt"`
	UpdatedByName string           `json:"updatedByName"`
}

func wikiItemDTOOf(it wiki.Item) wikiItemDTO {
	a := it.Article
	return wikiItemDTO{a.ID, a.ParentID, a.ProjectID, a.OwnerID, a.OwnerName, a.Title, a.Archived, a.InheritAccess,
		a.CompanyAccess, len(a.Grants), it.Level, a.UpdatedAt, a.UpdatedByName}
}

type wikiCrumbDTO struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type wikiPageDTO struct {
	wikiItemDTO
	Content string         `json:"content"`
	Path    []wikiCrumbDTO `json:"path"`
	// Whose access list applies (the article itself unless it inherits).
	AccessFrom wikiCrumbDTO `json:"accessFrom"`
}

func wikiPageDTOOf(p wiki.Page) wikiPageDTO {
	return wikiPageDTO{
		wikiItemDTO: wikiItemDTOOf(p.Item),
		Content:     p.Article.Content,
		Path:        mapSlice(nonNil(p.Path), func(a domain.WikiArticle) wikiCrumbDTO { return wikiCrumbDTO{a.ID, a.Title} }),
		AccessFrom:  wikiCrumbDTO{p.AccessSource.ID, p.AccessSource.Title},
	}
}

type wikiDraftInput struct {
	Title     string `json:"title"`
	Content   string `json:"content"`
	ParentID  *int64 `json:"parentId"`
	ProjectID *int64 `json:"projectId"`
}

func (in wikiDraftInput) draft() wiki.Draft {
	return wiki.Draft{Title: in.Title, Content: in.Content, ParentID: in.ParentID, ProjectID: in.ProjectID}
}

type wikiGrantDTO struct {
	UserID  int64            `json:"userId,omitempty"`
	GroupID int64            `json:"groupId,omitempty"`
	Level   domain.WikiLevel `json:"level"`
}

type wikiAccessDTO struct {
	Inherit bool             `json:"inherit"`
	Company domain.WikiLevel `json:"company"`
	Grants  []wikiGrantDTO   `json:"grants"`
}

func wikiAccessDTOOf(a wiki.Access) wikiAccessDTO {
	return wikiAccessDTO{a.Inherit, a.Company, mapSlice(nonNil(a.Grants), func(g domain.WikiGrant) wikiGrantDTO {
		return wikiGrantDTO{g.UserID, g.GroupID, g.Level}
	})}
}

func (in wikiAccessDTO) access() wiki.Access {
	return wiki.Access{Inherit: in.Inherit, Company: in.Company, Grants: mapSlice(in.Grants, func(g wikiGrantDTO) domain.WikiGrant {
		return domain.WikiGrant{UserID: g.UserID, GroupID: g.GroupID, Level: g.Level}
	})}
}

type wikiVersionDTO struct {
	ID         int64     `json:"id"`
	ArticleID  int64     `json:"articleId"`
	Title      string    `json:"title"`
	Content    *string   `json:"content,omitempty"`
	AuthorName string    `json:"authorName"` // "" when the author was deleted
	CreatedAt  time.Time `json:"createdAt"`
}

func wikiVersionDTOOf(v domain.WikiVersion) wikiVersionDTO {
	d := wikiVersionDTO{ID: v.ID, ArticleID: v.ArticleID, Title: v.Title, AuthorName: v.AuthorName, CreatedAt: v.CreatedAt}
	if v.Content != "" {
		d.Content = &v.Content
	}
	return d
}

type wikiGroupDTO struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	OwnerID   *int64  `json:"ownerId"`
	MemberIDs []int64 `json:"memberIds"`
}

func wikiGroupDTOOf(g domain.WikiGroup) wikiGroupDTO {
	return wikiGroupDTO{g.ID, g.Name, g.OwnerID, nonNil(g.MemberIDs)}
}

func (in wikiGroupDTO) group() domain.WikiGroup {
	return domain.WikiGroup{Name: in.Name, MemberIDs: in.MemberIDs}
}

type projectCommentDTO struct {
	ID         int64                         `json:"id"`
	ProjectID  int64                         `json:"projectId"`
	AuthorID   *int64                        `json:"authorId"`
	AuthorName string                        `json:"authorName"`
	Category   domain.ProjectCommentCategory `json:"category"`
	Title      string                        `json:"title"`
	Body       string                        `json:"body"`
	CreatedAt  time.Time                     `json:"createdAt"`
	UpdatedAt  time.Time                     `json:"updatedAt"`
}

func projectCommentDTOOf(c domain.ProjectComment) projectCommentDTO {
	return projectCommentDTO{c.ID, c.ProjectID, c.AuthorID, c.AuthorName, c.Category, c.Title, c.Body, c.CreatedAt, c.UpdatedAt}
}

type projectCommentInput struct {
	Category domain.ProjectCommentCategory `json:"category"`
	Title    string                        `json:"title"`
	Body     string                        `json:"body"`
}

func (in projectCommentInput) comment() domain.ProjectComment {
	return domain.ProjectComment{Category: in.Category, Title: in.Title, Body: in.Body}
}
