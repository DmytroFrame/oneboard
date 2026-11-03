package redisstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
)

// ResetTokens are one-time password-reset links: "pwreset:<sha256(token)>" = user id, with a TTL.
// Only the hash is stored, like refresh tokens.
type ResetTokens struct{ rdb *redis.Client }

func NewResetTokens(rdb *redis.Client) *ResetTokens { return &ResetTokens{rdb: rdb} }

func resetKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return "pwreset:" + hex.EncodeToString(sum[:])
}

func (s *ResetTokens) Issue(ctx context.Context, userID int64, ttl time.Duration) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	return token, s.rdb.Set(ctx, resetKey(token), userID, ttl).Err()
}

// Peek returns the token's user without using it up; domain.ErrNoRows if unknown or expired.
func (s *ResetTokens) Peek(ctx context.Context, token string) (int64, error) {
	return s.read(s.rdb.Get(ctx, resetKey(token)))
}

// Consume returns the token's user and deletes the token, atomically.
func (s *ResetTokens) Consume(ctx context.Context, token string) (int64, error) {
	return s.read(s.rdb.GetDel(ctx, resetKey(token)))
}

func (s *ResetTokens) read(cmd *redis.StringCmd) (int64, error) {
	v, err := cmd.Result()
	if errors.Is(err, redis.Nil) {
		return 0, domain.ErrNoRows
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(v, 10, 64)
}
