package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
)

// Presenter turns users into DTOs. It signs avatar URLs: an <img> can't send the Authorization
// header, so the avatar endpoint is public and the HMAC proves the URL came from an authenticated response.
type Presenter struct {
	avatarKey []byte
}

func NewPresenter(cfg config.Config) *Presenter {
	// A key derived for this one purpose, so an avatar signature can never double as anything else.
	mac := hmac.New(sha256.New, cfg.JWTSecret)
	mac.Write([]byte("flowboard/avatar-url"))
	return &Presenter{avatarKey: mac.Sum(nil)}
}

func (p *Presenter) avatarSig(userID, version int64) string {
	mac := hmac.New(sha256.New, p.avatarKey)
	fmt.Fprintf(mac, "%d:%d", userID, version)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

func (p *Presenter) validAvatarSig(userID, version int64, sig string) bool {
	return hmac.Equal([]byte(sig), []byte(p.avatarSig(userID, version)))
}

func (p *Presenter) User(u domain.User) userDTO {
	d := userDTO{ID: u.ID, Name: u.Name, Email: u.Email, Color: u.Color}
	if u.HasAvatar() {
		// The version changes on every upload, so the image can be cached forever.
		var v int64
		if u.AvatarUpdatedAt != nil {
			v = u.AvatarUpdatedAt.UnixMilli()
		}
		url := fmt.Sprintf("/api/users/%d/avatar?v=%d&sig=%s", u.ID, v, p.avatarSig(u.ID, v))
		d.AvatarURL = &url
	}
	return d
}

func (p *Presenter) Me(u domain.User) meDTO {
	return meDTO{p.User(u), settingsDTO{u.Settings.Language, u.Settings.Theme}}
}

func (p *Presenter) Members(ms []domain.Member) []memberDTO {
	out := make([]memberDTO, len(ms))
	for i, m := range ms {
		out[i] = memberDTO{p.User(m.User), m.Role, m.AssignedCount}
	}
	return out
}
