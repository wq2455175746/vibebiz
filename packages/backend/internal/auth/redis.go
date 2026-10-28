package auth

import (
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/config"
)

func NewRedisClient(cfg config.Config) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisHost,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
}
