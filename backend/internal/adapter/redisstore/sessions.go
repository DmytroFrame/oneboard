// Package redisstore keeps sessions in Redis.
package redisstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"clodapp/backend/internal/config"
	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase/auth"
)

var Module = fx.Module("redis",
	fx.Provide(
		NewClient,
		NewSessionStore,
		func(s *SessionStore) auth.Sessions { return s },
	),
)

func NewClient(lc fx.Lifecycle, cfg config.Config) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: cfg.RedisPassword})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	lc.Append(fx.StopHook(c.Close))
	return c, nil
}

// SessionStore keeps "session:<sha256(token)>" → user id with a TTL, so expired sessions vanish on their own.
// Only the hash is stored: a leaked Redis dump doesn't hand out live tokens.
type SessionStore struct{ rdb *redis.Client }

func NewSessionStore(rdb *redis.Client) *SessionStore { return &SessionStore{rdb: rdb} }

func key(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "session:" + hex.EncodeToString(sum[:])
}

func (s *SessionStore) Create(ctx context.Context, userID int64, ttl time.Duration) (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := hex.EncodeToString(buf)
	if err := s.rdb.Set(ctx, key(token), userID, ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

func (s *SessionStore) Resolve(ctx context.Context, token string) (int64, error) {
	v, err := s.rdb.Get(ctx, key(token)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, domain.ErrNoRows
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}

func (s *SessionStore) Delete(ctx context.Context, token string) error {
	return s.rdb.Del(ctx, key(token)).Err()
}
