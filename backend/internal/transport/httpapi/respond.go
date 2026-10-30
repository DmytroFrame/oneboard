package httpapi

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError sends {"error": <English message>, "code": <stable code the client translates>}.
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]string{"error": msg, "code": code})
}

var statusByKind = map[domain.Kind]int{
	domain.KindValidation:   http.StatusBadRequest,
	domain.KindUnauthorized: http.StatusUnauthorized,
	domain.KindForbidden:    http.StatusForbidden,
	domain.KindNotFound:     http.StatusNotFound,
	domain.KindConflict:     http.StatusConflict,
	domain.KindTooLarge:     http.StatusRequestEntityTooLarge,
}

// fail maps a use case error to a response; unexpected errors are logged and hidden from the client.
func fail(w http.ResponseWriter, r *http.Request, err error) {
	if e, ok := domain.AsError(err); ok {
		if status, ok := statusByKind[e.Kind]; ok {
			writeError(w, status, e.Code, e.Msg)
			return
		}
	}
	slog.ErrorContext(r.Context(), "internal error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "Internal server error")
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "Invalid id")
		return 0, false
	}
	return id, true
}

// spaHandler serves the built frontend and falls back to index.html for client-side routes.
func init() {
	// The PWA manifest; Go's built-in table doesn't know the extension.
	_ = mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// spaHandler serves the built frontend and falls back to index.html for client-side routes.
// Caching: content-hashed files under /assets/ never change, so they are cached for a year; the entry
// points (index.html, the service worker, the manifest) are always revalidated, so a new deploy is
// picked up — the service worker then updates the precached app shell.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	})
}
