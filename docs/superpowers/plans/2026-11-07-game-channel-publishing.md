# 游戏渠道发行包 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为项目生成微信小游戏、抖音小游戏和 Steam 桌面渠道包，并提供只读接入指引。

**Architecture:** CP 新增 `internal/channelpkg`，从工作区或项目资源取源，经适配器生成 zip，落盘并把记录写入 MySQL。Web 在现有发布弹窗内生成和下载，设置页只展示指引。现有分享发布链路不变。

**Tech Stack:** Go 1.24 + Echo v4 + GORM/MySQL 8 + `archive/zip` + `golang.org/x/net/html`；Vue 3 + `<script setup lang="ts">` + PrimeVue + vitest。

**Spec:** `docs/superpowers/specs/2026-11-07-game-channel-publishing-design.md`

## Global Constraints

- 统一响应 `{ code, message, result }`，成功 `code=0`。
- CP 路由前缀 `/cp/api/v1`；不注册 `/dp`；不改 sandbox。
- 所有项目接口必须校验登录和工作区成员关系。
- 产物路径必须 `filepath.Rel` 校验位于 `CHANNEL_PACKAGES_DIR` 内。
- zip 解压拒绝绝对路径、`..`、符号链接、设备文件。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 不新增 AI / 观测 SDK 到前端。
- 先写失败测试再实现；每个任务结束运行对应测试并提交。

---

### Task 1: 迁移、模型与 Repository

**Files:**
- Create: `packages/backend/migrations/00028_project_channel_package.sql`
- Create: `packages/backend/internal/dao/model/channel_package.go`
- Create: `packages/backend/internal/dao/channel_package_repository.go`
- Modify: `packages/backend/migrations/migrations_test.go`
- Test: `packages/backend/internal/dao/channel_package_repository_test.go`

**Interfaces:**
- Produces: `model.ProjectChannelPackage`
- Produces: `dao.ChannelPackageRepository` with `Create/Update/GetByPublicID/ListByProject/Delete`

- [ ] **Step 1: 写迁移**

```sql
-- +goose Up

CREATE TABLE `project_channel_package` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '渠道包公开ID',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `source_type` varchar(16) NOT NULL COMMENT '来源：html5/engine_artifact',
  `source_resource_id` char(36) DEFAULT NULL COMMENT '引擎成品资源ID',
  `channel` varchar(32) NOT NULL COMMENT '渠道',
  `status` varchar(16) NOT NULL COMMENT 'building/ready/failed',
  `version` varchar(64) NOT NULL DEFAULT '' COMMENT '来源版本',
  `file_path` varchar(512) NOT NULL DEFAULT '' COMMENT '相对产物根目录路径',
  `file_size_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '产物大小',
  `manifest` json DEFAULT NULL COMMENT 'package-manifest 摘要',
  `error_message` varchar(1024) NOT NULL DEFAULT '' COMMENT '失败原因',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_channel_package_public` (`public_id`),
  KEY `idx_project_channel_package_project` (`project_public_id`, `channel`, `created_at`),
  KEY `idx_project_channel_package_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目渠道发行包';

-- +goose Down

DROP TABLE IF EXISTS `project_channel_package`;
```

- [ ] **Step 2: 写模型**

`packages/backend/internal/dao/model/channel_package.go`:

```go
package model

import "time"

const (
	ChannelPackageStatusBuilding = "building"
	ChannelPackageStatusReady    = "ready"
	ChannelPackageStatusFailed   = "failed"
)

type ProjectChannelPackage struct {
	Base
	PublicID         string    `gorm:"column:public_id" json:"publicId"`
	ProjectPublicID  string    `gorm:"column:project_public_id" json:"projectPublicId"`
	WorkspaceID      int64     `gorm:"column:workspace_id" json:"workspaceId"`
	SourceType       string    `gorm:"column:source_type" json:"sourceType"`
	SourceResourceID *string   `gorm:"column:source_resource_id" json:"sourceResourceId,omitempty"`
	Channel          string    `gorm:"column:channel" json:"channel"`
	Status           string    `gorm:"column:status" json:"status"`
	Version          string    `gorm:"column:version" json:"version"`
	FilePath         string    `gorm:"column:file_path" json:"filePath"`
	FileSizeBytes    int64     `gorm:"column:file_size_bytes" json:"fileSizeBytes"`
	Manifest         JSON      `gorm:"column:manifest;type:json" json:"manifest,omitempty"`
	ErrorMessage     string    `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (ProjectChannelPackage) TableName() string { return "project_channel_package" }
```

- [ ] **Step 3: 写 Repository**

`packages/backend/internal/dao/channel_package_repository.go`:

```go
package dao

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type ChannelPackageRepository struct {
	db *gorm.DB
}

func NewChannelPackageRepository(db *gorm.DB) *ChannelPackageRepository {
	return &ChannelPackageRepository{db: db}
}

func (r *ChannelPackageRepository) Create(ctx context.Context, item *model.ProjectChannelPackage) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create channel package: %w", err)
	}
	return nil
}

func (r *ChannelPackageRepository) Update(ctx context.Context, item *model.ProjectChannelPackage) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update channel package: %w", err)
	}
	return nil
}

func (r *ChannelPackageRepository) GetByPublicID(ctx context.Context, publicID string) (*model.ProjectChannelPackage, error) {
	var item model.ProjectChannelPackage
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get channel package: %w", err)
	}
	return &item, nil
}

func (r *ChannelPackageRepository) ListByProject(ctx context.Context, projectPublicID string) ([]model.ProjectChannelPackage, error) {
	var items []model.ProjectChannelPackage
	if err := r.db.WithContext(ctx).
		Where("project_public_id = ?", projectPublicID).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list channel packages: %w", err)
	}
	return items, nil
}

func (r *ChannelPackageRepository) Delete(ctx context.Context, publicID string) error {
	result := r.db.WithContext(ctx).Where("public_id = ?", publicID).Delete(&model.ProjectChannelPackage{})
	if result.Error != nil {
		return fmt.Errorf("delete channel package: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
```

- [ ] **Step 4: 写失败测试**

`channel_package_repository_test.go` 至少覆盖 Create、GetByPublicID、ListByProject、Delete。测试用 `newMockDB(t)` 和 `sqlmock`，断言 SQL 包含 `project_channel_package`。示例：

```go
func TestChannelPackageRepositoryCreate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project_channel_package`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	item := &model.ProjectChannelPackage{PublicID: "pkg-1", ProjectPublicID: "p1", Channel: "wechat_minigame"}
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 5: 更新迁移测试**

在 `TestInitialSchemaCoversAllBusinessTables` 的 `tables` 列表加入 `"project_channel_package"`。

- [ ] **Step 6: 运行测试**

Run: `cd packages/backend && go test ./internal/dao/... ./migrations/...`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add packages/backend/migrations/00028_project_channel_package.sql \
  packages/backend/migrations/migrations_test.go \
  packages/backend/internal/dao/model/channel_package.go \
  packages/backend/internal/dao/channel_package_repository.go \
  packages/backend/internal/dao/channel_package_repository_test.go
git commit -m "feat: add channel package table and repository"
```

---

### Task 2: 资源用途 channel_build 与读取接口

**Files:**
- Modify: `packages/backend/internal/resource/service.go`
- Test: `packages/backend/internal/resource/service_test.go`

**Interfaces:**
- Produces: `resource.PurposeChannelBuild`
- Produces: `(*resource.Service).OpenReadyChannelBuild(ctx, projectID, resourceID) (io.ReadCloser, int64, error)`

- [ ] **Step 1: 写失败测试**

```go
func TestPresignChannelBuildAcceptsZip(t *testing.T) {
	repo := newFakeRepo()
	s3 := &fakeS3{}
	svc := NewService(repo, s3, nil)
	result, err := svc.Presign(context.Background(), 3, 7, PresignInput{
		Filename: "game-web-mobile.zip", ContentType: "application/zip",
		Size: 1024, Purpose: PurposeChannelBuild,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ResourceID == "" {
		t.Fatal("resource id is empty")
	}
}

func TestPresignChannelBuildRejectsNonZip(t *testing.T) {
	svc := NewService(newFakeRepo(), &fakeS3{}, nil)
	_, err := svc.Presign(context.Background(), 3, 7, PresignInput{
		Filename: "game.exe", ContentType: "application/octet-stream",
		Size: 1024, Purpose: PurposeChannelBuild,
	})
	if !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("err = %v, want ErrInvalidFile", err)
	}
}

func TestOpenReadyChannelBuildRejectsWrongProject(t *testing.T) {
	repo := newFakeRepo()
	item := &model.ProjectResource{
		ResourceID: "r1", ProjectPublicID: stringPtr("p2"), Purpose: PurposeChannelBuild,
		Status: "ready", S3Key: "k1",
	}
	repo.items["r1"] = item
	svc := NewService(repo, &fakeS3{}, nil)
	if _, _, err := svc.OpenReadyChannelBuild(context.Background(), "p1", "r1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/resource/ -run ChannelBuild -v`
Expected: FAIL，`PurposeChannelBuild` 未定义。

- [ ] **Step 3: 实现用途与读取**

在 `service.go` 常量区加入：

```go
const (
	PurposeChannelBuild = "channel_build"

	MaxChannelBuildFileBytes    int64 = 500 << 20
	MaxChannelBuildProjectBytes int64 = 500 << 20
)
```

`normalizePurpose` 加入 `PurposeChannelBuild`；`purposeLimits` 加入：

```go
case PurposeChannelBuild:
	return MaxChannelBuildFileBytes, MaxChannelBuildProjectBytes
```

`validateForPurpose` 加入：

```go
case PurposeChannelBuild:
	if ext != ".zip" {
		return "", "", ErrInvalidFile
	}
	contentType := normalizeContentType(rawType)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = "application/zip"
	}
	if !zipMIME[contentType] {
		return "", "", ErrInvalidFile
	}
	return contentType, "document", nil
```

新增方法：

```go
func (s *Service) OpenReadyChannelBuild(ctx context.Context, projectID, resourceID string) (io.ReadCloser, int64, error) {
	if s.S3 == nil {
		return nil, 0, ErrUnavailable
	}
	item, err := s.Repo.GetByResourceID(ctx, resourceID)
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return nil, 0, ErrNotFound
		}
		return nil, 0, err
	}
	if item.ProjectPublicID == nil || *item.ProjectPublicID != projectID ||
		item.Status != "ready" || item.Purpose != PurposeChannelBuild {
		return nil, 0, ErrNotFound
	}
	return s.S3.Get(ctx, item.S3Key)
}
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/resource/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/resource/service.go packages/backend/internal/resource/service_test.go
git commit -m "feat: add channel build resource purpose"
```

---

### Task 3: channelpkg 公共类型、兼容性分析、zip 工具

**Files:**
- Create: `packages/backend/internal/channelpkg/core/types.go`
- Create: `packages/backend/internal/channelpkg/core/analyze.go`
- Create: `packages/backend/internal/channelpkg/core/ziputil.go`
- Test: `packages/backend/internal/channelpkg/core/analyze_test.go`
- Test: `packages/backend/internal/channelpkg/core/ziputil_test.go`

**Interfaces:**
- Produces: `core.ChannelWeChat/Douyin/Steam`
- Produces: `core.SourceHTML5/EngineArtifact`
- Produces: `core.LevelA/LevelB/LevelC`
- Produces: `core.SourceInfo`, `core.Report`, `core.Manifest`, `core.Adapter`
- Produces: `core.AnalyzeHTML(dir, entry string) (Report, error)`
- Produces: `core.WriteFile`, `core.WriteJSON`, `core.CopyDirToZip`, `core.ExtractZip`

- [ ] **Step 1: 写失败分析测试**

```go
func TestAnalyzeHTMLClassifiesCanvasGame(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<html><body><canvas id="game"></canvas><script src="game.js"></script></body></html>`)
	write(t, dir, "game.js", `const c = document.getElementById('game'); const x = c.getContext('2d'); x.fillRect(0,0,10,10);`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelB {
		t.Fatalf("level = %s, want B; report=%+v", report.Level, report)
	}
}

func TestAnalyzeHTMLClassifiesDomGameAsC(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<div id="app"></div><script src="app.js"></script>`)
	write(t, dir, "app.js", `document.querySelector('#app').innerHTML = '<b>x</b>'; document.querySelectorAll('div');`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelC {
		t.Fatalf("level = %s, want C; report=%+v", report.Level, report)
	}
}

func TestAnalyzeHTMLRecognizesExistingMiniGame(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.js", "console.log('game')")
	write(t, dir, "game.json", `{"deviceOrientation":"portrait"}`)
	report, err := AnalyzeHTML(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelA {
		t.Fatalf("level = %s, want A", report.Level)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/core/...`
Expected: FAIL，包或符号不存在。

- [ ] **Step 3: 写类型**

`core/types.go`:

```go
package core

import (
	"context"
	"io"
	"time"
)

const (
	ChannelWeChat = "wechat_minigame"
	ChannelDouyin = "douyin_minigame"
	ChannelSteam  = "steam_desktop"

	SourceHTML5          = "html5"
	SourceEngineArtifact = "engine_artifact"

	LevelA = "A"
	LevelB = "B"
	LevelC = "C"
)

type Report struct {
	Level    string   `json:"level"`
	Warnings []string `json:"warnings"`
	Blockers []string `json:"blockers"`
}

type SourceInfo struct {
	Dir        string `json:"-"`
	Entry      string `json:"entry"`
	Version    string `json:"version"`
	FileCount  int    `json:"fileCount"`
	TotalBytes int64  `json:"totalBytes"`
	Report     Report `json:"report"`
}

type Manifest struct {
	ProjectID      string    `json:"projectId"`
	Channel        string    `json:"channel"`
	SourceType     string    `json:"sourceType"`
	SourceVersion  string    `json:"sourceVersion"`
	AdapterVersion string    `json:"adapterVersion"`
	Entry          string    `json:"entry"`
	FileCount      int       `json:"fileCount"`
	TotalBytes     int64     `json:"totalBytes"`
	GeneratedAt    time.Time `json:"generatedAt"`
	Level          string    `json:"level"`
	Warnings       []string  `json:"warnings"`
}

type Adapter interface {
	Channel() string
	Validate(source SourceInfo) Report
	Build(ctx context.Context, source SourceInfo, out io.Writer) (Manifest, error)
}
```

- [ ] **Step 4: 写分析器**

`core/analyze.go` 要求：

- 输入目录和入口；入口为空时查找 `index.html`、`app.html`。
- 若根目录同时存在 `game.js` 和 `game.json`，返回 `LevelA`。
- 遍历 `.html`、`.js`、`.css`，每个文件最多读 2MB。
- 统计 `document.querySelector`、`document.querySelectorAll`、`innerHTML`、`document.createElement`、`addEventListener`、`localStorage`、`fetch(`、`http://`、`https://`。
- 有 `<canvas` 且 DOM 重度标记数 `<= 2` → `LevelB`；否则 `LevelC`。
- 入口不存在 → 返回 `ErrInvalidSource`。
- 外部 URL 和 `fetch(` 写入 warnings。
- 强 DOM 时 blockers 写入 `dom_dependency`。

实现时用 `strings.Count(text, pattern)`，不要引入 HTML 解析器。

- [ ] **Step 5: 写 zip 工具测试**

```go
func TestCopyDirToZipRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "link")); err != nil {
		t.Skip("symlink unsupported")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, _, err := CopyDirToZip(context.Background(), zw, dir, "", nil); err == nil {
		t.Fatal("expected symlink error")
	}
}

func TestExtractZipRejectsZipSlip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("../escape.txt")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	err := ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 10})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("err = %v, want ErrUnsafeArchive", err)
	}
}
```

- [ ] **Step 6: 实现 zip 工具**

`core/ziputil.go` 提供：

```go
var (
	ErrUnsafeArchive = errors.New("unsafe archive")
	ErrArchiveTooLarge = errors.New("archive too large")
)

type ExtractLimits struct {
	MaxBytes int64
	MaxFiles int
}

func WriteFile(zw *zip.Writer, name string, data []byte) error
func WriteJSON(zw *zip.Writer, name string, value any) error
func CopyDirToZip(ctx context.Context, zw *zip.Writer, src, prefix string, skip func(rel string) bool) (int, int64, error)
func ExtractZip(ctx context.Context, src io.Reader, dst string, limits ExtractLimits) error
```

实现要求：

- `CopyDirToZip` 拒绝符号链接，跳过 `skip` 命中的路径。
- `ExtractZip` 用 `filepath.Rel` 校验目标在 `dst` 内；拒绝绝对路径、`..`、符号链接；累计大小和文件数超限返回 `ErrArchiveTooLarge`；压缩比超过 100:1 返回 `ErrUnsafeArchive`。
- 写入前 `os.MkdirAll(filepath.Dir(target))`。
- 文件名统一用 `/`，写入前 `filepath.ToSlash`。

- [ ] **Step 7: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/core/...`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add packages/backend/internal/channelpkg/core
git commit -m "feat: add channel package core types and zip utilities"
```

---

### Task 4: 工作区来源、资源来源与版本计算

**Files:**
- Create: `packages/backend/internal/channelpkg/source.go`
- Test: `packages/backend/internal/channelpkg/source_test.go`

**Interfaces:**
- Consumes: `core.SourceInfo`, `core.AnalyzeHTML`, `core.ExtractZip`
- Produces: `NewWorkspaceSource(root, projectID string) (*WorkspaceSource, error)`
- Produces: `(*WorkspaceSource).Materialize(ctx) (core.SourceInfo, error)`
- Produces: `NewResourceSource(projectID, resourceID string, opener ResourceOpener, tempRoot string) *ResourceSource`
- Produces: `(*ResourceSource).Materialize(ctx) (core.SourceInfo, error)`

```go
type ResourceOpener func(ctx context.Context, projectID, resourceID string) (io.ReadCloser, int64, error)
```

- [ ] **Step 1: 写失败测试**

```go
func TestWorkspaceSourceUsesIndexEntryAndVersion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('c').getContext('2d')`)
	src, err := NewWorkspaceSource(root, "p1")
	if err != nil {
		t.Fatal(err)
	}
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Entry != "index.html" || info.Version == "" || info.FileCount != 2 {
		t.Fatalf("info = %+v", info)
	}
}

func TestWorkspaceSourceRejectsMissingProject(t *testing.T) {
	src, err := NewWorkspaceSource(t.TempDir(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.Materialize(context.Background()); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("err = %v, want ErrSourceNotFound", err)
	}
}

func TestResourceSourceExtractsZipAndComputesVersion(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("index.html")
	_, _ = w.Write([]byte("<canvas></canvas>"))
	_ = zw.Close()
	opener := func(context.Context, string, string) (io.ReadCloser, int64, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), int64(buf.Len()), nil
	}
	src := NewResourceSource("p1", "r1", opener, t.TempDir())
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Entry != "index.html" || info.Version == "" {
		t.Fatalf("info = %+v", info)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/ -run Source -v`
Expected: FAIL。

- [ ] **Step 3: 实现来源**

`source.go` 要求：

```go
var (
	ErrSourceNotFound = errors.New("channel package source not found")
	ErrInvalidSource  = errors.New("invalid channel package source")
)

var projectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

type WorkspaceSource struct {
	Root      string
	ProjectID string
}

func NewWorkspaceSource(root, projectID string) (*WorkspaceSource, error) {
	if !projectIDPattern.MatchString(projectID) {
		return nil, ErrInvalidSource
	}
	return &WorkspaceSource{Root: root, ProjectID: projectID}, nil
}

func (s *WorkspaceSource) Materialize(ctx context.Context) (core.SourceInfo, error) {
	dir := filepath.Join(s.Root, s.ProjectID)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return core.SourceInfo{}, ErrSourceNotFound
	}
	entry, err := findEntry(dir)
	if err != nil && !hasMiniGame(dir) {
		return core.SourceInfo{}, err
	}
	report, err := core.AnalyzeHTML(dir, entry)
	if err != nil {
		return core.SourceInfo{}, err
	}
	count, total, version, err := dirStats(ctx, dir)
	if err != nil {
		return core.SourceInfo{}, err
	}
	return core.SourceInfo{Dir: dir, Entry: entry, Version: version, FileCount: count, TotalBytes: total, Report: report}, nil
}
```

`findEntry` 查找 `index.html`、`app.html`；两者都没有时，若存在 `game.js` + `game.json` 返回空 entry（A 级小游戏源）。`dirStats` 排除点目录、`docs`、`node_modules`，对每个文件计算 sha256，并把 `(相对路径, 大小, hash)` 写入 `sha256` 后取前 16 位。

`ResourceSource` 要求：

```go
type ResourceSource struct {
	ProjectID  string
	ResourceID string
	Opener     ResourceOpener
	TempRoot   string
}

func (s *ResourceSource) Materialize(ctx context.Context) (core.SourceInfo, error) {
	body, _, err := s.Opener(ctx, s.ProjectID, s.ResourceID)
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			return core.SourceInfo{}, ErrSourceNotFound
		}
		return core.SourceInfo{}, err
	}
	defer body.Close()
	dir, err := os.MkdirTemp(s.TempRoot, ".channel-src-"+s.ResourceID+"-")
	if err != nil {
		return core.SourceInfo{}, err
	}
	hasher := sha256.New()
	if err := core.ExtractZip(ctx, io.TeeReader(body, hasher), dir, core.ExtractLimits{MaxBytes: 1 << 30, MaxFiles: 20000}); err != nil {
		_ = os.RemoveAll(dir)
		return core.SourceInfo{}, err
	}
	entry, err := findEntry(dir)
	if err != nil && !hasMiniGame(dir) {
		_ = os.RemoveAll(dir)
		return core.SourceInfo{}, err
	}
	report, err := core.AnalyzeHTML(dir, entry)
	if err != nil {
		_ = os.RemoveAll(dir)
		return core.SourceInfo{}, err
	}
	count, total, _, err := dirStats(ctx, dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return core.SourceInfo{}, err
	}
	version := hex.EncodeToString(hasher.Sum(nil))[:16]
	return core.SourceInfo{Dir: dir, Entry: entry, Version: version, FileCount: count, TotalBytes: total, Report: report}, nil
}

func (s *ResourceSource) Cleanup() error { return nil }
```

服务层负责清理资源源的临时目录；`Materialize` 返回的 `Dir` 可由服务层 `os.RemoveAll`。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/channelpkg/source.go packages/backend/internal/channelpkg/source_test.go
git commit -m "feat: add workspace and resource channel sources"
```

---

### Task 5: 微信小游戏适配器

**Files:**
- Create: `packages/backend/internal/channelpkg/adapters/wechat/adapter.go`
- Create: `packages/backend/internal/channelpkg/adapters/wechat/adapter.js`
- Test: `packages/backend/internal/channelpkg/adapters/wechat/adapter_test.go`

**Interfaces:**
- Consumes: `core.SourceInfo`, `core.WriteFile`, `core.WriteJSON`, `core.CopyDirToZip`, `guides.Markdown`
- Produces: `wechat.New() *Adapter`，实现 `core.Adapter`

- [ ] **Step 1: 写失败测试**

```go
func TestWeChatBuildWritesMiniGameFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('game').getContext('2d')`)
	var buf bytes.Buffer
	adapter := New()
	manifest, err := adapter.Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != core.ChannelWeChat {
		t.Fatalf("channel = %s", manifest.Channel)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{"game.js", "game.json", "project.config.json", "adapter/index.js", "package-manifest.json", "CHANNEL_GUIDE.md"} {
		if !names[want] {
			t.Errorf("missing %s in %v", want, names)
		}
	}
}

func TestWeChatBuildRejectsDomGame(t *testing.T) {
	adapter := New()
	report := adapter.Validate(core.SourceInfo{Report: core.Report{Level: core.LevelC}})
	if len(report.Blockers) == 0 {
		t.Fatal("expected blocker for level C")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/wechat/...`
Expected: FAIL。

- [ ] **Step 3: 写适配层**

`adapter.js` 用统一平台探测：

```js
(function (global) {
  var api = typeof wx !== 'undefined' ? wx : tt;
  var canvas = api.createCanvas();
  var ctx = canvas.getContext('2d');
  global.window = global;
  global.canvas = canvas;
  global.document = {
    getElementById: function () { return canvas; },
    querySelector: function () { return canvas; },
    querySelectorAll: function () { return [canvas]; },
    createElement: function (tag) {
      if (tag === 'canvas') return canvas;
      return { style: {}, appendChild: function () {}, addEventListener: function () {} };
    },
    addEventListener: function () {},
    body: { appendChild: function () {}, style: {} }
  };
  global.navigator = { userAgent: 'minigame' };
  global.devicePixelRatio = api.getSystemInfoSync().pixelRatio;
  global.innerWidth = canvas.width;
  global.innerHeight = canvas.height;
  global.requestAnimationFrame = api.requestAnimationFrame || function (cb) {
    return setTimeout(function () { cb(Date.now()); }, 16);
  };
  global.addEventListener = function () {};
  global.localStorage = {
    getItem: function (k) { return api.getStorageSync(k) || null; },
    setItem: function (k, v) { api.setStorageSync(k, v); },
    removeItem: function (k) { api.removeStorageSync(k); }
  };
  global.Image = function () { return api.createImage(); };
  global.fetch = function (url, options) {
    return new Promise(function (resolve, reject) {
      api.request({
        url: url, method: (options && options.method) || 'GET',
        data: options && options.body, success: resolve, fail: reject
      });
    });
  };
  window.__channelAdapter = true;
})(typeof GameGlobal !== 'undefined' ? GameGlobal : globalThis);
```

`adapter.go` 要求：

- `Channel()` 返回 `core.ChannelWeChat`。
- `Validate` 在 `LevelC` 时追加 blocker `dom_dependency`；否则返回原 report。
- `Build`：
  1. `zip.NewWriter(out)`；
  2. 若 `source.Entry != ""`，用 `core.CopyDirToZip` 把源复制到 `game/`；
  3. 从 `game/index.html` 抽取 `<script>`，把内联和相对路径脚本按顺序合并为 `game/game.bundle.js`；使用 `golang.org/x/net/html`；
  4. 写 `adapter/index.js`（`adapter.js` 内容）；
  5. 写 `game.js`：`require('./adapter/index.js'); require('./game/game.bundle.js');`
  6. 写 `game.json`、`project.config.json`（`appid: "touristappid"`、`compileType: "game"`）；
  7. 写 `CHANNEL_GUIDE.md` 和 `package-manifest.json`；
  8. 返回 `core.Manifest`。
- A 级源（已有 `game.js` + `game.json`）直接复制根文件，不生成 bundle，不写适配层。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/wechat/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/channelpkg/adapters/wechat
git commit -m "feat: add wechat minigame adapter"
```

---

### Task 6: 抖音小游戏适配器

**Files:**
- Create: `packages/backend/internal/channelpkg/adapters/douyin/adapter.go`
- Create: `packages/backend/internal/channelpkg/adapters/douyin/adapter.js`
- Test: `packages/backend/internal/channelpkg/adapters/douyin/adapter_test.go`

**Interfaces:**
- Consumes: 与微信适配器相同的 `core` 工具
- Produces: `douyin.New() *Adapter`，实现 `core.Adapter`

- [ ] **Step 1: 写失败测试**

```go
func TestDouyinBuildWritesTTProject(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('game').getContext('2d')`)
	var buf bytes.Buffer
	adapter := New()
	manifest, err := adapter.Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != core.ChannelDouyin {
		t.Fatalf("channel = %s", manifest.Channel)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{"game.js", "game.json", "project.config.json", "adapter/index.js", "package-manifest.json", "CHANNEL_GUIDE.md"} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/douyin/...`
Expected: FAIL。

- [ ] **Step 3: 实现**

复制微信适配器结构，差异：

- `Channel()` 返回 `core.ChannelDouyin`。
- `project.config.json` 使用 `appid: "testappid"`，字段包含 `compileType: "game"`、`projectname`、`setting`。
- `adapter.js` 使用 `tt` 优先（`var api = typeof tt !== 'undefined' ? tt : wx;`）。
- `game.js` 同样先加载适配层再加载 bundle。
- 指引文件来自 `guides.Markdown(core.ChannelDouyin)`。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/douyin/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/channelpkg/adapters/douyin
git commit -m "feat: add douyin minigame adapter"
```

---

### Task 7: Steam 桌面适配器

**Files:**
- Create: `packages/backend/internal/channelpkg/adapters/steam/adapter.go`
- Create: `packages/backend/internal/channelpkg/adapters/steam/templates.go`
- Test: `packages/backend/internal/channelpkg/adapters/steam/adapter_test.go`

**Interfaces:**
- Consumes: `core.SourceInfo`, `core.CopyDirToZip`, `core.WriteFile`, `core.WriteJSON`, `guides.Markdown`
- Produces: `steam.New() *Adapter`，实现 `core.Adapter`

- [ ] **Step 1: 写失败测试**

```go
func TestSteamBuildWritesElectronShell(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas></canvas>`)
	var buf bytes.Buffer
	adapter := New()
	manifest, err := adapter.Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != core.ChannelSteam {
		t.Fatalf("channel = %s", manifest.Channel)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{
		"app/index.html", "electron/main.js", "electron/preload.js", "package.json",
		"electron-builder.yml", "build-steam.sh", "build-steam.ps1",
		"steam/app_build.vdf.example", "CHANNEL_GUIDE.md", "package-manifest.json",
	} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestSteamBuildDetectsNativeExe(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.exe", "MZ")
	adapter := New()
	report := adapter.Validate(core.SourceInfo{Dir: dir, FileCount: 1})
	if len(report.Warnings) == 0 {
		t.Fatal("expected warning for native build path")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/steam/...`
Expected: FAIL。

- [ ] **Step 3: 实现**

`templates.go` 存放 Electron 壳内容：

```go
const mainJS = `
const { app, BrowserWindow } = require('electron')
const path = require('path')
function createWindow() {
  const win = new BrowserWindow({
    width: 1280, height: 720,
    webPreferences: { preload: path.join(__dirname, 'preload.js'), contextIsolation: true }
  })
  win.loadFile(path.join(__dirname, '..', 'app', 'index.html'))
  win.setMenuBarVisibility(false)
}
app.whenReady().then(createWindow)
app.on('window-all-closed', () => { if (process.platform !== 'darwin') app.quit() })
`

const packageJSON = `{
  "name": "vibebiz-steam-game",
  "version": "1.0.0",
  "main": "electron/main.js",
  "scripts": {
    "start": "electron .",
    "dist": "electron-builder --win --x64"
  },
  "devDependencies": {
    "electron": "^33.0.0",
    "electron-builder": "^25.0.0"
  },
  "build": {
    "appId": "com.vibebiz.game",
    "productName": "VibebizGame",
    "directories": { "output": "dist" },
    "win": { "target": ["zip"] }
  }
}`

const vdf = `
"appbuild"
{
  "appid" "YOUR_APP_ID"
  "desc" "Vibebiz game build"
  "buildoutput" "..\\steam-output"
  "contentroot" "..\\dist\\win-unpacked"
  "depots"
  {
    "YOUR_DEPOT_ID" "depot_build.vdf"
  }
}
`
```

`adapter.go` 要求：

- `Channel()` 返回 `core.ChannelSteam`。
- `Validate`：若入口为空且目录内存在 `.exe`，返回 warnings `native_build_detected`；否则返回原 report。
- `Build`：
  - 若有 `index.html`/`app.html`：复制到 `app/`，写 `electron/main.js`、`electron/preload.js`、`package.json`、`electron-builder.yml`、`build-steam.sh`、`build-steam.ps1`。
  - 若检测到 `.exe`：复制全部文件到 `app/`，跳过 Electron 壳，只写 Steam VDF 示例。
  - 两种情况都写 `steam/app_build.vdf.example`、`CHANNEL_GUIDE.md`、`package-manifest.json`。

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/adapters/steam/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/channelpkg/adapters/steam
git commit -m "feat: add steam desktop adapter"
```

---

### Task 8: 渠道指引内容与 embed

**Files:**
- Create: `packages/backend/internal/channelpkg/guides/guides.go`
- Create: `packages/backend/internal/channelpkg/guides/wechat_minigame.md`
- Create: `packages/backend/internal/channelpkg/guides/douyin_minigame.md`
- Create: `packages/backend/internal/channelpkg/guides/steam_desktop.md`
- Test: `packages/backend/internal/channelpkg/guides/guides_test.go`

**Interfaces:**
- Produces: `guides.Markdown(channel string) string`
- Produces: `guides.List() []Item`，`Item{Channel,Title,Summary,Markdown,Steps,Requirements,UpdatedAt}`

- [ ] **Step 1: 写失败测试**

```go
func TestEveryChannelHasGuide(t *testing.T) {
	for _, channel := range []string{"wechat_minigame", "douyin_minigame", "steam_desktop"} {
		if strings.TrimSpace(Markdown(channel)) == "" {
			t.Errorf("missing guide for %s", channel)
		}
	}
}

func TestListHasStepsAndRequirements(t *testing.T) {
	items := List()
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	for _, item := range items {
		if len(item.Steps) == 0 || len(item.Requirements) == 0 || item.UpdatedAt == "" {
			t.Errorf("incomplete guide: %+v", item)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/guides/...`
Expected: FAIL。

- [ ] **Step 3: 写三份指引**

每份 markdown 必须包含以下小节，内容用真实平台要求，不要写“待补充”：

- `wechat_minigame.md`：企业主体、小游戏类目、软著、版号适用说明、AppID、微信开发者工具导入、真机调试、上传提审、包结构、常见失败。
- `douyin_minigame.md`：企业主体、软著、类目资质、AppID、抖音开发者工具、真机调试、提审、包结构、常见失败。
- `steam_desktop.md`：Steamworks 账号、100 美元 App 费、商店页素材、Depot 配置、SteamPipe、Electron 壳构建、启动项、常见失败。

- [ ] **Step 4: 实现 guides.go**

```go
package guides

import (
	"embed"
	"strings"
)

//go:embed *.md
var files embed.FS

type Item struct {
	Channel      string   `json:"channel"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
	UpdatedAt    string   `json:"updatedAt"`
}

type meta struct {
	title, summary, updated string
	steps, requirements     []string
}

var metas = map[string]meta{
	"wechat_minigame": {
		title: "微信小游戏", summary: "企业主体、软著、类目审核后导入微信开发者工具",
		updated: "2026-11-07",
		steps: []string{"生成渠道包", "注册小游戏 AppID", "导入微信开发者工具", "真机调试", "上传提审"},
		requirements: []string{"企业主体", "软件著作权", "对应类目资质"},
	},
	"douyin_minigame": {
		title: "抖音小游戏", summary: "企业主体、软著、类目资质后导入抖音开发者工具",
		updated: "2026-11-07",
		steps: []string{"生成渠道包", "注册小游戏 AppID", "导入抖音开发者工具", "真机调试", "上传提审"},
		requirements: []string{"企业主体", "软件著作权", "对应类目资质"},
	},
	"steam_desktop": {
		title: "Steam 桌面", summary: "Steamworks 账号、App 费、商店素材和 SteamPipe 上传",
		updated: "2026-11-07",
		steps: []string{"生成渠道包", "注册 Steamworks", "本地构建 Electron 壳", "配置 Depot 和 VDF", "SteamPipe 上传并提审"},
		requirements: []string{"Steamworks 账号", "100 美元 App 费", "商店页素材", "构建产物"},
	},
}

func Markdown(channel string) string {
	data, err := files.ReadFile(channel + ".md")
	if err != nil {
		return ""
	}
	return string(data)
}

func List() []Item {
	order := []string{"wechat_minigame", "douyin_minigame", "steam_desktop"}
	items := make([]Item, 0, len(order))
	for _, channel := range order {
		m := metas[channel]
		items = append(items, Item{
			Channel: channel, Title: m.title, Summary: m.summary,
			Markdown: Markdown(channel), Steps: m.steps,
			Requirements: m.requirements, UpdatedAt: m.updated,
		})
	}
	return items
}
```

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/guides/...`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add packages/backend/internal/channelpkg/guides
git commit -m "feat: add channel publishing guides"
```

---

### Task 9: 渠道包 Service 编排

**Files:**
- Create: `packages/backend/internal/channelpkg/service.go`
- Test: `packages/backend/internal/channelpkg/service_test.go`

**Interfaces:**
- Consumes: `core.Adapter`, `WorkspaceSource`, `ResourceSource`, `model.ProjectChannelPackage`, `Repository`
- Produces: `NewService(ServiceConfig) *Service`
- Produces: `(*Service).Generate(ctx, project *model.Project, input GenerateInput) (*model.ProjectChannelPackage, error)`
- Produces: `(*Service).List(ctx, projectID string) ([]model.ProjectChannelPackage, error)`
- Produces: `(*Service).Get(ctx, packageID string) (*model.ProjectChannelPackage, error)`
- Produces: `(*Service).Delete(ctx, projectID, packageID string) error`

```go
type Repository interface {
	Create(ctx context.Context, item *model.ProjectChannelPackage) error
	Update(ctx context.Context, item *model.ProjectChannelPackage) error
	GetByPublicID(ctx context.Context, publicID string) (*model.ProjectChannelPackage, error)
	ListByProject(ctx context.Context, projectPublicID string) ([]model.ProjectChannelPackage, error)
	Delete(ctx context.Context, publicID string) error
}

type ServiceConfig struct {
	Repo          Repository
	WorkspacesDir string
	PackagesDir   string
	OpenResource  ResourceOpener
	Adapters      map[string]core.Adapter
	MaxBytes      int64
	Timeout       time.Duration
	Concurrency   int
	Now           func() time.Time
	NewID         func() string
}
```

- [ ] **Step 1: 写失败测试**

```go
type fakeRepo struct{ items map[string]*model.ProjectChannelPackage }

func (f *fakeRepo) Create(_ context.Context, item *model.ProjectChannelPackage) error {
	f.items[item.PublicID] = item
	return nil
}
func (f *fakeRepo) Update(_ context.Context, item *model.ProjectChannelPackage) error {
	f.items[item.PublicID] = item
	return nil
}
func (f *fakeRepo) GetByPublicID(_ context.Context, id string) (*model.ProjectChannelPackage, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return item, nil
}
func (f *fakeRepo) ListByProject(_ context.Context, projectID string) ([]model.ProjectChannelPackage, error) {
	var out []model.ProjectChannelPackage
	for _, item := range f.items {
		if item.ProjectPublicID == projectID {
			out = append(out, *item)
		}
	}
	return out, nil
}
func (f *fakeRepo) Delete(_ context.Context, id string) error {
	if _, ok := f.items[id]; !ok {
		return dao.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

type fakeAdapter struct{ level string }

func (f fakeAdapter) Channel() string { return core.ChannelWeChat }
func (f fakeAdapter) Validate(core.SourceInfo) core.Report { return core.Report{Level: f.level} }
func (f fakeAdapter) Build(_ context.Context, src core.SourceInfo, out io.Writer) (core.Manifest, error) {
	zw := zip.NewWriter(out)
	w, _ := zw.Create("game.js")
	_, _ = w.Write([]byte("game"))
	_ = zw.Close()
	return core.Manifest{ProjectID: "p1", Channel: core.ChannelWeChat, SourceVersion: src.Version}, nil
}

func TestGenerateWritesPackageAndRecord(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('c').getContext('2d')`)
	repo := &fakeRepo{items: map[string]*model.ProjectChannelPackage{}}
	svc := NewService(ServiceConfig{
		Repo: repo, WorkspacesDir: root, PackagesDir: filepath.Join(root, "packages"),
		Adapters: map[string]core.Adapter{core.ChannelWeChat: fakeAdapter{level: core.LevelB}},
		Now: func() time.Time { return time.Unix(1700000000, 0).UTC() },
		NewID: func() string { return "pkg-1" },
	})
	item, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5})
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != model.ChannelPackageStatusReady || item.FileSizeBytes == 0 {
		t.Fatalf("item = %+v", item)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "p1", core.ChannelWeChat, "pkg-1.zip")); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateRejectsLevelC(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<div id="app"></div>`)
	repo := &fakeRepo{items: map[string]*model.ProjectChannelPackage{}}
	svc := NewService(ServiceConfig{
		Repo: repo, WorkspacesDir: root, PackagesDir: filepath.Join(root, "packages"),
		Adapters: map[string]core.Adapter{core.ChannelWeChat: fakeAdapter{level: core.LevelC}},
		NewID: func() string { return "pkg-1" },
	})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	if repo.items["pkg-1"].Status != model.ChannelPackageStatusFailed {
		t.Fatalf("status = %s", repo.items["pkg-1"].Status)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg/ -run Generate -v`
Expected: FAIL。

- [ ] **Step 3: 实现 Service**

```go
var (
	ErrInvalidChannel = errors.New("invalid channel")
	ErrInvalidSourceType = errors.New("invalid source type")
	ErrUnsupported = errors.New("channel is not supported")
	ErrPackageTooLarge = errors.New("channel package is too large")
	ErrBusy = errors.New("channel package builder is busy")
)

type GenerateInput struct {
	Channel    string
	SourceType string
	ResourceID string
}

type Service struct {
	cfg ServiceConfig
	sem chan struct{}
}

func NewService(cfg ServiceConfig) *Service {
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = 1 << 30
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 90 * time.Second
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 2
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.NewID == nil {
		cfg.NewID = func() string { return uuid.NewString() }
	}
	return &Service{cfg: cfg, sem: make(chan struct{}, cfg.Concurrency)}
}
```

`Generate` 实现顺序：

1. 校验 `input.Channel` 在 `cfg.Adapters` 中；`SourceType` 只允许两种。
2. 非阻塞获取信号量；失败返回 `ErrBusy`。
3. 创建记录 `building` 并 `Repo.Create`。
4. `ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)`。
5. 根据来源类型创建 source：`html5` 用 `NewWorkspaceSource(cfg.WorkspacesDir, project.ID)`；`engine_artifact` 用 `NewResourceSource(project.ID, input.ResourceID, cfg.OpenResource, cfg.PackagesDir)`；然后 `Materialize`。
6. 若 `source.Report.Level == core.LevelC`，失败并返回 `ErrUnsupported`。
7. 调用适配器 `Validate` 再次检查 blockers。
8. 创建临时文件 `PackagesDir/.tmp-<id>-*.zip`，调用 `Build`。
9. 检查文件大小 <= `MaxBytes`。
10. 目标路径 `projectID/channel/<id>.zip`，`MkdirAll` 后 `os.Rename`。
11. 更新记录：`status=ready`、`version`、`file_path` 相对路径、`file_size_bytes`、`manifest`。
12. 失败时更新记录 `status=failed`、`error_message`，清理临时文件；保留记录便于前端展示。
13. 资源来源的临时目录必须 `os.RemoveAll`。

`Delete` 校验记录属于项目后删除文件（不存在也继续）并软删除记录。

同时提供：

```go
func (s *Service) PackagesDir() string { return s.cfg.PackagesDir }

func (s *Service) Get(ctx context.Context, packageID string) (*model.ProjectChannelPackage, error) {
	return s.cfg.Repo.GetByPublicID(ctx, packageID)
}

func (s *Service) List(ctx context.Context, projectID string) ([]model.ProjectChannelPackage, error) {
	return s.cfg.Repo.ListByProject(ctx, projectID)
}

func (s *Service) Delete(ctx context.Context, projectID, packageID string) error {
	item, err := s.cfg.Repo.GetByPublicID(ctx, packageID)
	if err != nil {
		return err
	}
	if item.ProjectPublicID != projectID {
		return dao.ErrNotFound
	}
	full := filepath.Join(s.cfg.PackagesDir, item.FilePath)
	if inside, relErr := filepath.Rel(s.cfg.PackagesDir, full); relErr != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return ErrInvalidSource
	}
	_ = os.Remove(full)
	return s.cfg.Repo.Delete(ctx, packageID)
}
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/channelpkg/...`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/channelpkg/service.go packages/backend/internal/channelpkg/service_test.go
git commit -m "feat: orchestrate channel package generation"
```

---

### Task 10: CP Handler、路由、配置与装配

**Files:**
- Create: `packages/backend/internal/handler/channel_package.go`
- Test: `packages/backend/internal/handler/channel_package_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/.env.example`

**Interfaces:**
- Consumes: `channelpkg.Service`, `store.ProjectStore`
- Produces: `handler.NewChannelPackageHandler(service *channelpkg.Service, projects store.ProjectStore) *ChannelPackageHandler`
- Produces: routes:
  - `GET /cp/api/v1/channel-guides`
  - `POST/GET /cp/api/v1/projects/:id/channel-packages`
  - `GET /cp/api/v1/projects/:id/channel-packages/:packageId/download`
  - `DELETE /cp/api/v1/projects/:id/channel-packages/:packageId`

- [ ] **Step 1: 写 handler 失败测试**

```go
func TestChannelPackageGenerateRequiresWorkspaceMember(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	h := NewChannelPackageHandler(nil, projects)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/channel-packages", h.Generate, sessionForTest(99))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/channel-packages", strings.NewReader(`{"channel":"wechat_minigame","sourceType":"html5"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelGuidesReturnsThreeChannels(t *testing.T) {
	h := NewChannelPackageHandler(nil, nil)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/channel-guides", h.Guides, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/channel-guides", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") || !strings.Contains(rec.Body.String(), "steam_desktop") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler/ -run ChannelPackage -v`
Expected: FAIL。

- [ ] **Step 3: 实现 handler**

`channel_package.go` 要求：

```go
type ChannelPackageHandler struct {
	Service  *channelpkg.Service
	Projects store.ProjectStore
}
```

- `Generate`：会话校验、项目加载、`containsInt64(session.WorkspaceIDs, project.WorkspaceID)`、`project.IsHidden` 拒绝；绑定 `{channel, sourceType, resourceId}`；调用 `Service.Generate`，返回 `{package: dto}`。
- `List`：校验项目归属后返回 `{packages: []dto}`。

响应 DTO：

```go
type channelPackageResponse struct {
	ID            string   `json:"id"`
	Channel       string   `json:"channel"`
	SourceType    string   `json:"sourceType"`
	Status        string   `json:"status"`
	Version       string   `json:"version"`
	FileSizeBytes int64    `json:"fileSizeBytes"`
	Level         string   `json:"level"`
	Warnings      []string `json:"warnings"`
	ErrorMessage  string   `json:"errorMessage"`
	CreatedAt     string   `json:"createdAt"`
	DownloadURL   string   `json:"downloadUrl"`
	GuideURL      string   `json:"guideUrl"`
}

func toChannelPackageResponse(item *model.ProjectChannelPackage) channelPackageResponse {
	var manifest core.Manifest
	if len(item.Manifest) > 0 {
		_ = json.Unmarshal(item.Manifest, &manifest)
	}
	warnings := manifest.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return channelPackageResponse{
		ID: item.PublicID, Channel: item.Channel, SourceType: item.SourceType,
		Status: item.Status, Version: item.Version, FileSizeBytes: item.FileSizeBytes,
		Level: manifest.Level, Warnings: warnings, ErrorMessage: item.ErrorMessage,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
		DownloadURL: "/cp/api/v1/projects/" + item.ProjectPublicID + "/channel-packages/" + item.PublicID + "/download",
		GuideURL: "/settings/channel-guides?channel=" + item.Channel,
	}
}
```
- `Download`：加载项目、记录，校验 `record.ProjectPublicID == project.ID`；用 `os.Open` 打开 `filepath.Join(Service.PackagesDir(), record.FilePath)`；设置下载头并 `c.Stream`。
- `Delete`：同上下载校验后调用 `Service.Delete`。
- `Guides`：不依赖 Service，直接 `response.OK(c, map[string]any{"guides": guides.List()})`。
- 错误映射：

```go
func channelPackageError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, channelpkg.ErrInvalidChannel), errors.Is(err, channelpkg.ErrInvalidSourceType):
		return response.Fail(c, http.StatusBadRequest, 40001, err.Error())
	case errors.Is(err, channelpkg.ErrUnsupported):
		return response.Fail(c, http.StatusConflict, 40901, err.Error())
	case errors.Is(err, channelpkg.ErrSourceNotFound):
		return response.Fail(c, http.StatusNotFound, 40410, "channel source not found")
	case errors.Is(err, channelpkg.ErrPackageTooLarge), errors.Is(err, core.ErrArchiveTooLarge), errors.Is(err, core.ErrUnsafeArchive):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, err.Error())
	case errors.Is(err, channelpkg.ErrBusy):
		return response.Fail(c, http.StatusTooManyRequests, 42901, err.Error())
	default:
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
}
```

- [ ] **Step 4: 注册路由**

`RegisterControl` 增加参数 `channelPackages *handler.ChannelPackageHandler`，放在 `publishApp` 之后。注册：

```go
if channelPackages != nil {
	meAPI.GET("/channel-guides", channelPackages.Guides)
	projectsAPI.POST("/:id/channel-packages", channelPackages.Generate)
	projectsAPI.GET("/:id/channel-packages", channelPackages.List)
	projectsAPI.GET("/:id/channel-packages/:packageId/download", channelPackages.Download)
	projectsAPI.DELETE("/:id/channel-packages/:packageId", channelPackages.Delete)
}
```

同步更新 `helpers_test.go` 和 `span_test.go` 的 `RegisterControl(...)` 调用，插入 `nil`。

- [ ] **Step 5: 增加配置**

`config.go` 增加字段：

```go
ChannelPackagesDir         string
ChannelPackageMaxBytes     int64
ChannelPackageTimeout      time.Duration
ChannelPackageConcurrency  int
```

`Load` 增加：

```go
ChannelPackagesDir:        getString("CHANNEL_PACKAGES_DIR", "../../data/channel-packages"),
ChannelPackageMaxBytes:    int64(getInt("CHANNEL_PACKAGE_MAX_BYTES", 1<<30)),
ChannelPackageTimeout:     getDuration("CHANNEL_PACKAGE_TIMEOUT", 90*time.Second),
ChannelPackageConcurrency: getInt("CHANNEL_PACKAGE_CONCURRENCY", 2),
```

`.env.example` 追加同名变量。

- [ ] **Step 6: main 装配**

在 `publishHandler` 之后：

```go
channelPackageService := channelpkg.NewService(channelpkg.ServiceConfig{
	Repo:          dao.NewChannelPackageRepository(db),
	WorkspacesDir: cfg.WorkspacesDir,
	PackagesDir:   cfg.ChannelPackagesDir,
	OpenResource:  resourceService.OpenReadyChannelBuild,
	Adapters: map[string]core.Adapter{
		core.ChannelWeChat: wechat.New(),
		core.ChannelDouyin: douyin.New(),
		core.ChannelSteam:  steam.New(),
	},
	MaxBytes:    cfg.ChannelPackageMaxBytes,
	Timeout:     cfg.ChannelPackageTimeout,
	Concurrency: cfg.ChannelPackageConcurrency,
})
channelPackageHandler := handler.NewChannelPackageHandler(channelPackageService, projectStore)
```

把 `channelPackageHandler` 加入 `router.RegisterControl` 调用。

- [ ] **Step 7: 运行测试**

Run: `cd packages/backend && go test ./internal/handler/ ./internal/router/ ./internal/channelpkg/... && go vet ./...`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add packages/backend/internal/handler/channel_package.go \
  packages/backend/internal/handler/channel_package_test.go \
  packages/backend/internal/router/control.go \
  packages/backend/internal/router/helpers_test.go \
  packages/backend/internal/router/span_test.go \
  packages/backend/internal/config/config.go \
  packages/backend/cmd/cp/main.go \
  packages/backend/.env.example
git commit -m "feat: expose channel package APIs"
```

---

### Task 11: Web API、类型与下载

**Files:**
- Create: `packages/web/src/api/channelPackages.ts`
- Create: `packages/web/src/api/channelPackages.test.ts`
- Modify: `packages/web/src/types/index.ts`

**Interfaces:**
- Produces: `ChannelPackage`, `ChannelGuide`, `ChannelGuideItem`
- Produces: `generateChannelPackage`, `listChannelPackages`, `deleteChannelPackage`, `listChannelGuides`, `downloadChannelPackage`

- [ ] **Step 1: 写失败测试**

```ts
import { describe, expect, it, vi } from 'vitest'

import { downloadChannelPackage, generateChannelPackage } from './channelPackages'

describe('channelPackages api', () => {
  it('unwraps generate envelope', async () => {
    const result = await generateChannelPackage('p1', {
      channel: 'wechat_minigame',
      sourceType: 'html5',
    })
    expect(result.channel).toBe('wechat_minigame')
  })

  it('parses filename from content-disposition', async () => {
    const blob = new Blob(['zip'])
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({
      ok: true,
      blob: async () => blob,
      headers: new Headers({ 'content-disposition': 'attachment; filename="game-wechat.zip"' }),
    }))
    const result = await downloadChannelPackage('p1', 'pkg-1')
    expect(result.filename).toBe('game-wechat.zip')
  })
})
```

测试使用与 `publish.test.ts` 相同的 axios mock 方式；先读该文件再复刻。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- channelPackages`
Expected: FAIL。

- [ ] **Step 3: 写类型**

`types/index.ts` 追加：

```ts
export type ChannelPackageChannel = 'wechat_minigame' | 'douyin_minigame' | 'steam_desktop'
export type ChannelPackageSourceType = 'html5' | 'engine_artifact'

export interface ChannelPackage {
  id: string
  channel: ChannelPackageChannel
  sourceType: ChannelPackageSourceType
  status: 'building' | 'ready' | 'failed'
  version: string
  fileSizeBytes: number
  level: 'A' | 'B' | 'C'
  warnings: string[]
  errorMessage: string
  createdAt: string
  downloadUrl: string
  guideUrl: string
}

export interface ChannelGuide {
  channel: ChannelPackageChannel
  title: string
  summary: string
  markdown: string
  steps: string[]
  requirements: string[]
  updatedAt: string
}
```

- [ ] **Step 4: 写 API**

```ts
import { cpHttp, request } from './client'
import type { ChannelGuide, ChannelPackage, ChannelPackageChannel, ChannelPackageSourceType } from '@/types'

export interface GenerateChannelPackageInput {
  channel: ChannelPackageChannel
  sourceType: ChannelPackageSourceType
  resourceId?: string
}

export async function generateChannelPackage(
  projectId: string,
  input: GenerateChannelPackageInput,
): Promise<ChannelPackage> {
  const result = await request<{ package: ChannelPackage }>(cpHttp, {
    method: 'post',
    url: `/projects/${encodeURIComponent(projectId)}/channel-packages`,
    data: input,
  })
  return result.package
}

export async function listChannelPackages(projectId: string): Promise<ChannelPackage[]> {
  const result = await request<{ packages: ChannelPackage[] }>(cpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/channel-packages`,
  })
  return result.packages
}

export async function deleteChannelPackage(projectId: string, packageId: string): Promise<void> {
  await request<{ deleted: boolean }>(cpHttp, {
    method: 'delete',
    url: `/projects/${encodeURIComponent(projectId)}/channel-packages/${encodeURIComponent(packageId)}`,
  })
}

export async function listChannelGuides(): Promise<ChannelGuide[]> {
  const result = await request<{ guides: ChannelGuide[] }>(cpHttp, {
    method: 'get',
    url: '/channel-guides',
  })
  return result.guides
}

export interface DownloadedPackage {
  blob: Blob
  filename: string
}

export async function downloadChannelPackage(projectId: string, packageId: string): Promise<DownloadedPackage> {
  const base = import.meta.env.VITE_CP_BASE || '/cp/api/v1'
  const response = await fetch(
    `${base}/projects/${encodeURIComponent(projectId)}/channel-packages/${encodeURIComponent(packageId)}/download`,
    { credentials: 'include' },
  )
  if (!response.ok) {
    throw new Error(`下载失败（${response.status}）`)
  }
  const disposition = response.headers.get('content-disposition') || ''
  const match = /filename="?([^";]+)"?/.exec(disposition)
  return {
    blob: await response.blob(),
    filename: match?.[1] || `${projectId}-channel.zip`,
  }
}
```

- [ ] **Step 5: 运行测试**

Run: `cd packages/web && npm test -- channelPackages`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add packages/web/src/api/channelPackages.ts \
  packages/web/src/api/channelPackages.test.ts \
  packages/web/src/types/index.ts
git commit -m "feat: add channel package web api"
```

---

### Task 12: 发布弹窗渠道包面板

**Files:**
- Create: `packages/web/src/components/editor/ChannelPackagePanel.vue`
- Create: `packages/web/src/utils/channelPackages.ts`
- Create: `packages/web/src/utils/channelPackages.test.ts`
- Modify: `packages/web/src/components/editor/PublishDialog.vue`
- Modify: `packages/web/src/styles/editor.css`

**Interfaces:**
- Consumes: `generateChannelPackage`, `listChannelPackages`, `downloadChannelPackage`
- Produces: `channelOptions()` / `channelLabel(channel)` from `src/utils/channelPackages.ts`
- Produces: `ChannelPackagePanel` props `{ projectId: string }`

- [ ] **Step 1: 写失败测试**

仓库没有 `@vue/test-utils`，不新增测试依赖；把渠道选项和等级文案抽成纯函数测试。

```ts
import { describe, expect, it } from 'vitest'

import { channelLabel, channelOptions, levelLabel } from './channelPackages'

describe('channel package helpers', () => {
  it('exposes three channels in display order', () => {
    expect(channelOptions.map((item) => item.value)).toEqual([
      'wechat_minigame',
      'douyin_minigame',
      'steam_desktop',
    ])
    expect(channelOptions.map((item) => item.label)).toEqual([
      '微信小游戏',
      '抖音小游戏',
      'Steam 桌面',
    ])
  })

  it('maps levels and labels', () => {
    expect(channelLabel('wechat_minigame')).toBe('微信小游戏')
    expect(levelLabel('A')).toContain('直接兼容')
    expect(levelLabel('C')).toContain('需要改造')
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- channelPackages`
Expected: FAIL。

- [ ] **Step 3: 写纯函数**

`src/utils/channelPackages.ts`:

```ts
import type { ChannelPackageChannel } from '@/types'

export const channelOptions: { value: ChannelPackageChannel; label: string }[] = [
  { value: 'wechat_minigame', label: '微信小游戏' },
  { value: 'douyin_minigame', label: '抖音小游戏' },
  { value: 'steam_desktop', label: 'Steam 桌面' },
]

export function channelLabel(channel: ChannelPackageChannel): string {
  return channelOptions.find((item) => item.value === channel)?.label ?? channel
}

export function levelLabel(level: 'A' | 'B' | 'C'): string {
  if (level === 'A') return 'A 级：直接兼容'
  if (level === 'B') return 'B 级：适配层兜底，需真机验证'
  return 'C 级：需要改造，暂不支持生成'
}
```

- [ ] **Step 4: 实现面板**

组件要求：

- 渠道三选一，默认微信。
- 来源：`当前工作区` / `上传引擎成品`。
- 引擎成品复用 `resources.ts` 的 `presignResource`、`uploadToS3`、`uploadStaging`、`adoptResource`，`purpose: 'channel_build'`。
- 生成中禁用按钮并显示 `生成中…`。
- 结果展示：等级 A/B/C、警告、文件大小、下载、查看指引。
- `download` 用 `URL.createObjectURL` 和 `<a download>`，完成后 `URL.revokeObjectURL`。
- 颜色只用现有 CSS 变量。

骨架：

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'

import {
  downloadChannelPackage,
  generateChannelPackage,
  listChannelPackages,
} from '@/api/channelPackages'
import type { ChannelPackage, ChannelPackageChannel } from '@/types'

import { channelOptions, channelLabel } from '@/utils/channelPackages'
import { adoptResource, presignResource, uploadStaging, uploadToS3 } from '@/api/resources'

const props = defineProps<{ projectId: string }>()

const sourceType = ref<'html5' | 'engine_artifact'>('html5')
const artifactResourceId = ref('')
const uploading = ref(false)
const channel = ref<ChannelPackageChannel>('wechat_minigame')
const packages = ref<ChannelPackage[]>([])
const generating = ref(false)
const error = ref('')

async function load() {
  packages.value = await listChannelPackages(props.projectId)
}

async function uploadArtifact(file: File) {
  uploading.value = true
  error.value = ''
  try {
    const presign = await presignResource({
      filename: file.name,
      contentType: file.type || 'application/zip',
      size: file.size,
      purpose: 'channel_build',
    })
    try {
      await uploadToS3(presign, file)
    } catch {
      await uploadStaging(presign.resourceId, file)
    }
    const adopted = await adoptResource(props.projectId, presign.resourceId)
    artifactResourceId.value = adopted.id
  } catch (err) {
    error.value = err instanceof Error ? err.message : '上传失败'
  } finally {
    uploading.value = false
  }
}

async function generate() {
  if (sourceType.value === 'engine_artifact' && !artifactResourceId.value) {
    error.value = '请先上传引擎成品 zip'
    return
  }
  generating.value = true
  error.value = ''
  try {
    const result = await generateChannelPackage(props.projectId, {
      channel: channel.value,
      sourceType: sourceType.value,
      resourceId: sourceType.value === 'engine_artifact' ? artifactResourceId.value : undefined,
    })
    packages.value = [result, ...packages.value.filter((item) => item.id !== result.id)]
  } catch (err) {
    error.value = err instanceof Error ? err.message : '生成失败'
  } finally {
    generating.value = false
  }
}

async function download(item: ChannelPackage) {
  const result = await downloadChannelPackage(props.projectId, item.id)
  const url = URL.createObjectURL(result.blob)
  const link = document.createElement('a')
  link.href = url
  link.download = result.filename
  link.click()
  URL.revokeObjectURL(url)
}

onMounted(load)
</script>
```

模板包含渠道按钮、来源切换、生成按钮、错误提示、结果列表。引擎成品上传作为第二个来源，首期先展示上传入口并在 `sourceType` 为 `engine_artifact` 时要求先完成上传和 adopt。

- [ ] **Step 5: 集成到 PublishDialog**

`PublishDialog.vue` 增加 `tab` ref（`share` / `channel`），顶部两个按钮切换；`share` 保留现有内容，`channel` 渲染 `<ChannelPackagePanel :project-id="projectId" />`。

- [ ] **Step 6: 样式**

在 `editor.css` 增加 `.channel-package-*` 规则，只使用 `var(--ink)`、`var(--text)`、`var(--muted)`、`var(--line)`、`var(--line-strong)`、`var(--surface)`、`var(--canvas)`、`var(--soft)`、`var(--blue)`、`var(--blue-soft)`。

- [ ] **Step 7: 运行测试**

Run: `cd packages/web && npm test && npm run lint && npm run build`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add packages/web/src/components/editor/ChannelPackagePanel.vue \
  packages/web/src/utils/channelPackages.ts \
  packages/web/src/utils/channelPackages.test.ts \
  packages/web/src/components/editor/PublishDialog.vue \
  packages/web/src/styles/editor.css
git commit -m "feat: add channel package panel to publish dialog"
```

---

### Task 13: 设置页发布渠道指引

**Files:**
- Create: `packages/web/src/views/settings/ChannelGuideView.vue`
- Create: `packages/web/src/views/settings/ChannelGuideView.test.ts`
- Modify: `packages/web/src/router/settingsRoutes.ts`
- Modify: `packages/web/src/router/settingsRoutes.test.ts`
- Modify: `packages/web/src/styles/settings.css`

**Interfaces:**
- Consumes: `listChannelGuides`, `renderMarkdown`

- [ ] **Step 1: 更新路由测试**

在 `settingsRoutes.test.ts` 的 children 和菜单断言中加入 `channel-guides`，位置放在 `notification-channels` 之后，分组为 `企业工程`，标签 `发布渠道指引`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- settingsRoutes`
Expected: FAIL。

- [ ] **Step 3: 加路由与菜单**

`settingsRoutes.ts`：

```ts
{
  label: '发布渠道指引',
  icon: 'pi pi-share-alt',
  to: '/settings/channel-guides',
  name: 'settings-channel-guides',
  group: '企业工程',
},
```

并增加 child：

```ts
{
  path: 'channel-guides',
  name: 'settings-channel-guides',
  component: () => import('@/views/settings/ChannelGuideView.vue'),
},
```

- [ ] **Step 4: 实现指引页**

要求：

- `onMounted` 调 `listChannelGuides`。
- 左侧渠道按钮，选中项高亮。
- 右侧显示标题、`summary`、资质清单、五步流程图、`renderMarkdown(markdown)` 渲染的正文。
- 流程图用 `v-for` 渲染步骤条，步骤之间用 CSS 箭头，不引入新依赖。
- 支持 `route.query.channel` 初始选中。
- 加载/错误/空态。

- [ ] **Step 5: 样式**

`settings.css` 增加 `.channel-guide-*`，只使用现有颜色变量。

- [ ] **Step 6: 运行测试**

Run: `cd packages/web && npm test && npm run lint && npm run build`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add packages/web/src/views/settings/ChannelGuideView.vue \
  packages/web/src/views/settings/ChannelGuideView.test.ts \
  packages/web/src/router/settingsRoutes.ts \
  packages/web/src/router/settingsRoutes.test.ts \
  packages/web/src/styles/settings.css
git commit -m "feat: add channel guide settings page"
```

---

### Task 14: 部署挂载与端到端验证

**Files:**
- Modify: `deployment/docker/docker-compose.yml`
- Modify: `deployment/docker/docker-compose.public.yml`
- Modify: `packages/backend/.env.example`
- Modify: `README.md`（仅补运行目录说明）

- [ ] **Step 1: 更新 compose**

dev compose：

- `publish-init` 命令增加 `mkdir -p /channel-packages`，卷 `../../data/channel-packages:/channel-packages`。
- backend 环境增加 `CHANNEL_PACKAGES_DIR: /data/channel-packages`，卷 `../../data/channel-packages:/data/channel-packages`。
- backend `depends_on.publish-init` 已存在，无需改。

public compose 同样增加 `./channel-packages` 目录和 `/data/channel-packages` 环境变量。

- [ ] **Step 2: 更新 README**

在数据目录说明中补：

```text
data/channel-packages/  渠道包产物（微信/抖音/Steam），按项目/渠道归档
```

- [ ] **Step 3: 运行全量验证**

Run: `make validate`
Expected: 全部通过。

Run: `make test`
Expected: Go、pytest、web、ops-web 全部通过。

- [ ] **Step 4: 手工验收**

1. 用内置账号创建一个 canvas 小游戏项目。
2. 打开发布弹窗 → 渠道包 → 微信小游戏 → 生成。
3. 确认下载 zip 包含 `game.js`、`game.json`、`project.config.json`、`adapter/`、`game/`、`CHANNEL_GUIDE.md`、`package-manifest.json`。
4. 对抖音和 Steam 重复，确认结构和指引不同。
5. 导入微信/抖音开发者工具（如有环境）验证 B 级 canvas 游戏可预览。
6. 解压 Steam 包，执行 `npm install && npm start` 验证壳可启动，再执行 `npm run dist` 验证 Windows 包可构建。
7. 用非成员账号访问生成/下载接口，确认 404。
8. 打开设置 → 发布渠道指引，确认三渠道内容、步骤和流程图显示正常。

- [ ] **Step 5: 提交**

```bash
git add deployment/docker/docker-compose.yml \
  deployment/docker/docker-compose.public.yml \
  packages/backend/.env.example \
  README.md
git commit -m "chore: mount channel package storage"
```

---

## Self-Review

- Spec 覆盖：迁移/模型、资源用途、来源分级、三个适配器、Service、API、Web 弹窗、设置指引、部署与测试均有对应任务。
- 类型一致性：`core.SourceInfo`、`core.Report`、`core.Manifest`、`channelpkg.GenerateInput`、`ServiceConfig` 在各任务中名称一致。
- 无占位符：所有代码步骤均给出可执行内容；指引 markdown 内容按小节要求编写，不使用占位文本。
- 测试：每个后端任务有 Go 测试，前端任务有 vitest 和 lint/build 检查。
