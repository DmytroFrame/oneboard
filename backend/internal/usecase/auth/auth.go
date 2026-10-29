// Package auth implements registration, login and session management.
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

const SessionTTL = 30 * 24 * time.Hour

// Users is the user storage this use case needs.
type Users interface {
	Create(ctx context.Context, u *domain.User) error // fills u.ID; domain.ErrDuplicate on a taken email
	ByEmail(ctx context.Context, email string) (domain.User, error)
	ByID(ctx context.Context, id int64) (domain.User, error)
	Count(ctx context.Context) (int, error)
}

// Sessions stores opaque session tokens.
type Sessions interface {
	Create(ctx context.Context, userID int64, ttl time.Duration) (token string, err error)
	Resolve(ctx context.Context, token string) (userID int64, err error) // domain.ErrNoRows if unknown or expired
	Delete(ctx context.Context, token string) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}

type Service struct {
	users    Users
	sessions Sessions
	hasher   PasswordHasher
}

func NewService(users Users, sessions Sessions, hasher PasswordHasher) *Service {
	return &Service{users: users, sessions: sessions, hasher: hasher}
}

type RegisterInput struct {
	Name, Email, Password string
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (domain.User, string, error) {
	name := strings.TrimSpace(in.Name)
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if name == "" || len(name) > 100 {
		return domain.User{}, "", domain.Validation("вкажіть ім'я (до 100 символів)")
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return domain.User{}, "", domain.Validation("некоректний email")
	}
	if len(in.Password) < 8 || len(in.Password) > 72 {
		return domain.User{}, "", domain.Validation("пароль має містити від 8 до 72 символів")
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return domain.User{}, "", err
	}
	count, err := s.users.Count(ctx)
	if err != nil {
		return domain.User{}, "", err
	}
	u := domain.User{
		Name:         name,
		Email:        email,
		PasswordHash: hash,
		Color:        domain.AvatarColors[count%len(domain.AvatarColors)],
	}
	if err := s.users.Create(ctx, &u); err != nil {
		if errors.Is(err, domain.ErrDuplicate) {
			return domain.User{}, "", domain.Conflict("користувач з таким email вже існує")
		}
		return domain.User{}, "", err
	}
	token, err := s.sessions.Create(ctx, u.ID, SessionTTL)
	return u, token, err
}

func (s *Service) Login(ctx context.Context, email, password string) (domain.User, string, error) {
	u, err := s.users.ByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil && !errors.Is(err, domain.ErrNoRows) {
		return domain.User{}, "", err
	}
	if err != nil || !s.hasher.Compare(u.PasswordHash, password) {
		return domain.User{}, "", domain.Unauthorized("невірний email або пароль")
	}
	token, err := s.sessions.Create(ctx, u.ID, SessionTTL)
	return u, token, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.sessions.Delete(ctx, token)
}

// Authenticate resolves a session token to its user.
func (s *Service) Authenticate(ctx context.Context, token string) (domain.User, error) {
	id, err := s.sessions.Resolve(ctx, token)
	if errors.Is(err, domain.ErrNoRows) {
		return domain.User{}, domain.Unauthorized("сесія недійсна")
	}
	if err != nil {
		return domain.User{}, err
	}
	u, err := s.users.ByID(ctx, id)
	if errors.Is(err, domain.ErrNoRows) {
		_ = s.sessions.Delete(ctx, token)
		return domain.User{}, domain.Unauthorized("сесія недійсна")
	}
	return u, err
}
