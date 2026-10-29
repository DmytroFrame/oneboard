package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"

	"clodapp/backend/internal/domain"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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
	if status, ok := statusByKind[domain.KindOf(err)]; ok {
		writeError(w, status, err.Error())
		return
	}
	slog.ErrorContext(r.Context(), "internal error", "method", r.Method, "path", r.URL.Path, "err", err)
	writeError(w, http.StatusInternalServerError, "внутрішня помилка сервера")
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "некоректний JSON")
		return false
	}
	return true
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "некоректний id")
		return 0, false
	}
	return id, true
}

// spaHandler serves the built frontend and falls back to index.html for client-side routes.
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}
