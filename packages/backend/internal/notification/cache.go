package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"vibebiz/internal/dao/model"
)

var ErrIndexMissing = errors.New("notification index missing")

type Published struct {
	ID          int64                  `json:"id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Type        model.NotificationType `json:"type"`
	Version     string                 `json:"version"`
	Link        string                 `json:"link"`
	PublishedAt time.Time              `json:"publishedAt"`
	ExpireAt    time.Time              `json:"expireAt"`
}

type Cache struct {
	client     *redis.Client
	index      string
	prefix     string
	readPrefix string
}

func NewCache(client *redis.Client, prefix string) *Cache {
	return &Cache{
		client:     client,
		index:      prefix + ":notification:index",
		prefix:     prefix + ":notification:item:",
		readPrefix: prefix + ":notification:read:",
	}
}

func (c *Cache) itemKey(id int64) string {
	return fmt.Sprintf("%s%d", c.prefix, id)
}

func (c *Cache) readKey(userID int64) string {
	return fmt.Sprintf("%s%d", c.readPrefix, userID)
}

const readStateTTL = 90 * 24 * time.Hour

func (c *Cache) LastRead(ctx context.Context, userID int64) (time.Time, error) {
	value, err := c.client.Get(ctx, c.readKey(userID)).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("read notification state: %w", err)
	}
	return time.UnixMilli(value).UTC(), nil
}

func (c *Cache) MarkRead(ctx context.Context, userID int64, readAt time.Time) error {
	current, err := c.LastRead(ctx, userID)
	if err != nil {
		return err
	}
	if !readAt.After(current) && !current.IsZero() {
		return nil
	}
	if err := c.client.Set(ctx, c.readKey(userID), readAt.UnixMilli(), readStateTTL).Err(); err != nil {
		return fmt.Errorf("store notification read state: %w", err)
	}
	return nil
}

func publishedFromModel(item model.OpsNotification) Published {
	publishedAt := item.CreatedAt
	if item.PublishedAt != nil {
		publishedAt = *item.PublishedAt
	}
	return Published{
		ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
		Version: item.Version, Link: item.Link, PublishedAt: publishedAt, ExpireAt: item.ExpireAt,
	}
}

func (c *Cache) Put(ctx context.Context, item model.OpsNotification) error {
	if !item.ExpireAt.After(time.Now()) {
		return nil
	}
	payload := publishedFromModel(item)
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	pipe := c.client.TxPipeline()
	pipe.Set(ctx, c.itemKey(item.ID), data, 0)
	pipe.ExpireAt(ctx, c.itemKey(item.ID), item.ExpireAt)
	pipe.ZAdd(ctx, c.index, redis.Z{Score: float64(payload.PublishedAt.UnixMilli()), Member: item.ID})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache notification: %w", err)
	}
	return nil
}

func (c *Cache) Remove(ctx context.Context, id int64) error {
	pipe := c.client.TxPipeline()
	pipe.Del(ctx, c.itemKey(id))
	pipe.ZRem(ctx, c.index, id)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("remove notification cache: %w", err)
	}
	return nil
}

func (c *Cache) List(ctx context.Context) ([]Published, error) {
	exists, err := c.client.Exists(ctx, c.index).Result()
	if err != nil {
		return nil, fmt.Errorf("check notification index: %w", err)
	}
	if exists == 0 {
		return nil, ErrIndexMissing
	}
	ids, err := c.client.ZRevRange(ctx, c.index, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read notification index: %w", err)
	}
	if len(ids) == 0 {
		return []Published{}, nil
	}
	keys := make([]string, 0, len(ids))
	members := make([]string, 0, len(ids))
	for _, raw := range ids {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			_ = c.client.ZRem(ctx, c.index, raw).Err()
			continue
		}
		keys = append(keys, c.itemKey(id))
		members = append(members, raw)
	}
	if len(keys) == 0 {
		return []Published{}, nil
	}
	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("read notification items: %w", err)
	}
	out := make([]Published, 0, len(values))
	stale := make([]any, 0)
	for i, value := range values {
		raw, ok := value.(string)
		if !ok {
			stale = append(stale, members[i])
			continue
		}
		var item Published
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			stale = append(stale, members[i])
			continue
		}
		out = append(out, item)
	}
	if len(stale) > 0 {
		_ = c.client.ZRem(ctx, c.index, stale...).Err()
	}
	return out, nil
}

func (c *Cache) Rebuild(ctx context.Context, items []model.OpsNotification) error {
	if err := c.client.Del(ctx, c.index).Err(); err != nil {
		return fmt.Errorf("reset notification index: %w", err)
	}
	for _, item := range items {
		if err := c.Put(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
