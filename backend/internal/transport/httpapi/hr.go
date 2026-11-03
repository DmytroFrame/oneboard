package httpapi

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/hr"
)

func init() { registerRoutes(NewHRHandler) }

// HRHandler serves candidates and the HR statistics.
type HRHandler struct {
	hr      *hr.Service
	present *Presenter
}

func NewHRHandler(s *hr.Service, present *Presenter) *HRHandler {
	return &HRHandler{hr: s, present: present}
}

func (h *HRHandler) Mount(_, private *echo.Group) {
	private.GET("/candidates", h.list)
	private.GET("/candidates/:id", h.get)
	private.PUT("/candidates/:id", h.update)
	private.PUT("/candidates/:id/status", h.setStatus)
	private.PUT("/candidates/:id/rejection", h.reject)
	private.POST("/candidates/:id/hire", h.hire)
	private.GET("/hr/statistics", h.stats)
	private.DELETE("/hr/events/:id", h.deleteEvent)
}

func (h *HRHandler) list(c *echo.Context) error {
	f := domain.CandidateFilter{
		Q: c.QueryParam("q"), Status: domain.CandidateStatus(c.QueryParam("status")), Attribute: c.QueryParam("attribute"),
		Specialization: c.QueryParam("specialization"), Offset: intParam(c, "offset", 0), Limit: intParam(c, "limit", 30),
	}
	if r := optID(c.QueryParam("recruiter")); r != nil {
		f.RecruiterID = *r
	}
	page, err := h.hr.List(c.Request().Context(), currentUser(c), f)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"candidates": mapSlice(page.Candidates, h.present.Candidate), "total": page.Total})
}

func (h *HRHandler) get(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	cand, err := h.hr.Get(c.Request().Context(), currentUser(c), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Candidate(cand))
}

func (h *HRHandler) update(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in candidateInput
	if err := bind(c, &in); err != nil {
		return err
	}
	cand, err := h.hr.Update(c.Request().Context(), currentUser(c), id, in.domain())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Candidate(cand))
}

func (h *HRHandler) setStatus(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Status domain.CandidateStatus `json:"status"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	cand, err := h.hr.SetStatus(c.Request().Context(), currentUser(c), id, in.Status)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Candidate(cand))
}

func (h *HRHandler) reject(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Reason string `json:"reason"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	cand, err := h.hr.Reject(c.Request().Context(), currentUser(c), id, in.Reason)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.Candidate(cand))
}

func (h *HRHandler) hire(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Role domain.CompanyRole `json:"role"`
	}
	if err := bind(c, &in); err != nil {
		return err
	}
	u, err := h.hr.Hire(c.Request().Context(), currentUser(c), id, in.Role)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.User(u))
}

func (h *HRHandler) stats(c *echo.Context) error {
	year := intParam(c, "year", domain.Today().Year())
	st, err := h.hr.Stats(c.Request().Context(), currentUser(c), year, intParam(c, "from", 1), intParam(c, "to", 12))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, h.present.HRStats(st))
}

func (h *HRHandler) deleteEvent(c *echo.Context) error {
	id, err := pathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.hr.DeleteEvent(c.Request().Context(), currentUser(c), id); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
