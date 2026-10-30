// Package auth implements registration, login and JWT-based sessions:
// a short-lived access token (JWT) plus a long-lived, rotating refresh token.
package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
)

var Module = fx.Module("auth", fx.Provide(NewService))

// Users is the user storage this use case needs.
type Users interface {
	Create(ctx context.Context, u *domain.User) error // fills u.ID; domain.ErrDuplicate on a taken email
	ByEmail(ctx context.Context, email string) (domain.User, error)
	ByID(ctx context.Context, id int64) (domain.User, error)
	Count(ctx context.Context) (int, error)
}

// ClockSkew is how long past "exp" an access token is still accepted, to tolerate clock drift.
// Revocations must outlive the token by the same margin.
const ClockSkew = 30 * time.Second

// AccessToken is a signed, self-contained credential (JWT).
type AccessToken struct {
	Value     string
	ID        string // jti, used to revoke the token on logout
	ExpiresAt time.Time
}

// AccessClaims is what a verified access token says.
type AccessClaims struct {
	UserID    int64
	ID        string
	ExpiresAt time.Time
}

type AccessTokens interface {
	Issue(userID int64) (AccessToken, error)
	// Parse verifies the signature and claims; it returns domain.ErrTokenExpired or domain.ErrTokenInvalid.
	Parse(token string) (AccessClaims, error)
}

// RefreshTokens stores opaque refresh tokens. Each login starts a "family"; every refresh rotates
// the token within its family, and presenting an already rotated token revokes the whole family.
type RefreshTokens interface {
	Issue(ctx context.Context, userID int64) (string, error)
	// Rotate exchanges a refresh token for a new one. next is "" when the token was just rotated by a
	// concurrent request (e.g. a second tab) — still valid, but the caller keeps the newer cookie.
	// Unknown, expired or reused tokens give domain.ErrNoRows.
	Rotate(ctx context.Context, token string) (userID int64, next string, err error)
	Revoke(ctx context.Context, token string) error
}

// Revocations is a denylist of access tokens revoked before they expire.
type Revocations interface {
	Revoke(ctx context.Context, tokenID string, until time.Time) error
	IsRevoked(ctx context.Context, tokenID string) (bool, error)
}

// Invitations turns pending organization invitations for an email into memberships.
type Invitations interface {
	AcceptPending(ctx context.Context, userID int64, email string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

// Session is what a successful login, registration or refresh hands back.
type Session struct {
	User    domain.User
	Access  AccessToken
	Refresh string // "" = keep the current refresh token
}

type Service struct {
	users       Users
	access      AccessTokens
	refresh     RefreshTokens
	revocations Revocations
	invitations Invitations
	hasher      PasswordHasher
}

type Deps struct {
	fx.In
	Users       Users
	Access      AccessTokens
	Refresh     RefreshTokens
	Revocations Revocations
	Invitations Invitations
	Hasher      PasswordHasher
}

func NewService(d Deps) *Service {
	return &Service{users: d.Users, access: d.Access, refresh: d.Refresh, revocations: d.Revocations, invitations: d.Invitations, hasher: d.Hasher}
}

type RegisterInput struct {
	Name, Email, Password string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (Session, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if name == "" || len(name) > 100 {
		return Session{}, domain.ErrNameInvalid
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return Session{}, domain.ErrEmailInvalid
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return Session{}, domain.ErrPasswordInvalid
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return Session{}, err
	}
	count, err := s.users.Count(ctx)
	if err != nil {
		return Session{}, err
	}
	u := domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Color:        domain.AvatarColors[count%len(domain.AvatarColors)],
	}
	if err := s.users.Create(ctx, &u); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			return Session{}, domain.ErrEmailTaken
		}
		return Session{}, err
	}
	// Organizations may have invited this email before the account existed.
	if err := s.invitations.AcceptPending(ctx, u.ID, u.Email); err != nil {
		return Session{}, err
	}
	return s.startSession(ctx, u)
}

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	u, err := s.users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil && !errors.Is(err, domain.ErrNoRows) {
		return Session{}, err
	}
	if err != nil || !s.hasher.Compare(u.PasswordHash, password) {
		return Session{}, domain.ErrInvalidCredentials
	}
	return s.startSession(ctx, u)
}

func (s *Service) startSession(ctx context.Context, u domain.User) (Session, error) {
	refresh, err := s.refresh.Issue(ctx, u.ID)
	if err != nil {
		return Session{}, err
	}
	access, err := s.access.Issue(u.ID)
	if err != nil {
		return Session{}, err
	}
	return Session{User: u, Access: access, Refresh: refresh}, nil
}

// Refresh exchanges a refresh token for a new access token (and a rotated refresh token).
func (s *Service) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	if refreshToken == "" {
		return Session{}, domain.ErrNotAuthenticated
	}
	userID, next, err := s.refresh.Rotate(ctx, refreshToken)
	if errors.Is(err, domain.ErrNoRows) {
		return Session{}, domain.ErrSessionInvalid
	}
	if err != nil {
		return Session{}, err
	}
	u, err := s.users.ByID(ctx, userID)
	if errors.Is(err, domain.ErrNoRows) {
		_ = s.refresh.Revoke(ctx, next)
		return Session{}, domain.ErrSessionInvalid
	}
	if err != nil {
		return Session{}, err
	}
	access, err := s.access.Issue(u.ID)
	if err != nil {
		return Session{}, err
	}
	return Session{User: u, Access: access, Refresh: next}, nil
}

// Logout revokes the refresh token's family and, if given, the access token right away.
func (s *Service) Logout(ctx context.Context, refreshToken, accessToken string) error {
	if refreshToken != "" {
		if err := s.refresh.Revoke(ctx, refreshToken); err != nil {
			return err
		}
	}
	if accessToken != "" {
		if claims, err := s.access.Parse(accessToken); err == nil {
			return s.revocations.Revoke(ctx, claims.ID, claims.ExpiresAt.Add(ClockSkew))
		}
	}
	return nil
}

// Authenticate verifies an access token and loads its user.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (domain.User, error) {
	claims, err := s.access.Parse(accessToken)
	if err != nil {
		return domain.User{}, err
	}
	revoked, err := s.revocations.IsRevoked(ctx, claims.ID)
	if err != nil {
		return domain.User{}, err
	}
	if revoked {
		return domain.User{}, domain.ErrTokenInvalid
	}
	u, err := s.users.ByID(ctx, claims.UserID)
	if errors.Is(err, domain.ErrNoRows) {
		return domain.User{}, domain.ErrTokenInvalid
	}
	return u, err
}
