// Package redisstore keeps refresh tokens and revoked access tokens in Redis.
package redisstore

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"

	"github.com/DmytroFrame/oneboard/backend/internal/config"
	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/health"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/auth"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
)

var Module = fx.Module("redis",
	health.Provide(func(c *redis.Client) health.Check {
		return health.Check{Name: "redis", Run: func(ctx context.Context) error { return c.Ping(ctx).Err() }}
	}),
	fx.Provide(
		NewClient,
		NewRefreshStore,
		NewDenylist,
		func(s *RefreshStore) auth.RefreshTokens { return s },
		func(d *Denylist) auth.Revocations { return d },

		NewBus, NewPresence, NewDocLog,
		func(b *Bus) usecase.BoardEvents { return b },
		func(b *Bus) usecase.UserEvents { return b },
		func(b *Bus) collab.Broker { return b },
		func(p *Presence) collab.Presence { return p },
		func(d *DocLog) collab.DocLog { return d },
	),
)

func NewClient(lc fx.Lifecycle, cfg config.Config) (*redis.Client, error) {
	opts := &redis.Options{Addr: cfg.RedisAddr, Username: cfg.RedisUsername, Password: cfg.RedisPassword, DB: cfg.RedisDB}
	if cfg.RedisTLS { // ElastiCache with in-transit encryption
		host, _, _ := net.SplitHostPort(cfg.RedisAddr)
		opts.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12, ServerName: host}
	}
	c := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := c.Ping(ctx).Err(); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("redis: %w", err)
	}
	lc.Append(fx.StopHook(c.Close))
	return c, nil
}

// rotationGrace is how long the previous refresh token of a family still works after a rotation,
// so two tabs refreshing at the same moment don't trip reuse detection.
const rotationGrace = 30 * time.Second

// RefreshStore keeps only SHA-256 hashes of refresh tokens:
//
//	rt:<hash>       → "<userID>:<family>"  (kept until expiry, so a reused old token is recognised)
//	fam:<family>    → hash of the family's current token; deleting it revokes the whole family
//	famprev:<family>→ hash of the previous token, for rotationGrace
type RefreshStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewRefreshStore(rdb *redis.Client, cfg config.Config) *RefreshStore {
	return &RefreshStore{rdb: rdb, ttl: cfg.RefreshTokenTTL}
}

func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *RefreshStore) Issue(ctx context.Context, userID int64) (string, error) {
	family, err := randomHex(16)
	if err != nil {
		return "", err
	}
	token, err := randomHex(32)
	if err != nil {
		return "", err
	}
	h := hash(token)
	_, err = s.rdb.TxPipelined(ctx, func(p redis.Pipeliner) error {
		p.Set(ctx, "rt:"+h, fmt.Sprintf("%d:%s", userID, family), s.ttl)
		p.Set(ctx, "fam:"+family, h, s.ttl)
		return nil
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// rotateScript atomically checks the presented token against its family and rotates it.
// Returns {status, value}: status 1 = rotated, 2 = grace (rotated moments ago), 0 = invalid/reused.
var rotateScript = redis.NewScript(`
local value = redis.call('GET', 'rt:' .. ARGV[1])
if not value then return {0, ''} end
local family = string.sub(value, string.find(value, ':') + 1)
local current = redis.call('GET', 'fam:' .. family)
if current == ARGV[1] then
  redis.call('SET', 'rt:' .. ARGV[2], value, 'EX', ARGV[3])
  redis.call('SET', 'fam:' .. family, ARGV[2], 'EX', ARGV[3])
  redis.call('SET', 'famprev:' .. family, ARGV[1], 'EX', ARGV[4])
  return {1, value}
end
if current and redis.call('GET', 'famprev:' .. family) == ARGV[1] then
  return {2, value}
end
-- An old token came back: someone else may hold the family, so end it.
redis.call('DEL', 'fam:' .. family, 'famprev:' .. family)
return {0, ''}
`)

func (s *RefreshStore) Rotate(ctx context.Context, token string) (int64, string, error) {
	next, err := randomHex(32)
	if err != nil {
		return 0, "", err
	}
	res, err := rotateScript.Run(ctx, s.rdb, nil,
		hash(token), hash(next), int(s.ttl.Seconds()), int(rotationGrace.Seconds())).Slice()
	if err != nil {
		return 0, "", err
	}
	status, _ := res[0].(int64)
	value, _ := res[1].(string)
	if status == 0 {
		return 0, "", domain.ErrNoRows
	}
	userID, err := strconv.ParseInt(value[:strings.IndexByte(value, ':')], 10, 64)
	if err != nil {
		return 0, "", err
	}
	if status == 2 {
		return userID, "", nil
	}
	return userID, next, nil
}

func (s *RefreshStore) Revoke(ctx context.Context, token string) error {
	h := hash(token)
	value, err := s.rdb.Get(ctx, "rt:"+h).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	family := value[strings.IndexByte(value, ':')+1:]
	return s.rdb.Del(ctx, "rt:"+h, "fam:"+family, "famprev:"+family).Err()
}

// Denylist remembers revoked access tokens (by jti) until they would have expired anyway.
type Denylist struct{ rdb *redis.Client }

func NewDenylist(rdb *redis.Client) *Denylist { return &Denylist{rdb: rdb} }

func (d *Denylist) Revoke(ctx context.Context, tokenID string, until time.Time) error {
	ttl := time.Until(until)
	if ttl <= 0 {
		return nil
	}
	return d.rdb.Set(ctx, "revoked:"+tokenID, 1, ttl).Err()
}

func (d *Denylist) IsRevoked(ctx context.Context, tokenID string) (bool, error) {
	n, err := d.rdb.Exists(ctx, "revoked:"+tokenID).Result()
	return n > 0, err
}
