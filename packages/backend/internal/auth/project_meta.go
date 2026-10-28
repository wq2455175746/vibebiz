package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/response"
)

var ErrProjectMetaNotFound = errors.New("project metadata not found")

type ProjectMeta struct {
	WorkspaceID int64 `json:"workspaceId"`
	OwnerUserID int64 `json:"ownerUserId"`
}

type ProjectMetaStore interface {
	Set(ctx context.Context, projectID string, meta ProjectMeta) error
	Get(ctx context.Context, projectID string) (ProjectMeta, error)
	Delete(ctx context.Context, projectID string) error
}

type RedisProjectMetaStore struct {
	client *redis.Client
	prefix string
}

func NewProjectMetaStore(client *redis.Client, prefix string) *RedisProjectMetaStore {
	return &RedisProjectMetaStore{client: client, prefix: prefix}
}

func (s *RedisProjectMetaStore) Set(ctx context.Context, projectID string, meta ProjectMeta) error {
	payload, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshal project meta: %w", err)
	}
	if err := s.client.Set(ctx, s.key(projectID), payload, 0).Err(); err != nil {
		return fmt.Errorf("set project meta: %w", err)
	}
	return nil
}

func (s *RedisProjectMetaStore) Get(ctx context.Context, projectID string) (ProjectMeta, error) {
	payload, err := s.client.Get(ctx, s.key(projectID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ProjectMeta{}, ErrProjectMetaNotFound
		}
		return ProjectMeta{}, fmt.Errorf("get project meta: %w", err)
	}
	var meta ProjectMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		return ProjectMeta{}, fmt.Errorf("unmarshal project meta: %w", err)
	}
	return meta, nil
}

func (s *RedisProjectMetaStore) Delete(ctx context.Context, projectID string) error {
	if err := s.client.Del(ctx, s.key(projectID)).Err(); err != nil {
		return fmt.Errorf("delete project meta: %w", err)
	}
	return nil
}

func (s *RedisProjectMetaStore) key(projectID string) string {
	return s.prefix + ":project:meta:" + projectID
}

func RequireProjectAccess(metas ProjectMetaStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			session, ok := SessionFrom(c)
			if !ok {
				return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
			}
			meta, err := metas.Get(c.Request().Context(), c.Param("id"))
			if err != nil {
				if errors.Is(err, ErrProjectMetaNotFound) {
					return response.Fail(c, http.StatusNotFound, 40401, "project not found")
				}
				return response.Fail(c, http.StatusServiceUnavailable, 50301, "project metadata unavailable")
			}
			if !containsWorkspace(session.WorkspaceIDs, meta.WorkspaceID) {
				return response.Fail(c, http.StatusNotFound, 40401, "project not found")
			}
			c.Set("auth.project_meta", meta)
			return next(c)
		}
	}
}

func containsWorkspace(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}
