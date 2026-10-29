// Package user implements the user directory and avatars.
package user

import (
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
	List(ctx context.Context) ([]domain.User, error)
	ByID(ctx context.Context, id int64) (domain.User, error)
	// SetAvatar stores the new key (or "" to clear) and returns the previous one.
	SetAvatar(ctx context.Context, userID int64, key string, at time.Time) (oldKey string, err error)
}

type ObjectInfo struct {
	ContentType string
	Size        int64
}

type AvatarStorage interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
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

func (s *Service) List(ctx context.Context) ([]domain.User, error) {
	return s.users.List(ctx)
}

// UploadAvatar validates the image, stores it under a fresh key and replaces the user's previous avatar.
func (s *Service) UploadAvatar(ctx context.Context, userID int64, r io.Reader) (domain.User, error) {
	data, err := io.ReadAll(io.LimitReader(r, MaxAvatarBytes+1))
	if err != nil {
		return domain.User{}, err
	}
	if len(data) == 0 {
		return domain.User{}, domain.Validation("файл порожній")
	}
	if len(data) > MaxAvatarBytes {
		return domain.User{}, domain.TooLarge("аватарка має бути до 5 МБ")
	}
	contentType := http.DetectContentType(data)
	ext, ok := avatarTypes[contentType]
	if !ok {
		return domain.User{}, domain.Validation("підтримуються PNG, JPEG, WebP або GIF")
	}

	// A new key per upload lets clients cache avatars forever.
	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return domain.User{}, err
	}
	key := fmt.Sprintf("avatars/%d/%s.%s", userID, hex.EncodeToString(suffix), ext)
	if err := s.storage.Put(ctx, key, contentType, data); err != nil {
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
		return nil, ObjectInfo{}, usecase.NotFoundAs(err, "користувача не знайдено")
	}
	if !u.HasAvatar() {
		return nil, ObjectInfo{}, domain.NotFound("аватарки немає")
	}
	body, info, err := s.storage.Get(ctx, u.AvatarKey)
	if err != nil {
		return nil, ObjectInfo{}, usecase.NotFoundAs(err, "аватарки немає")
	}
	return body, info, nil
}
