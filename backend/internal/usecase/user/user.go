// Package user implements the user directory and avatars.
package user

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/modules"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
)

var Module = modules.Register(modules.UseCases, fx.Module("user", fx.Provide(NewService)))

const MaxAvatarBytes = 5 << 20

// avatarTypes maps allowed content types (sniffed, not trusted from the client) to file extensions.
// SVG is deliberately absent: it can carry scripts.
var avatarTypes = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
	"image/gif":  "gif",
}

type Users interface {
	ByID(ctx context.Context, id int64) (domain.User, error)
	// SetAvatar stores the new key (or "" to clear) and returns the previous one.
	SetAvatar(ctx context.Context, userID int64, key string, at time.Time) (oldKey string, err error)
	SetSettings(ctx context.Context, userID int64, s domain.Settings) error
	// InOrg loads a person with their membership in an organization; domain.ErrNoRows for outsiders.
	InOrg(ctx context.Context, orgID, userID int64) (domain.User, error)
	SetActive(ctx context.Context, orgID, userID int64, active bool) error
	Dismiss(ctx context.Context, orgID, userID int64, reason string, at time.Time) error
}

// HRLog records hires and dismissals for the HR statistics.
type HRLog interface {
	Record(ctx context.Context, e domain.HREvent) error
}

type ObjectInfo struct {
	ContentType string
	Size        int64
}

type AvatarStorage interface {
	Put(ctx context.Context, key, contentType string, body io.ReadSeeker, size int64) error
	Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) // domain.ErrNoRows if missing
	Delete(ctx context.Context, key string) error
}

type Service struct {
	users   Users
	storage AvatarStorage
	events  usecase.UserEvents
	hr      HRLog
	log     *slog.Logger
}

func NewService(users Users, storage AvatarStorage, events usecase.UserEvents, hr HRLog, log *slog.Logger) *Service {
	return &Service{users: users, storage: storage, events: events, hr: hr, log: log}
}

// UploadAvatar validates the image, stores it under a fresh key and replaces the user's previous avatar.
func (s *Service) UploadAvatar(ctx context.Context, userID int64, r io.Reader) (domain.User, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxAvatarBytes+1))
	if err != nil {
		return domain.User{}, err
	}
	if len(data) == 0 {
		return domain.User{}, domain.ErrAvatarEmpty
	}
	if len(data) > MaxAvatarBytes {
		return domain.User{}, domain.ErrAvatarTooLarge
	}
	contentType := http.DetectContentType(data)
	ext, ok := avatarTypes[contentType]
	if !ok {
		return domain.User{}, domain.ErrAvatarType
	}

	// A new key per upload lets clients cache avatars forever.
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return domain.User{}, err
	}
	key := fmt.Sprintf("avatars/%d/%s.%s", userID, hex.EncodeToString(suffix), ext)
	if err := s.storage.Put(ctx, key, contentType, bytes.NewReader(data), int64(len(data))); err != nil {
		return domain.User{}, fmt.Errorf("store avatar: %w", err)
	}
	return s.replaceAvatar(ctx, userID, key)
}

func (s *Service) RemoveAvatar(ctx context.Context, userID int64) (domain.User, error) {
	return s.replaceAvatar(ctx, userID, "")
}

func (s *Service) replaceAvatar(ctx context.Context, userID int64, key string) (domain.User, error) {
	old, err := s.users.SetAvatar(ctx, userID, key, time.Now())
	if err != nil {
		if key != "" {
			s.deleteObject(ctx, key)
		}
		return domain.User{}, err
	}
	if old != "" && old != key {
		s.deleteObject(ctx, old)
	}
	return s.users.ByID(ctx, userID)
}

// deleteObject is best-effort: an orphaned object is harmless, a failed request is not.
func (s *Service) deleteObject(ctx context.Context, key string) {
	if err := s.storage.Delete(ctx, key); err != nil {
		s.log.Warn("delete avatar object", "key", key, "err", err)
	}
}

// Avatar opens the user's avatar image. The caller must close the reader.
func (s *Service) Avatar(ctx context.Context, userID int64) (io.ReadCloser, ObjectInfo, error) {
	u, err := s.users.ByID(ctx, userID)
	if err != nil {
		return nil, ObjectInfo{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	if !u.HasAvatar() {
		return nil, ObjectInfo{}, domain.ErrAvatarNotFound
	}
	body, info, err := s.storage.Get(ctx, u.AvatarKey)
	if err != nil {
		return nil, ObjectInfo{}, usecase.NotFoundAs(err, domain.ErrAvatarNotFound)
	}
	return body, info, nil
}

// UpdateSettings stores the user's language and theme preferences.
func (s *Service) UpdateSettings(ctx context.Context, userID int64, settings domain.Settings) (domain.User, error) {
	if err := settings.Validate(); err != nil {
		return domain.User{}, err
	}
	if err := s.users.SetSettings(ctx, userID, settings); err != nil {
		return domain.User{}, err
	}
	return s.users.ByID(ctx, userID)
}

// target loads the person an admin action is about, in the actor's organization; its owners are out
// of reach of all of them.
func (s *Service) target(ctx context.Context, actor domain.User, id int64) (domain.User, error) {
	u, err := s.users.InOrg(ctx, actor.OrgID, id)
	if err != nil {
		return domain.User{}, usecase.NotFoundAs(err, domain.ErrUserNotFound)
	}
	if u.Role == domain.CompanyOwner {
		return domain.User{}, domain.ErrPermissionDenied
	}
	return u, nil
}

// Dismiss deactivates an employee in the actor's organization and records why (it shows up in HR statistics). Dismissing yourself
// takes an extra permission.
func (s *Service) Dismiss(ctx context.Context, actor domain.User, targetID int64, reason string) (domain.User, error) {
	if !actor.Can(domain.PermFireUser) || (actor.ID == targetID && !actor.Can(domain.PermFireYourself)) {
		return domain.User{}, domain.ErrPermissionDenied
	}
	u, err := s.target(ctx, actor, targetID)
	if err != nil {
		return domain.User{}, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 2000 {
		return domain.User{}, domain.ErrDismissalReason
	}
	now := time.Now()
	if err := s.users.Dismiss(ctx, actor.OrgID, u.ID, reason, now); err != nil {
		return domain.User{}, err
	}
	if err := s.hr.Record(ctx, domain.HREvent{OrgID: actor.OrgID, User: u, Kind: domain.HRDismissed, Day: domain.DateOf(now), Reason: reason, ActorID: &actor.ID}); err != nil {
		s.log.Error("record dismissal", "user", u.ID, "err", err) // the dismissal itself stands
	}
	u.Active, u.DismissedAt, u.DismissalReason = false, &now, reason
	return u, nil
}

// SetActive deactivates (locks them out of the organization at once) or reactivates a member.
func (s *Service) SetActive(ctx context.Context, actor domain.User, targetID int64, active bool) (domain.User, error) {
	if !actor.Can(domain.PermEditUser) {
		return domain.User{}, domain.ErrPermissionDenied
	}
	if actor.ID == targetID {
		return domain.User{}, domain.ErrCannotDeactivateSelf
	}
	u, err := s.target(ctx, actor, targetID)
	if err != nil {
		return domain.User{}, err
	}
	if err := s.users.SetActive(ctx, actor.OrgID, u.ID, active); err != nil {
		return domain.User{}, err
	}
	u.Active = active
	if active {
		u.DismissedAt, u.DismissalReason = nil, ""
	}
	return u, nil
}
