package businesscard

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

type CommentRateLimiter interface {
	Allow(ctx context.Context, userID, cardID int64) bool
}

type RedisCommentRateLimiter struct {
	Client *redis.Client
	Prefix string
	Now    func() time.Time
	Logger *slog.Logger
}

func (l *RedisCommentRateLimiter) Allow(ctx context.Context, userID, cardID int64) bool {
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	bucket := now().Unix() / 10
	shortKey := fmt.Sprintf("%s:comment:rate:short:%d:%d:%d", l.Prefix, userID, cardID, bucket)
	hourKey := fmt.Sprintf("%s:comment:rate:hour:%d", l.Prefix, userID)
	pipe := l.Client.TxPipeline()
	short := pipe.Incr(ctx, shortKey)
	pipe.Expire(ctx, shortKey, 15*time.Second)
	hour := pipe.Incr(ctx, hourKey)
	pipe.Expire(ctx, hourKey, time.Hour)
	_, err := pipe.Exec(ctx)
	if err != nil {
		if l.Logger != nil {
			l.Logger.Warn("comment rate limiter failed, allowing request",
				"user_id", userID, "card_id", cardID, "error", err)
		}
		return true
	}
	return short.Val() <= 5 && hour.Val() <= 30
}
