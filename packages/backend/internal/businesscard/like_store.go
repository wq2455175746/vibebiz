package businesscard

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type LikeState struct {
	Count int64
	Liked bool
	Found bool
}

type LikeStore interface {
	SetLiked(ctx context.Context, cardID, userID int64, liked bool) (LikeState, error)
	States(ctx context.Context, cardIDs []int64, userID int64) (map[int64]LikeState, error)
	Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error)
	SeedCount(ctx context.Context, cardID, count int64) error
	SeedState(ctx context.Context, cardID, userID int64, state LikeState) error
}

type RedisLikeStore struct {
	client *redis.Client
	prefix string
	maxLen int64
	shards int
	script *redis.Script
}

var likeScript = redis.NewScript(`
local liked = redis.call('SISMEMBER', KEYS[2], ARGV[1])
local want = ARGV[2] == '1'
local maxlen = tonumber(ARGV[6]) or 0
local function addEvent(action)
  if maxlen > 0 then
    redis.call('XADD', KEYS[3], 'MAXLEN', '~', maxlen, '*', 'eventId', ARGV[5], 'cardId', ARGV[3], 'userId', ARGV[1], 'action', action, 'ts', ARGV[4])
  else
    redis.call('XADD', KEYS[3], '*', 'eventId', ARGV[5], 'cardId', ARGV[3], 'userId', ARGV[1], 'action', action, 'ts', ARGV[4])
  end
end
if want and liked == 0 then
  redis.call('SADD', KEYS[2], ARGV[1])
  redis.call('HINCRBY', KEYS[1], ARGV[3], 1)
  addEvent('like')
elseif (not want) and liked == 1 then
  redis.call('SREM', KEYS[2], ARGV[1])
  local count = redis.call('HINCRBY', KEYS[1], ARGV[3], -1)
  if count < 0 then
    redis.call('HSET', KEYS[1], ARGV[3], 0)
  end
  addEvent('unlike')
end
return {redis.call('SISMEMBER', KEYS[2], ARGV[1]), redis.call('HGET', KEYS[1], ARGV[3]) or '0'}
`)

func NewRedisLikeStore(client *redis.Client, prefix string, maxLen int64, shards int) *RedisLikeStore {
	if shards <= 0 {
		shards = 1
	}
	return &RedisLikeStore{client: client, prefix: prefix, maxLen: maxLen, shards: shards, script: likeScript}
}

func (s *RedisLikeStore) countKey() string {
	return s.prefix + ":card:like:count"
}

func (s *RedisLikeStore) usersKey(cardID int64) string {
	return fmt.Sprintf("%s:card:like:users:%d", s.prefix, cardID)
}

func (s *RedisLikeStore) streamKey(cardID int64) string {
	if s.shards <= 1 {
		return s.prefix + ":card:like:stream"
	}
	return fmt.Sprintf("%s:card:like:stream:%d", s.prefix, cardID%int64(s.shards))
}

func (s *RedisLikeStore) SetLiked(ctx context.Context, cardID, userID int64, liked bool) (LikeState, error) {
	action := "0"
	if liked {
		action = "1"
	}
	values, err := s.script.Run(ctx, s.client,
		[]string{s.countKey(), s.usersKey(cardID), s.streamKey(cardID)},
		userID, action, cardID, time.Now().UnixMilli(), uuid.NewString(), s.maxLen,
	).Slice()
	if err != nil {
		return LikeState{}, err
	}
	if len(values) < 2 {
		return LikeState{}, fmt.Errorf("like script returned %d values, want 2", len(values))
	}
	return LikeState{Liked: luaInt64(values[0]) == 1, Count: luaInt64(values[1]), Found: true}, nil
}

func (s *RedisLikeStore) States(ctx context.Context, cardIDs []int64, userID int64) (map[int64]LikeState, error) {
	out := make(map[int64]LikeState, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	pipe := s.client.Pipeline()
	counts := make(map[int64]*redis.StringCmd, len(cardIDs))
	liked := make(map[int64]*redis.BoolCmd, len(cardIDs))
	for _, id := range cardIDs {
		counts[id] = pipe.HGet(ctx, s.countKey(), strconv.FormatInt(id, 10))
		liked[id] = pipe.SIsMember(ctx, s.usersKey(id), userID)
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	for _, id := range cardIDs {
		count, countErr := counts[id].Int64()
		found := countErr == nil
		if errors.Is(countErr, redis.Nil) {
			count = 0
		}
		out[id] = LikeState{Count: count, Liked: liked[id].Val(), Found: found}
	}
	return out, nil
}

func (s *RedisLikeStore) Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	fields := make([]string, 0, len(cardIDs))
	for _, id := range cardIDs {
		out[id] = 0
		fields = append(fields, strconv.FormatInt(id, 10))
	}
	values, err := s.client.HMGet(ctx, s.countKey(), fields...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range cardIDs {
		out[id] = luaInt64(values[i])
	}
	return out, nil
}

func (s *RedisLikeStore) SeedCount(ctx context.Context, cardID, count int64) error {
	return s.client.HSetNX(ctx, s.countKey(), strconv.FormatInt(cardID, 10), count).Err()
}

func (s *RedisLikeStore) SeedState(ctx context.Context, cardID, userID int64, state LikeState) error {
	if err := s.client.HSetNX(ctx, s.countKey(), strconv.FormatInt(cardID, 10), state.Count).Err(); err != nil {
		return err
	}
	if userID <= 0 {
		return nil
	}
	if state.Liked {
		return s.client.SAdd(ctx, s.usersKey(cardID), userID).Err()
	}
	return s.client.SRem(ctx, s.usersKey(cardID), userID).Err()
}

func luaInt64(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(v, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(v), 10, 64)
		return n
	default:
		return 0
	}
}
