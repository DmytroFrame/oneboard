package redisstore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/DmytroFrame/oneboard/backend/internal/domain"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/collab"
	"github.com/DmytroFrame/oneboard/backend/internal/usecase/voice"
)

// ---------- pub/sub between server instances ----------

const (
	boardChannelPrefix  = "board:"
	userChannelPrefix   = "user:"
	signalChannelPrefix = "rtc:" // + connection id: messages for one connection (voice signaling)
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

// PublishSignal implements voice.Signals on "rtc:<connID>" channels.
func (b *Bus) PublishSignal(ctx context.Context, toConnID string, payload []byte) error {
	return b.rdb.Publish(ctx, signalChannelPrefix+toConnID, payload).Err()
}

// Subscribe listens to every board, user and connection channel (pattern subscriptions, one set per
// server instance).
func (b *Bus) Subscribe(ctx context.Context) (<-chan collab.Message, error) {
	sub := b.rdb.PSubscribe(ctx, boardChannelPrefix+"*", userChannelPrefix+"*", signalChannelPrefix+"*")
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
				if connID, ok := strings.CutPrefix(m.Channel, signalChannelPrefix); ok {
					out <- collab.Message{SignalTo: connID, Payload: []byte(m.Payload)}
					continue
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

// Presence keeps, per board, a hash "presence:<projectID>" (connection id → peer JSON) and a sorted
// set "presence:<projectID>:exp" (connection id → expiry). Peers of a crashed server instance stop
// being refreshed and are swept out on the next read. Works on any Redis/Valkey version, including
// ElastiCache (no per-field hash TTLs needed).
type Presence struct{ rdb *redis.Client }

func NewPresence(rdb *redis.Client) *Presence { return &Presence{rdb: rdb} }

func presenceKeys(projectID int64) []string {
	k := fmt.Sprintf("presence:%d", projectID)
	return []string{k, k + ":exp"}
}

var presencePutScript = redis.NewScript(`
redis.call('HSET', KEYS[1], ARGV[1], ARGV[2])
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[1])
redis.call('EXPIRE', KEYS[1], 86400)
redis.call('EXPIRE', KEYS[2], 86400)
return 1
`)

// presenceListScript drops expired peers, then returns the rest.
var presenceListScript = redis.NewScript(`
local expired = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', ARGV[1])
for _, id in ipairs(expired) do redis.call('HDEL', KEYS[1], id) end
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', ARGV[1])
return redis.call('HVALS', KEYS[1])
`)

func (p *Presence) Put(ctx context.Context, projectID int64, peer domain.Peer, ttl time.Duration) error {
	return expiringPut(ctx, p.rdb, presenceKeys(projectID), peer.ConnID, peer, ttl)
}

func (p *Presence) Remove(ctx context.Context, projectID int64, connID string) error {
	return expiringRemove(ctx, p.rdb, presenceKeys(projectID), connID)
}

func (p *Presence) List(ctx context.Context, projectID int64) ([]domain.Peer, error) {
	return expiringList[domain.Peer](ctx, p.rdb, presenceKeys(projectID))
}

// expiringPut, expiringRemove and expiringList manage a "hash + expiry sorted set" pair of keys.

func expiringPut(ctx context.Context, rdb *redis.Client, keys []string, id string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return presencePutScript.Run(ctx, rdb, keys, id, data, time.Now().Add(ttl).UnixMilli()).Err()
}

func expiringRemove(ctx context.Context, rdb *redis.Client, keys []string, id string) error {
	_, err := rdb.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.HDel(ctx, keys[0], id)
		pipe.ZRem(ctx, keys[1], id)
		return nil
	})
	return err
}

func expiringList[T any](ctx context.Context, rdb *redis.Client, keys []string) ([]T, error) {
	vals, err := presenceListScript.Run(ctx, rdb, keys, time.Now().UnixMilli()).StringSlice()
	if err != nil {
		return nil, err
	}
	out := make([]T, 0, len(vals))
	for _, v := range vals {
		var item T
		if json.Unmarshal([]byte(v), &item) == nil {
			out = append(out, item)
		}
	}
	return out, nil
}

// ---------- voice rooms ----------

// VoiceRooms keeps each board's voice room like board presence ("voice:<projectID>" hash plus
// "voice:<projectID>:exp"), and each user's current slot as "voice:user:<userID>" =
// "<projectID>:<connID>", so joining from another tab or board can vacate the old one.
type VoiceRooms struct{ rdb *redis.Client }

func NewVoiceRooms(rdb *redis.Client) *VoiceRooms { return &VoiceRooms{rdb: rdb} }

func voiceKeys(projectID int64) []string {
	k := fmt.Sprintf("voice:%d", projectID)
	return []string{k, k + ":exp"}
}

func voiceUserKey(userID int64) string { return fmt.Sprintf("voice:user:%d", userID) }

func (v *VoiceRooms) Put(ctx context.Context, projectID int64, p domain.VoiceParticipant, ttl time.Duration) error {
	return expiringPut(ctx, v.rdb, voiceKeys(projectID), p.ConnID, p, ttl)
}

func (v *VoiceRooms) Remove(ctx context.Context, projectID int64, connID string) error {
	return expiringRemove(ctx, v.rdb, voiceKeys(projectID), connID)
}

func (v *VoiceRooms) List(ctx context.Context, projectID int64) ([]domain.VoiceParticipant, error) {
	return expiringList[domain.VoiceParticipant](ctx, v.rdb, voiceKeys(projectID))
}

var voiceClaimScript = redis.NewScript(`
local old = redis.call('GET', KEYS[1])
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[2])
return old
`)

func (v *VoiceRooms) Claim(ctx context.Context, userID int64, slot voice.Slot, ttl time.Duration) (*voice.Slot, error) {
	value := strconv.FormatInt(slot.ProjectID, 10) + ":" + slot.ConnID
	old, err := voiceClaimScript.Run(ctx, v.rdb, []string{voiceUserKey(userID)}, value, ttl.Milliseconds()).Text()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	project, conn, ok := strings.Cut(old, ":")
	id, perr := strconv.ParseInt(project, 10, 64)
	if !ok || perr != nil {
		return nil, nil
	}
	return &voice.Slot{ProjectID: id, ConnID: conn}, nil
}

// voiceReleaseScript deletes the slot only if it is still this connection's (a newer tab may own it).
var voiceReleaseScript = redis.NewScript(`
local old = redis.call('GET', KEYS[1])
if old and string.sub(old, -string.len(ARGV[1]) - 1) == ':' .. ARGV[1] then
  redis.call('DEL', KEYS[1])
end
return 1
`)

func (v *VoiceRooms) Release(ctx context.Context, userID int64, connID string) error {
	return voiceReleaseScript.Run(ctx, v.rdb, []string{voiceUserKey(userID)}, connID).Err()
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
