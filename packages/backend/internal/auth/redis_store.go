package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	prefix string
}

func NewRedisStore(client *redis.Client, prefix string) *RedisStore {
	return &RedisStore{client: client, prefix: prefix}
}

func (s *RedisStore) Create(ctx context.Context, session *Session, ttl time.Duration) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	if err := s.client.Set(ctx, s.key(session.Realm, session.ID), payload, ttl).Err(); err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

func (s *RedisStore) Get(ctx context.Context, realm Realm, id string) (*Session, error) {
	payload, err := s.client.Get(ctx, s.key(realm, id)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("get session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}
	return &session, nil
}

func (s *RedisStore) Delete(ctx context.Context, realm Realm, id string) error {
	if err := s.client.Del(ctx, s.key(realm, id)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *RedisStore) key(realm Realm, id string) string {
	return s.prefix + ":session:" + string(realm) + ":" + id
}
