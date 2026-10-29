package requestcount

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

const ttlDuration = 48 * time.Hour

type Counter struct {
	client *redis.Client
	prefix string
	now    func() time.Time
}

func NewCounter(client *redis.Client, prefix string) *Counter {
	return &Counter{client: client, prefix: prefix, now: time.Now}
}

func (c *Counter) key(now time.Time) string {
	return fmt.Sprintf("%s:cp:requests:%s", c.prefix, now.UTC().Format("2006-01-02"))
}

func (c *Counter) Inc(ctx context.Context) error {
	key := c.key(c.now())
	pipe := c.client.TxPipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, ttlDuration)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Counter) Today(ctx context.Context) (int64, error) {
	value, err := c.client.Get(ctx, c.key(c.now())).Int64()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return 0, nil
		}
		return 0, err
	}
	return value, nil
}

func (c *Counter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			request := ctx.Request()
			if request.Method == http.MethodOptions || request.URL.Path == "/health" || request.URL.Path == "/metrics" {
				return next(ctx)
			}
			_ = c.Inc(request.Context())
			return next(ctx)
		}
	}
}
