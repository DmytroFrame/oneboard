package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/tasks"
)

func init() { registerRoutes(NewTaskHandler) }

// TaskHandler serves OmiTeam's task management on top of the board: details, templates, tags,
// checklists, comments, history, column statuses, bulk moves and planning.
type TaskHandler struct {
	tasks   *tasks.Service
	present *Presenter
}

func NewTaskHandler(t *tasks.Service, present *Presenter) *TaskHandler {
	return &TaskHandler{tasks: t, present: present}
}

func (h *TaskHandler) Mount(_, private *echo.Group) {
	private.PUT("/cards/:id/details", h.setDetails)
	private.PUT("/cards/:id/template", h.setTemplate)
	private.GET("/cards/:id/history", h.history)
	private.GET("/projects/:id/templates", h.templates)
	private.GET("/projects/:id/task-search", h.findTasks)
	private.POST("/templates/:id/use", h.fromTemplate)

	private.GET("/projects/:id/tags", h.tags)
	private.POST("/projects/:id/tags", h.createTag)
	private.PUT("/tags/:id", h.updateTag)
	private.DELETE("/tags/:id", h.deleteTag)

	private.GET("/cards/:id/checklists", h.checklists)
	private.POST("/cards/:id/checklists", h.createChecklist)
	private.PUT("/checklists/:id", h.updateChecklist)
	private.DELETE("/checklists/:id", h.deleteChecklist)
	private.POST("/checklists/:id/items", h.addItem)
	private.PUT("/checklist-items/:id", h.updateItem)
	private.DELETE("/checklist-items/:id", h.deleteItem)

	private.GET("/cards/:id/comments", h.comments)
	private.POST("/cards/:id/comments", h.addComment)
	private.PUT("/card-comments/:id", h.updateComment)
	private.DELETE("/card-comments/:id", h.deleteComment)

	private.PUT("/columns/:id/status", h.setColumnStatus)
	private.POST("/columns/:id/bulk-move", h.bulkMove)

	private.GET("/projects/:id/milestones", h.milestones)
	private.POST("/projects/:id/milestones", h.saveMilestone)
	private.PUT("/milestones/:itemId", h.saveMilestone)
	private.GET("/projects/:id/sprints", h.sprints)
	private.POST("/projects/:id/sprints", h.saveSprint)
	private.PUT("/sprints/:itemId", h.saveSprint)
	private.GET("/projects/:id/versions", h.versions)
	private.POST("/projects/:id/versions", h.saveVersion)
	private.PUT("/versions/:itemId", h.saveVersion)
	private.DELETE("/milestones/:itemId", h.deletePlanning("milestones"))
	private.DELETE("/sprints/:itemId", h.deletePlanning("sprints"))
	private.DELETE("/versions/:itemId", h.deletePlanning("versions"))
}

// withID runs fn with the :id path parameter.
func withID(c *echo.Context, fn func(id int64) error) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	return fn(id)
}

type detailsDTO struct {
	StoryPoints *int               `json:"storyPoints"`
	BudgetHours *float64           `json:"budgetHours"`
	Billable    bool               `json:"billable"`
	SpecialType domain.SpecialType `json:"specialType"`
	MilestoneID *int64             `json:"milestoneId"`
	SprintID    *int64             `json:"sprintId"`
	VersionID   *int64             `json:"versionId"`
	TagIDs      []int64            `json:"tagIds"`
}

func (h *TaskHandler) setDetails(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in detailsDTO
		if err := bind(c, &in); err != nil {
			return err
		}
		card, err := h.tasks.SetDetails(c.Request().Context(), currentUser(c), id, domain.TaskDetails(in))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, h.present.Card(card))
	})
}

func (h *TaskHandler) setTemplate(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			Template bool `json:"template"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		card, err := h.tasks.SetTemplate(c.Request().Context(), currentUser(c), id, in.Template)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, h.present.Card(card))
	})
}

func (h *TaskHandler) history(c *echo.Context) error {
	return withID(c, func(id int64) error {
		offset, _ := strconv.Atoi(c.QueryParam("offset"))
		items, err := h.tasks.History(c.Request().Context(), currentUser(c), id, offset)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(items, toHistory))
	})
}

func (h *TaskHandler) findTasks(c *echo.Context) error {
	return withID(c, func(id int64) error {
		cards, err := h.tasks.FindTasks(c.Request().Context(), currentUser(c), id, c.QueryParam("q"))
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(cards, h.present.Card))
	})
}

func (h *TaskHandler) templates(c *echo.Context) error {
	return withID(c, func(id int64) error {
		cards, err := h.tasks.Templates(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(cards, h.present.Card))
	})
}

func (h *TaskHandler) fromTemplate(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			ColumnID int64 `json:"columnId"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		card, err := h.tasks.FromTemplate(c.Request().Context(), currentUser(c), id, in.ColumnID)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, h.present.Card(card))
	})
}

// --- tags ---

func (h *TaskHandler) tags(c *echo.Context) error {
	return withID(c, func(id int64) error {
		items, err := h.tasks.Tags(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(items, toTag))
	})
}

func (h *TaskHandler) createTag(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in tagDTO
		if err := bind(c, &in); err != nil {
			return err
		}
		t, err := h.tasks.CreateTag(c.Request().Context(), currentUser(c), id, domain.Tag{Name: in.Name, Color: in.Color})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, toTag(t))
	})
}

func (h *TaskHandler) updateTag(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in tagDTO
		if err := bind(c, &in); err != nil {
			return err
		}
		t, err := h.tasks.UpdateTag(c.Request().Context(), currentUser(c), id, in.Name, in.Color)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toTag(t))
	})
}

func (h *TaskHandler) deleteTag(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.tasks.DeleteTag(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

// --- checklists ---

func (h *TaskHandler) checklists(c *echo.Context) error {
	return withID(c, func(id int64) error {
		lists, err := h.tasks.Checklists(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(lists, toChecklist))
	})
}

func (h *TaskHandler) createChecklist(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			Title string `json:"title"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		l, err := h.tasks.CreateChecklist(c.Request().Context(), currentUser(c), id, in.Title)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, toChecklist(l))
	})
}

func (h *TaskHandler) updateChecklist(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			Title     string `json:"title"`
			Collapsed bool   `json:"collapsed"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		l, err := h.tasks.UpdateChecklist(c.Request().Context(), currentUser(c), id, in.Title, in.Collapsed)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toChecklist(l))
	})
}

func (h *TaskHandler) deleteChecklist(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.tasks.DeleteChecklist(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

func (h *TaskHandler) addItem(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in checklistItemDTO
		if err := bind(c, &in); err != nil {
			return err
		}
		it, err := h.tasks.AddItem(c.Request().Context(), currentUser(c), id, in.Text)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, toChecklistItem(it))
	})
}

func (h *TaskHandler) updateItem(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in checklistItemDTO
		if err := bind(c, &in); err != nil {
			return err
		}
		it, err := h.tasks.UpdateItem(c.Request().Context(), currentUser(c), id, in.Text, in.Done)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toChecklistItem(it))
	})
}

func (h *TaskHandler) deleteItem(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.tasks.DeleteItem(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

// --- comments ---

func (h *TaskHandler) comments(c *echo.Context) error {
	return withID(c, func(id int64) error {
		items, err := h.tasks.Comments(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(items, toCardComment))
	})
}

type commentBody struct {
	Body string `json:"body"`
}

func (h *TaskHandler) addComment(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in commentBody
		if err := bind(c, &in); err != nil {
			return err
		}
		cm, err := h.tasks.AddComment(c.Request().Context(), currentUser(c), id, in.Body)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, toCardComment(cm))
	})
}

func (h *TaskHandler) updateComment(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in commentBody
		if err := bind(c, &in); err != nil {
			return err
		}
		cm, err := h.tasks.UpdateComment(c.Request().Context(), currentUser(c), id, in.Body)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, toCardComment(cm))
	})
}

func (h *TaskHandler) deleteComment(c *echo.Context) error {
	return withID(c, func(id int64) error {
		if err := h.tasks.DeleteComment(c.Request().Context(), currentUser(c), id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

// --- columns ---

func (h *TaskHandler) setColumnStatus(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			Status domain.ColumnStatus `json:"status"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		if err := h.tasks.SetColumnStatus(c.Request().Context(), currentUser(c), id, in.Status); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	})
}

func (h *TaskHandler) bulkMove(c *echo.Context) error {
	return withID(c, func(id int64) error {
		var in struct {
			Target   string `json:"target"`
			TargetID *int64 `json:"targetId"`
		}
		if err := bind(c, &in); err != nil {
			return err
		}
		n, err := h.tasks.BulkMove(c.Request().Context(), currentUser(c), id, tasks.BulkTarget{Kind: in.Target, ID: in.TargetID})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]int{"moved": n, "limit": tasks.BulkLimit})
	})
}

// --- planning ---

// planningIDs reads the project (POST) or the item (PUT) from the path.
func planningIDs(c *echo.Context) (projectID, itemID int64, err error) {
	if c.Param("itemId") != "" {
		itemID, err = pathID(c, "itemId")
		return
	}
	projectID, err = pathID(c, "id")
	return
}

func (h *TaskHandler) milestones(c *echo.Context) error {
	return withID(c, func(id int64) error {
		items, err := h.tasks.Milestones(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(items, toMilestone))
	})
}

func (h *TaskHandler) saveMilestone(c *echo.Context) error {
	projectID, itemID, err := planningIDs(c)
	if err != nil {
		return err
	}
	var in milestoneDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	if itemID != 0 {
		projectID = in.ProjectID
	}
	m, err := h.tasks.SaveMilestone(c.Request().Context(), currentUser(c), domain.Milestone{
		ID: itemID, ProjectID: projectID, Name: in.Name, Description: in.Description, Start: in.StartDate, End: in.EndDate})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toMilestone(m))
}

func (h *TaskHandler) sprints(c *echo.Context) error {
	return withID(c, func(id int64) error {
		items, err := h.tasks.Sprints(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(items, toSprint))
	})
}

func (h *TaskHandler) saveSprint(c *echo.Context) error {
	projectID, itemID, err := planningIDs(c)
	if err != nil {
		return err
	}
	var in sprintDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	if itemID != 0 {
		projectID = in.ProjectID
	}
	s, err := h.tasks.SaveSprint(c.Request().Context(), currentUser(c), domain.Sprint{
		ID: itemID, ProjectID: projectID, Name: in.Name, Goal: in.Goal, Start: in.StartDate, End: in.EndDate, Status: in.Status})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toSprint(s))
}

func (h *TaskHandler) versions(c *echo.Context) error {
	return withID(c, func(id int64) error {
		items, err := h.tasks.Versions(c.Request().Context(), currentUser(c), id)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, mapSlice(items, toVersion))
	})
}

func (h *TaskHandler) saveVersion(c *echo.Context) error {
	projectID, itemID, err := planningIDs(c)
	if err != nil {
		return err
	}
	var in versionDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	if itemID != 0 {
		projectID = in.ProjectID
	}
	v, err := h.tasks.SaveVersion(c.Request().Context(), currentUser(c), domain.Version{
		ID: itemID, ProjectID: projectID, Name: in.Name, Description: in.Description, ReleaseDate: in.ReleaseDate, Status: in.Status})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toVersion(v))
}

func (h *TaskHandler) deletePlanning(table string) echo.HandlerFunc {
	return func(c *echo.Context) error {
		id, err := pathID(c, "itemId")
		if err != nil {
			return err
		}
		if err := h.tasks.DeletePlanning(c.Request().Context(), currentUser(c), table, id); err != nil {
			return err
		}
		return c.NoContent(http.StatusNoContent)
	}
}

// --- shapes ---

type checklistItemDTO struct {
	ID          int64  `json:"id"`
	ChecklistID int64  `json:"checklistId"`
	Text        string `json:"text"`
	Done        bool   `json:"done"`
}

type checklistDTO struct {
	ID        int64              `json:"id"`
	CardID    int64              `json:"cardId"`
	Title     string             `json:"title"`
	Collapsed bool               `json:"collapsed"`
	Items     []checklistItemDTO `json:"items"`
}

func toChecklistItem(it domain.ChecklistItem) checklistItemDTO {
	return checklistItemDTO{it.ID, it.ChecklistID, it.Text, it.Done}
}

func toChecklist(l domain.Checklist) checklistDTO {
	return checklistDTO{l.ID, l.CardID, l.Title, l.Collapsed, mapSlice(l.Items, toChecklistItem)}
}

type cardCommentDTO struct {
	ID         int64     `json:"id"`
	CardID     int64     `json:"cardId"`
	AuthorID   *int64    `json:"authorId"`
	AuthorName string    `json:"authorName"`
	Body       string    `json:"body"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func toCardComment(c domain.CardComment) cardCommentDTO {
	return cardCommentDTO{c.ID, c.CardID, c.AuthorID, c.AuthorName, c.Body, c.CreatedAt, c.UpdatedAt}
}

type historyDTO struct {
	ID        int64     `json:"id"`
	ActorID   *int64    `json:"actorId"`
	ActorName string    `json:"actorName"`
	Field     string    `json:"field"`
	Old       string    `json:"old"`
	New       string    `json:"new"`
	CreatedAt time.Time `json:"createdAt"`
}

func toHistory(e domain.HistoryEntry) historyDTO {
	return historyDTO{e.ID, e.ActorID, e.ActorName, e.Field, e.Old, e.New, e.CreatedAt}
}

type milestoneDTO struct {
	ID          int64        `json:"id"`
	ProjectID   int64        `json:"projectId"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	StartDate   *domain.Date `json:"startDate"`
	EndDate     *domain.Date `json:"endDate"`
	CardCount   int          `json:"cardCount"`
	DoneCount   int          `json:"doneCount"`
}

func toMilestone(m domain.Milestone) milestoneDTO {
	return milestoneDTO{m.ID, m.ProjectID, m.Name, m.Description, m.Start, m.End, m.CardCount, m.DoneCount}
}

type sprintDTO struct {
	ID        int64               `json:"id"`
	ProjectID int64               `json:"projectId"`
	Name      string              `json:"name"`
	Goal      string              `json:"goal"`
	StartDate *domain.Date        `json:"startDate"`
	EndDate   *domain.Date        `json:"endDate"`
	Status    domain.SprintStatus `json:"status"`
	CardCount int                 `json:"cardCount"`
	DoneCount int                 `json:"doneCount"`
}

func toSprint(s domain.Sprint) sprintDTO {
	return sprintDTO{s.ID, s.ProjectID, s.Name, s.Goal, s.Start, s.End, s.Status, s.CardCount, s.DoneCount}
}

type versionDTO struct {
	ID          int64                `json:"id"`
	ProjectID   int64                `json:"projectId"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	ReleaseDate *domain.Date         `json:"releaseDate"`
	Status      domain.VersionStatus `json:"status"`
	CardCount   int                  `json:"cardCount"`
	DoneCount   int                  `json:"doneCount"`
}

func toVersion(v domain.Version) versionDTO {
	return versionDTO{v.ID, v.ProjectID, v.Name, v.Description, v.ReleaseDate, v.Status, v.CardCount, v.DoneCount}
}
