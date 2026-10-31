package scheduledtask

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisFlags struct {
	client *redis.Client
	prefix string
}

func NewRedisFlags(client *redis.Client, prefix string) *RedisFlags {
	return &RedisFlags{client: client, prefix: strings.TrimSuffix(prefix, ":")}
}

func (f *RedisFlags) lockKey(taskID int64) string {
	return fmt.Sprintf("%s:scheduled:run:%d", f.prefix, taskID)
}

func (f *RedisFlags) stopKey(taskID int64) string {
	return fmt.Sprintf("%s:scheduled:stop:%d", f.prefix, taskID)
}

func (f *RedisFlags) Acquire(ctx context.Context, taskID int64, ttl time.Duration) (bool, error) {
	return f.client.SetNX(ctx, f.lockKey(taskID), "1", ttl).Result()
}

func (f *RedisFlags) Release(ctx context.Context, taskID int64) {
	_ = f.client.Del(ctx, f.lockKey(taskID)).Err()
}

func (f *RedisFlags) MarkStop(ctx context.Context, taskID int64, ttl time.Duration) error {
	return f.client.Set(ctx, f.stopKey(taskID), "1", ttl).Err()
}

func (f *RedisFlags) Stopped(ctx context.Context, taskID int64) bool {
	value, err := f.client.Exists(ctx, f.stopKey(taskID)).Result()
	return err == nil && value == 1
}

func (f *RedisFlags) ClearStop(ctx context.Context, taskID int64) {
	_ = f.client.Del(ctx, f.stopKey(taskID)).Err()
}
