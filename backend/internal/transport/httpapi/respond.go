package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// Handlers return errors; errorHandler turns them into {"error": <English message>, "code": <stable
// code the client translates>}. Use case errors carry their code in domain.Error; apiError covers what
// the transport itself rejects.

type apiError struct {
	status    int
	code, msg string
}

func (e *apiError) Error() string   { return e.msg }
func (e *apiError) StatusCode() int { return e.status }

var (
	errInvalidJSON      = &apiError{http.StatusBadRequest, "invalid_json", "Invalid JSON"}
	errInvalidID        = &apiError{http.StatusBadRequest, "invalid_id", "Invalid id"}
	errNotFound         = &apiError{http.StatusNotFound, "not_found", "Not found"}
	errMethodNotAllowed = &apiError{http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed"}
	errInternal         = &apiError{http.StatusInternalServerError, "internal", "Internal server error"}
)

var statusByKind = map[domain.Kind]int{
	domain.KindValidation:   http.StatusBadRequest,
	domain.KindUnauthorized: http.StatusUnauthorized,
	domain.KindForbidden:    http.StatusForbidden,
	domain.KindNotFound:     http.StatusNotFound,
	domain.KindConflict:     http.StatusConflict,
	domain.KindTooLarge:     http.StatusRequestEntityTooLarge,
}

// resolveError maps an error to the response; nil means it is unexpected (logged, hidden from the client).
func resolveError(err error) *apiError {
	if e, ok := domain.AsError(err); ok {
		if status, ok := statusByKind[e.Kind]; ok {
			return &apiError{status, e.Code, e.Msg}
		}
	}
	var ae *apiError
	if errors.As(err, &ae) {
		return ae
	}
	switch echo.StatusCode(err) { // the router's own errors
	case http.StatusNotFound:
		return errNotFound
	case http.StatusMethodNotAllowed:
		return errMethodNotAllowed
	}
	return nil
}

func errorHandler(c *echo.Context, err error) {
	if resp, _ := echo.UnwrapResponse(c.Response()); resp != nil && resp.Committed {
		return
	}
	e := resolveError(err)
	if e == nil {
		r := c.Request()
		slog.ErrorContext(r.Context(), "internal error", "method", r.Method, "path", r.URL.Path, "err", err)
		e = errInternal
	}
	if c.Request().Method == http.MethodHead {
		_ = c.NoContent(e.status)
		return
	}
	_ = c.JSON(e.status, map[string]string{"error": e.msg, "code": e.code})
}

// bind decodes a JSON body of up to 1 MB.
func bind(c *echo.Context, v any) error {
	r := c.Request()
	r.Body = http.MaxBytesReader(c.Response(), r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errInvalidJSON
	}
	return nil
}

func pathID(c *echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil {
		return 0, errInvalidID
	}
	return id, nil
}

func init() {
	// The PWA manifest; Go's built-in table doesn't know the extension.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// spaHandler serves the built frontend and falls back to index.html for client-side routes.
// Caching: content-hashed files under /assets/ never change, so they are cached for a year; the entry
// points (index.html, the service worker, the manifest) are always revalidated, so a new deploy is
// picked up — the service worker then updates the precached app shell.
func spaHandler(dir string) echo.HandlerFunc {
	fs := http.FileServer(http.Dir(dir))
	return echo.WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/assets/"):
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case strings.HasPrefix(r.URL.Path, "/sw.js"), strings.HasPrefix(r.URL.Path, "/workbox-"), r.URL.Path == "/registerSW.js",
			r.URL.Path == "/manifest.webmanifest":
			w.Header().Set("Cache-Control", "no-cache")
		}
		fs.ServeHTTP(w, r)
	}))
}
