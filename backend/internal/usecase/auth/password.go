package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// ResetTokenTTL is how long a password-reset link works.
const ResetTokenTTL = time.Hour

// ResetTokens stores one-time password-reset tokens.
type ResetTokens interface {
	Issue(ctx context.Context, userID int64, ttl time.Duration) (string, error)
	Peek(ctx context.Context, token string) (int64, error)    // domain.ErrNoRows if unknown or expired
	Consume(ctx context.Context, token string) (int64, error) // the same, and the token is gone
}

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

// RequestPasswordReset emails a one-time link. It never says whether the email has an account.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	u, err := s.users.ByEmail(ctx, domain.NormalizeEmail(email))
	if errors.Is(err, domain.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.appURL == "" {
		return domain.ErrMailNotConfigured
	}
	token, err := s.resets.Issue(ctx, u.ID, ResetTokenTTL)
	if err != nil {
		return err
	}
	m := messagesFor(u)
	return s.mailer.Send(ctx, u.Email, m.resetSubject, fmt.Sprintf(m.resetBody, u.Name, s.appURL+"/reset-password/"+token))
}

// CheckResetToken tells the reset page whether its link still works.
func (s *Service) CheckResetToken(ctx context.Context, token string) error {
	_, err := s.resets.Peek(ctx, token)
	if errors.Is(err, domain.ErrNoRows) {
		return domain.ErrResetLinkExpired
	}
	return err
}

// ResetPassword sets a new password with a reset link, which is used up.
func (s *Service) ResetPassword(ctx context.Context, token, password string) error {
	if err := domain.ValidatePassword(password); err != nil {
		return err // checked first, so a typo doesn't burn the link
	}
	userID, err := s.resets.Consume(ctx, token)
	if errors.Is(err, domain.ErrNoRows) {
		return domain.ErrResetLinkExpired
	}
	if err != nil {
		return err
	}
	return s.setPassword(ctx, userID, password)
}

// ChangePassword is the signed-in user changing their own password.
func (s *Service) ChangePassword(ctx context.Context, userID int64, current, next string) error {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return err
	}
	if !s.hasher.Compare(u.PasswordHash, current) {
		return domain.ErrWrongPassword
	}
	if err := domain.ValidatePassword(next); err != nil {
		return err
	}
	return s.setPassword(ctx, userID, next)
}

// AdminResetPassword generates a new password for someone and emails it to them (OWNER/ADMIN).
func (s *Service) AdminResetPassword(ctx context.Context, actor domain.User, targetID int64) error {
	if !actor.Can(domain.PermResetPassword) {
		return domain.ErrPermissionDenied
	}
	u, err := s.users.ByID(ctx, targetID)
	if errors.Is(err, domain.ErrNoRows) {
		return domain.ErrUserNotFound
	}
	if err != nil {
		return err
	}
	// An account is shared by every organization its person works in, so an organization may only
	// reset the password of someone who works nowhere else (otherwise it could sign in as them there).
	ms, err := s.users.Memberships(ctx, targetID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(ms, func(m domain.Membership) bool { return m.OrgID == actor.OrgID }) {
		return domain.ErrUserNotFound
	}
	if len(ms) > 1 {
		return domain.ErrSharedAccount
	}
	password, err := generatePassword()
	if err != nil {
		return err
	}
	if err := s.setPassword(ctx, u.ID, password); err != nil {
		return err
	}
	m := messagesFor(u)
	return s.mailer.Send(ctx, u.Email, m.newPasswordSubject, fmt.Sprintf(m.newPasswordBody, u.Name, password, s.appURL))
}

func (s *Service) setPassword(ctx context.Context, userID int64, password string) error {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	return s.users.SetPassword(ctx, userID, hash)
}

// generatePassword makes a 14-character password that satisfies domain.ValidatePassword.
func generatePassword() (string, error) {
	sets := []string{"ABCDEFGHJKLMNPQRSTUVWXYZ", "abcdefghijkmnopqrstuvwxyz", "23456789", "!@#$%*?-_"}
	all := sets[0] + sets[1] + sets[2] + sets[3]
	out := make([]byte, 14)
	for i := range out {
		set := all
		if i < len(sets) {
			set = sets[i] // one of each class first, then shuffled below
		}
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
		if err != nil {
			return "", err
		}
		out[i] = set[n.Int64()]
	}
	for i := len(out) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		out[i], out[j.Int64()] = out[j.Int64()], out[i]
	}
	return string(out), nil
}

type messages struct {
	resetSubject, resetBody             string // body: name, link
	newPasswordSubject, newPasswordBody string // body: name, password, app URL
}

var mailText = map[domain.Language]messages{
	domain.LanguageEN: {
		resetSubject:       "Reset your OneBoard password",
		resetBody:          "Hi %s,\n\nOpen this link to choose a new password (it works for one hour, once):\n%s\n\nIf you didn't ask for it, ignore this email.",
		newPasswordSubject: "Your new OneBoard password",
		newPasswordBody:    "Hi %s,\n\nAn administrator has reset your password. Your new password is:\n\n%s\n\nSign in at %s and change it in Settings.",
	},
	domain.LanguageUK: {
		resetSubject:       "Відновлення пароля OneBoard",
		resetBody:          "Вітаємо, %s!\n\nВідкрийте посилання, щоб задати новий пароль (діє одну годину й один раз):\n%s\n\nЯкщо ви цього не просили, просто проігноруйте лист.",
		newPasswordSubject: "Ваш новий пароль OneBoard",
		newPasswordBody:    "Вітаємо, %s!\n\nАдміністратор скинув ваш пароль. Новий пароль:\n\n%s\n\nУвійдіть на %s і змініть його в налаштуваннях.",
	},
	domain.LanguageDE: {
		resetSubject:       "OneBoard-Passwort zurücksetzen",
		resetBody:          "Hallo %s,\n\nÖffne diesen Link, um ein neues Passwort festzulegen (eine Stunde gültig, einmalig):\n%s\n\nFalls du das nicht angefordert hast, ignoriere diese E-Mail.",
		newPasswordSubject: "Dein neues OneBoard-Passwort",
		newPasswordBody:    "Hallo %s,\n\nein Administrator hat dein Passwort zurückgesetzt. Dein neues Passwort:\n\n%s\n\nMelde dich unter %s an und ändere es in den Einstellungen.",
	},
}

func messagesFor(u domain.User) messages {
	if m, ok := mailText[u.Settings.Language]; ok {
		return m
	}
	return mailText[domain.LanguageEN]
}
