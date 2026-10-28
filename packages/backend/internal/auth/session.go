package auth

import (
	"context"
	"errors"
	"time"
)

type Realm string

const (
	RealmUser Realm = "user"
	RealmOps  Realm = "ops"
)

type Session struct {
	ID           string    `json:"id"`
	Realm        Realm     `json:"realm"`
	UserID       int64     `json:"userId"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Role         string    `json:"role,omitempty"`
	WorkspaceIDs []int64   `json:"workspaceIds,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
}

var ErrSessionNotFound = errors.New("session not found")

type Store interface {
	Create(ctx context.Context, session *Session, ttl time.Duration) error
	Get(ctx context.Context, realm Realm, id string) (*Session, error)
	Delete(ctx context.Context, realm Realm, id string) error
}
