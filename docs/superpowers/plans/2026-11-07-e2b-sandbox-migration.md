# E2B Embed Sandbox Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把项目执行从共享 Docker sandbox 迁移为每项目一个 E2B Embed Firecracker microVM，同时保持 CP/DP 现有 HTTP/SSE 契约。

**Architecture:** CP 用 E2B REST 创建 volume + sandbox 并绑定 `project.sandbox_id`；DP 通过 E2B `connect` 取 endpoint 和 traffic token，再按原 sandbox HTTP 契约访问。`SANDBOX_PROVIDER=local|e2b` 控制新项目；旧项目按 `sandbox_instance.provider` 双运行，迁移工具逐个项目导入工作区。

**Tech Stack:** Go 1.24、Python 3.13、E2B Embed、MySQL 8、现有 `packages/sandbox`、现有 CP/DP。

**Spec:** `docs/superpowers/specs/2026-11-07-e2b-embed-sandbox-migration-design.md`

## Global Constraints

- `SANDBOX_PROVIDER` 默认 `local`；E2B 只在新项目/已迁移项目启用。
- E2B 创建参数固定 `allowPublicTraffic=false`；所有 sandbox 请求带 `e2b-traffic-access-token`。
- `E2B_API_KEY` 只存在 CP/DP；不得写入 template、sandbox env、日志或 git。
- `trafficAccessToken` 只放内存缓存，不落库、不写日志。
- E2B 模式不挂宿主目录；项目文件只在每项目 E2B volume 中。
- 单元测试不依赖真实 E2B；用 `httptest` 和 fake repository。
- 每个任务结束运行 `gofmt`、相关 `go test`，并单独 commit。
- 不修改 web/ops-web 的 API 契约；不改变用户 skill 上传默认关闭。
- 迁移期间 local sandbox 必须保留可回滚。

## File Structure

| 文件 | 责任 |
|---|---|
| `packages/backend/internal/config/config.go` | E2B / provider 配置 |
| `packages/backend/internal/e2b/client.go` | E2B REST 客户端 |
| `packages/backend/internal/e2b/provisioner.go` | volume + sandbox + 项目绑定 |
| `packages/backend/internal/e2b/resolver.go` | endpoint / traffic token / local fallback |
| `packages/backend/internal/e2b/repository.go` | E2B 需要的 repository 接口 |
| `packages/backend/internal/client/endpoint.go` | `Endpoint` 与 resolver 接口 |
| `packages/backend/internal/client/sandbox.go` | 请求头注入 |
| `packages/backend/internal/client/router.go` | endpoint 路由、preview endpoint |
| `packages/backend/internal/sandboxregistry/model.go` | provider / external 字段 |
| `packages/backend/internal/sandboxregistry/mysql.go` | external 实例读写、provider 过滤 |
| `packages/backend/migrations/00027_e2b_sandbox.sql` | schema 增量 |
| `packages/backend/internal/handler/preview.go` | traffic token 转发 |
| `packages/backend/internal/publish/service.go` | 从 sandbox archive 发布 |
| `packages/backend/cmd/cp/main.go` | CP 装配 |
| `packages/backend/cmd/dp/main.go` | DP 装配 |
| `packages/backend/cmd/sandbox-migrate/main.go` | 项目迁移 CLI |
| `packages/backend/cmd/egress-proxy/main.go` | LLM/egress proxy |
| `deployment/e2b/template.py` | E2B template 定义 |
| `deployment/e2b/build.py` | template 构建 |
| `deployment/e2b/README.md` | Embed 部署、切流、回滚 |
| `deployment/e2b/smoke.py` | staging 冒烟 |
| `deployment/helm/backend/` | CP/DP E2B 配置 |

---

### Task 1: 增加 E2B 配置与 provider 开关

**Files:**
- Modify: `packages/backend/internal/config/config.go`
- Test: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/.env.example`
- Modify: `deployment/docker/.env.public.example`

**Interfaces:**
- Produces: `Config.SandboxProvider`, `Config.E2BAPIURL`, `Config.E2BAPIKey`, `Config.E2BTemplateID`, `Config.E2BDomain`, `Config.E2BSandboxTimeout`, `Config.E2BAllowOut`, `Config.E2BTrafficTokenTTL`, `Config.EgressProxyURL`.

- [ ] **Step 1: 写失败测试**

在 `config_test.go` 增加：

```go
func TestLoadE2BDefaults(t *testing.T) {
	t.Setenv("SANDBOX_PROVIDER", "")
	t.Setenv("E2B_API_URL", "")
	t.Setenv("E2B_TEMPLATE_ID", "")
	t.Setenv("E2B_ALLOW_OUT", "")
	cfg := Load("cp")
	if cfg.SandboxProvider != "local" {
		t.Fatalf("provider = %q", cfg.SandboxProvider)
	}
	if cfg.E2BSandboxTimeout.String() != "30m0s" {
		t.Fatalf("timeout = %s", cfg.E2BSandboxTimeout)
	}
	if cfg.E2BTrafficTokenTTL.String() != "5m0s" {
		t.Fatalf("token ttl = %s", cfg.E2BTrafficTokenTTL)
	}
}

func TestLoadE2BValues(t *testing.T) {
	t.Setenv("SANDBOX_PROVIDER", "e2b")
	t.Setenv("E2B_API_URL", "https://e2b.internal")
	t.Setenv("E2B_TEMPLATE_ID", "tmpl-1")
	t.Setenv("E2B_DOMAIN", "e2b.internal")
	t.Setenv("E2B_ALLOW_OUT", "api.deepseek.com,pypi.org")
	cfg := Load("cp")
	if cfg.SandboxProvider != "e2b" || cfg.E2BAPIURL != "https://e2b.internal" {
		t.Fatalf("cfg = %+v", cfg)
	}
	if len(cfg.E2BAllowOut) != 2 || cfg.E2BAllowOut[0] != "api.deepseek.com" {
		t.Fatalf("allow out = %#v", cfg.E2BAllowOut)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/config -run TestLoadE2B -v`  
Expected: FAIL，字段不存在。

- [ ] **Step 3: 实现配置**

在 `Config` 增加：

```go
SandboxProvider       string
E2BAPIURL             string
E2BAPIKey             string
E2BTemplateID         string
E2BDomain             string
E2BSandboxTimeout     time.Duration
E2BAllowOut           []string
E2BTrafficTokenTTL    time.Duration
EgressProxyURL        string
```

在 `Load` 增加：

```go
SandboxProvider:    strings.ToLower(getString("SANDBOX_PROVIDER", "local")),
E2BAPIURL:          strings.TrimRight(getString("E2B_API_URL", ""), "/"),
E2BAPIKey:          getString("E2B_API_KEY", ""),
E2BTemplateID:      getString("E2B_TEMPLATE_ID", ""),
E2BDomain:          strings.TrimSpace(getString("E2B_DOMAIN", "")),
E2BSandboxTimeout:  getDuration("E2B_SANDBOX_TIMEOUT", 30*time.Minute),
E2BAllowOut:        splitCSV(getString("E2B_ALLOW_OUT", "")),
E2BTrafficTokenTTL: getDuration("E2B_TRAFFIC_TOKEN_TTL", 5*time.Minute),
EgressProxyURL:     strings.TrimRight(getString("EGRESS_PROXY_URL", ""), "/"),
```

在 `.env.example` 增加：

```dotenv
# ---- sandbox provider ----
SANDBOX_PROVIDER=local
E2B_API_URL=
E2B_API_KEY=
E2B_TEMPLATE_ID=
E2B_DOMAIN=
E2B_SANDBOX_TIMEOUT=30m
E2B_ALLOW_OUT=
E2B_TRAFFIC_TOKEN_TTL=5m
EGRESS_PROXY_URL=
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && gofmt -w internal/config/config.go && go test ./internal/config -v`  
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/backend/internal/config packages/backend/.env.example deployment/docker/.env.public.example
git commit -m "feat: add e2b sandbox provider config"
```

---

### Task 2: 实现 E2B REST 客户端

**Files:**
- Create: `packages/backend/internal/e2b/client.go`
- Test: `packages/backend/internal/e2b/client_test.go`

**Interfaces:**
- Produces:
  - `type Client struct`
  - `func NewClient(apiURL, apiKey string, httpClient *http.Client) *Client`
  - `func (c *Client) CreateVolume(ctx context.Context, name string) (Volume, error)`
  - `func (c *Client) EnsureVolume(ctx context.Context, name string) (Volume, error)`
  - `func (c *Client) DeleteVolume(ctx context.Context, id string) error`
  - `func (c *Client) CreateSandbox(ctx context.Context, opts CreateOptions) (Sandbox, error)`
  - `func (c *Client) ConnectSandbox(ctx context.Context, id string, timeout time.Duration) (Sandbox, error)`
  - `func (c *Client) PauseSandbox(ctx context.Context, id string) error`
  - `func (c *Client) KillSandbox(ctx context.Context, id string) error`
  - `type Sandbox struct { ID, Domain, TrafficAccessToken string }`
  - `func (s Sandbox) BaseURL(port int, domain string) string`

- [ ] **Step 1: 写失败测试**

```go
func TestCreateSandboxUsesPrivateTrafficAndVolume(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sandboxes" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "team-key" {
			t.Fatalf("api key = %q", r.Header.Get("X-API-Key"))
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"sandboxID":"sbx-1","domain":"e2b.internal","trafficAccessToken":"traffic-1"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "team-key", server.Client())
	sandbox, err := client.CreateSandbox(context.Background(), CreateOptions{
		TemplateID: "tmpl-1",
		Timeout:    30 * time.Minute,
		Metadata:   map[string]string{"project_id": "p1"},
		VolumeName: "project-p1",
		VolumePath: "/data/workspaces/p1",
		AllowOut:   []string{"api.deepseek.com"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sandbox.ID != "sbx-1" || sandbox.TrafficAccessToken != "traffic-1" {
		t.Fatalf("sandbox = %+v", sandbox)
	}
	network := got["network"].(map[string]any)
	if network["allowPublicTraffic"] != false {
		t.Fatalf("network = %#v", network)
	}
	mounts := got["volumeMounts"].([]any)
	if mounts[0].(map[string]any)["path"] != "/data/workspaces/p1" {
		t.Fatalf("mounts = %#v", mounts)
	}
}

func TestConnectSandboxPathAndTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/sandboxes/sbx-1/connect" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"sandboxID":"sbx-1","domain":"e2b.internal","trafficAccessToken":"traffic-2"}`))
	}))
	defer server.Close()
	got, err := NewClient(server.URL, "k", server.Client()).ConnectSandbox(context.Background(), "sbx-1", time.Minute)
	if err != nil || got.TrafficAccessToken != "traffic-2" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/e2b -v`  
Expected: FAIL，包/类型不存在。

- [ ] **Step 3: 实现客户端**

`client.go` 使用 `net/http` 和 JSON，不引入新依赖。关键结构：

```go
type APIError struct {
	Status  int
	Code    string
	Message string
}

type CreateOptions struct {
	TemplateID string
	Timeout    time.Duration
	AutoPause  bool
	AutoResume bool
	RequestKey string
	Metadata   map[string]string
	EnvVars    map[string]string
	VolumeName string
	VolumePath string
	AllowOut   []string
	DenyOut    []string
}

type Volume struct {
	ID   string `json:"volumeID"`
	Name string `json:"name"`
}

type Sandbox struct {
	ID                 string `json:"sandboxID"`
	TemplateID         string `json:"templateID"`
	Domain             string `json:"domain"`
	TrafficAccessToken string `json:"trafficAccessToken"`
}

func NewClient(apiURL, apiKey string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{apiURL: strings.TrimRight(apiURL, "/"), apiKey: apiKey, http: httpClient}
}

func (s Sandbox) BaseURL(port int, domain string) string {
	if s.Domain != "" {
		domain = s.Domain
	}
	return fmt.Sprintf("https://%d-%s.%s", port, s.ID, domain)
}
```

`CreateSandbox` 请求体：

```go
body := map[string]any{
	"templateID": opts.TemplateID,
	"timeout":    int(opts.Timeout.Seconds()),
	"autoPause":  opts.AutoPause,
	"autoResume": map[string]bool{"enabled": opts.AutoResume},
	"network": map[string]any{
		"allowPublicTraffic": false,
		"allowOut":           opts.AllowOut,
		"denyOut":            opts.DenyOut,
	},
	"metadata": opts.Metadata,
	"envVars":  opts.EnvVars,
	"volumeMounts": []map[string]string{{
		"name": opts.VolumeName,
		"path": opts.VolumePath,
	}},
}
```

`CreateSandbox` 调用 `c.doWithIdempotency(ctx, http.MethodPost, "/v2/sandboxes", body, &sandbox, opts.RequestKey)`；其他调用使用 `c.do`。

`do` 统一：

```go
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	return c.doWithIdempotency(ctx, method, path, body, out, "")
}

func (c *Client) doWithIdempotency(ctx context.Context, method, path string, body any, out any, idempotencyKey string) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		var payload struct {
			Code    string `json:"error_code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &payload)
		return &APIError{Status: resp.StatusCode, Code: payload.Code, Message: payload.Message}
	}
	if out != nil && len(raw) > 0 {
		return json.Unmarshal(raw, out)
	}
	return nil
}
```

路径：

```go
CreateVolume:    POST /volumes
ListVolumes:     GET /volumes
DeleteVolume:    DELETE /volumes/{id}
CreateSandbox:   POST /v2/sandboxes
ConnectSandbox:  POST /v2/sandboxes/{id}/connect
PauseSandbox:    POST /sandboxes/{id}/pause
KillSandbox:     DELETE /sandboxes/{id}
```

Volume 重试用 `EnsureVolume`：

```go
func (c *Client) EnsureVolume(ctx context.Context, name string) (Volume, error) {
	volume, err := c.CreateVolume(ctx, name)
	var apiErr *APIError
	if err == nil || !errors.As(err, &apiErr) || apiErr.Status != http.StatusConflict {
		return volume, err
	}
	var volumes []Volume
	if err := c.do(ctx, http.MethodGet, "/volumes", nil, &volumes); err != nil {
		return Volume{}, err
	}
	for _, item := range volumes {
		if item.Name == name {
			return item, nil
		}
	}
	return Volume{}, err
}
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && gofmt -w internal/e2b/client.go && go test ./internal/e2b -v`  
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/backend/internal/e2b
git commit -m "feat: add e2b rest client"
```

---

### Task 3: 扩展 sandbox_instance schema / model / repository

**Files:**
- Create: `packages/backend/migrations/00027_e2b_sandbox.sql`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/internal/dao/model/sandbox.go`
- Modify: `packages/backend/internal/sandboxregistry/model.go`
- Modify: `packages/backend/internal/sandboxregistry/mysql.go`
- Test: `packages/backend/internal/sandboxregistry/mysql_test.go`

**Interfaces:**
- Produces on `Instance`: `Provider`, `ExternalID`, `VolumeID`, `TemplateID`, `LastUsedAt`, `ExpiresAt`.
- Produces on `Repository`:
  - `CreateExternal(ctx context.Context, instance Instance, projectInternalID int64, expectedOldSandboxID *int64) (Instance, error)`
  - `TouchExternal(ctx context.Context, id int64, at time.Time) error`
  - `UpdateProjectBinding(ctx context.Context, projectInternalID int64, oldSandboxID *int64, newSandboxID int64) error`（已有）

- [ ] **Step 1: 写迁移测试**

在 `migrations_test.go` 增加：

```go
func TestE2BSandboxMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00027_e2b_sandbox.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"ADD COLUMN `provider`",
		"ADD COLUMN `external_id`",
		"ADD COLUMN `volume_id`",
		"idx_sandbox_instance_provider",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("migration missing %q", want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./migrations -run TestE2BSandboxMigration -v`  
Expected: FAIL，文件不存在。

- [ ] **Step 3: 写迁移**

```sql
-- +goose Up

ALTER TABLE `sandbox_instance`
  ADD COLUMN `provider` varchar(16) NOT NULL DEFAULT 'local' COMMENT 'local/e2b',
  ADD COLUMN `external_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'E2B sandbox ID',
  ADD COLUMN `volume_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'E2B volume ID',
  ADD COLUMN `template_id` varchar(128) NOT NULL DEFAULT '' COMMENT 'E2B template ID',
  ADD COLUMN `last_used_at` datetime(3) DEFAULT NULL,
  ADD COLUMN `expires_at` datetime(3) DEFAULT NULL,
  ADD KEY `idx_sandbox_instance_provider` (`provider`, `status`, `health_status`),
  ADD KEY `idx_sandbox_instance_external_id` (`external_id`);

-- +goose Down

ALTER TABLE `sandbox_instance`
  DROP INDEX `idx_sandbox_instance_external_id`,
  DROP INDEX `idx_sandbox_instance_provider`,
  DROP COLUMN `expires_at`,
  DROP COLUMN `last_used_at`,
  DROP COLUMN `template_id`,
  DROP COLUMN `volume_id`,
  DROP COLUMN `external_id`,
  DROP COLUMN `provider`;
```

- [ ] **Step 4: 更新 model 与 repository**

`dao/model/sandbox.go` 增加字段：

```go
Provider   string     `gorm:"column:provider" json:"provider"`
ExternalID string     `gorm:"column:external_id" json:"externalId"`
VolumeID   string     `gorm:"column:volume_id" json:"volumeId"`
TemplateID string     `gorm:"column:template_id" json:"templateId"`
LastUsedAt *time.Time `gorm:"column:last_used_at" json:"lastUsedAt,omitempty"`
ExpiresAt  *time.Time `gorm:"column:expires_at" json:"expiresAt,omitempty"`
```

`sandboxregistry.Instance` 同步增加字段，`toInstance` 映射。

本地健康检查/调度只处理 `provider='local'`：

```go
func (r *MySQLRepository) MarkStaleOffline(...) error {
	return r.db.WithContext(ctx).
		Model(&model.SandboxInstance{}).
		Where("provider = ? AND status = ? AND last_heartbeat_at < ?", "local", StatusOnline, now.Add(-ttl)).
		Update("status", StatusOffline).Error
}
```

```go
func (r *MySQLRepository) ListPendingHealth(...) ([]Instance, error) {
	query := r.db.WithContext(ctx).Where("provider = ?", "local")
	// 保留原 where/order
}
```

```go
func (r *MySQLRepository) Select(...) (Instance, error) {
	// 原 query 增加 Where("provider = ?", "local")
}
```

新增：

```go
func (r *MySQLRepository) CreateExternal(ctx context.Context, instance Instance, projectInternalID int64, expectedOldSandboxID *int64) (Instance, error) {
	var result Instance
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := model.SandboxInstance{
			PublicID:       instance.PublicID,
			Name:           instance.Name,
			Scheme:         instance.Scheme,
			Host:           instance.Host,
			Port:           instance.Port,
			Status:         StatusOnline,
			HealthStatus:   HealthHealthy,
			MaxConcurrency: instance.MaxConcurrency,
			Provider:       instance.Provider,
			ExternalID:     instance.ExternalID,
			VolumeID:       instance.VolumeID,
			TemplateID:     instance.TemplateID,
			LastUsedAt:     instance.LastUsedAt,
			ExpiresAt:      instance.ExpiresAt,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		query := tx.Model(&model.Project{}).
			Where("id = ? AND deleted_at IS NULL", projectInternalID)
		if expectedOldSandboxID == nil {
			query = query.Where("sandbox_id IS NULL")
		} else {
			query = query.Where("sandbox_id = ?", *expectedOldSandboxID)
		}
		res := query.Update("sandbox_id", row.ID)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrBindingConflict
		}
		result = toInstance(&row)
		return nil
	})
	return result, err
}

func (r *MySQLRepository) TouchExternal(ctx context.Context, id int64, at time.Time) error {
	return r.db.WithContext(ctx).
		Model(&model.SandboxInstance{}).
		Where("id = ?", id).
		Update("last_used_at", at).Error
}
```

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && gofmt -w internal/dao/model/sandbox.go internal/sandboxregistry && go test ./migrations ./internal/sandboxregistry -v`  
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/backend/migrations packages/backend/internal/dao/model/sandbox.go packages/backend/internal/sandboxregistry
git commit -m "feat: persist e2b sandbox instances"
```

---

### Task 4: Endpoint + traffic token 注入

**Files:**
- Create: `packages/backend/internal/client/endpoint.go`
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`
- Modify: `packages/backend/internal/sandboxregistry/resolver.go`
- Test: `packages/backend/internal/client/sandbox_test.go`
- Test: `packages/backend/internal/client/router_test.go`

**Interfaces:**
- Produces:
  - `type Endpoint struct { BaseURL string; Headers map[string]string }`
  - `func (e Endpoint) CacheKey() string`
  - `type ProjectResolver interface { ResolveEndpoint(ctx context.Context, projectID string) (Endpoint, error) }`
  - `func NewSandboxWithHeaders(baseURL string, headers map[string]string, obs *observability.Provider) *Sandbox`
  - `func (r *Router) PreviewEndpoint(ctx context.Context, id string) (Endpoint, error)`

- [ ] **Step 1: 写失败测试**

在 `sandbox_test.go` 增加：

```go
func TestSandboxInjectsTrafficToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("e2b-traffic-access-token") != "traffic-1" {
			t.Fatalf("token = %q", r.Header.Get("e2b-traffic-access-token"))
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer server.Close()
	sandbox := NewSandboxWithHeaders(server.URL, map[string]string{
		"e2b-traffic-access-token": "traffic-1",
	}, nil)
	if _, err := sandbox.ListFiles(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
}
```

在 `router_test.go` 增加：

```go
func TestRouterUsesEndpointHeaders(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("e2b-traffic-access-token")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer server.Close()
	router := NewRouter(fakeEndpointResolver{endpoint: Endpoint{
		BaseURL: server.URL,
		Headers: map[string]string{"e2b-traffic-access-token": "t1"},
	}}, nil)
	if _, err := router.ListFiles(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if got != "t1" {
		t.Fatalf("token = %q", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/client -run 'TestSandboxInjectsTrafficToken|TestRouterUsesEndpointHeaders' -v`  
Expected: FAIL，类型/方法不存在。

- [ ] **Step 3: 实现 Endpoint 与 headers**

`endpoint.go`：

```go
package client

import (
	"context"
	"sort"
	"strings"
)

type Endpoint struct {
	BaseURL string
	Headers map[string]string
}

func (e Endpoint) CacheKey() string {
	keys := make([]string, 0, len(e.Headers))
	for key := range e.Headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(e.BaseURL)
	for _, key := range keys {
		b.WriteString("|")
		b.WriteString(key)
		b.WriteString("=")
		b.WriteString(e.Headers[key])
	}
	return b.String()
}

type ProjectResolver interface {
	ResolveEndpoint(ctx context.Context, projectID string) (Endpoint, error)
}
```

`Sandbox` 增加：

```go
type Sandbox struct {
	baseURL string
	headers http.Header
	http    *http.Client
	obs     *observability.Provider
}

func NewSandbox(baseURL string, obs *observability.Provider) *Sandbox {
	return NewSandboxWithHeaders(baseURL, nil, obs)
}

func NewSandboxWithHeaders(baseURL string, headers map[string]string, obs *observability.Provider) *Sandbox {
	client := &http.Client{Timeout: 10 * time.Minute}
	if obs != nil {
		client.Transport = otelhttp.NewTransport(http.DefaultTransport)
	}
	h := make(http.Header, len(headers))
	for key, value := range headers {
		h.Set(key, value)
	}
	return &Sandbox{baseURL: strings.TrimRight(baseURL, "/"), headers: h, http: client, obs: obs}
}
```

所有请求创建后统一：

```go
func (s *Sandbox) applyHeaders(req *http.Request) {
	for key, values := range s.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}
```

在 `doJSON`、`Chat`、`WriteResource`、`ExportArchive`、`ImportArchive` 的 `http.NewRequest*` 后调用 `s.applyHeaders(req)`。

- [ ] **Step 4: 改 Router**

```go
type Router struct {
	resolver  ProjectResolver
	obs       *observability.Provider
	clients   sync.Map
	newClient func(baseURL string, headers map[string]string, obs *observability.Provider) *Sandbox
}

func NewRouter(resolver ProjectResolver, obs *observability.Provider) *Router {
	return &Router{resolver: resolver, obs: obs, newClient: NewSandboxWithHeaders}
}

func (r *Router) client(ctx context.Context, projectID string) (*Sandbox, error) {
	endpoint, err := r.resolver.ResolveEndpoint(ctx, projectID)
	if err != nil {
		return nil, err
	}
	key := endpoint.CacheKey()
	if value, ok := r.clients.Load(key); ok {
		return value.(*Sandbox), nil
	}
	created := r.newClient(endpoint.BaseURL, endpoint.Headers, r.obs)
	actual, _ := r.clients.LoadOrStore(key, created)
	return actual.(*Sandbox), nil
}
```

增加：

```go
func (r *Router) PreviewEndpoint(ctx context.Context, id string) (Endpoint, error) {
	return r.resolver.ResolveEndpoint(ctx, id)
}
```

- [ ] **Step 5: 本地 resolver 适配**

`sandboxregistry/resolver.go` 增加：

```go
func (r *Resolver) ResolveEndpoint(ctx context.Context, projectID string) (client.Endpoint, error) {
	baseURL, err := r.ResolveBaseURL(ctx, projectID)
	if err != nil {
		return client.Endpoint{}, err
	}
	return client.Endpoint{BaseURL: baseURL}, nil
}
```

需要 `import "vibebiz/internal/client"`；`client` 不反向 import `sandboxregistry`，无循环。

- [ ] **Step 6: 运行测试**

Run: `cd packages/backend && gofmt -w internal/client internal/sandboxregistry && go test ./internal/client ./internal/sandboxregistry -v`  
Expected: PASS。

- [ ] **Step 7: Commit**

```bash
git add packages/backend/internal/client packages/backend/internal/sandboxregistry
git commit -m "feat: carry sandbox endpoint headers"
```

---

### Task 5: 实现 E2B provisioner

**Files:**
- Create: `packages/backend/internal/e2b/provisioner.go`
- Create: `packages/backend/internal/e2b/repository.go`
- Test: `packages/backend/internal/e2b/provisioner_test.go`

**Interfaces:**
- Consumes: `Client`, `sandboxregistry.Instance`, `client.NewSandboxWithHeaders`.
- Produces:
  - `type Repository interface { GetProjectBinding; CreateExternal; TouchExternal; UpdateProjectBinding }`
  - `func NewProvisioner(e2bClient *Client, repo Repository, opts ProvisionerOptions) *Provisioner`
  - `func (p *Provisioner) InitProject(ctx context.Context, projectID string) (int64, error)`
  - `func (p *Provisioner) DeleteProject(ctx context.Context, projectID string) error`

- [ ] **Step 1: 写失败测试**

```go
func TestProvisionerCreatesVolumeSandboxAndInit(t *testing.T) {
	var calls []string
	e2bServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.URL.Path {
		case "/volumes":
			_, _ = w.Write([]byte(`{"volumeID":"vol-1","name":"project-p1"}`))
		case "/v2/sandboxes":
			_, _ = w.Write([]byte(`{"sandboxID":"sbx-1","domain":"e2b.internal","trafficAccessToken":"t1"}`))
		default:
			t.Fatalf("unexpected e2b call %s %s", r.Method, r.URL.Path)
		}
	}))
	defer e2bServer.Close()
	sandboxServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, "sandbox "+r.Method+" "+r.URL.Path)
		if r.URL.Path != "/internal/projects/p1/init" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{}}`))
	}))
	defer sandboxServer.Close()

	repo := &fakeRepository{binding: sandboxregistry.Binding{ProjectInternalID: 10, ProjectPublicID: "p1"}}
	provisioner := NewProvisioner(NewClient(e2bServer.URL, "k", e2bServer.Client()), repo, ProvisionerOptions{
		TemplateID: "tmpl-1",
		Domain:     "e2b.internal",
		Timeout:    time.Minute,
		VolumeRoot: "/data/workspaces",
		InitBase:   sandboxServer.URL,
	})
	id, err := provisioner.InitProject(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("sandbox id is zero")
	}
	if len(calls) != 3 || calls[0] != "POST /volumes" || calls[1] != "POST /v2/sandboxes" {
		t.Fatalf("calls = %#v", calls)
	}
}
```

`fakeRepository` 在测试文件里实现 `GetProjectBinding`、`CreateExternal`、`TouchExternal`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/e2b -run TestProvisioner -v`  
Expected: FAIL。

- [ ] **Step 3: 实现**

```go
type Repository interface {
	GetProjectBinding(ctx context.Context, publicProjectID string) (sandboxregistry.Binding, error)
	CreateExternal(ctx context.Context, instance sandboxregistry.Instance, projectInternalID int64, expectedOldSandboxID *int64) (sandboxregistry.Instance, error)
	TouchExternal(ctx context.Context, id int64, at time.Time) error
	UpdateProjectBinding(ctx context.Context, projectInternalID int64, oldSandboxID *int64, newSandboxID int64) error
}

type ProvisionerOptions struct {
	TemplateID    string
	Domain        string
	Timeout       time.Duration
	VolumeRoot    string
	AllowOut      []string
	EgressProxyURL string
	InitBase      string
}

type Provisioner struct {
	client *Client
	repo   Repository
	opts   ProvisionerOptions
}

func NewProvisioner(e2bClient *Client, repo Repository, opts ProvisionerOptions) *Provisioner {
	return &Provisioner{client: e2bClient, repo: repo, opts: opts}
}

func (p *Provisioner) InitProject(ctx context.Context, projectID string) (int64, error) {
	binding, err := p.repo.GetProjectBinding(ctx, projectID)
	if err != nil {
		return 0, err
	}
	if binding.Instance != nil && binding.Instance.Provider == "e2b" {
		return binding.Instance.ID, nil
	}
	volumeName := "project-" + binding.ProjectPublicID
	volume, err := p.client.EnsureVolume(ctx, volumeName)
	if err != nil {
		return 0, err
	}
	envVars := map[string]string{}
	if p.opts.EgressProxyURL != "" {
		envVars["ANTHROPIC_BASE_URL"] = p.opts.EgressProxyURL
		envVars["ANTHROPIC_API_KEY"] = "proxy-managed"
	}
	sandbox, err := p.client.CreateSandbox(ctx, CreateOptions{
		TemplateID: p.opts.TemplateID,
		Timeout:    p.opts.Timeout,
		AutoPause:  true,
		AutoResume: true,
		RequestKey: binding.ProjectPublicID,
		Metadata:   map[string]string{"project_id": binding.ProjectPublicID},
		EnvVars:    envVars,
		VolumeName: volume.Name,
		VolumePath: strings.TrimRight(p.opts.VolumeRoot, "/") + "/" + binding.ProjectPublicID,
		AllowOut:   p.opts.AllowOut,
	})
	if err != nil {
		_ = p.client.DeleteVolume(ctx, volume.ID)
		return 0, err
	}
	instance, err := p.repo.CreateExternal(ctx, sandboxregistry.Instance{
		Name:           "e2b:" + binding.ProjectPublicID,
		Scheme:         "https",
		Host:           strings.TrimPrefix(sandbox.BaseURL(8000, p.opts.Domain), "https://"),
		Port:           443,
		Provider:       "e2b",
		ExternalID:     sandbox.ID,
		VolumeID:       volume.ID,
		TemplateID:     p.opts.TemplateID,
		MaxConcurrency: 1,
	}, binding.ProjectInternalID, binding.SandboxID)
	if err != nil {
		_ = p.client.KillSandbox(ctx, sandbox.ID)
		_ = p.client.DeleteVolume(ctx, volume.ID)
		return 0, err
	}
	baseURL := sandbox.BaseURL(8000, p.opts.Domain)
	if p.opts.InitBase != "" {
		baseURL = p.opts.InitBase
	}
	if err := client.NewSandboxWithHeaders(baseURL, map[string]string{
		"e2b-traffic-access-token": sandbox.TrafficAccessToken,
	}, nil).InitProject(ctx, projectID); err != nil {
		_ = p.client.KillSandbox(ctx, sandbox.ID)
		_ = p.client.DeleteVolume(ctx, volume.ID)
		return 0, err
	}
	return instance.ID, nil
}
```

`DeleteProject`：读取绑定，kill sandbox，delete volume，repo 清理/软删除。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && gofmt -w internal/e2b && go test ./internal/e2b -v`  
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/backend/internal/e2b
git commit -m "feat: provision per-project e2b sandboxes"
```

---

### Task 6: 实现 E2B resolver 与 local fallback

**Files:**
- Create: `packages/backend/internal/e2b/resolver.go`
- Test: `packages/backend/internal/e2b/resolver_test.go`

**Interfaces:**
- Consumes: `Client`, `Repository`, `client.ProjectResolver`（local fallback）。
- Produces: `func NewResolver(client *Client, repo Repository, local client.ProjectResolver, domain string, ttl time.Duration) *Resolver`；实现 `client.ProjectResolver`。

- [ ] **Step 1: 写失败测试**

```go
func TestResolverConnectsAndReturnsToken(t *testing.T) {
	e2bServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/sandboxes/sbx-1/connect" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"sandboxID":"sbx-1","domain":"e2b.internal","trafficAccessToken":"t1"}`))
	}))
	defer e2bServer.Close()
	repo := &fakeRepository{binding: sandboxregistry.Binding{
		ProjectInternalID: 1,
		ProjectPublicID:   "p1",
		Instance: &sandboxregistry.Instance{
			ID:         7,
			Provider:   "e2b",
			ExternalID: "sbx-1",
		},
	}}
	resolver := NewResolver(NewClient(e2bServer.URL, "k", e2bServer.Client()), repo, nil, "e2b.internal", time.Minute)
	endpoint, err := resolver.ResolveEndpoint(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.BaseURL != "https://8000-sbx-1.e2b.internal" {
		t.Fatalf("base url = %s", endpoint.BaseURL)
	}
	if endpoint.Headers["e2b-traffic-access-token"] != "t1" {
		t.Fatalf("headers = %#v", endpoint.Headers)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/e2b -run TestResolver -v`  
Expected: FAIL。

- [ ] **Step 3: 实现**

```go
type cachedEndpoint struct {
	endpoint client.Endpoint
	expires  time.Time
}

type Resolver struct {
	client *Client
	repo   Repository
	local  client.ProjectResolver
	domain string
	ttl    time.Duration
	mu     sync.Mutex
	cache  map[string]cachedEndpoint
}

func (r *Resolver) ResolveEndpoint(ctx context.Context, projectID string) (client.Endpoint, error) {
	binding, err := r.repo.GetProjectBinding(ctx, projectID)
	if err != nil {
		return client.Endpoint{}, err
	}
	if binding.Instance == nil || binding.Instance.Provider != "e2b" {
		if r.local == nil {
			return client.Endpoint{}, sandboxregistry.ErrNoHealthySandbox
		}
		return r.local.ResolveEndpoint(ctx, projectID)
	}
	key := binding.Instance.ExternalID
	r.mu.Lock()
	if item, ok := r.cache[key]; ok && time.Now().Before(item.expires) {
		r.mu.Unlock()
		return item.endpoint, nil
	}
	r.mu.Unlock()

	sandbox, err := r.client.ConnectSandbox(ctx, key, 5*time.Minute)
	if err != nil {
		return client.Endpoint{}, err
	}
	endpoint := client.Endpoint{
		BaseURL: sandbox.BaseURL(8000, r.domain),
		Headers: map[string]string{"e2b-traffic-access-token": sandbox.TrafficAccessToken},
	}
	r.mu.Lock()
	r.cache[key] = cachedEndpoint{endpoint: endpoint, expires: time.Now().Add(r.ttl)}
	r.mu.Unlock()
	go r.repo.TouchExternal(context.Background(), binding.Instance.ID, time.Now().UTC())
	return endpoint, nil
}
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && gofmt -w internal/e2b/resolver.go && go test ./internal/e2b -v`  
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/backend/internal/e2b
git commit -m "feat: resolve e2b sandbox endpoints"
```

---

### Task 7: CP / DP 接入 provider

**Files:**
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/cmd/dp/main.go`
- Test: `packages/backend/internal/e2b/factory_test.go`（新增 factory）
- Create: `packages/backend/internal/e2b/factory.go`

**Interfaces:**
- Produces: `func NewRuntime(cfg config.Config, repo *sandboxregistry.MySQLRepository, local client.ProjectResolver) (*Client, *Resolver, *Provisioner, error)`。

- [ ] **Step 1: 写失败测试**

```go
func TestNewRuntimeRejectsMissingTemplate(t *testing.T) {
	_, _, _, err := NewRuntime(config.Config{SandboxProvider: "e2b", E2BAPIURL: "http://e2b", E2BAPIKey: "k"}, nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/e2b -run TestNewRuntime -v`  
Expected: FAIL。

- [ ] **Step 3: 实现 factory**

```go
func NewRuntime(cfg config.Config, repo *sandboxregistry.MySQLRepository, local client.ProjectResolver) (*Client, *Resolver, *Provisioner, error) {
	if cfg.SandboxProvider != "e2b" {
		return nil, nil, nil, nil
	}
	if cfg.E2BAPIURL == "" || cfg.E2BAPIKey == "" || cfg.E2BTemplateID == "" || cfg.E2BDomain == "" {
		return nil, nil, nil, fmt.Errorf("e2b config incomplete")
	}
	e2bClient := NewClient(cfg.E2BAPIURL, cfg.E2BAPIKey, nil)
	resolver := NewResolver(e2bClient, repo, local, cfg.E2BDomain, cfg.E2BTrafficTokenTTL)
	provisioner := NewProvisioner(e2bClient, repo, ProvisionerOptions{
		TemplateID:     cfg.E2BTemplateID,
		Domain:         cfg.E2BDomain,
		Timeout:        cfg.E2BSandboxTimeout,
		VolumeRoot:     "/data/workspaces",
		AllowOut:       cfg.E2BAllowOut,
		EgressProxyURL: cfg.EgressProxyURL,
	})
	return e2bClient, resolver, provisioner, nil
}
```

- [ ] **Step 4: 改 CP / DP**

CP：

```go
registryRepo := sandboxregistry.NewMySQLRepository(db)
localSelector := sandboxregistry.NewSelector(registryRepo, cfg.SandboxHeartbeatTTL, time.Now)
localResolver := sandboxregistry.NewResolver(registryRepo, localSelector, cfg.SandboxHeartbeatTTL, time.Now)
_, e2bResolver, e2bProvisioner, err := e2b.NewRuntime(cfg, registryRepo, localResolver)
if err != nil {
	obs.Logger.Error("e2b init failed", "error", err)
	os.Exit(1)
}
var resolver client.ProjectResolver = localResolver
var provisioner handler.ProjectInitializer = sandboxregistry.NewProvisioner(localSelector, sandboxregistry.NewHTTPInitializer(obs), 3)
if cfg.SandboxProvider == "e2b" {
	resolver = e2bResolver
	provisioner = e2bProvisioner
}
sandboxRouter := client.NewRouter(resolver, obs)
```

DP：

```go
_, e2bResolver, _, err := e2b.NewRuntime(cfg, registryRepo, localResolver)
if err != nil { ... }
var resolver client.ProjectResolver = localResolver
if cfg.SandboxProvider == "e2b" {
	resolver = e2bResolver
}
sandbox := client.NewRouter(resolver, obs)
```

保留 `registryRepo` 与 local health checker；E2B 行由 `provider` 过滤，不会被本地心跳误判。

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && gofmt -w cmd/cp cmd/dp internal/e2b && go test ./... && go vet ./...`  
Expected: 全部 PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/backend/cmd packages/backend/internal/e2b
git commit -m "feat: wire e2b provider into cp and dp"
```

---

### Task 8: 预览请求带 traffic token

**Files:**
- Modify: `packages/backend/internal/handler/preview.go`
- Modify: `packages/backend/internal/handler/preview_test.go`
- Modify: `packages/backend/internal/client/router.go`

**Interfaces:**
- Produces: `PreviewClient.PreviewEndpoint(ctx, id) (client.Endpoint, error)`。
- `PreviewHandler` 在反向代理时设置 endpoint headers。

- [ ] **Step 1: 写失败测试**

在 `preview_test.go` 增加：

```go
func TestPreviewForwardsTrafficToken(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("e2b-traffic-access-token") != "t1" {
			t.Fatalf("token = %q", r.Header.Get("e2b-traffic-access-token"))
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()
	h := NewPreviewHandler(fakePreviewEndpoint{endpoint: client.Endpoint{
		BaseURL: upstream.URL,
		Headers: map[string]string{"e2b-traffic-access-token": "t1"},
	}}, nil)
	// 构造 GET /dp/api/v1/projects/p1/preview/ 并断言 200
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler -run TestPreviewForwardsTrafficToken -v`  
Expected: FAIL。

- [ ] **Step 3: 实现**

```go
type PreviewClient interface {
	PreviewEndpoint(ctx context.Context, id string) (client.Endpoint, error)
}
```

`Serve` 改为：

```go
endpoint, err := h.Sandbox.PreviewEndpoint(c.Request().Context(), projectID)
target, err := url.Parse(endpoint.BaseURL)
proxy := &httputil.ReverseProxy{
	Director: func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = target.Path + wildcard
		req.Host = target.Host
		for key, value := range endpoint.Headers {
			req.Header.Set(key, value)
		}
		otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	},
	Transport: previewTransport, FlushInterval: -1,
}
```

`client.Router` 的 `PreviewEndpoint` 已在 Task 4 添加。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && gofmt -w internal/handler/preview.go && go test ./internal/handler -run TestPreview -v`  
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/backend/internal/handler/preview.go packages/backend/internal/handler/preview_test.go
git commit -m "feat: authenticate preview proxy to e2b"
```

---

### Task 9: 发布改为从 sandbox archive 读取

**Files:**
- Modify: `packages/backend/internal/publish/service.go`
- Modify: `packages/backend/internal/publish/service_test.go`
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Produces: `type Source interface { ExportArchive(ctx context.Context, projectID string) ([]byte, error) }`
- `Service.Source Source`。
- `Source != nil` 时从 tar.gz 解压到 `gameDir`。

- [ ] **Step 1: 写失败测试**

```go
func TestPublishUsesSandboxArchiveSource(t *testing.T) {
	dir := t.TempDir()
	source := &fakeSource{data: tarGz(t, map[string]string{"index.html": "<h1>hi</h1>"})}
	service := NewService("", dir)
	service.Source = source
	result, err := service.Publish(context.Background(), "p1", Input{Title: "t"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Entry != "index.html" {
		t.Fatalf("entry = %s", result.Entry)
	}
	body, err := os.ReadFile(filepath.Join(dir, "p1", "game", "index.html"))
	if err != nil || string(body) != "<h1>hi</h1>" {
		t.Fatalf("body=%q err=%v", body, err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/publish -run TestPublishUsesSandboxArchiveSource -v`  
Expected: FAIL。

- [ ] **Step 3: 实现**

`Publish` 在确定 staging 后：

```go
if s.Source != nil {
	data, err := s.Source.ExportArchive(ctx, projectID)
	if err != nil {
		return Result{}, err
	}
	if err := extractTarGz(data, gameDir, s.MaxBytes); err != nil {
		return Result{}, err
	}
} else {
	if err := s.copyTree(ctx, source, gameDir); err != nil {
		return Result{}, err
	}
}
```

`like`:

```go
func extractTarGz(data []byte, target string, maxBytes int64) error {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer reader.Close()
	archive := tar.NewReader(reader)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		clean := filepath.Clean(header.Name)
		if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			return ErrInvalid
		}
		destination := filepath.Join(target, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, io.LimitReader(archive, maxBytes))
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}
```

`cmd/cp`：

```go
publishService := publish.NewService(cfg.WorkspacesDir, cfg.PublishDir)
if cfg.SandboxProvider == "e2b" {
	publishService.Source = sandboxRouter
}
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && gofmt -w internal/publish && go test ./internal/publish -v`  
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/backend/internal/publish packages/backend/cmd/cp/main.go
git commit -m "feat: publish from sandbox archive"
```

---

### Task 10: E2B template 与公共 skills

**Files:**
- Create: `deployment/e2b/template.py`
- Create: `deployment/e2b/build.py`
- Create: `deployment/e2b/README.md`
- Modify: `packages/backend/internal/handler/skill_manage.go`
- Modify: `packages/backend/cmd/cp/main.go`（E2B 模式下 skill router 处理）

**Interfaces:**
- Template 内置 `packages/sandbox`、Node/Claude Code、`data/skills/public`。
- 启动 `uvicorn app.main:app --host 0.0.0.0 --port 8000`。
- `E2B_TEMPLATE_ID` 指向构建产物。

- [ ] **Step 1: 写 template.py**

```python
from e2b import Template, wait_for_port

template = (
    Template()
    .from_ubuntu_image("24.04")
    .apt_install([
        "ca-certificates",
        "curl",
        "git",
        "python3",
        "python3-venv",
        "python3-pip",
        "nodejs",
        "npm",
    ])
    .make_dir("/app")
    .make_dir("/data/workspaces")
    .make_dir("/data/skills/public")
    .make_dir("/data/skills/users")
    .copy_items([
        {"src": "./packages/sandbox/app/", "dest": "/app/app/"},
        {"src": "./packages/sandbox/requirements.txt", "dest": "/app/requirements.txt"},
    ])
    .copy_items([{"src": "./data/skills/public/", "dest": "/data/skills/public/"}])
    .run_cmd("curl -fsSL https://deb.nodesource.com/setup_20.x | bash - && apt-get install -y nodejs")
    .run_cmd("python3 -m venv /opt/venv && /opt/venv/bin/pip install --no-cache-dir -r /app/requirements.txt")
    .run_cmd("npm install -g @anthropic-ai/claude-code")
    .set_envs({
        "WORKSPACES_DIR": "/data/workspaces",
        "SKILLS_ROOT": "/data/skills",
        "AGENT_MODE": "claude",
        "AGENT_NETWORK_RESTRICTED": "false",
        "CLONE_TEST_RUNNER_ENABLED": "false",
        "LOG_DIR": "",
    })
    .set_workdir("/app")
    .set_start_cmd(
        "/opt/venv/bin/uvicorn app.main:app --host 0.0.0.0 --port 8000 --no-access-log",
        wait_for_port(8000),
    )
)
```

- [ ] **Step 2: 写 build.py**

```python
import os
from e2b import Template, default_build_logger
from template import template

if __name__ == "__main__":
    tag = os.environ["E2B_TEMPLATE_TAG"]
    Template.build(
        template,
        tag,
        cpu_count=2,
        memory_mb=2048,
        on_build_logs=default_build_logger(),
    )
```

- [ ] **Step 3: 构建**

Run:

```bash
cd deployment/e2b
E2B_API_KEY=... E2B_API_URL=https://e2b.internal E2B_TEMPLATE_TAG=vibebiz-sandbox-v1 \
  python3 build.py
```

Expected: 构建成功，输出 template ID；把 ID 写入 `E2B_TEMPLATE_ID`。

- [ ] **Step 4: 处理 skills 写操作**

在 `cmd/cp` 的 E2B 模式：

- E2B 模式使用 `handler` 的 `UnavailableSkillClient`（本任务新增），所有 list/upload/delete 返回 `50305 "skills are read-only in e2b mode"`；公共 skills 仍由 template 提供。

`handler/skill_manage.go` 增加：

```go
type UnavailableSkillClient struct{}

func (UnavailableSkillClient) ListStoredSkills(context.Context, string, int64) ([]client.SkillInfo, error) {
	return nil, errSkillsUnavailable
}
func (UnavailableSkillClient) UploadSkill(context.Context, string, int64, string, io.Reader, int64, bool) (client.SkillInfo, error) {
	return client.SkillInfo{}, errSkillsUnavailable
}
func (UnavailableSkillClient) DeleteSkill(context.Context, string, int64, string) error {
	return errSkillsUnavailable
}
var errSkillsUnavailable = errors.New("skills are read-only in e2b mode")
```

`skillFailure` 对 `errSkillsUnavailable` 返回 `50305`。

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && gofmt -w internal/handler/skill_manage.go && go test ./internal/handler -run Skill -v`  
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add deployment/e2b packages/backend/internal/handler/skill_manage.go packages/backend/cmd/cp/main.go
git commit -m "feat: add e2b sandbox template"
```

---

### Task 11: 宿主侧 LLM/egress proxy

**Files:**
- Create: `packages/backend/cmd/egress-proxy/main.go`
- Create: `packages/backend/internal/egressproxy/proxy.go`
- Test: `packages/backend/internal/egressproxy/proxy_test.go`
- Create: `deployment/e2b/proxy.Dockerfile`

**Interfaces:**
- `NewHandler(upstream *url.URL, header, value string) http.Handler`。
- 只接受 `POST /v1/*`；删除入站伪造认证头，设置真实上游头。

- [ ] **Step 1: 写失败测试**

```go
func TestProxyInjectsUpstreamKeyAndStripsClientKey(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "real-key" {
			t.Fatalf("upstream key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("authorization leaked: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	handler := NewHandler(target, "x-api-key", "real-key")
	req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer client-key")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/egressproxy -v`  
Expected: FAIL。

- [ ] **Step 3: 实现**

```go
func NewHandler(upstream *url.URL, header, value string) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	original := proxy.Director
	proxy.Director = func(req *http.Request) {
		original(req)
		req.Header.Del("Authorization")
		req.Header.Del("x-api-key")
		req.Header.Set(header, value)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/v1/") {
			http.NotFound(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}
```

`main.go` 读取 `EGRESS_PROXY_LISTEN`、`EGRESS_PROXY_UPSTREAM`、`EGRESS_PROXY_AUTH_HEADER`、`EGRESS_PROXY_AUTH_VALUE`，启动 `http.Server`。

- [ ] **Step 4: template 接入**

E2B template 的 `ANTHROPIC_BASE_URL` 设为 `http://egress-proxy.<namespace>.svc:8090`，`ANTHROPIC_API_KEY` 使用占位值；构建/运行时通过 env 注入。`E2B_ALLOW_OUT` 只放行 proxy 和包管理器域名。

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && gofmt -w internal/egressproxy cmd/egress-proxy && go test ./internal/egressproxy -v`  
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/backend/internal/egressproxy packages/backend/cmd/egress-proxy deployment/e2b/proxy.Dockerfile
git commit -m "feat: add egress proxy for llm credentials"
```

---

### Task 12: 项目迁移工具

**Files:**
- Create: `packages/backend/cmd/sandbox-migrate/main.go`
- Test: `packages/backend/internal/e2b/migrate_test.go`（核心逻辑抽到 `internal/e2b/migrate.go`）
- Create: `packages/backend/internal/e2b/migrate.go`

**Interfaces:**
- `func MigrateProject(ctx context.Context, deps MigrateDeps, projectID string, dryRun bool) error`
- `type MigrateDeps struct { Repo Repository; Provisioner *Provisioner; Resolver *Resolver; Local *client.Sandbox; Log *slog.Logger }`。

- [ ] **Step 1: 写失败测试**

```go
func TestMigrateProjectExportsImportsAndVerifies(t *testing.T) {
	archive := tarGz(t, map[string]string{"index.html": "hi"})
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/projects/p1/archive" {
			t.Fatalf("local path = %s", r.URL.Path)
		}
		_, _ = w.Write(archive)
	}))
	defer local.Close()
	// e2b provisioner + sandbox server 断言 /archive/import
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/e2b -run TestMigrate -v`  
Expected: FAIL。

- [ ] **Step 3: 实现**

流程：

```go
type MigrateDeps struct {
	Repo        Repository
	Provisioner *Provisioner
	Resolver    *Resolver
	Local       *client.Sandbox
	Log         *slog.Logger
}

func MigrateProject(ctx context.Context, deps MigrateDeps, projectID string, dryRun bool) error {
if binding.Instance == nil { return ErrProjectNotFound }
if binding.Instance.Provider == "e2b" { return nil }
local := client.NewSandbox(binding.Instance.BaseURL(), obs)
archive := local.ExportArchive(ctx, projectID)
if dryRun { log size; return nil }
oldID := binding.Instance.ID
newID := provisioner.InitProject(ctx, projectID)
e2bBinding := repo.GetProjectBinding(projectID)
endpoint := resolver.ResolveEndpoint(ctx, projectID)
remote := client.NewSandboxWithHeaders(endpoint.BaseURL, endpoint.Headers, obs)
remote.ImportArchive(ctx, projectID, archive)
verify := compareStats(local.ProjectStats, remote.ProjectStats)
if !verify {
  _ = deps.Repo.UpdateProjectBinding(ctx, binding.ProjectInternalID, &newID, oldID)
  return ErrVerifyFailed
}
return nil
}

func compareStats(a, b client.ProjectStats) bool {
	return a.FileCount == b.FileCount && a.FileSizeBytes == b.FileSizeBytes
}
```

- [ ] **Step 4: CLI**

`cmd/sandbox-migrate` 读 `SANDBOX_MIGRATE_PROJECT` / `--project`、`--dry-run`、`--batch`，连接 MySQL，遍历 `project` 表 `deleted_at IS NULL AND sandbox_id IS NOT NULL`，逐项目迁移并输出 JSON 报告。

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && gofmt -w internal/e2b cmd/sandbox-migrate && go test ./internal/e2b -run TestMigrate -v`  
Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/backend/internal/e2b packages/backend/cmd/sandbox-migrate
git commit -m "feat: migrate project workspaces to e2b"
```

---

### Task 13: Helm / 部署配置

**Files:**
- Modify: `deployment/helm/backend/values.yaml`
- Modify: `deployment/helm/backend/templates/configmap.yaml`
- Modify: `deployment/helm/backend/templates/deployment-cp.yaml`
- Modify: `deployment/helm/backend/templates/deployment-dp.yaml`
- Modify: `deployment/helm/backend/templates/deployment-combined.yaml`
- Create: `deployment/e2b/README.md`

**Interfaces:**
- Helm secret `e2b.apiKey`；configmap 注入 `SANDBOX_PROVIDER` / `E2B_*` / `EGRESS_PROXY_URL`。

- [ ] **Step 1: 增加 values**

```yaml
config:
  sandboxProvider: local
  e2bApiUrl: ""
  e2bTemplateId: ""
  e2bDomain: ""
  e2bSandboxTimeout: 30m
  e2bAllowOut: ""
  e2bTrafficTokenTTL: 5m
  e2bExistingSecret: ""
  e2bApiKeyKey: api-key
  egressProxyUrl: ""
```

- [ ] **Step 2: configmap**

```yaml
SANDBOX_PROVIDER: {{ .Values.config.sandboxProvider | quote }}
E2B_API_URL: {{ .Values.config.e2bApiUrl | quote }}
E2B_TEMPLATE_ID: {{ .Values.config.e2bTemplateId | quote }}
E2B_DOMAIN: {{ .Values.config.e2bDomain | quote }}
E2B_SANDBOX_TIMEOUT: {{ .Values.config.e2bSandboxTimeout | quote }}
E2B_ALLOW_OUT: {{ .Values.config.e2bAllowOut | quote }}
E2B_TRAFFIC_TOKEN_TTL: {{ .Values.config.e2bTrafficTokenTTL | quote }}
EGRESS_PROXY_URL: {{ .Values.config.egressProxyUrl | quote }}
```

Secret 引用：

```yaml
- name: E2B_API_KEY
  valueFrom:
    secretKeyRef:
      name: {{ .Values.config.e2bExistingSecret }}
      key: {{ .Values.config.e2bApiKeyKey }}
```

- [ ] **Step 3: E2B Embed runbook**

`deployment/e2b/README.md` 必须写清：

- Embed 需要 Linux x86-64 + KVM；Docker Compose 需独占主机；K8s 需 KVM 节点。
- `E2B_API_URL` / team API key 从 Embed 安装输出获取。
- 构建 template 的命令和 `E2B_TEMPLATE_ID` 写入 Helm values。
- CP/DP 到 E2B API 的 NetworkPolicy。
- 切流顺序、迁移命令、回滚步骤。

- [ ] **Step 4: 验证模板**

Run:

```bash
helm template vibebiz deployment/helm/backend >/tmp/vibebiz-helm.yaml
python3 -c "import yaml; list(yaml.safe_load_all(open('/tmp/vibebiz-helm.yaml')))"
```

Expected: 无报错；configmap 出现 `SANDBOX_PROVIDER`。

- [ ] **Step 5: Commit**

```bash
git add deployment/helm/backend deployment/e2b
git commit -m "feat: deploy e2b sandbox provider config"
```

---

### Task 14: staging 冒烟与切流验收

**Files:**
- Create: `deployment/e2b/smoke.py`
- Create: `deployment/e2b/cutover.md`
- Modify: `docs/TECHNICAL.md`（E2B 架构与限制）

- [ ] **Step 1: 写 smoke.py**

覆盖：

```python
from e2b import Sandbox
import httpx

sandbox = Sandbox.create(template="vibebiz-sandbox-v1", timeout=600, network={"allow_public_traffic": False})
health = sandbox.commands.run("curl -fsS http://127.0.0.1:8000/health")
assert health.exit_code == 0
url = f"https://{sandbox.get_host(8000)}"
assert httpx.get(url).status_code == 403
assert httpx.get(url, headers={"e2b-traffic-access-token": sandbox.traffic_access_token}).status_code == 200
```

再覆盖：

- 写入文件，pause，connect，读回。
- `curl` 到不在 `allowOut` 的域名失败。
- 不带 token 访问 preview 403。
- 一个项目一个 volume；两个项目文件互不可见。

- [ ] **Step 2: 运行 staging**

Run:

```bash
E2B_API_KEY=... E2B_API_URL=https://e2b.internal python3 deployment/e2b/smoke.py
```

Expected: 输出每项 `ok`；失败时打印 sandbox ID、URL、状态码。

- [ ] **Step 3: 切流验收**

Runbook 顺序：

1. `SANDBOX_PROVIDER=local` 部署，确认旧链路。
2. 构建 template，配置 E2B，切换 `SANDBOX_PROVIDER=e2b`。
3. 创建新项目，验证文件/聊天/预览/发布。
4. `sandbox-migrate --dry-run` 选中一个项目。
5. 正式迁移一个项目，验证统计/文件哈希。
6. 观察 24h，再批量迁移。
7. 回滚：把 `project.sandbox_id` 指回 local 实例，`SANDBOX_PROVIDER=local`。

- [ ] **Step 4: 更新文档**

`docs/TECHNICAL.md` 增加：

- E2B Embed 需要 KVM，不是普通 Docker 集群。
- E2B 模式下 sandbox 不再挂宿主 workspace；使用 per-project volume。
- 动态 skills 的限制。
- 迁移/回滚命令。

- [ ] **Step 5: 最终验证**

Run:

```bash
make validate
cd packages/backend && go test ./... && go vet ./...
```

Expected: 全绿。

- [ ] **Step 6: Commit**

```bash
git add deployment/e2b docs/TECHNICAL.md
git commit -m "docs: add e2b staging runbook and smoke test"
```

---

## Self-Review

- Spec coverage：每项目 microVM（Task 5/6）、volume（Task 5）、private traffic（Task 4/8）、egress（Task 11）、发布（Task 9）、迁移/回滚（Task 12/14）、双运行（Task 7）、template（Task 10）、部署（Task 13）均有任务。
- 未覆盖：动态 skills 热更新、定时任务 utility sandbox、E2B 指标采集、跨副本 lease fencing；这些明确在 spec 的 Phase 1 边界/Phase 2，不在本计划。
- 类型一致性：`client.Endpoint`、`ProjectResolver.ResolveEndpoint`、`Provisioner.InitProject`、`Repository.CreateExternal` 在后续任务中保持一致。
- 每个任务都有独立测试/验证命令和 commit。
