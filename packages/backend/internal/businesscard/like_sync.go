package businesscard

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/observability"
)

const TaskTypeLikeSync = "card:like:sync"

const (
	defaultLikeSyncGroup    = "card-like-sync"
	defaultLikeSyncConsumer = "cp"
	defaultLikeSyncBatch    = 1000
)

type LikeSync struct {
	Client   *redis.Client
	Repo     LikeSyncRepository
	Prefix   string
	Group    string
	Consumer string
	Batch    int
	MaxLen   int64
	TTL      time.Duration
	Shards   int
	Logger   *slog.Logger
	Metrics  *observability.Metrics
}

func (s *LikeSync) streamKeys() []string {
	if s.Shards <= 1 {
		return []string{s.Prefix + ":card:like:stream"}
	}
	keys := make([]string, 0, s.Shards)
	for i := 0; i < s.Shards; i++ {
		keys = append(keys, fmt.Sprintf("%s:card:like:stream:%d", s.Prefix, i))
	}
	return keys
}

func (s *LikeSync) HandleTask(ctx context.Context, _ *asynq.Task) error {
	return s.Run(ctx)
}

func (s *LikeSync) Run(ctx context.Context) error {
	if s.Client == nil || s.Repo == nil {
		return errors.New("like sync: client and repo are required")
	}
	start := time.Now()
	defer func() {
		if s.Metrics != nil {
			s.Metrics.ObserveCardLikeSync(time.Since(start).Seconds())
		}
	}()
	var pending int64
	pendingKnown := true
	for _, key := range s.streamKeys() {
		if err := s.runStream(ctx, key); err != nil {
			return err
		}
		count, ok := s.pendingCount(ctx, key)
		if !ok {
			pendingKnown = false
			continue
		}
		pending += count
	}
	if s.Metrics != nil && pendingKnown {
		s.Metrics.SetCardLikeStreamPending(float64(pending))
	}
	return nil
}

func (s *LikeSync) pendingCount(ctx context.Context, key string) (int64, bool) {
	pending, err := s.Client.XPending(ctx, key, s.group()).Result()
	if err != nil {
		s.logWarn("card like stream pending check failed", key, err)
		return 0, false
	}
	return pending.Count, true
}

func (s *LikeSync) runStream(ctx context.Context, key string) error {
	group := s.group()
	consumer := s.consumer()
	batch := s.batch()
	if err := s.Client.XGroupCreateMkStream(ctx, key, group, "0").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	if err := s.drainPending(ctx, key, group, consumer, batch); err != nil {
		return err
	}
	for {
		result, err := s.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: group, Consumer: consumer,
			Streams: []string{key, ">"}, Count: int64(batch), Block: -1,
		}).Result()
		if errors.Is(err, redis.Nil) {
			break
		}
		if err != nil {
			return err
		}
		if len(result) == 0 || len(result[0].Messages) == 0 {
			break
		}
		if err := s.process(ctx, key, group, result[0].Messages); err != nil {
			return err
		}
	}
	if s.MaxLen > 0 {
		if err := s.Client.XTrimMaxLenApprox(ctx, key, s.MaxLen, 0).Err(); err != nil {
			s.logWarn("card like stream trim maxlen failed", key, err)
		}
	}
	if s.TTL > 0 {
		minID := strconv.FormatInt(time.Now().Add(-s.TTL).UnixMilli(), 10) + "-0"
		if err := s.Client.XTrimMinIDApprox(ctx, key, minID, 0).Err(); err != nil {
			s.logWarn("card like stream trim ttl failed", key, err)
		}
	}
	return nil
}

func (s *LikeSync) drainPending(ctx context.Context, key, group, consumer string, batch int) error {
	start := "0-0"
	for {
		claimed, next, err := s.Client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream: key, Group: group, Consumer: consumer,
			MinIdle: 0, Start: start, Count: int64(batch),
		}).Result()
		if err != nil {
			return err
		}
		if len(claimed) > 0 {
			if err := s.process(ctx, key, group, claimed); err != nil {
				return err
			}
		}
		if next == "0-0" || next == start {
			break
		}
		start = next
	}
	return nil
}

func (s *LikeSync) process(ctx context.Context, key, group string, messages []redis.XMessage) error {
	events := make([]LikeEvent, 0, len(messages))
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
		event := LikeEvent{
			EventID: fmt.Sprint(message.Values["eventId"]),
			CardID:  parseInt64(message.Values["cardId"]),
			UserID:  parseInt64(message.Values["userId"]),
			Action:  fmt.Sprint(message.Values["action"]),
			TS:      parseInt64(message.Values["ts"]),
		}
		if event.CardID <= 0 || event.UserID <= 0 {
			s.logDroppedEvent(message.ID, event, "invalid identifiers")
			continue
		}
		if event.Action != "like" && event.Action != "unlike" {
			s.logDroppedEvent(message.ID, event, "unknown action")
			continue
		}
		events = append(events, event)
	}
	if len(events) > 0 {
		affected, err := s.Repo.Apply(ctx, events)
		if err != nil {
			return err
		}
		if _, err := s.Repo.Recount(ctx, affected); err != nil {
			return err
		}
	}
	return s.Client.XAck(ctx, key, group, ids...).Err()
}

func (s *LikeSync) group() string {
	if s.Group == "" {
		return defaultLikeSyncGroup
	}
	return s.Group
}

func (s *LikeSync) consumer() string {
	if s.Consumer == "" {
		return defaultLikeSyncConsumer
	}
	return s.Consumer
}

func (s *LikeSync) batch() int {
	if s.Batch <= 0 {
		return defaultLikeSyncBatch
	}
	return s.Batch
}

func (s *LikeSync) logWarn(message, key string, err error) {
	if s.Logger != nil {
		s.Logger.Warn(message, "stream", key, "error", err)
	}
}

func (s *LikeSync) logDroppedEvent(messageID string, event LikeEvent, reason string) {
	if s.Logger == nil {
		return
	}
	s.Logger.Warn("card like sync dropped invalid event",
		"message_id", messageID,
		"event_id", event.EventID,
		"card_id", event.CardID,
		"user_id", event.UserID,
		"action", event.Action,
		"reason", reason,
	)
}

func parseInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}
