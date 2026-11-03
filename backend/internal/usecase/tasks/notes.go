package tasks

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

// --- tags ---

var colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func cleanTag(t *domain.Tag) error {
	t.Name = strings.TrimSpace(t.Name)
	if t.Name == "" || len([]rune(t.Name)) > 50 || !colorRe.MatchString(t.Color) {
		return domain.ErrTagInvalid
	}
	return nil
}

func (s *Service) Tags(ctx context.Context, actor domain.User, projectID int64) ([]domain.Tag, error) {
	if _, err := s.member(ctx, actor, projectID); err != nil {
		return nil, err
	}
	return s.tags.List(ctx, projectID)
}

func (s *Service) CreateTag(ctx context.Context, actor domain.User, projectID int64, t domain.Tag) (domain.Tag, error) {
	if err := s.manager(ctx, actor, projectID); err != nil {
		return t, err
	}
	t.ProjectID = projectID
	if err := cleanTag(&t); err != nil {
		return t, err
	}
	if err := s.tags.Create(ctx, &t); errors.Is(err, domain.ErrDuplicate) {
		return t, domain.ErrTagExists
	} else if err != nil {
		return t, err
	}
	return t, nil
}

func (s *Service) UpdateTag(ctx context.Context, actor domain.User, tagID int64, name, color string) (domain.Tag, error) {
	t, err := s.tags.Get(ctx, tagID)
	if err != nil {
		return t, usecase.NotFoundAs(err, domain.ErrTagNotFound)
	}
	if err := s.manager(ctx, actor, t.ProjectID); err != nil {
		return t, err
	}
	t.Name, t.Color = name, color
	if err := cleanTag(&t); err != nil {
		return t, err
	}
	if err := s.tags.Update(ctx, t); errors.Is(err, domain.ErrDuplicate) {
		return t, domain.ErrTagExists
	} else if err != nil {
		return t, err
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventColumnsChanged, ProjectID: t.ProjectID, ActorID: actor.ID})
	return t, nil
}

// DeleteTag also removes it from every task.
func (s *Service) DeleteTag(ctx context.Context, actor domain.User, tagID int64) error {
	t, err := s.tags.Get(ctx, tagID)
	if err != nil {
		return usecase.NotFoundAs(err, domain.ErrTagNotFound)
	}
	if err := s.manager(ctx, actor, t.ProjectID); err != nil {
		return err
	}
	if err := s.tags.Delete(ctx, tagID); err != nil {
		return err
	}
	s.events.Publish(ctx, domain.BoardEvent{Type: domain.EventColumnsChanged, ProjectID: t.ProjectID, ActorID: actor.ID})
	return nil
}

// --- checklists ---

func cleanText(text string) (string, error) {
	text = strings.TrimSpace(text)
	if text == "" || len([]rune(text)) > 500 {
		return "", domain.ErrChecklistInvalid
	}
	return text, nil
}

func (s *Service) Checklists(ctx context.Context, actor domain.User, cardID int64) ([]domain.Checklist, error) {
	if _, err := s.card(ctx, actor, cardID); err != nil {
		return nil, err
	}
	return s.checklists.List(ctx, cardID)
}

// checklist loads a checklist on a card the actor can see.
func (s *Service) checklist(ctx context.Context, actor domain.User, id int64) (domain.Checklist, error) {
	c, err := s.checklists.Get(ctx, id)
	if err != nil {
		return c, usecase.NotFoundAs(err, domain.ErrChecklistNotFound)
	}
	if _, err := s.card(ctx, actor, c.CardID); err != nil {
		return c, domain.ErrChecklistNotFound
	}
	return c, nil
}

// afterChecklist re-sends the card, so its "3/5" progress updates on everyone's board.
func (s *Service) afterChecklist(ctx context.Context, actor domain.User, cardID int64) {
	if _, err := s.changed(ctx, actor.ID, cardID); err != nil {
		s.log.Warn("card after checklist change", "err", err)
	}
}

func (s *Service) CreateChecklist(ctx context.Context, actor domain.User, cardID int64, title string) (domain.Checklist, error) {
	if _, err := s.card(ctx, actor, cardID); err != nil {
		return domain.Checklist{}, err
	}
	title, err := cleanText(title)
	if err != nil {
		return domain.Checklist{}, err
	}
	c := domain.Checklist{CardID: cardID, Title: title, Items: []domain.ChecklistItem{}}
	if err := s.checklists.Create(ctx, &c); err != nil {
		return c, err
	}
	return c, nil
}

func (s *Service) UpdateChecklist(ctx context.Context, actor domain.User, id int64, title string, collapsed bool) (domain.Checklist, error) {
	c, err := s.checklist(ctx, actor, id)
	if err != nil {
		return c, err
	}
	if c.Title, err = cleanText(title); err != nil {
		return c, err
	}
	c.Collapsed = collapsed
	return c, s.checklists.Update(ctx, c)
}

func (s *Service) DeleteChecklist(ctx context.Context, actor domain.User, id int64) error {
	c, err := s.checklist(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := s.checklists.Delete(ctx, id); err != nil {
		return err
	}
	s.afterChecklist(ctx, actor, c.CardID)
	return nil
}

func (s *Service) AddItem(ctx context.Context, actor domain.User, checklistID int64, text string) (domain.ChecklistItem, error) {
	c, err := s.checklist(ctx, actor, checklistID)
	if err != nil {
		return domain.ChecklistItem{}, err
	}
	text, err = cleanText(text)
	if err != nil {
		return domain.ChecklistItem{}, err
	}
	it := domain.ChecklistItem{ChecklistID: checklistID, Text: text}
	if err := s.checklists.AddItem(ctx, &it); err != nil {
		return it, err
	}
	s.afterChecklist(ctx, actor, c.CardID)
	return it, nil
}

func (s *Service) UpdateItem(ctx context.Context, actor domain.User, itemID int64, text string, done bool) (domain.ChecklistItem, error) {
	it, err := s.checklists.Item(ctx, itemID)
	if err != nil {
		return it, usecase.NotFoundAs(err, domain.ErrChecklistNotFound)
	}
	c, err := s.checklist(ctx, actor, it.ChecklistID)
	if err != nil {
		return it, err
	}
	if it.Text, err = cleanText(text); err != nil {
		return it, err
	}
	it.Done = done
	if err := s.checklists.UpdateItem(ctx, it); err != nil {
		return it, err
	}
	s.afterChecklist(ctx, actor, c.CardID)
	return it, nil
}

func (s *Service) DeleteItem(ctx context.Context, actor domain.User, itemID int64) error {
	it, err := s.checklists.Item(ctx, itemID)
	if err != nil {
		return usecase.NotFoundAs(err, domain.ErrChecklistNotFound)
	}
	c, err := s.checklist(ctx, actor, it.ChecklistID)
	if err != nil {
		return err
	}
	if err := s.checklists.DeleteItem(ctx, itemID); err != nil {
		return err
	}
	s.afterChecklist(ctx, actor, c.CardID)
	return nil
}

// --- comments ---

func cleanComment(body string) (string, error) {
	body = strings.TrimSpace(body)
	if body == "" || len(body) > 10000 {
		return "", domain.ErrCommentInvalid
	}
	return body, nil
}

func (s *Service) Comments(ctx context.Context, actor domain.User, cardID int64) ([]domain.CardComment, error) {
	if _, err := s.card(ctx, actor, cardID); err != nil {
		return nil, err
	}
	return s.comments.List(ctx, cardID)
}

func (s *Service) AddComment(ctx context.Context, actor domain.User, cardID int64, body string) (domain.CardComment, error) {
	if _, err := s.card(ctx, actor, cardID); err != nil {
		return domain.CardComment{}, err
	}
	body, err := cleanComment(body)
	if err != nil {
		return domain.CardComment{}, err
	}
	author := actor.ID
	c := domain.CardComment{CardID: cardID, AuthorID: &author, AuthorName: actor.Name, Body: body}
	if err := s.comments.Create(ctx, &c); err != nil {
		return c, err
	}
	s.afterChecklist(ctx, actor, cardID) // the comment count on the card face
	return c, nil
}

// comment loads a comment the actor may change: their own, or any for project owners and admins.
func (s *Service) comment(ctx context.Context, actor domain.User, id int64) (domain.CardComment, error) {
	c, err := s.comments.Get(ctx, id)
	if err != nil {
		return c, usecase.NotFoundAs(err, domain.ErrCommentNotFound)
	}
	card, err := s.card(ctx, actor, c.CardID)
	if err != nil {
		return c, domain.ErrCommentNotFound
	}
	if c.AuthorID != nil && *c.AuthorID == actor.ID {
		return c, nil
	}
	if role, _ := s.members.Role(ctx, card.ProjectID, actor.ID); role == domain.RoleOwner || role == domain.RoleAdmin {
		return c, nil
	}
	return c, domain.ErrPermissionDenied
}

func (s *Service) UpdateComment(ctx context.Context, actor domain.User, id int64, body string) (domain.CardComment, error) {
	c, err := s.comment(ctx, actor, id)
	if err != nil {
		return c, err
	}
	if c.AuthorID == nil || *c.AuthorID != actor.ID {
		return c, domain.ErrPermissionDenied // others' words are only deleted, never edited
	}
	if c.Body, err = cleanComment(body); err != nil {
		return c, err
	}
	return c, s.comments.Update(ctx, id, c.Body)
}

func (s *Service) DeleteComment(ctx context.Context, actor domain.User, id int64) error {
	c, err := s.comment(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := s.comments.Delete(ctx, id); err != nil {
		return err
	}
	s.afterChecklist(ctx, actor, c.CardID)
	return nil
}
