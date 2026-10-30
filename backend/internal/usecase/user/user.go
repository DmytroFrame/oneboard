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
	"time"

	"go.uber.org/fx"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
)

var Module = fx.Module("user", fx.Provide(NewService))

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
	log     *slog.Logger
}

func NewService(users Users, storage AvatarStorage, log *slog.Logger) *Service {
	return &Service{users: users, storage: storage, log: log}
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
