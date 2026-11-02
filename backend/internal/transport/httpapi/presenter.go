package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
)

// DTOs keep the JSON contract in the transport layer; domain entities carry no tags. Each module's
// DTOs and Presenter methods live in <module>_dto.go next to its handlers.
//
// Presenter turns domain objects into DTOs and signs the URLs inside them. <img> and <video> can't send
// the Authorization header, so file endpoints are public and the HMAC proves the URL came from an
// authenticated response.
type Presenter struct {
	key []byte
	now func() time.Time
}

func NewPresenter(cfg config.Config) *Presenter {
	// A key derived for this one purpose, so a URL signature can never double as anything else.
	mac := hmac.New(sha256.New, cfg.JWTSecret)
	mac.Write([]byte("oneboard/signed-urls"))
	return &Presenter{key: mac.Sum(nil), now: time.Now}
}

func (p *Presenter) sign(message string) string {
	mac := hmac.New(sha256.New, p.key)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

func (p *Presenter) verify(message, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(p.sign(message)))
}

func mapSlice[T, R any](in []T, f func(T) R) []R {
	out := make([]R, len(in))
	for i, v := range in {
		out[i] = f(v)
	}
	return out
}
