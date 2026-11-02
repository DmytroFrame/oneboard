package httpapi

import (
	"fmt"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// JSON shapes of the users module.

type userDTO struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Color     string  `json:"color"`
	AvatarURL *string `json:"avatarUrl"`
}

type settingsDTO struct {
	Language domain.Language `json:"language"`
	Theme    domain.Theme    `json:"theme"`
}

// meDTO is the signed-in user; settings are private and only ever returned to their owner.
type meDTO struct {
	userDTO
	Settings settingsDTO `json:"settings"`
}

// Avatars have permanent URLs, versioned by upload time.
func avatarMessage(userID, version int64) string { return fmt.Sprintf("avatar:%d:%d", userID, version) }

func (p *Presenter) validAvatarSig(userID, version int64, sig string) bool {
	return p.verify(avatarMessage(userID, version), sig)
}

func (p *Presenter) User(u domain.User) userDTO {
	d := userDTO{ID: u.ID, Name: u.Name, Email: u.Email, Color: u.Color}
	if u.HasAvatar() {
		var v int64
		if u.AvatarUpdatedAt != nil {
			v = u.AvatarUpdatedAt.UnixMilli()
		}
		url := fmt.Sprintf("/api/users/%d/avatar?v=%d&sig=%s", u.ID, v, p.sign(avatarMessage(u.ID, v)))
		d.AvatarURL = &url
	}
	return d
}

func (p *Presenter) Me(u domain.User) meDTO {
	return meDTO{p.User(u), settingsDTO{u.Settings.Language, u.Settings.Theme}}
}
