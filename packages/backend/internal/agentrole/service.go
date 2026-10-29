package agentrole

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const MaxAvatarBytes = 5 << 20

var ErrNotFound = errors.New("role not found")

var (
	ErrInvalid            = errors.New("invalid role")
	ErrConflict           = errors.New("role code already exists")
	ErrAvatarInvalid      = errors.New("invalid role avatar")
	ErrAvatarNotFound     = errors.New("role avatar not found")
	ErrStorageUnavailable = errors.New("role avatar storage not configured")
)

var roleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

var avatarExtensions = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}

type Presigner interface {
	PresignPut(ctx context.Context, key, contentType string, size int64, expiry time.Duration) (string, map[string]string, error)
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	Head(ctx context.Context, key string) (int64, string, error)
}

type Role struct {
	ID           int64     `json:"id"`
	RoleCode     string    `json:"roleCode"`
	RoleNameEN   string    `json:"roleNameEn"`
	RoleNameZH   string    `json:"roleNameZh"`
	Description  string    `json:"description"`
	SystemPrompt string    `json:"systemPrompt"`
	AvatarURL    string    `json:"avatarUrl"`
	IsBuiltin    bool      `json:"isBuiltin"`
	Status       int8      `json:"status"`
	SortOrder    int       `json:"sortOrder"`
	Version      int       `json:"version"`
	CreatedBy    string    `json:"createdBy"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type CreateInput struct {
	RoleCode     string
	RoleNameEN   string
	RoleNameZH   string
	Description  string
	SystemPrompt string
	AvatarKey    string
	Status       int8
	SortOrder    int
}

type Presign struct {
	UploadURL string            `json:"uploadUrl"`
	Key       string            `json:"key"`
	Headers   map[string]string `json:"headers"`
}

type Repository interface {
	List(ctx context.Context) ([]Role, error)
	ListActive(ctx context.Context) ([]Role, error)
	GetActive(ctx context.Context, id int64) (Role, error)
	Create(ctx context.Context, input CreateInput, createdBy string) (Role, error)
}

type Service struct {
	Repo   Repository
	S3     Presigner
	Expiry time.Duration
	NewID  func() string
	Now    func() time.Time
}

func NewService(repo Repository, s3 Presigner, expiry time.Duration) *Service {
	return &Service{Repo: repo, S3: s3, Expiry: expiry, NewID: uuid.NewString, Now: time.Now}
}

func (s *Service) List(ctx context.Context) ([]Role, error) {
	roles, err := s.Repo.List(ctx)
	if err != nil {
		return nil, err
	}
	for index := range roles {
		roles[index].AvatarURL = s.avatarURL(ctx, roles[index].AvatarURL)
	}
	return roles, nil
}

func (s *Service) ListActive(ctx context.Context) ([]Role, error) {
	roles, err := s.Repo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	for index := range roles {
		roles[index].AvatarURL = s.avatarURL(ctx, roles[index].AvatarURL)
	}
	return roles, nil
}

func (s *Service) GetActive(ctx context.Context, id int64) (Role, error) {
	role, err := s.Repo.GetActive(ctx, id)
	if err != nil {
		return Role{}, err
	}
	role.AvatarURL = s.avatarURL(ctx, role.AvatarURL)
	return role, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput, createdBy string) (Role, error) {
	normalized, err := normalize(input)
	if err != nil {
		return Role{}, err
	}
	if normalized.AvatarKey != "" {
		if s.S3 == nil {
			return Role{}, ErrStorageUnavailable
		}
		if _, _, err := s.S3.Head(ctx, normalized.AvatarKey); err != nil {
			return Role{}, ErrAvatarNotFound
		}
	}
	role, err := s.Repo.Create(ctx, normalized, createdBy)
	if err != nil {
		return Role{}, err
	}
	role.AvatarURL = s.avatarURL(ctx, role.AvatarURL)
	return role, nil
}

func (s *Service) PresignAvatar(ctx context.Context, filename, contentType string, size int64) (Presign, error) {
	ext, ok := avatarExtensions[strings.TrimSpace(contentType)]
	if !ok || size <= 0 || size > MaxAvatarBytes {
		return Presign{}, ErrAvatarInvalid
	}
	if s.S3 == nil {
		return Presign{}, ErrStorageUnavailable
	}
	key := fmt.Sprintf("roles/avatar/%s.%s", s.newID(), ext)
	uploadURL, headers, err := s.S3.PresignPut(ctx, key, contentType, size, s.expiry())
	if err != nil {
		return Presign{}, err
	}
	return Presign{UploadURL: uploadURL, Key: key, Headers: headers}, nil
}

func (s *Service) UploadAvatar(ctx context.Context, contentType string, size int64, body io.Reader) (string, error) {
	ext, ok := avatarExtensions[strings.TrimSpace(contentType)]
	if !ok || size <= 0 || size > MaxAvatarBytes {
		return "", ErrAvatarInvalid
	}
	if s.S3 == nil {
		return "", ErrStorageUnavailable
	}
	key := fmt.Sprintf("roles/avatar/%s.%s", s.newID(), ext)
	if err := s.S3.Put(ctx, key, contentType, size, body); err != nil {
		return "", err
	}
	return key, nil
}

func (s *Service) avatarURL(ctx context.Context, key string) string {
	if key == "" || s.S3 == nil {
		return ""
	}
	url, err := s.S3.PresignGet(ctx, key, s.expiry())
	if err != nil {
		return ""
	}
	return url
}

func (s *Service) newID() string {
	if s.NewID != nil {
		return s.NewID()
	}
	return uuid.NewString()
}

func (s *Service) expiry() time.Duration {
	if s.Expiry <= 0 {
		return 15 * time.Minute
	}
	return s.Expiry
}

func normalize(input CreateInput) (CreateInput, error) {
	input.RoleCode = strings.TrimSpace(input.RoleCode)
	input.RoleNameEN = strings.TrimSpace(input.RoleNameEN)
	input.RoleNameZH = strings.TrimSpace(input.RoleNameZH)
	input.Description = strings.TrimSpace(input.Description)
	input.SystemPrompt = strings.TrimSpace(input.SystemPrompt)
	input.AvatarKey = strings.TrimSpace(input.AvatarKey)

	if !roleCodePattern.MatchString(input.RoleCode) {
		return CreateInput{}, ErrInvalid
	}
	if input.RoleNameEN == "" || input.RoleNameZH == "" || input.SystemPrompt == "" {
		return CreateInput{}, ErrInvalid
	}
	if utf8.RuneCountInString(input.RoleNameEN) > 64 || utf8.RuneCountInString(input.RoleNameZH) > 64 {
		return CreateInput{}, ErrInvalid
	}
	if utf8.RuneCountInString(input.Description) > 1000 || utf8.RuneCountInString(input.SystemPrompt) > 100000 {
		return CreateInput{}, ErrInvalid
	}
	if input.Status != 0 && input.Status != 1 {
		return CreateInput{}, ErrInvalid
	}
	if input.AvatarKey != "" {
		if len(input.AvatarKey) > 1024 || !strings.HasPrefix(input.AvatarKey, "roles/avatar/") || strings.Contains(input.AvatarKey, "..") {
			return CreateInput{}, ErrInvalid
		}
	}
	return input, nil
}
