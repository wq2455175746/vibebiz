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

func (s *RedisStore) Update(ctx context.Context, session *Session) error {
	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	key := s.key(session.Realm, session.ID)
	ttl, err := s.client.TTL(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("session ttl: %w", err)
	}
	if ttl == -2*time.Nanosecond {
		return ErrSessionNotFound
	}
	expiration := time.Duration(0)
	if ttl > 0 {
		expiration = ttl
	}
	if err := s.client.Set(ctx, key, payload, expiration).Err(); err != nil {
		return fmt.Errorf("update session: %w", err)
	}
	return nil
}

func (s *RedisStore) Delete(ctx context.Context, realm Realm, id string) error {
	if err := s.client.Del(ctx, s.key(realm, id)).Err(); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *RedisStore) DeleteByUser(ctx context.Context, realm Realm, userID int64) error {
	pattern := s.prefix + ":session:" + string(realm) + ":*"
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan sessions: %w", err)
		}
		for _, key := range keys {
			payload, err := s.client.Get(ctx, key).Bytes()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return fmt.Errorf("get session: %w", err)
			}
			var session Session
			if err := json.Unmarshal(payload, &session); err != nil {
				continue
			}
			if session.UserID != userID {
				continue
			}
			if err := s.client.Del(ctx, key).Err(); err != nil {
				return fmt.Errorf("delete session: %w", err)
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return nil
}

func (s *RedisStore) DeleteOthersByUser(ctx context.Context, realm Realm, userID int64, keepSessionID string) error {
	pattern := s.prefix + ":session:" + string(realm) + ":*"
	var cursor uint64
	for {
		keys, next, err := s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("scan sessions: %w", err)
		}
		for _, key := range keys {
			payload, err := s.client.Get(ctx, key).Bytes()
			if errors.Is(err, redis.Nil) {
				continue
			}
			if err != nil {
				return fmt.Errorf("get session: %w", err)
			}
			var session Session
			if err := json.Unmarshal(payload, &session); err != nil {
				continue
			}
			if session.UserID != userID || session.ID == keepSessionID {
				continue
			}
			if err := s.client.Del(ctx, key).Err(); err != nil {
				return fmt.Errorf("delete session: %w", err)
			}
		}
		if next == 0 {
			break
		}
		cursor = next
	}
	return nil
}

func (s *RedisStore) key(realm Realm, id string) string {
	return s.prefix + ":session:" + string(realm) + ":" + id
}
