package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

type testRoutes struct{}

func (testRoutes) Mount(public, private *echo.Group) {
	public.GET("/test/domain-error", func(*echo.Context) error { return domain.ErrCardNotFound })
	public.GET("/test/boom", func(*echo.Context) error { return errors.New("database exploded") })
	public.POST("/test/json", func(c *echo.Context) error {
		var v struct{ A int }
		if err := bind(c, &v); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, v)
	})
	public.GET("/test/:id", func(c *echo.Context) error {
		id, err := pathID(c, "id")
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, map[string]int64{"id": id})
	})
	private.GET("/test-private", func(c *echo.Context) error { return c.NoContent(http.StatusOK) })
}

func newTestRouter(t *testing.T) http.Handler {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html>app"), 0o644); err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	// No auth service: requests in these tests never get as far as checking a token.
	return NewRouter(config.Config{StaticDir: dir}, &Middleware{log: log}, log, []Routes{testRoutes{}}, nil)
}

func TestRouter(t *testing.T) {
	h := newTestRouter(t)
	cases := []struct {
		method, path, body string
		status             int
		code               string // expected error code; "" = not an error
		contains           string
	}{
		{"GET", "/healthz", "", 200, "", ""},
		{"GET", "/api/test/7", "", 200, "", `"id":7`},
		{"GET", "/api/test/abc", "", 400, "invalid_id", ""},
		{"GET", "/api/test/domain-error", "", 404, "card_not_found", ""},
		{"GET", "/api/test/boom", "", 500, "internal", ""},
		{"POST", "/api/test/json", "{nope", 400, "invalid_json", ""},
		{"POST", "/api/test/json", `{"A":3}`, 200, "", `"A":3`},
		{"GET", "/api/test-private", "", 401, "not_authenticated", ""},
		// Unknown API paths are 404 (not 401 from the private group's catch-all, not the SPA).
		{"GET", "/api/does-not-exist", "", 404, "not_found", ""},
		{"GET", "/api", "", 404, "not_found", ""},
		{"DELETE", "/api/test/domain-error", "", 405, "method_not_allowed", ""},
		// Client-side routes get the SPA.
		{"GET", "/projects/1", "", 200, "", "<!doctype html>app"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.status, rec.Body)
			}
			if tc.code != "" {
				var e struct{ Error, Code string }
				if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Code != tc.code || e.Error == "" {
					t.Fatalf("body = %s, want code %q", rec.Body, tc.code)
				}
				if tc.code == "internal" && strings.Contains(rec.Body.String(), "exploded") {
					t.Fatal("internal error details leaked to the client")
				}
			}
			if tc.contains != "" && !strings.Contains(rec.Body.String(), tc.contains) {
				t.Fatalf("body = %s, want %q", rec.Body, tc.contains)
			}
		})
	}
}
