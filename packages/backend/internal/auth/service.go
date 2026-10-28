package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"vibebiz/internal/dao/model"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountDisabled    = errors.New("account disabled")
	ErrNoWorkspace        = errors.New("no workspace available")
)

type UserFinder interface {
	FindUserByIdentifier(ctx context.Context, identifier string) (*model.User, error)
	WorkspaceIDs(ctx context.Context, userID int64) ([]int64, error)
	TouchUserLogin(ctx context.Context, id int64, ip string, at time.Time) error
}

type OpsFinder interface {
	FindOpsByIdentifier(ctx context.Context, identifier string) (*model.OpsUser, error)
	TouchOpsLogin(ctx context.Context, id int64, ip string, at time.Time) error
}

type Service struct {
	Users    UserFinder
	Ops      OpsFinder
	Sessions Store
	UserTTL  time.Duration
	OpsTTL   time.Duration
	Now      func() time.Time
}

func NewService(users UserFinder, ops OpsFinder, sessions Store, userTTL, opsTTL time.Duration) *Service {
	return &Service{
		Users:    users,
		Ops:      ops,
		Sessions: sessions,
		UserTTL:  userTTL,
		OpsTTL:   opsTTL,
		Now:      time.Now,
	}
}

func (s *Service) LoginUser(ctx context.Context, identifier, password, ip string) (*Session, error) {
	user, err := s.Users.FindUserByIdentifier(ctx, identifier)
	if err != nil || user == nil || user.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}
	if !VerifyPassword(*user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	if user.Status != 1 {
		return nil, ErrAccountDisabled
	}

	workspaceIDs, err := s.Users.WorkspaceIDs(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("load workspaces: %w", err)
	}
	if len(workspaceIDs) == 0 {
		return nil, ErrNoWorkspace
	}

	sid, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	now := s.now()
	session := &Session{
		ID:           sid,
		Realm:        RealmUser,
		UserID:       user.ID,
		Username:     user.Username,
		Email:        user.Email,
		WorkspaceIDs: workspaceIDs,
		CreatedAt:    now,
	}
	if err := s.Sessions.Create(ctx, session, s.userTTL()); err != nil {
		return nil, err
	}
	_ = s.Users.TouchUserLogin(ctx, user.ID, ip, now)
	return session, nil
}

func (s *Service) LoginOps(ctx context.Context, identifier, password, ip string) (*Session, error) {
	user, err := s.Ops.FindOpsByIdentifier(ctx, identifier)
	if err != nil || user == nil || user.PasswordHash == nil {
		return nil, ErrInvalidCredentials
	}
	if !VerifyPassword(*user.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	if user.Status != 1 {
		return nil, ErrAccountDisabled
	}

	sid, err := newSessionID()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}

	now := s.now()
	session := &Session{
		ID:        sid,
		Realm:     RealmOps,
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.RoleCode,
		CreatedAt: now,
	}
	if err := s.Sessions.Create(ctx, session, s.opsTTL()); err != nil {
		return nil, err
	}
	_ = s.Ops.TouchOpsLogin(ctx, user.ID, ip, now)
	return session, nil
}

func (s *Service) Current(ctx context.Context, realm Realm, sid string) (*Session, error) {
	if sid == "" {
		return nil, ErrSessionNotFound
	}
	return s.Sessions.Get(ctx, realm, sid)
}

func (s *Service) Logout(ctx context.Context, realm Realm, sid string) error {
	if sid == "" {
		return nil
	}
	return s.Sessions.Delete(ctx, realm, sid)
}

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *Service) userTTL() time.Duration {
	if s.UserTTL > 0 {
		return s.UserTTL
	}
	return 168 * time.Hour
}

func (s *Service) opsTTL() time.Duration {
	if s.OpsTTL > 0 {
		return s.OpsTTL
	}
	return 12 * time.Hour
}

func newSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random session id: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
