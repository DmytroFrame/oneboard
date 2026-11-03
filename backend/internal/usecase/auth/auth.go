// Package auth implements registration, login and JWT-based sessions:
// a short-lived access token (JWT) plus a long-lived, rotating refresh token.
package auth

import (
	"context"
	"errors"
	"net/mail"
	"slices"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
)

var Module = modules.Register(modules.UseCases, fx.Module("auth", fx.Provide(
	NewService,
	// The real-time hub authenticates WebSocket connections with the same access tokens.
	func(s *Service) collab.Authenticator { return s },
)))

// Users is the user storage this use case needs.
type Users interface {
	Create(ctx context.Context, u *domain.User) error // fills u.ID; domain.ErrDuplicate on a taken email
	ByEmail(ctx context.Context, email string) (domain.User, error)
	ByID(ctx context.Context, id int64) (domain.User, error)
	Count(ctx context.Context) (int, error)
	SetPassword(ctx context.Context, userID int64, hash string) error
	// Memberships lists the organizations a user belongs to, oldest first; Membership is one of them
	// (domain.ErrNoRows for outsiders).
	Memberships(ctx context.Context, userID int64) ([]domain.Membership, error)
	Membership(ctx context.Context, orgID, userID int64) (domain.Membership, error)
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
	HasPending(ctx context.Context, email string) (bool, error)
	AcceptPending(ctx context.Context, userID int64, email string) error
}

// Founder creates an organization owned by a user (the very first account founds the first one).
type Founder interface {
	Found(ctx context.Context, name string, ownerID int64) (int64, error)
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
	founder     Founder
	tx          usecase.Transactor
	hasher      PasswordHasher
	resets      ResetTokens
	mailer      Mailer
	platform    []string // PLATFORM_ADMINS, lower-case emails
	appURL      string
}

type Deps struct {
	fx.In
	Users       Users
	Access      AccessTokens
	Refresh     RefreshTokens
	Revocations Revocations
	Invitations Invitations
	Founder     Founder
	Tx          usecase.Transactor
	Hasher      PasswordHasher
	Resets      ResetTokens
	Mailer      Mailer
	Config      config.Config
}

func NewService(d Deps) *Service {
	return &Service{users: d.Users, access: d.Access, refresh: d.Refresh, revocations: d.Revocations, invitations: d.Invitations,
		founder: d.Founder, tx: d.Tx, hasher: d.Hasher, resets: d.Resets, mailer: d.Mailer, platform: d.Config.PlatformAdmins, appURL: d.Config.AppURL}
}

type RegisterInput struct {
	Name, Email, Password string
}

// Register is by invitation only: the email must have a pending invitation to an organization. The
// very first account of an installation is the exception: it founds the first organization.
func (s *Service) Register(ctx context.Context, in RegisterInput) (Session, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	count, err := s.users.Count(ctx)
	if err != nil {
		return Session{}, err
	}
	if count > 0 {
		invited, err := s.invitations.HasPending(ctx, email)
		if err != nil {
			return Session{}, err
		}
		if !invited {
			return Session{}, domain.ErrRegistrationClosed
		}
	}
	if name == "" || len(name) > 100 {
		return Session{}, domain.ErrNameInvalid
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return Session{}, domain.ErrEmailInvalid
	}
	if err := domain.ValidatePassword(in.Password); err != nil {
		return Session{}, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return Session{}, err
	}
	u := domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Color:        domain.AvatarColors[count%len(domain.AvatarColors)],
	}
	err = s.tx.WithinTx(ctx, func(ctx context.Context) error {
		if err := s.users.Create(ctx, &u); err != nil {
			if errors.Is(err, domain.ErrDuplicate) {
				return domain.ErrEmailTaken
			}
			return err
		}
		if count == 0 {
			_, err := s.founder.Found(ctx, name+"'s organization", u.ID)
			return err
		}
		// The invitations become memberships with the roles they were sent with.
		return s.invitations.AcceptPending(ctx, u.ID, u.Email)
	})
	if err != nil {
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
	// Someone deactivated or dismissed everywhere they worked can't sign in; without any organization
	// (e.g. all of them deleted) they still can, to found a new one.
	ms, err := s.users.Memberships(ctx, u.ID)
	if err != nil {
		return Session{}, err
	}
	if len(ms) > 0 && !slices.ContainsFunc(ms, func(m domain.Membership) bool { return m.Active }) {
		return Session{}, domain.ErrAccountDeactivated
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
	u.PlatformAdmin = slices.Contains(s.platform, strings.ToLower(u.Email))
	return u, err
}

// InOrg puts the user into the organization a request works in: wanted (the X-Organization header),
// or their oldest active membership when wanted is 0. Asking for an organization where they aren't
// an active member is ErrOrgUnavailable, so a stale choice never acts on another organization. A user
// without any organization gets OrgID 0 and no role.
func (s *Service) InOrg(ctx context.Context, u domain.User, wanted int64) (domain.User, error) {
	if wanted != 0 {
		m, err := s.users.Membership(ctx, wanted, u.ID)
		if errors.Is(err, domain.ErrNoRows) || (err == nil && !m.Active) {
			return u, domain.ErrOrgUnavailable
		}
		if err != nil {
			return u, err
		}
		return u.In(m), nil
	}
	ms, err := s.users.Memberships(ctx, u.ID)
	if err != nil {
		return u, err
	}
	for _, m := range ms {
		if m.Active {
			return u.In(m), nil
		}
	}
	return u, nil
}

// Account is what the client shows about the signed-in user: the user in the organization they want
// to work in (falling back to their first one when that is no longer possible) and all their
// organizations, for the switcher.
func (s *Service) Account(ctx context.Context, u domain.User, wanted int64) (domain.User, []domain.Membership, error) {
	u.PlatformAdmin = slices.Contains(s.platform, strings.ToLower(u.Email))
	in, err := s.InOrg(ctx, u, wanted)
	if errors.Is(err, domain.ErrOrgUnavailable) {
		in, err = s.InOrg(ctx, u, 0)
	}
	if err != nil {
		return u, nil, err
	}
	ms, err := s.users.Memberships(ctx, u.ID)
	return in, ms, err
}
