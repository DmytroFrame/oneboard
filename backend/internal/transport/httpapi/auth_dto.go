package httpapi

import (
	"time"
)

// JSON shapes of the auth module.

// sessionDTO answers login, registration and refresh. The refresh token travels only in its HttpOnly cookie.
type sessionDTO struct {
	User        meDTO     `json:"user"`
	AccessToken string    `json:"accessToken"`
	ExpiresAt   time.Time `json:"expiresAt"`
}
