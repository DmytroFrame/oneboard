package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/people"
)

func init() { registerRoutes(NewPeopleHandler) }

type PeopleHandler struct {
	people  *people.Service
	present *Presenter
}

func NewPeopleHandler(p *people.Service, present *Presenter) *PeopleHandler {
	return &PeopleHandler{people: p, present: present}
}

func (h *PeopleHandler) Mount(_, private *echo.Group) {
	private.GET("/people", h.list)
	private.POST("/people", h.create)
	private.GET("/people/:id", h.get)
	private.PUT("/people/:id", h.update)

	private.GET("/people/:id/comments", h.comments)
	private.POST("/people/:id/comments", h.addComment)
	private.DELETE("/person-comments/:id", h.deleteComment)
	private.GET("/people/:id/feedback", h.feedback)
	private.POST("/people/:id/feedback", h.addFeedback)

	private.GET("/people/:id/documents", h.documents)
	private.POST("/people/:id/documents", h.addDocument)
	private.PUT("/documents/:id", h.updateDocument)
	private.DELETE("/documents/:id", h.deleteDocument)
	private.GET("/documents/:id/file", h.documentFile)

	private.GET("/directories/:kind", h.directory)
	private.POST("/directories/:kind", h.addDirectoryItem)
	private.PUT("/directory-items/:id", h.renameDirectoryItem)
	private.DELETE("/directory-items/:id", h.deleteDirectoryItem)
}

func (h *PeopleHandler) list(c *echo.Context) error {
	q := c.Request().URL.Query()
	f := domain.PersonFilter{
		Query: q.Get("q"), Quick: q.Get("quick"), Specialization: q.Get("specialization"), TechStack: q.Get("techStack"),
		Grade: q.Get("grade"), EnglishLevel: q.Get("englishLevel"), City: q.Get("city"),
		IncludeInactive: q.Get("inactive") == "1", Sort: q.Get("sort"), Desc: q.Get("order") == "desc",
	}
	for _, r := range strings.Split(q.Get("roles"), ",") {
		if r != "" {
			f.Roles = append(f.Roles, domain.CompanyRole(r))
		}
	}
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	page, access, err := h.people.List(c.Request().Context(), currentUser(c), f)
	if err != nil {
		return err
	}
	out := personPageDTO{Total: page.Total, People: make([]profileDTO, len(page.People))}
	for i, p := range page.People {
		out.People[i] = h.present.Profile(p, access[i], currentUser(c))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *PeopleHandler) create(c *echo.Context) error {
	var in struct {
		Name  string             `json:"name"`
		Email string             `json:"email"`
		Role  domain.CompanyRole `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u, err := h.people.Create(c.Request().Context(), currentUser(c), people.NewPerson{Name: in.Name, Email: in.Email, Role: in.Role})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, h.present.User(u))
}

func (h *PeopleHandler) get(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	v, err := h.people.Get(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Profile(v.Profile, v.Access, currentUser(c)))
}

func (h *PeopleHandler) update(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in profileInput
	if err := bind(c, &in); err != nil {
		return err
	}
	v, err := h.people.Update(c.Request().Context(), currentUser(c), id, people.ProfileUpdate{Name: in.Name, Profile: domain.Profile{
		NameCyrillic: in.NameCyrillic, Birthday: in.Birthday, StartDate: in.StartDate, TrialUntil: in.TrialUntil,
		WorkingHours: in.WorkingHours, Billable: in.Billable, Details: in.Details, Finance: in.Finance,
	}})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Profile(v.Profile, v.Access, currentUser(c)))
}

// --- comments and feedback ---

func (h *PeopleHandler) comments(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	items, err := h.people.Comments(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(items, toPersonComment))
}

func (h *PeopleHandler) addComment(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Kind      string `json:"kind"`
		Body      string `json:"body"`
		Anonymous bool   `json:"anonymous"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	cm, err := h.people.AddComment(c.Request().Context(), currentUser(c), id, in.Kind, in.Body, in.Anonymous)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toPersonComment(cm))
}

func (h *PeopleHandler) deleteComment(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.people.DeleteComment(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *PeopleHandler) feedback(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	items, err := h.people.Feedback(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(items, toFeedback))
}

func (h *PeopleHandler) addFeedback(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in feedbackDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	f, err := h.people.AddFeedback(c.Request().Context(), currentUser(c), id, domain.Feedback{
		StartDoing: in.StartDoing, StopDoing: in.StopDoing, KeepDoing: in.KeepDoing, Anonymous: in.Anonymous})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toFeedback(f))
}

// --- documents ---

func (h *PeopleHandler) documents(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	docs, err := h.people.Documents(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(docs, toDocument))
}

// addDocument takes multipart/form-data: name, description, type, private ("1"), and a "file" or a "link".
func (h *PeopleHandler) addDocument(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, domain.MaxDocumentBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return domain.ErrDocumentTooLarge
		}
		return domain.ErrDocumentInvalid
	}
	in := people.NewDocument{
		Name: r.FormValue("name"), Description: r.FormValue("description"), Type: r.FormValue("type"),
		Link: r.FormValue("link"), Private: r.FormValue("private") == "1",
	}
	if f, hdr, err := r.FormFile("file"); err == nil {
		defer f.Close()
		in.FileName, in.Body = hdr.Filename, f
	}
	d, err := h.people.AddDocument(r.Context(), currentUser(c), id, in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toDocument(d))
}

func (h *PeopleHandler) updateDocument(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in documentDTO
	if err := bind(c, &in); err != nil {
		return err
	}
	d, err := h.people.UpdateDocument(c.Request().Context(), currentUser(c), id, in.Name, in.Description, in.Type, in.Link, in.Private)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toDocument(d))
}

func (h *PeopleHandler) deleteDocument(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.people.DeleteDocument(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// documentFile streams the file as a download. It needs the bearer token (fetched by the app as a
// blob), unlike attachments, which are embedded in <img> and use signed URLs.
func (h *PeopleHandler) documentFile(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	d, body, err := h.people.OpenDocument(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	defer body.Close()
	w := c.Response()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, d.FileName))
	w.Header().Set("Content-Length", strconv.FormatInt(d.Size, 10))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, body)
	return err
}

// --- reference lists ---

func (h *PeopleHandler) directory(c *echo.Context) error {
	items, err := h.people.Directory(c.Request().Context(), currentUser(c), domain.DirectoryKind(c.Param("kind")))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, mapSlice(items, toDirectoryItem))
}

func (h *PeopleHandler) addDirectoryItem(c *echo.Context) error {
	var in struct {
		Name string `json:"name"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	it, err := h.people.AddDirectoryItem(c.Request().Context(), currentUser(c), domain.DirectoryKind(c.Param("kind")), in.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toDirectoryItem(it))
}

func (h *PeopleHandler) renameDirectoryItem(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Name string `json:"name"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	it, err := h.people.RenameDirectoryItem(c.Request().Context(), currentUser(c), id, in.Name)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toDirectoryItem(it))
}

func (h *PeopleHandler) deleteDirectoryItem(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.people.DeleteDirectoryItem(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
