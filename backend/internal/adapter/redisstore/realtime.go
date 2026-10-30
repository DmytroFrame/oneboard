package redisstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"clodapp/backend/internal/domain"
	"clodapp/backend/internal/usecase"
	"clodapp/backend/internal/usecase/collab"
)

// ---------- pub/sub between server instances ----------

const (
	boardChannelPrefix = "board:"
	userChannelPrefix  = "user:"
)

// envelope is what travels over a board channel.
type envelope struct {
	From    string             `json:"from,omitempty"`
	Event   *domain.BoardEvent `json:"event,omitempty"`
	Payload json.RawMessage    `json:"payload,omitempty"`
}

// Bus publishes board events and peer messages on "board:<projectID>" channels.
type Bus struct {
	rdb *redis.Client
	log *slog.Logger
}

func NewBus(rdb *redis.Client, log *slog.Logger) *Bus { return &Bus{rdb: rdb, log: log} }

func (b *Bus) publish(ctx context.Context, projectID int64, env envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, boardChannelPrefix+strconv.FormatInt(projectID, 10), data).Err()
}

// Publish implements usecase.BoardEvents; failures are logged, never returned.
func (b *Bus) Publish(ctx context.Context, e domain.BoardEvent) {
	e.Origin = usecase.OriginFrom(ctx)
	if err := b.publish(ctx, e.ProjectID, envelope{Event: &e}); err != nil {
		b.log.Error("publish board event", "type", e.Type, "project", e.ProjectID, "err", err)
	}
}

func (b *Bus) PublishPeer(ctx context.Context, projectID int64, from string, payload []byte) error {
	return b.publish(ctx, projectID, envelope{From: from, Payload: payload})
}

// PublishUser implements usecase.UserEvents on "user:<id>" channels.
func (b *Bus) PublishUser(ctx context.Context, e domain.UserEvent) {
	data, err := json.Marshal(e)
	if err == nil {
		err = b.rdb.Publish(ctx, userChannelPrefix+strconv.FormatInt(e.UserID, 10), data).Err()
	}
	if err != nil {
		b.log.Error("publish user event", "type", e.Type, "user", e.UserID, "err", err)
	}
}

// Subscribe listens to every board and user channel (pattern subscriptions, one set per server instance).
func (b *Bus) Subscribe(ctx context.Context) (<-chan collab.Message, error) {
	sub := b.rdb.PSubscribe(ctx, boardChannelPrefix+"*", userChannelPrefix+"*")
	if _, err := sub.Receive(ctx); err != nil { // wait for the subscription to be active
		_ = sub.Close()
		return nil, err
	}
	out := make(chan collab.Message, 256)
	go func() {
		defer close(out)
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-ch:
				if !ok {
					return
				}
				if rest, ok := strings.CutPrefix(m.Channel, userChannelPrefix); ok {
					var e domain.UserEvent
					if _, err := strconv.ParseInt(rest, 10, 64); err != nil || json.Unmarshal([]byte(m.Payload), &e) != nil {
						continue
					}
					out <- collab.Message{UserEvent: &e}
					continue
				}
				projectID, err := strconv.ParseInt(strings.TrimPrefix(m.Channel, boardChannelPrefix), 10, 64)
				if err != nil {
					continue
				}
				var env envelope
				if err := json.Unmarshal([]byte(m.Payload), &env); err != nil {
					b.log.Warn("bad board message", "err", err)
					continue
				}
				out <- collab.Message{ProjectID: projectID, From: env.From, Event: env.Event, Payload: env.Payload}
			}
		}
	}()
	return out, nil
}

// ---------- presence ----------

// Presence keeps "presence:<projectID>" hashes: connection id → peer JSON, each field with its own TTL
// (HEXPIRE, Redis 7.4+), so peers of a crashed server instance disappear on their own.
type Presence struct{ rdb *redis.Client }

func NewPresence(rdb *redis.Client) *Presence { return &Presence{rdb: rdb} }

func presenceKey(projectID int64) string { return fmt.Sprintf("presence:%d", projectID) }

func (p *Presence) Put(ctx context.Context, projectID int64, peer domain.Peer, ttl time.Duration) error {
	data, err := json.Marshal(peer)
	if err != nil {
		return err
	}
	key := presenceKey(projectID)
	_, err = p.rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HSet(ctx, key, peer.ConnID, data)
		pipe.HExpire(ctx, key, ttl, peer.ConnID)
		pipe.Expire(ctx, key, 24*time.Hour) // the hash itself, in case it is ever left empty-but-present
		return nil
	})
	return err
}

func (p *Presence) Remove(ctx context.Context, projectID int64, connID string) error {
	return p.rdb.HDel(ctx, presenceKey(projectID), connID).Err()
}

func (p *Presence) List(ctx context.Context, projectID int64) ([]domain.Peer, error) {
	m, err := p.rdb.HGetAll(ctx, presenceKey(projectID)).Result()
	if err != nil {
		return nil, err
	}
	peers := make([]domain.Peer, 0, len(m))
	for _, v := range m {
		var peer domain.Peer
		if json.Unmarshal([]byte(v), &peer) == nil {
			peers = append(peers, peer)
		}
	}
	return peers, nil
}

// ---------- collaborative documents ----------

// DocLog stores the Yjs updates of a card document as a Redis list "ydoc:<cardID>". Entries are
// numbered by a running sequence: "ydoc:<cardID>:base" counts entries compacted away, so the entry
// at list index i has sequence base+i+1. Idle documents expire; the card's markdown in Postgres
// remains the source of truth and seeds the document next time.
type DocLog struct{ rdb *redis.Client }

func NewDocLog(rdb *redis.Client) *DocLog { return &DocLog{rdb: rdb} }

const docTTL = time.Hour

func docKeys(cardID int64) []string {
	k := fmt.Sprintf("ydoc:%d", cardID)
	return []string{k, k + ":base", k + ":seed"}
}

var loadScript = redis.NewScript(`
local base = tonumber(redis.call('GET', KEYS[2]) or '0')
return {base, redis.call('LRANGE', KEYS[1], 0, -1)}
`)

func (d *DocLog) Load(ctx context.Context, cardID int64) (int64, [][]byte, error) {
	res, err := loadScript.Run(ctx, d.rdb, docKeys(cardID)).Slice()
	if err != nil {
		return 0, nil, err
	}
	base, _ := res[0].(int64)
	raw, _ := res[1].([]any)
	updates := make([][]byte, 0, len(raw))
	for _, r := range raw {
		s, _ := r.(string)
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return 0, nil, err
		}
		updates = append(updates, b)
	}
	return base, updates, nil
}

func (d *DocLog) ClaimSeed(ctx context.Context, cardID int64, connID string) (bool, error) {
	return d.rdb.SetNX(ctx, docKeys(cardID)[2], connID, 15*time.Second).Result()
}

var appendScript = redis.NewScript(`
local len = redis.call('RPUSH', KEYS[1], ARGV[1])
local base = tonumber(redis.call('GET', KEYS[2]) or '0')
redis.call('EXPIRE', KEYS[1], ARGV[2])
if base > 0 then redis.call('EXPIRE', KEYS[2], ARGV[2]) end
return base + len
`)

func (d *DocLog) Append(ctx context.Context, cardID int64, update []byte) (int64, error) {
	return appendScript.Run(ctx, d.rdb, docKeys(cardID),
		base64.StdEncoding.EncodeToString(update), int(docTTL.Seconds())).Int64()
}

// compactScript drops entries with sequence <= upto and puts the snapshot (which includes them) first.
var compactScript = redis.NewScript(`
local base = tonumber(redis.call('GET', KEYS[2]) or '0')
local i = tonumber(ARGV[1]) - base
if i < 2 then return 0 end
local len = redis.call('LLEN', KEYS[1])
if i > len then return 0 end
redis.call('LTRIM', KEYS[1], i, -1)
redis.call('LPUSH', KEYS[1], ARGV[2])
redis.call('SET', KEYS[2], tonumber(ARGV[1]) - 1, 'EX', ARGV[3])
redis.call('EXPIRE', KEYS[1], ARGV[3])
return 1
`)

func (d *DocLog) Compact(ctx context.Context, cardID, upto int64, state []byte) error {
	return compactScript.Run(ctx, d.rdb, docKeys(cardID), upto,
		base64.StdEncoding.EncodeToString(state), int(docTTL.Seconds())).Err()
}
