# Ops 模型录入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 ops-web 增加模型列表与新增表单，通过 CP admin 接口把平台模型加密写入 `llm_model`。

**Architecture:** 新增 `internal/secret` 做 AES-GCM，新增 `internal/llmmodel` 承载模型视图、校验、加密和仓储；CP 注册 admin 路由，ops-web 增加模型管理页。

**Tech Stack:** Go / Echo / GORM / MySQL / crypto/aes；Vue 3 / PrimeVue / Vitest。

**Spec:** `docs/superpowers/specs/2026-10-29-ops-llm-model-entry-design.md`

## Global Constraints

- 只写平台模型：`scope=platform`、`user_id=0`。
- 类型：`LLM` / `EMBEDDING` / `RERANK` / `OCR` / `ASR` / `TTS` / `OTHER`。
- 默认模型按 `model_type` 唯一。
- API Key 只存密文，接口永不回传。
- 不做编辑、删除、启停。
- 不新增数据库迁移；`model_type` 为 `varchar(32)`。
- 未获用户明确要求不执行 `git commit`。
- 每个任务以测试通过结束。

---

### Task 1: AES-GCM 加密包

**Files:**
- Create: `packages/backend/internal/secret/aesgcm.go`
- Test: `packages/backend/internal/secret/aesgcm_test.go`

**Interfaces:**
- Produces: `secret.NewCipher(encoded string) (*secret.Cipher, error)`、`(*Cipher).Encrypt(plaintext string) (string, error)`、`(*Cipher).Decrypt(encoded string) (string, error)`

- [ ] **Step 1: 写失败测试**

```go
package secret

import (
	"encoding/base64"
	"strings"
	"testing"
)

func testKey() string {
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	cipher, err := NewCipher(testKey())
	if err != nil {
		t.Fatal(err)
	}
	first, err := cipher.Encrypt("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	second, err := cipher.Encrypt("sk-test")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("nonce should make ciphertext differ")
	}
	plain, err := cipher.Decrypt(first)
	if err != nil {
		t.Fatal(err)
	}
	if plain != "sk-test" {
		t.Fatalf("plain = %q", plain)
	}
}

func TestNewCipherRejectsInvalidKey(t *testing.T) {
	if _, err := NewCipher(""); err == nil {
		t.Fatal("empty key should fail")
	}
	if _, err := NewCipher(base64.StdEncoding.EncodeToString([]byte("short"))); err == nil {
		t.Fatal("short key should fail")
	}
}

func TestDecryptRejectsTamperedCiphertext(t *testing.T) {
	cipher, _ := NewCipher(testKey())
	encrypted, _ := cipher.Encrypt("sk-test")
	if _, err := cipher.Decrypt(encrypted + strings.Repeat("A", 4)); err == nil {
		t.Fatal("tampered ciphertext should fail")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/secret -count=1`  
Expected: FAIL，包不存在

- [ ] **Step 3: 实现**

```go
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

var ErrInvalidKey = errors.New("invalid model secret key")

type Cipher struct {
	aead cipher.AEAD
}

func NewCipher(encoded string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil || len(key) != 32 {
		return nil, ErrInvalidKey
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrInvalidKey
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, ErrInvalidKey
	}
	return &Cipher{aead: aead}, nil
}

func (c *Cipher) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

func (c *Cipher) Decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	nonceSize := c.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", ErrInvalidKey
	}
	plain, err := c.aead.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/secret -count=1`  
Expected: PASS

---

### Task 2: llmmodel 类型、校验与内存仓储

**Files:**
- Create: `packages/backend/internal/llmmodel/service.go`
- Create: `packages/backend/internal/llmmodel/memory.go`
- Test: `packages/backend/internal/llmmodel/service_test.go`

**Interfaces:**
- Produces: `llmmodel.Model`、`llmmodel.CreateInput`、`llmmodel.Repository`、`llmmodel.Service`、`llmmodel.NewService(repo, cipher)`、`llmmodel.NewMemoryRepository()`
- Errors: `ErrInvalid`、`ErrConflict`、`ErrKeyMissing`

- [ ] **Step 1: 写失败测试**

```go
package llmmodel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"vibebiz/internal/secret"
)

func testCipher(t *testing.T) *secret.Cipher {
	t.Helper()
	key := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	c, err := secret.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func validInput() CreateInput {
	return CreateInput{
		ModelName:    "DeepSeek Chat",
		ModelCode:    "deepseek-chat",
		APIURL:       "https://api.deepseek.com/anthropic",
		APIKey:       "sk-test",
		ProviderCode: "DeepSeek",
		ModelType:    "LLM",
		MaxTokens:    4096,
		Status:       1,
	}
}

func TestCreateEncryptsKeyAndStoresModel(t *testing.T) {
	repo := NewMemoryRepository()
	service := NewService(repo, testCipher(t))

	model, err := service.Create(context.Background(), validInput(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if model.ModelCode != "deepseek-chat" || !model.HasAPIKey {
		t.Fatalf("model = %+v", model)
	}
	if model.APIKey != "" {
		t.Fatal("view must not carry plaintext key")
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	service := NewService(NewMemoryRepository(), testCipher(t))
	cases := []CreateInput{
		{},
		{ModelName: "x", ModelCode: "bad code", APIURL: "https://x", APIKey: "k", ModelType: "LLM"},
		{ModelName: "x", ModelCode: "x", APIURL: "ftp://x", APIKey: "k", ModelType: "LLM"},
		{ModelName: "x", ModelCode: "x", APIURL: "https://x", APIKey: "k", ModelType: "VIDEO"},
		{ModelName: "x", ModelCode: "x", APIURL: "https://x", APIKey: "k", ModelType: "EMBEDDING", ExtraParams: json.RawMessage(`[]`)},
	}
	for index, input := range cases {
		if _, err := service.Create(context.Background(), input, "admin"); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d err = %v, want ErrInvalid", index, err)
		}
	}
}

func TestCreateWithoutCipherReturnsKeyMissing(t *testing.T) {
	service := NewService(NewMemoryRepository(), nil)
	if _, err := service.Create(context.Background(), validInput(), "admin"); !errors.Is(err, ErrKeyMissing) {
		t.Fatalf("err = %v", err)
	}
}

func TestDefaultIsUniquePerType(t *testing.T) {
	service := NewService(NewMemoryRepository(), testCipher(t))
	first := validInput()
	first.IsDefault = true
	if _, err := service.Create(context.Background(), first, "admin"); err != nil {
		t.Fatal(err)
	}
	second := validInput()
	second.ModelCode = "deepseek-reasoner"
	second.IsDefault = true
	if _, err := service.Create(context.Background(), second, "admin"); err != nil {
		t.Fatal(err)
	}
	items, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d", len(items))
	}
	defaults := map[string]bool{}
	for _, item := range items {
		defaults[item.ModelCode] = item.IsDefault
	}
	if !defaults["deepseek-reasoner"] || defaults["deepseek-chat"] {
		t.Fatalf("default flags = %+v", items)
	}
}

func TestDuplicateCodeReturnsConflict(t *testing.T) {
	service := NewService(NewMemoryRepository(), testCipher(t))
	if _, err := service.Create(context.Background(), validInput(), "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), validInput(), "admin"); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/llmmodel -count=1`  
Expected: FAIL，包不存在

- [ ] **Step 3: 实现 service 与 memory**

`service.go`：

```go
package llmmodel

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"vibebiz/internal/secret"
)

var (
	ErrInvalid   = errors.New("invalid model")
	ErrConflict  = errors.New("model code already exists")
	ErrKeyMissing = errors.New("model secret key not configured")
)

var codePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

var allowedTypes = map[string]struct{}{
	"LLM": {}, "EMBEDDING": {}, "RERANK": {}, "OCR": {}, "ASR": {}, "TTS": {}, "OTHER": {},
}

var allowedProviders = map[string]struct{}{
	"OpenAI": {}, "DeepSeek": {}, "Anthropic": {}, "Qwen": {}, "Other": {},
}

type Model struct {
	ID           int64           `json:"id"`
	ModelName    string          `json:"modelName"`
	ModelCode    string          `json:"modelCode"`
	APIURL       string          `json:"apiUrl"`
	ProviderCode string          `json:"providerCode"`
	ModelType    string          `json:"modelType"`
	MaxTokens    int             `json:"maxTokens"`
	IsDefault    bool            `json:"isDefault"`
	Status       int8            `json:"status"`
	HasAPIKey    bool            `json:"hasApiKey"`
	ExtraParams  json.RawMessage `json:"extraParams,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

type CreateInput struct {
	ModelName    string
	ModelCode    string
	APIURL       string
	APIKey       string
	ProviderCode string
	ModelType    string
	MaxTokens    int
	IsDefault    bool
	Status       int8
	ExtraParams  json.RawMessage
}

type Repository interface {
	List(ctx context.Context) ([]Model, error)
	Create(ctx context.Context, input CreateInput, apiKeyCipher, createdBy string) (Model, error)
}

type Service struct {
	Repo   Repository
	Cipher *secret.Cipher
}

func NewService(repo Repository, cipher *secret.Cipher) *Service {
	return &Service{Repo: repo, Cipher: cipher}
}

func (s *Service) List(ctx context.Context) ([]Model, error) {
	return s.Repo.List(ctx)
}

func (s *Service) Create(ctx context.Context, input CreateInput, createdBy string) (Model, error) {
	normalized, err := normalize(input)
	if err != nil {
		return Model{}, err
	}
	if s.Cipher == nil {
		return Model{}, ErrKeyMissing
	}
	encrypted, err := s.Cipher.Encrypt(normalized.APIKey)
	if err != nil {
		return Model{}, err
	}
	return s.Repo.Create(ctx, normalized, encrypted, createdBy)
}

func normalize(input CreateInput) (CreateInput, error) {
	input.ModelName = strings.TrimSpace(input.ModelName)
	input.ModelCode = strings.TrimSpace(input.ModelCode)
	input.APIURL = strings.TrimSpace(input.APIURL)
	input.APIKey = strings.TrimSpace(input.APIKey)
	input.ProviderCode = strings.TrimSpace(input.ProviderCode)
	input.ModelType = strings.ToUpper(strings.TrimSpace(input.ModelType))

	if input.ModelName == "" || input.ModelCode == "" || input.APIURL == "" || input.APIKey == "" {
		return CreateInput{}, ErrInvalid
	}
	if !codePattern.MatchString(input.ModelCode) {
		return CreateInput{}, ErrInvalid
	}
	if !strings.HasPrefix(input.APIURL, "http://") && !strings.HasPrefix(input.APIURL, "https://") {
		return CreateInput{}, ErrInvalid
	}
	if _, ok := allowedTypes[input.ModelType]; !ok {
		return CreateInput{}, ErrInvalid
	}
	if input.ProviderCode == "" {
		input.ProviderCode = "Other"
	}
	if _, ok := allowedProviders[input.ProviderCode]; !ok {
		return CreateInput{}, ErrInvalid
	}
	if input.Status != 0 && input.Status != 1 {
		return CreateInput{}, ErrInvalid
	}
	if input.ModelType == "LLM" {
		if input.MaxTokens <= 0 {
			input.MaxTokens = 4096
		}
	} else {
		input.MaxTokens = 0
	}
	if len(input.ExtraParams) > 0 {
		if !json.Valid(input.ExtraParams) {
			return CreateInput{}, ErrInvalid
		}
		var object map[string]any
		if err := json.Unmarshal(input.ExtraParams, &object); err != nil {
			return CreateInput{}, ErrInvalid
		}
	}
	return input, nil
}
```

`memory.go`：

```go
package llmmodel

import (
	"context"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu    sync.Mutex
	next  int64
	items []Model
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (r *MemoryRepository) List(context.Context) ([]Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Model, len(r.items))
	copy(out, r.items)
	return out, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput, apiKeyCipher, _ string) (Model, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.ModelCode == input.ModelCode {
			return Model{}, ErrConflict
		}
	}
	if input.IsDefault {
		for index := range r.items {
			if r.items[index].ModelType == input.ModelType {
				r.items[index].IsDefault = false
			}
		}
	}
	r.next++
	now := time.Now().UTC()
	item := Model{
		ID: r.next, ModelName: input.ModelName, ModelCode: input.ModelCode,
		APIURL: input.APIURL, ProviderCode: input.ProviderCode, ModelType: input.ModelType,
		MaxTokens: input.MaxTokens, IsDefault: input.IsDefault, Status: input.Status,
		HasAPIKey: apiKeyCipher != "", ExtraParams: input.ExtraParams, CreatedAt: now, UpdatedAt: now,
	}
	r.items = append([]Model{item}, r.items...)
	return item, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/llmmodel -count=1`  
Expected: PASS

---

### Task 3: MySQL 仓储

**Files:**
- Create: `packages/backend/internal/llmmodel/mysql.go`
- Test: `packages/backend/internal/llmmodel/mysql_test.go`

**Interfaces:**
- Produces: `llmmodel.NewMySQLRepository(db *gorm.DB) *MySQLRepository`

- [ ] **Step 1: 写失败测试**

使用 `sqlmock` 验证列表 SQL 和默认清除事务。测试代码：

```go
package llmmodel

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

func TestMySQLListModels(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `llm_model`").
		WithArgs("platform", 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "model_name", "model_code", "api_url", "api_key_cipher", "provider_code", "model_type", "max_tokens", "is_default", "status", "created_at", "updated_at"}).
			AddRow(1, "DeepSeek", "deepseek-chat", "https://x", "cipher", "DeepSeek", "LLM", 4096, 1, 1, now, now))

	repo := NewMySQLRepository(db)
	items, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].IsDefault || !items[0].HasAPIKey {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/llmmodel -run TestMySQLListModels -count=1`  
Expected: FAIL，`NewMySQLRepository` 未定义

- [ ] **Step 3: 实现**

```go
package llmmodel

import (
	"context"
	"errors"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
)

const (
	scopePlatform = "platform"
	platformUser  = 0
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) List(ctx context.Context) ([]Model, error) {
	var rows []daomodel.LLMModel
	err := r.db.WithContext(ctx).
		Where("scope = ? AND user_id = ?", scopePlatform, platformUser).
		Order("created_at DESC").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]Model, 0, len(rows))
	for index := range rows {
		out = append(out, toModel(&rows[index]))
	}
	return out, nil
}

func (r *MySQLRepository) Create(ctx context.Context, input CreateInput, apiKeyCipher, createdBy string) (Model, error) {
	var result Model
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if input.IsDefault {
			if err := tx.Model(&daomodel.LLMModel{}).
				Where("scope = ? AND user_id = ? AND model_type = ? AND is_default = ?", scopePlatform, platformUser, input.ModelType, 1).
				Update("is_default", 0).Error; err != nil {
				return err
			}
		}
		row := &daomodel.LLMModel{
			Scope: scopePlatform, UserID: platformUser,
			ModelName: input.ModelName, ModelCode: input.ModelCode, APIURL: input.APIURL,
			APIKeyCipher: apiKeyCipher, ProviderCode: input.ProviderCode, ModelType: input.ModelType,
			MaxTokens: input.MaxTokens, IsDefault: boolToInt8(input.IsDefault), Status: input.Status,
			ExtraParams: input.ExtraParams, CreatedBy: createdBy,
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		result = toModel(row)
		return nil
	})
	if err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return Model{}, ErrConflict
		}
		return Model{}, err
	}
	return result, nil
}

func toModel(row *daomodel.LLMModel) Model {
	return Model{
		ID: row.ID, ModelName: row.ModelName, ModelCode: row.ModelCode, APIURL: row.APIURL,
		ProviderCode: row.ProviderCode, ModelType: row.ModelType, MaxTokens: row.MaxTokens,
		IsDefault: row.IsDefault == 1, Status: row.Status, HasAPIKey: row.APIKeyCipher != "",
		ExtraParams: row.ExtraParams, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func boolToInt8(value bool) int8 {
	if value {
		return 1
	}
	return 0
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/llmmodel -count=1`  
Expected: PASS

---

### Task 4: Handler、配置、路由与 CP 装配

**Files:**
- Create: `packages/backend/internal/handler/model.go`
- Test: `packages/backend/internal/handler/model_test.go`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/.env.example`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`
- Create: `packages/backend/internal/router/models_test.go`
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Consumes: `llmmodel.Service`、`secret.NewCipher`、`config.Config.ModelSecretKey`
- Produces: `handler.ModelService`、`handler.ModelHandler`、`GET/POST /cp/api/v1/admin/models`

- [ ] **Step 1: 写失败测试**

`handler/model_test.go`：

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

	"vibebiz/internal/auth"
	"vibebiz/internal/llmmodel"
)

func withModelSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("auth.session", &auth.Session{Username: "admin"})
		return next(c)
	}
}

type fakeModelService struct {
	models  []llmmodel.Model
	created llmmodel.Model
	input   llmmodel.CreateInput
	by      string
	err     error
}

func (f *fakeModelService) List(context.Context) ([]llmmodel.Model, error) {
	return f.models, f.err
}

func (f *fakeModelService) Create(_ context.Context, input llmmodel.CreateInput, by string) (llmmodel.Model, error) {
	f.input = input
	f.by = by
	return f.created, f.err
}

func TestModelList(t *testing.T) {
	h := NewModelHandler(&fakeModelService{models: []llmmodel.Model{{ID: 1, ModelName: "DeepSeek", ModelCode: "deepseek-chat", ModelType: "LLM"}}})
	e := echo.New()
	e.GET("/cp/api/v1/admin/models", withModelSession(h.List))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/models", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Models []llmmodel.Model `json:"models"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Models) != 1 {
		t.Fatalf("models = %+v", env.Result.Models)
	}
}

func TestModelCreateMapsErrors(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{llmmodel.ErrInvalid, http.StatusBadRequest},
		{llmmodel.ErrConflict, http.StatusConflict},
		{llmmodel.ErrKeyMissing, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		h := NewModelHandler(&fakeModelService{err: tc.err})
		e := echo.New()
		e.POST("/cp/api/v1/admin/models", withModelSession(h.Create))
		body := `{"modelName":"x","modelCode":"x","apiUrl":"https://x","apiKey":"k","modelType":"LLM"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/models", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Fatalf("err %v status = %d, want %d", tc.err, rec.Code, tc.code)
		}
	}
}

func TestModelCreatePassesInput(t *testing.T) {
	service := &fakeModelService{created: llmmodel.Model{ID: 2, ModelCode: "embed"}}
	h := NewModelHandler(service)
	e := echo.New()
	e.POST("/cp/api/v1/admin/models", withModelSession(h.Create))
	body := `{"modelName":"Embed","modelCode":"embed","apiUrl":"https://x","apiKey":"k","providerCode":"Other","modelType":"EMBEDDING","isDefault":true,"status":1,"extraParams":{"dimension":1024}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/models", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.input.ModelType != "EMBEDDING" || !service.input.IsDefault {
		t.Fatalf("input = %+v", service.input)
	}
	if !strings.Contains(string(service.input.ExtraParams), "1024") {
		t.Fatalf("extra = %s", service.input.ExtraParams)
	}
}
```

`router/models_test.go`：

```go
package router

import (
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestControlServesModelAdminRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/models", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("ops status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/models", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -run 'TestModel|TestControlServesModel' -count=1`  
Expected: FAIL，`NewModelHandler` 未定义

- [ ] **Step 3: 实现 handler**

`handler/model.go`：

```go
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/llmmodel"
	"vibebiz/internal/response"
)

type ModelService interface {
	List(ctx context.Context) ([]llmmodel.Model, error)
	Create(ctx context.Context, input llmmodel.CreateInput, createdBy string) (llmmodel.Model, error)
}

type ModelHandler struct {
	Service ModelService
}

func NewModelHandler(service ModelService) *ModelHandler {
	return &ModelHandler{Service: service}
}

type modelRequest struct {
	ModelName    string          `json:"modelName"`
	ModelCode    string          `json:"modelCode"`
	APIURL       string          `json:"apiUrl"`
	APIKey       string          `json:"apiKey"`
	ProviderCode string          `json:"providerCode"`
	ModelType    string          `json:"modelType"`
	MaxTokens    int             `json:"maxTokens"`
	IsDefault    bool            `json:"isDefault"`
	Status       *int8           `json:"status"`
	ExtraParams  json.RawMessage `json:"extraParams"`
}

func (h *ModelHandler) List(c echo.Context) error {
	models, err := h.Service.List(c.Request().Context())
	if err != nil {
		return modelError(c, err)
	}
	return response.OK(c, map[string]any{"models": models})
}

func (h *ModelHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req modelRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	model, err := h.Service.Create(c.Request().Context(), llmmodel.CreateInput{
		ModelName: req.ModelName, ModelCode: req.ModelCode, APIURL: req.APIURL,
		APIKey: req.APIKey, ProviderCode: req.ProviderCode, ModelType: req.ModelType,
		MaxTokens: req.MaxTokens, IsDefault: req.IsDefault, Status: status,
		ExtraParams: req.ExtraParams,
	}, session.Username)
	if err != nil {
		return modelError(c, err)
	}
	return response.OK(c, model)
}

func modelError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, llmmodel.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid model")
	case errors.Is(err, llmmodel.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "model code already exists")
	case errors.Is(err, llmmodel.ErrKeyMissing):
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "model secret key not configured")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "model operation failed")
	}
}
```

- [ ] **Step 4: 实现配置**

`config.go` 增加字段与读取：

```go
	ModelSecretKey string
...
		ModelSecretKey: getString("MODEL_SECRET_KEY", ""),
```

`config_test.go` 增加：

```go
func TestLoadModelSecretKey(t *testing.T) {
	t.Setenv("MODEL_SECRET_KEY", "abc")
	if cfg := Load("cp"); cfg.ModelSecretKey != "abc" {
		t.Fatalf("model secret key = %q", cfg.ModelSecretKey)
	}
}
```

`.env.example` 追加：

```text
# base64 编码的 32 字节密钥，用于加密 llm_model.api_key_cipher
MODEL_SECRET_KEY=
```

- [ ] **Step 5: 注册路由并装配 CP**

`router/control.go` 签名末尾增加 `models *handler.ModelHandler`，并注册：

```go
	if models != nil {
		adminProtected.GET("/models", models.List)
		adminProtected.POST("/models", models.Create)
	}
```

`router/helpers_test.go` 与 `span_test.go` 的 `RegisterControl` 调用末尾传入 `newModelHandler(t)`。

在 `helpers_test.go` 增加：

```go
func newModelHandler(t *testing.T) *handler.ModelHandler {
	t.Helper()
	return handler.NewModelHandler(llmmodel.NewService(llmmodel.NewMemoryRepository(), nil))
}
```

需要 import `vibebiz/internal/llmmodel`。

`cmd/cp/main.go` 在 `adminRepo` 附近增加：

```go
	var modelCipher *secret.Cipher
	if cfg.ModelSecretKey != "" {
		modelCipher, err = secret.NewCipher(cfg.ModelSecretKey)
		if err != nil {
			obs.Logger.Error("model secret key invalid", "error", err)
			os.Exit(1)
		}
	}
	modelRepo := llmmodel.NewMySQLRepository(db)
	modelService := llmmodel.NewService(modelRepo, modelCipher)
	modelHandler := handler.NewModelHandler(modelService)
```

`RegisterControl` 调用末尾追加 `modelHandler`。需要 import `vibebiz/internal/llmmodel` 与 `vibebiz/internal/secret`。

- [ ] **Step 6: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/secret ./internal/llmmodel ./internal/handler ./internal/router ./internal/config -count=1`  
Expected: PASS

---

### Task 5: ops-web API、类型与表单校验

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/utils/modelForm.ts`
- Test: `packages/ops-web/src/utils/modelForm.test.ts`
- Test: `packages/ops-web/src/api/models.test.ts`

**Interfaces:**
- Produces: `AdminLLMModel`、`ModelInput`、`getModels()`、`createModel(input)`、`buildModelInput(form)`、`MODEL_TYPES`、`MODEL_PROVIDERS`

- [ ] **Step 1: 写失败测试**

`utils/modelForm.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { buildModelInput, type ModelFormState } from './modelForm'

function form(overrides: Partial<ModelFormState> = {}): ModelFormState {
  return {
    modelName: 'DeepSeek Chat',
    modelCode: 'deepseek-chat',
    apiUrl: 'https://api.deepseek.com/anthropic',
    apiKey: 'sk-test',
    providerCode: 'DeepSeek',
    modelType: 'LLM',
    maxTokens: 4096,
    isDefault: true,
    status: 1,
    extraParams: '',
    ...overrides,
  }
}

describe('buildModelInput', () => {
  it('builds an LLM input', () => {
    const result = buildModelInput(form())
    expect('input' in result && result.input.modelType).toBe('LLM')
    expect('input' in result && result.input.maxTokens).toBe(4096)
  })

  it('forces non-LLM max tokens to zero', () => {
    const result = buildModelInput(form({ modelType: 'ASR', maxTokens: 4096 }))
    expect('input' in result && result.input.maxTokens).toBe(0)
  })

  it('rejects missing required fields', () => {
    expect(buildModelInput(form({ modelName: '' }))).toEqual({ error: '请填写模型名称' })
    expect(buildModelInput(form({ apiKey: '' }))).toEqual({ error: '请填写 API Key' })
  })

  it('rejects invalid code and url', () => {
    expect('error' in buildModelInput(form({ modelCode: 'bad code' }))).toBe(true)
    expect('error' in buildModelInput(form({ apiUrl: 'ftp://x' }))).toBe(true)
  })

  it('validates extra params as an object', () => {
    expect('error' in buildModelInput(form({ extraParams: '[]' }))).toBe(true)
    const result = buildModelInput(form({ extraParams: '{"voice":"alloy"}' }))
    expect('input' in result && result.input.extraParams).toEqual({ voice: 'alloy' })
  })
})
```

`api/models.test.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'

import { http } from './client'
import { getModels } from './admin'

describe('model admin api', () => {
  it('unwraps the models envelope', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { models: [{ id: 1, modelCode: 'x' }] } },
    })
    await expect(getModels()).resolves.toEqual([{ id: 1, modelCode: 'x' }])
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/ops-web && npm run test -- src/utils/modelForm.test.ts src/api/models.test.ts`  
Expected: FAIL，模块不存在

- [ ] **Step 3: 实现类型与 API**

`types/index.ts` 追加：

```ts
export interface AdminLLMModel {
  id: number
  modelName: string
  modelCode: string
  apiUrl: string
  providerCode: string
  modelType: string
  maxTokens: number
  isDefault: boolean
  status: number
  hasApiKey: boolean
  extraParams?: Record<string, unknown> | null
  createdAt: string
  updatedAt: string
}

export interface ModelInput {
  modelName: string
  modelCode: string
  apiUrl: string
  apiKey: string
  providerCode: string
  modelType: string
  maxTokens: number
  isDefault: boolean
  status: number
  extraParams?: Record<string, unknown>
}
```

`api/admin.ts` 追加：

```ts
export async function getModels(): Promise<AdminLLMModel[]> {
  const result = await request<AdminLLMModel[] | { models: AdminLLMModel[] }>({
    method: 'get',
    url: '/models',
  })
  return toList(result, 'models')
}

export async function createModel(input: ModelInput): Promise<AdminLLMModel> {
  return request<AdminLLMModel>({ method: 'post', url: '/models', data: input })
}
```

并更新 import 类型。

- [ ] **Step 4: 实现表单校验**

`utils/modelForm.ts`：

```ts
import type { ModelInput } from '@/types'

export const MODEL_TYPES = [
  { label: 'LLM 对话', value: 'LLM' },
  { label: 'Embedding', value: 'EMBEDDING' },
  { label: 'Rerank', value: 'RERANK' },
  { label: 'OCR', value: 'OCR' },
  { label: 'ASR 语音识别', value: 'ASR' },
  { label: 'TTS 语音合成', value: 'TTS' },
  { label: 'Other', value: 'OTHER' },
]

export const MODEL_PROVIDERS = [
  { label: 'OpenAI', value: 'OpenAI' },
  { label: 'DeepSeek', value: 'DeepSeek' },
  { label: 'Anthropic', value: 'Anthropic' },
  { label: 'Qwen', value: 'Qwen' },
  { label: 'Other', value: 'Other' },
]

export interface ModelFormState {
  modelName: string
  modelCode: string
  apiUrl: string
  apiKey: string
  providerCode: string
  modelType: string
  maxTokens: number
  isDefault: boolean
  status: number
  extraParams: string
}

const codePattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

export function buildModelInput(
  form: ModelFormState,
): { input: ModelInput } | { error: string } {
  const modelName = form.modelName.trim()
  const modelCode = form.modelCode.trim()
  const apiUrl = form.apiUrl.trim()
  const apiKey = form.apiKey.trim()

  if (!modelName) return { error: '请填写模型名称' }
  if (!modelCode) return { error: '请填写模型编码' }
  if (!codePattern.test(modelCode)) return { error: '模型编码只能包含字母、数字、点、下划线和中划线' }
  if (!apiUrl) return { error: '请填写 API URL' }
  if (!apiUrl.startsWith('http://') && !apiUrl.startsWith('https://')) {
    return { error: 'API URL 必须以 http:// 或 https:// 开头' }
  }
  if (!apiKey) return { error: '请填写 API Key' }

  const maxTokens = form.modelType === 'LLM' ? Number(form.maxTokens) : 0
  if (form.modelType === 'LLM' && (!Number.isFinite(maxTokens) || maxTokens <= 0)) {
    return { error: '最大 tokens 必须大于 0' }
  }

  let extraParams: Record<string, unknown> | undefined
  if (form.extraParams.trim()) {
    try {
      const parsed = JSON.parse(form.extraParams)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        return { error: 'extra_params 必须是 JSON 对象' }
      }
      extraParams = parsed as Record<string, unknown>
    } catch {
      return { error: 'extra_params 不是合法 JSON' }
    }
  }

  return {
    input: {
      modelName,
      modelCode,
      apiUrl,
      apiKey,
      providerCode: form.providerCode,
      modelType: form.modelType,
      maxTokens,
      isDefault: form.isDefault,
      status: form.status,
      extraParams,
    },
  }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/ops-web && npm run test -- src/utils/modelForm.test.ts src/api/models.test.ts`  
Expected: PASS

---

### Task 6: ModelsView、路由与导航

**Files:**
- Create: `packages/ops-web/src/views/ModelsView.vue`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: `getModels`、`createModel`、`buildModelInput`、`MODEL_TYPES`、`MODEL_PROVIDERS`

- [ ] **Step 1: 更新路由与导航测试**

`navigation.test.ts` 改为 9 个模块，路径和名称中插入 `/models`、`模型管理`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/ops-web && npm run test -- src/router/navigation.test.ts`  
Expected: FAIL

- [ ] **Step 3: 更新路由与导航**

`routes.ts` 增加 import 和路由：

```ts
import ModelsView from '@/views/ModelsView.vue'
...
  { path: 'models', name: 'models', component: ModelsView, meta: { title: '模型管理' } },
```

`navigation.ts` 在 `sandboxes` 后插入：

```ts
  { name: 'models', label: '模型管理', path: '/models', icon: 'pi pi-box' },
```

- [ ] **Step 4: 创建 ModelsView**

```vue
<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import { createModel, getModels } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { AdminLLMModel, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  buildModelInput,
  MODEL_PROVIDERS,
  MODEL_TYPES,
  type ModelFormState,
} from '@/utils/modelForm'

const { rows, loading, error, load } = useAdminList(getModels)

const columns: TableColumn[] = [
  { key: 'modelName', label: '模型名称', width: '160px' },
  { key: 'modelCode', label: '编码', width: '150px' },
  { key: 'modelType', label: '类型', width: '110px', tag: true, highlight: ['LLM'] },
  { key: 'providerCode', label: '供应商', width: '110px' },
  { key: 'apiUrl', label: 'API URL' },
  {
    key: 'maxTokens',
    label: 'Tokens',
    width: '90px',
    format: (value) => (Number(value) > 0 ? String(value) : '—'),
  },
  { key: 'isDefault', label: '默认', width: '70px', format: (value) => (value ? '是' : '否') },
  {
    key: 'status',
    label: '状态',
    width: '80px',
    format: (value) => (Number(value) === 1 ? '启用' : '禁用'),
  },
  { key: 'createdAt', label: '创建时间', width: '160px', format: (value) => formatDateTime(value) },
]

const dialogVisible = ref(false)
const saving = ref(false)
const actionError = ref('')
const notice = ref('')

function emptyForm(): ModelFormState {
  return {
    modelName: '',
    modelCode: '',
    apiUrl: '',
    apiKey: '',
    providerCode: 'Other',
    modelType: 'LLM',
    maxTokens: 4096,
    isDefault: false,
    status: 1,
    extraParams: '',
  }
}

const form = ref<ModelFormState>(emptyForm())

function openCreate() {
  form.value = emptyForm()
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

async function submit() {
  const result = buildModelInput(form.value)
  if ('error' in result) {
    actionError.value = result.error
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    await createModel(result.input)
    notice.value = '模型已创建'
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>模型管理</h1>
        <p class="page-desc">平台 LLM、Embedding、ASR、TTS 等模型目录。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ rows.length }} 条</span>
        <Button label="新增模型" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无模型"
      @retry="load"
    />

    <Dialog
      v-model:visible="dialogVisible"
      header="新增模型"
      modal
      :style="{ width: '620px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>模型名称</span>
          <InputText v-model="form.modelName" maxlength="128" />
        </label>
        <label class="field">
          <span>模型编码</span>
          <InputText v-model="form.modelCode" maxlength="64" placeholder="例如 deepseek-chat" />
        </label>
        <label class="field">
          <span>模型类型</span>
          <Select
            v-model="form.modelType"
            :options="MODEL_TYPES"
            option-label="label"
            option-value="value"
          />
        </label>
        <label class="field">
          <span>供应商</span>
          <Select
            v-model="form.providerCode"
            :options="MODEL_PROVIDERS"
            option-label="label"
            option-value="value"
          />
        </label>
        <label class="field">
          <span>API URL</span>
          <InputText v-model="form.apiUrl" placeholder="https://" />
        </label>
        <label class="field">
          <span>API Key</span>
          <InputText v-model="form.apiKey" type="password" />
        </label>
        <label v-if="form.modelType === 'LLM'" class="field">
          <span>最大 tokens</span>
          <InputNumber v-model="form.maxTokens" :min="1" :max="1000000" />
        </label>
        <label class="field">
          <span>extra_params（JSON 对象，可选）</span>
          <Textarea v-model="form.extraParams" rows="4" placeholder='{"voice":"alloy"}' />
        </label>
        <label class="field-inline">
          <Checkbox v-model="form.isDefault" binary />
          <span>设为该类型的默认模型</span>
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

.field-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 13px;
}
</style>
```

- [ ] **Step 5: 运行测试与类型检查**

Run: `cd packages/ops-web && npm run test -- src/router/navigation.test.ts src/utils/modelForm.test.ts src/api/models.test.ts && npx vue-tsc --noEmit`  
Expected: PASS

---

### Task 7: 全量验证

**Files:**
- Modify: 无（只修复验证发现的问题）

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
