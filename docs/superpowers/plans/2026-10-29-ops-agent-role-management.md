# Ops Agent 角色管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 ops-web 增加角色列表与新增表单，通过 CP admin 接口写入 `agent_role`，支持可选 S3 头像。

**Architecture:** 新增 `internal/agentrole` 承载角色视图、校验、头像 key 处理和仓储；CP 复用现有 S3 presigner，ops-web 增加角色管理页。

**Tech Stack:** Go / Echo / GORM / MySQL / AWS S3 presigner；Vue 3 / PrimeVue / Vitest。

**Spec:** `docs/superpowers/specs/2026-10-29-ops-agent-role-management-design.md`

## Global Constraints

- 新角色固定 `is_builtin=0`、`version=1`。
- `role_code` 格式 `^[a-z][a-z0-9_-]{0,31}$`，全表唯一。
- 头像可选，key 前缀 `roles/avatar/`，只允许 JPG / PNG / WebP，最大 5MB。
- 不做编辑、删除、启停。
- 不新增数据库迁移。
- 未获用户明确要求不执行 `git commit`。
- 每个任务以测试通过结束。

---

### Task 1: agentrole 类型、校验、头像与内存仓储

**Files:**
- Create: `packages/backend/internal/agentrole/service.go`
- Create: `packages/backend/internal/agentrole/memory.go`
- Test: `packages/backend/internal/agentrole/service_test.go`

**Interfaces:**
- Produces: `agentrole.Role`、`CreateInput`、`Repository`、`Service`、`NewService(repo, s3, expiry)`、`NewMemoryRepository()`、`Presigner`
- Errors: `ErrInvalid`、`ErrConflict`、`ErrAvatarInvalid`、`ErrAvatarNotFound`、`ErrStorageUnavailable`

- [ ] **Step 1: 写失败测试**

```go
package agentrole

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakePresigner struct {
	putKey   string
	getKey   string
	headErr  error
	putErr   error
	headCall int
}

func (f *fakePresigner) PresignPut(_ context.Context, key, _ string, _ int64, _ time.Duration) (string, map[string]string, error) {
	f.putKey = key
	return "https://upload/" + key, map[string]string{"Content-Type": "image/png"}, f.putErr
}

func (f *fakePresigner) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	f.getKey = key
	return "https://read/" + key, nil
}

func (f *fakePresigner) Put(_ context.Context, key, _ string, _ int64, _ interface{ Read([]byte) (int, error) }) error {
	f.putKey = key
	return f.putErr
}

func (f *fakePresigner) Head(_ context.Context, key string) (int64, string, error) {
	f.headCall++
	f.headErr = nil
	f.getKey = key
	return 1, "image/png", nil
}

func validRole() CreateInput {
	return CreateInput{
		RoleCode:     "frank",
		RoleNameEN:   "Frank",
		RoleNameZH:   "测试工程师",
		Description:  "负责测试策略与用例。",
		SystemPrompt: "你是 Frank，测试工程师。",
		Status:       1,
		SortOrder:    60,
	}
}

func TestCreateRole(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	role, err := service.Create(context.Background(), validRole(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if role.RoleCode != "frank" || role.IsBuiltin || role.Version != 1 {
		t.Fatalf("role = %+v", role)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	cases := []CreateInput{
		{},
		{RoleCode: "Frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x"},
		{RoleCode: "bad code", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x"},
		{RoleCode: "frank", RoleNameEN: "", RoleNameZH: "F", SystemPrompt: "x"},
		{RoleCode: "frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: ""},
		{RoleCode: "frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x", Status: 2},
		{RoleCode: "frank", RoleNameEN: "F", RoleNameZH: "F", SystemPrompt: "x", AvatarKey: "avatar/x.png"},
	}
	for index, input := range cases {
		if _, err := service.Create(context.Background(), input, "admin"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d err = %v", index, err)
		}
	}
}

func TestDuplicateRoleCodeReturnsConflict(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	if _, err := service.Create(context.Background(), validRole(), "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), validRole(), "admin"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestListSortsBySortOrderThenID(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	second := validRole()
	second.RoleCode = "bob"
	second.SortOrder = 20
	first := validRole()
	first.RoleCode = "alex"
	first.SortOrder = 10
	if _, err := service.Create(context.Background(), second, "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), first, "admin"); err != nil {
		t.Fatal(err)
	}
	roles, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 || roles[0].RoleCode != "alex" || roles[1].RoleCode != "bob" {
		t.Fatalf("roles = %+v", roles)
	}
}

func TestAvatarPresignAndUploadRequireStorage(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil, 0)
	if _, err := service.PresignAvatar(context.Background(), "a.png", "image/png", 10); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("presign err = %v", err)
	}
	if _, err := service.UploadAvatar(context.Background(), "image/png", 10, nil); !errors.Is(err, ErrStorageUnavailable) {
		t.Fatalf("upload err = %v", err)
	}
}

func TestAvatarKeyPrefixIsValidated(t *testing.T) {
	service := NewService(NewMemoryRepository(), &fakePresigner{}, 0)
	input := validRole()
	input.AvatarKey = "roles/avatar/ok.png"
	if _, err := service.Create(context.Background(), input, "admin"); err != nil {
		t.Fatal(err)
	}
	input.RoleCode = "other"
	input.AvatarKey = "avatar/bad.png"
	if _, err := service.Create(context.Background(), input, "admin"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestPresignAvatarBuildsKey(t *testing.T) {
	s3 := &fakePresigner{}
	service := NewService(NewMemoryRepository(), s3, time.Minute)
	result, err := service.PresignAvatar(context.Background(), "frank.png", "image/png", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if result.Key == "" || s3.putKey == "" {
		t.Fatalf("result = %+v", result)
	}
}
```

> `fakePresigner.Put` 的 `io.Reader` 参数请使用标准 `io.Reader`，不要使用测试里的匿名接口；实现时以 `io.Reader` 为准。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/agentrole -count=1`  
Expected: FAIL，包不存在

- [ ] **Step 3: 实现 service 与 memory**

`service.go`：

```go
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
```

`memory.go`：

```go
package agentrole

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu    sync.Mutex
	next  int64
	items []Role
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (r *MemoryRepository) List(context.Context) ([]Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Role, len(r.items))
	copy(out, r.items)
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput, createdBy string) (Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.RoleCode == input.RoleCode {
			return Role{}, ErrConflict
		}
	}
	r.next++
	now := time.Now().UTC()
	item := Role{
		ID: r.next, RoleCode: input.RoleCode, RoleNameEN: input.RoleNameEN,
		RoleNameZH: input.RoleNameZH, Description: input.Description,
		SystemPrompt: input.SystemPrompt, AvatarURL: input.AvatarKey,
		IsBuiltin: false, Status: input.Status, SortOrder: input.SortOrder,
		Version: 1, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	r.items = append(r.items, item)
	return item, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/agentrole -count=1`  
Expected: PASS

---

### Task 2: agentrole MySQL 仓储

**Files:**
- Create: `packages/backend/internal/agentrole/mysql.go`
- Test: `packages/backend/internal/agentrole/mysql_test.go`

**Interfaces:**
- Produces: `agentrole.NewMySQLRepository(db *gorm.DB) *MySQLRepository`

- [ ] **Step 1: 写失败测试**

```go
package agentrole

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return gormDB, mock
}

func TestMySQLListRoles(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "avatar_url", "is_builtin", "status", "sort_order", "version", "created_by", "created_at", "updated_at"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "desc", "prompt", "", 1, 1, 10, 1, "", now, now))

	repo := NewMySQLRepository(db)
	roles, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].RoleCode != "alex" || !roles[0].IsBuiltin {
		t.Fatalf("roles = %+v", roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/agentrole -run TestMySQLListRoles -count=1`  
Expected: FAIL，`NewMySQLRepository` 未定义

- [ ] **Step 3: 实现**

```go
package agentrole

import (
	"context"
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) List(ctx context.Context) ([]Role, error) {
	var rows []daomodel.AgentRole
	if err := r.db.WithContext(ctx).Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Role, 0, len(rows))
	for index := range rows {
		out = append(out, toRole(&rows[index]))
	}
	return out, nil
}

func (r *MySQLRepository) Create(ctx context.Context, input CreateInput, createdBy string) (Role, error) {
	row := &daomodel.AgentRole{
		RoleCode: input.RoleCode, RoleNameEN: input.RoleNameEN, RoleNameZH: input.RoleNameZH,
		Description: input.Description, SystemPrompt: input.SystemPrompt, AvatarURL: input.AvatarKey,
		IsBuiltin: 0, Status: input.Status, SortOrder: input.SortOrder, Version: 1,
		CreatedBy: createdBy,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return Role{}, ErrConflict
		}
		return Role{}, err
	}
	return toRole(row), nil
}

func toRole(row *daomodel.AgentRole) Role {
	return Role{
		ID: row.ID, RoleCode: row.RoleCode, RoleNameEN: row.RoleNameEN, RoleNameZH: row.RoleNameZH,
		Description: row.Description, SystemPrompt: row.SystemPrompt, AvatarURL: row.AvatarURL,
		IsBuiltin: row.IsBuiltin == 1, Status: row.Status, SortOrder: row.SortOrder,
		Version: row.Version, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/agentrole -count=1`  
Expected: PASS

---

### Task 3: Handler、路由与 CP 装配

**Files:**
- Create: `packages/backend/internal/handler/role.go`
- Test: `packages/backend/internal/handler/role_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`
- Create: `packages/backend/internal/router/roles_test.go`
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Consumes: `agentrole.Service`、`account.Presigner`
- Produces: `handler.RoleService`、`handler.RoleHandler`、`GET/POST /cp/api/v1/admin/roles`、头像接口

- [ ] **Step 1: 写失败测试**

`handler/role_test.go`：

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/agentrole"
)

type fakeRoleService struct {
	roles  []agentrole.Role
	role   agentrole.Role
	input  agentrole.CreateInput
	by     string
	err    error
	key    string
}

func (f *fakeRoleService) List(context.Context) ([]agentrole.Role, error) {
	return f.roles, f.err
}

func (f *fakeRoleService) Create(_ context.Context, input agentrole.CreateInput, by string) (agentrole.Role, error) {
	f.input = input
	f.by = by
	return f.role, f.err
}

func (f *fakeRoleService) PresignAvatar(context.Context, string, string, int64) (agentrole.Presign, error) {
	return agentrole.Presign{UploadURL: "https://upload", Key: "roles/avatar/x.png", Headers: map[string]string{"Content-Type": "image/png"}}, f.err
}

func (f *fakeRoleService) UploadAvatar(context.Context, string, int64, interface{ Read([]byte) (int, error) }) (string, error) {
	return f.key, f.err
}

func TestRoleList(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{roles: []agentrole.Role{{ID: 1, RoleCode: "alex", RoleNameZH: "全栈工程师"}}})
	e := echo.New()
	e.GET("/cp/api/v1/admin/roles", withModelSession(h.List))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/roles", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Roles []agentrole.Role `json:"roles"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Roles) != 1 {
		t.Fatalf("roles = %+v", env.Result.Roles)
	}
}

func TestRoleCreateMapsErrors(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{agentrole.ErrInvalid, http.StatusBadRequest},
		{agentrole.ErrConflict, http.StatusConflict},
		{agentrole.ErrStorageUnavailable, http.StatusServiceUnavailable},
		{agentrole.ErrAvatarNotFound, http.StatusBadRequest},
	}
	for _, tc := range cases {
		h := NewRoleHandler(&fakeRoleService{err: tc.err})
		e := echo.New()
		e.POST("/cp/api/v1/admin/roles", withModelSession(h.Create))
		body := `{"roleCode":"frank","roleNameEn":"Frank","roleNameZh":"测试","systemPrompt":"x"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/roles", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Fatalf("err %v status = %d, want %d", tc.err, rec.Code, tc.code)
		}
	}
}

func TestRoleCreatePassesInput(t *testing.T) {
	service := &fakeRoleService{role: agentrole.Role{ID: 2, RoleCode: "frank"}}
	h := NewRoleHandler(service)
	e := echo.New()
	e.POST("/cp/api/v1/admin/roles", withModelSession(h.Create))
	body := `{"roleCode":"frank","roleNameEn":"Frank","roleNameZh":"测试","description":"d","systemPrompt":"p","avatarKey":"roles/avatar/x.png","sortOrder":60,"status":1}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/roles", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.input.RoleCode != "frank" || service.input.AvatarKey != "roles/avatar/x.png" || service.by != "admin" {
		t.Fatalf("input = %+v by = %q", service.input, service.by)
	}
}

var _ = errors.Is
```

> 实现时 `UploadAvatar` 参数使用标准 `io.Reader`，测试里的 fake 方法也使用 `io.Reader`。

`router/roles_test.go`：

```go
package router

import (
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestControlServesRoleAdminRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/roles", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("ops status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/roles", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -run 'TestRole|TestControlServesRole' -count=1`  
Expected: FAIL，`NewRoleHandler` 未定义

- [ ] **Step 3: 实现 handler**

`handler/role.go`：

```go
package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/response"
)

type RoleService interface {
	List(ctx context.Context) ([]agentrole.Role, error)
	Create(ctx context.Context, input agentrole.CreateInput, createdBy string) (agentrole.Role, error)
	PresignAvatar(ctx context.Context, filename, contentType string, size int64) (agentrole.Presign, error)
	UploadAvatar(ctx context.Context, contentType string, size int64, body io.Reader) (string, error)
}

type RoleHandler struct {
	Service RoleService
}

func NewRoleHandler(service RoleService) *RoleHandler {
	return &RoleHandler{Service: service}
}

type roleRequest struct {
	RoleCode     string `json:"roleCode"`
	RoleNameEN   string `json:"roleNameEn"`
	RoleNameZH   string `json:"roleNameZh"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	AvatarKey    string `json:"avatarKey"`
	SortOrder    int    `json:"sortOrder"`
	Status       *int8  `json:"status"`
}

type roleAvatarPresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

func (h *RoleHandler) List(c echo.Context) error {
	roles, err := h.Service.List(c.Request().Context())
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, map[string]any{"roles": roles})
}

func (h *RoleHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req roleRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	role, err := h.Service.Create(c.Request().Context(), agentrole.CreateInput{
		RoleCode: req.RoleCode, RoleNameEN: req.RoleNameEN, RoleNameZH: req.RoleNameZH,
		Description: req.Description, SystemPrompt: req.SystemPrompt, AvatarKey: req.AvatarKey,
		Status: status, SortOrder: req.SortOrder,
	}, session.Username)
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, role)
}

func (h *RoleHandler) PresignAvatar(c echo.Context) error {
	var req roleAvatarPresignRequest
	if err := c.Bind(&req); err != nil || req.Filename == "" || req.ContentType == "" || req.Size <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	result, err := h.Service.PresignAvatar(c.Request().Context(), req.Filename, req.ContentType, req.Size)
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, result)
}

func (h *RoleHandler) UploadAvatar(c echo.Context) error {
	contentType := strings.TrimSpace(c.Request().Header.Get(echo.HeaderContentType))
	size := c.Request().ContentLength
	if contentType == "" || size <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid avatar")
	}
	key, err := h.Service.UploadAvatar(c.Request().Context(), contentType, size, io.LimitReader(c.Request().Body, size))
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, map[string]any{"key": key})
}

func roleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, agentrole.ErrInvalid), errors.Is(err, agentrole.ErrAvatarInvalid), errors.Is(err, agentrole.ErrAvatarNotFound):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid role")
	case errors.Is(err, agentrole.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "role code already exists")
	case errors.Is(err, agentrole.ErrStorageUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "role avatar storage not configured")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "role operation failed")
	}
}
```

- [ ] **Step 4: 注册路由与装配**

`router/control.go` 签名末尾增加 `roles *handler.RoleHandler`，并注册：

```go
	if roles != nil {
		adminProtected.GET("/roles", roles.List)
		adminProtected.POST("/roles", roles.Create)
		adminProtected.POST("/roles/avatar/presign", roles.PresignAvatar)
		adminProtected.POST("/roles/avatar/upload", roles.UploadAvatar)
	}
```

`router/helpers_test.go` 增加 import `vibebiz/internal/agentrole` 与：

```go
func newRoleHandler(t *testing.T) *handler.RoleHandler {
	t.Helper()
	return handler.NewRoleHandler(agentrole.NewService(agentrole.NewMemoryRepository(), nil, 0))
}
```

`RegisterControl` 调用末尾追加 `newRoleHandler(t)`；`span_test.go` 同步。

`cmd/cp/main.go` 在 `modelHandler` 后增加：

```go
	roleService := agentrole.NewService(agentrole.NewMySQLRepository(db), avatarPresigner, cfg.S3PresignExpiry())
	roleHandler := handler.NewRoleHandler(roleService)
```

`RegisterControl` 调用末尾追加 `roleHandler`。需要 import `vibebiz/internal/agentrole`。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/agentrole ./internal/handler ./internal/router -count=1`  
Expected: PASS

---

### Task 4: ops-web 类型、API 与角色表单校验

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/utils/roleForm.ts`
- Test: `packages/ops-web/src/utils/roleForm.test.ts`
- Test: `packages/ops-web/src/api/roles.test.ts`

**Interfaces:**
- Produces: `AdminAgentRole`、`RoleInput`、`RoleAvatarPresign`、`getRoles()`、`createRole(input)`、`presignRoleAvatar(input)`、`uploadRoleAvatar(file)`、`buildRoleInput(form)`

- [ ] **Step 1: 写失败测试**

`utils/roleForm.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { buildRoleInput, type RoleFormState } from './roleForm'

function form(overrides: Partial<RoleFormState> = {}): RoleFormState {
  return {
    roleCode: 'frank',
    roleNameEn: 'Frank',
    roleNameZh: '测试工程师',
    description: '负责测试策略与用例。',
    systemPrompt: '你是 Frank，测试工程师。',
    avatarKey: '',
    sortOrder: 60,
    status: 1,
    ...overrides,
  }
}

describe('buildRoleInput', () => {
  it('builds a role input', () => {
    const result = buildRoleInput(form())
    expect('input' in result && result.input.roleCode).toBe('frank')
  })

  it('rejects missing required fields', () => {
    expect(buildRoleInput(form({ roleCode: '' }))).toEqual({ error: '请填写角色编码' })
    expect(buildRoleInput(form({ systemPrompt: '' }))).toEqual({ error: '请填写 System Prompt' })
  })

  it('rejects invalid role code', () => {
    expect('error' in buildRoleInput(form({ roleCode: 'Frank' }))).toBe(true)
    expect('error' in buildRoleInput(form({ roleCode: 'bad code' }))).toBe(true)
  })

  it('rejects overlong names and description', () => {
    expect('error' in buildRoleInput(form({ roleNameZh: 'a'.repeat(65) }))).toBe(true)
    expect('error' in buildRoleInput(form({ description: 'a'.repeat(1001) }))).toBe(true)
  })

  it('normalizes sort order and status', () => {
    const result = buildRoleInput(form({ sortOrder: Number.NaN, status: 1 }))
    expect('input' in result && result.input.sortOrder).toBe(0)
  })
})
```

`api/roles.test.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'

import { getRoles } from './admin'
import { http } from './client'

describe('role admin api', () => {
  it('unwraps the roles envelope', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { roles: [{ id: 1, roleCode: 'alex' }] } },
    })
    await expect(getRoles()).resolves.toEqual([{ id: 1, roleCode: 'alex' }])
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/ops-web && npm run test -- src/utils/roleForm.test.ts src/api/roles.test.ts`  
Expected: FAIL，模块不存在

- [ ] **Step 3: 实现类型与 API**

`types/index.ts` 追加：

```ts
export interface AdminAgentRole {
  id: number
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  systemPrompt: string
  avatarUrl: string
  isBuiltin: boolean
  status: number
  sortOrder: number
  version: number
  createdBy: string
  createdAt: string
  updatedAt: string
}

export interface RoleInput {
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  systemPrompt: string
  avatarKey?: string
  sortOrder: number
  status: number
}

export interface RoleAvatarPresign {
  uploadUrl: string
  key: string
  headers: Record<string, string>
}
```

`api/admin.ts` 追加：

```ts
export async function getRoles(): Promise<AdminAgentRole[]> {
  const result = await request<AdminAgentRole[] | { roles: AdminAgentRole[] }>({
    method: 'get',
    url: '/roles',
  })
  return toList(result, 'roles')
}

export async function createRole(input: RoleInput): Promise<AdminAgentRole> {
  return request<AdminAgentRole>({ method: 'post', url: '/roles', data: input })
}

export async function presignRoleAvatar(input: {
  filename: string
  contentType: string
  size: number
}): Promise<RoleAvatarPresign> {
  return request<RoleAvatarPresign>({ method: 'post', url: '/roles/avatar/presign', data: input })
}

export async function uploadRoleAvatar(file: File): Promise<string> {
  const result = await request<{ key: string }>({
    method: 'post',
    url: '/roles/avatar/upload',
    data: file,
    headers: { 'Content-Type': file.type },
  })
  return result.key
}
```

更新 import 类型。

- [ ] **Step 4: 实现表单校验**

`utils/roleForm.ts`：

```ts
import type { RoleInput } from '@/types'

export interface RoleFormState {
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  systemPrompt: string
  avatarKey: string
  sortOrder: number
  status: number
}

const roleCodePattern = /^[a-z][a-z0-9_-]{0,31}$/

export function buildRoleInput(
  form: RoleFormState,
): { input: RoleInput } | { error: string } {
  const roleCode = form.roleCode.trim()
  const roleNameEn = form.roleNameEn.trim()
  const roleNameZh = form.roleNameZh.trim()
  const description = form.description.trim()
  const systemPrompt = form.systemPrompt.trim()

  if (!roleCode) return { error: '请填写角色编码' }
  if (!roleCodePattern.test(roleCode)) {
    return { error: '角色编码必须以小写字母开头，只能包含小写字母、数字、下划线和中划线' }
  }
  if (!roleNameEn) return { error: '请填写英文名' }
  if (!roleNameZh) return { error: '请填写中文名' }
  if (roleNameEn.length > 64 || roleNameZh.length > 64) return { error: '角色名称不能超过 64 个字符' }
  if (description.length > 1000) return { error: '角色描述不能超过 1000 个字符' }
  if (!systemPrompt) return { error: '请填写 System Prompt' }
  if (systemPrompt.length > 100000) return { error: 'System Prompt 过长' }

  const sortOrder = Number.isFinite(form.sortOrder) ? Math.trunc(form.sortOrder) : 0
  const status = form.status === 0 ? 0 : 1

  return {
    input: {
      roleCode,
      roleNameEn,
      roleNameZh,
      description,
      systemPrompt,
      avatarKey: form.avatarKey || undefined,
      sortOrder,
      status,
    },
  }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/ops-web && npm run test -- src/utils/roleForm.test.ts src/api/roles.test.ts`  
Expected: PASS

---

### Task 5: RolesView、路由与导航

**Files:**
- Create: `packages/ops-web/src/views/RolesView.vue`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: `getRoles`、`createRole`、`presignRoleAvatar`、`uploadRoleAvatar`、`buildRoleInput`

- [ ] **Step 1: 更新路由与导航测试**

`navigation.test.ts` 改为 10 个模块，在 `/models` 后加入 `/roles`、`角色管理`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/ops-web && npm run test -- src/router/navigation.test.ts`  
Expected: FAIL

- [ ] **Step 3: 更新路由与导航**

`routes.ts` 增加：

```ts
import RolesView from '@/views/RolesView.vue'
...
  { path: 'roles', name: 'roles', component: RolesView, meta: { title: '角色管理' } },
```

放在 models 后。

`navigation.ts` 在 models 后插入：

```ts
  { name: 'roles', label: '角色管理', path: '/roles', icon: 'pi pi-user' },
```

- [ ] **Step 4: 创建 RolesView**

实现列表、Dialog 表单、头像上传：

```vue
<script setup lang="ts">
import { ref } from 'vue'
import axios from 'axios'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import { createRole, getRoles, presignRoleAvatar, uploadRoleAvatar } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'
import { buildRoleInput, type RoleFormState } from '@/utils/roleForm'

const { rows, loading, error, load } = useAdminList(getRoles)

const columns: TableColumn[] = [
  { key: 'avatarUrl', label: '头像', width: '70px' },
  { key: 'roleNameZh', label: '中文名', width: '130px' },
  { key: 'roleNameEn', label: '英文名', width: '110px' },
  { key: 'roleCode', label: '编码', width: '110px' },
  { key: 'isBuiltin', label: '来源', width: '80px', format: (value) => (value ? '内置' : '自定义') },
  { key: 'status', label: '状态', width: '80px', format: (value) => (Number(value) === 1 ? '启用' : '禁用') },
  { key: 'sortOrder', label: '排序', width: '70px' },
  { key: 'createdAt', label: '创建时间', width: '160px', format: (value) => formatDateTime(value) },
]

const dialogVisible = ref(false)
const saving = ref(false)
const uploading = ref(false)
const actionError = ref('')
const notice = ref('')
const avatarPreview = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)

function emptyForm(): RoleFormState {
  return {
    roleCode: '',
    roleNameEn: '',
    roleNameZh: '',
    description: '',
    systemPrompt: '',
    avatarKey: '',
    sortOrder: 0,
    status: 1,
  }
}

const form = ref<RoleFormState>(emptyForm())

function openCreate() {
  form.value = emptyForm()
  avatarPreview.value = ''
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

async function chooseAvatar(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) return
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    actionError.value = '头像只支持 JPG、PNG、WebP'
    return
  }
  if (file.size <= 0 || file.size > 5 * 1024 * 1024) {
    actionError.value = '头像不能超过 5MB'
    return
  }
  uploading.value = true
  actionError.value = ''
  try {
    let key = ''
    try {
      const presign = await presignRoleAvatar({
        filename: file.name,
        contentType: file.type,
        size: file.size,
      })
      await axios.put(presign.uploadUrl, file, { headers: presign.headers })
      key = presign.key
    } catch {
      key = await uploadRoleAvatar(file)
    }
    form.value.avatarKey = key
    avatarPreview.value = URL.createObjectURL(file)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '头像上传失败'
  } finally {
    uploading.value = false
  }
}

function updateSortOrder(value: number | null) {
  form.value.sortOrder = Number(value ?? 0)
}

async function submit() {
  const result = buildRoleInput(form.value)
  if ('error' in result) {
    actionError.value = result.error
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    await createRole(result.input)
    notice.value = '角色已创建'
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

function initial(row: Record<string, unknown>): string {
  return String(row.roleNameZh ?? row.roleNameEn ?? '?').slice(0, 1).toUpperCase()
}
</script>
```

模板包含 DataTable、头像 slot、Dialog 表单、头像按钮。完整模板：

```vue
<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>角色管理</h1>
        <p class="page-desc">内置与自定义 Agent 角色、提示词和头像。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ rows.length }} 条</span>
        <Button label="新增角色" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无角色"
      @retry="load"
    >
      <template #cell-avatarUrl="{ row }">
        <img v-if="row.avatarUrl" class="role-avatar" :src="String(row.avatarUrl)" alt="" />
        <span v-else class="role-avatar role-avatar-fallback">{{ initial(row) }}</span>
      </template>
    </DataTable>

    <Dialog
      v-model:visible="dialogVisible"
      header="新增角色"
      modal
      :style="{ width: '680px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <div class="avatar-field">
          <img v-if="avatarPreview" class="role-avatar role-avatar-large" :src="avatarPreview" alt="" />
          <span v-else class="role-avatar role-avatar-large role-avatar-fallback">
            {{ form.roleNameZh.slice(0, 1).toUpperCase() || '?' }}
          </span>
          <input ref="avatarInput" class="hidden-file" type="file" accept="image/jpeg,image/png,image/webp" @change="chooseAvatar" />
          <Button
            label="上传头像"
            severity="secondary"
            :loading="uploading"
            @click="avatarInput?.click()"
          />
        </div>
        <label class="field">
          <span>角色编码</span>
          <InputText v-model="form.roleCode" maxlength="32" placeholder="例如 frank" />
        </label>
        <label class="field">
          <span>中文名</span>
          <InputText v-model="form.roleNameZh" maxlength="64" />
        </label>
        <label class="field">
          <span>英文名</span>
          <InputText v-model="form.roleNameEn" maxlength="64" />
        </label>
        <label class="field">
          <span>描述</span>
          <Textarea v-model="form.description" rows="2" maxlength="1000" />
        </label>
        <label class="field">
          <span>System Prompt</span>
          <Textarea v-model="form.systemPrompt" rows="6" />
        </label>
        <label class="field">
          <span>排序</span>
          <InputNumber :model-value="form.sortOrder" :min="0" :max="100000" @update:model-value="updateSortOrder" />
        </label>
        <label class="field">
          <span>状态</span>
          <Select
            v-model="form.status"
            :options="[
              { label: '启用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
            option-label="label"
            option-value="value"
          />
        </label>
        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>

      <template #footer>
        <Button label="取消" severity="secondary" text @click="dialogVisible = false" />
        <Button label="保存" :loading="saving" @click="submit" />
      </template>
    </Dialog>
  </div>
</template>
```

样式：

```vue
<style scoped>
.inline-notice {
  margin-bottom: 16px;
  color: var(--blue);
  font-size: 13px;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--text);
  font-size: 13px;
}

.field span {
  color: var(--muted);
  font-size: 12px;
}

.avatar-field {
  display: flex;
  align-items: center;
  gap: 12px;
}

.role-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
}

.role-avatar-large {
  width: 56px;
  height: 56px;
}

.role-avatar-fallback {
  display: inline-grid;
  place-items: center;
  background: var(--soft);
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
}

.hidden-file {
  display: none;
}
</style>
```

- [ ] **Step 5: 运行测试与类型检查**

Run: `cd packages/ops-web && npm run test -- src/router/navigation.test.ts src/utils/roleForm.test.ts src/api/roles.test.ts && npx vue-tsc --noEmit`  
Expected: PASS

---

### Task 6: 全量验证

- [ ] **Step 1: 运行 Go 测试**

Run: `make -C packages/backend test`  
Expected: PASS

- [ ] **Step 2: 运行 ops-web 测试**

Run: `make -C packages/ops-web test`  
Expected: PASS

- [ ] **Step 3: 运行质量门禁**

Run: `make quality`  
Expected: PASS

- [ ] **Step 4: 运行完整校验**

Run: `make validate`  
Expected: PASS

- [ ] **Step 5: 更新检查点**

Run: `make checkpoint`  
Expected: `.ai` 检查点更新成功
