# 发布作品排行榜与发布数据统计 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为已发布作品增加 Redis 累计 PV 排行榜、发布页曝光/点击统计、web 排行榜页、ops 榜单管理页与账户设置发布数据面板。

**Architecture:** CP 新增 `internal/leaderboard`，用 Redis ZSET/HASH/SET 存 PV、曝光、点击、隐藏与元数据；发布页根文档访问写 PV，分享页「开始体验」写点击，用户榜返回时写曝光。web 与 ops-web 只通过 CP 契约读取与管理榜单，不接触 Redis。

**Tech Stack:** Go 1.24 + Echo v4 + redis/go-redis/v9 + miniredis + GORM；Vue 3 + TypeScript + Pinia + PrimeVue + Vitest。

**Spec:** `docs/superpowers/specs/2026-11-07-published-leaderboard-design.md`

## Global Constraints

- Redis key 前缀固定 `vibebiz`，榜单 ZSET `vibebiz:leaderboard:pv` 候选集最多 200 条，对外用户榜始终最多 50 条。
- 只统计已发布作品；未发布、项目不存在、全局隐藏的项目不得出现在榜单。
- ops 的隐藏 / 删除只作用于榜单条目，不影响 `/p/:slug` 的访问。
- 停留时长与消费额度本期固定返回 `0`。
- 不新增数据库表、迁移或环境变量。
- 所有 Redis 统计写入失败只记录日志，不阻断主流程。
- 前端颜色仅黑 / 白 / 灰 / 蓝，不引入新色系。
- 先写失败测试再实现；每个任务结束必须运行该任务指定的测试命令。
- **不要执行 git commit，除非用户明确要求。** 本计划用测试通过作为任务完成条件。

---

### Task 1: Redis 排行榜 Store

**Files:**
- Create: `packages/backend/internal/leaderboard/store.go`
- Test: `packages/backend/internal/leaderboard/store_test.go`

**Interfaces:**
- Produces:
  - `const MaxEntries = 50`
  - `const MaxCandidates = 200`
  - `type Meta struct { Title, Author string }`
  - `type Ranked struct { ProjectID string; Visits int64 }`
  - `type Counters struct { Exposure, Clicks int64 }`
  - `func NewStore(client *redis.Client, prefix string) *Store`
  - `func (s *Store) RecordView(ctx context.Context, projectID string) error`
  - `func (s *Store) RecordClick(ctx context.Context, projectID string) error`
  - `func (s *Store) RecordExposure(ctx context.Context, projectIDs []string) error`
  - `func (s *Store) Top(ctx context.Context, limit int) ([]Ranked, error)`
  - `func (s *Store) Counters(ctx context.Context, projectIDs []string) (map[string]Counters, error)`
  - `func (s *Store) Meta(ctx context.Context, projectIDs []string) (map[string]Meta, error)`
  - `func (s *Store) PutMeta(ctx context.Context, projectID string, meta Meta) error`
  - `func (s *Store) HiddenSet(ctx context.Context) (map[string]bool, error)`
  - `func (s *Store) Hide(ctx context.Context, projectID string, hidden bool) error`
  - `func (s *Store) Delete(ctx context.Context, projectID string) error`
  - `func (s *Store) PruneRanking(ctx context.Context, projectIDs []string) error`

- [ ] **Step 1: Write the failing test**

Create `packages/backend/internal/leaderboard/store_test.go`:

```go
package leaderboard

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewStore(client, "vibebiz"), mr
}

func TestRecordViewKeepsTopFifty(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 55; i++ {
		id := fmt.Sprintf("p%02d", i)
		for n := 0; n <= i; n++ {
			if err := store.RecordView(ctx, id); err != nil {
				t.Fatalf("record view: %v", err)
			}
		}
	}

	ranked, err := store.Top(ctx, 100)
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if len(ranked) != 55 {
		t.Fatalf("candidate len = %d, want 55", len(ranked))
	}
	if ranked[0].ProjectID != "p54" || ranked[0].Visits != 55 {
		t.Fatalf("top entry = %+v", ranked[0])
	}

	top, err := store.Top(ctx, 50)
	if err != nil {
		t.Fatalf("top 50: %v", err)
	}
	if len(top) != 50 {
		t.Fatalf("len = %d, want 50", len(top))
	}
	if top[len(top)-1].Visits != 6 {
		t.Fatalf("last entry = %+v, want visits 6", top[len(top)-1])
	}
}

func TestCountersHideDeleteAndPrune(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.RecordExposure(ctx, []string{"p1", "p1", "p2"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordClick(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	counters, err := store.Counters(ctx, []string{"p1", "p2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if counters["p1"].Exposure != 2 || counters["p1"].Clicks != 1 || counters["missing"].Exposure != 0 {
		t.Fatalf("counters = %+v", counters)
	}

	if err := store.Hide(ctx, "p1", true); err != nil {
		t.Fatal(err)
	}
	hidden, err := store.HiddenSet(ctx)
	if err != nil || !hidden["p1"] {
		t.Fatalf("hidden = %+v, err = %v", hidden, err)
	}
	if err := store.Hide(ctx, "p1", false); err != nil {
		t.Fatal(err)
	}
	hidden, _ = store.HiddenSet(ctx)
	if hidden["p1"] {
		t.Fatal("p1 should be visible")
	}

	if err := store.PutMeta(ctx, "p1", Meta{Title: "小游戏", Author: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordView(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := store.PruneRanking(ctx, []string{"p1"}); err != nil {
		t.Fatal(err)
	}
	ranked, _ := store.Top(ctx, 10)
	if len(ranked) != 0 {
		t.Fatalf("ranked = %+v, want empty", ranked)
	}
	meta, _ := store.Meta(ctx, []string{"p1"})
	if _, ok := meta["p1"]; ok {
		t.Fatal("meta should be pruned")
	}
	counters, _ = store.Counters(ctx, []string{"p1"})
	if counters["p1"].Exposure != 2 || counters["p1"].Clicks != 1 {
		t.Fatalf("counters should survive prune: %+v", counters["p1"])
	}

	if err := store.Delete(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	counters, _ = store.Counters(ctx, []string{"p1"})
	if counters["p1"].Exposure != 0 || counters["p1"].Clicks != 0 {
		t.Fatalf("counters should be deleted: %+v", counters["p1"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/backend && go test ./internal/leaderboard -run TestRecordViewKeepsTopFifty -v`

Expected: FAIL because `NewStore` and related types are undefined.

- [ ] **Step 3: Implement the store**

Create `packages/backend/internal/leaderboard/store.go`:

```go
package leaderboard

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/redis/go-redis/v9"
)

const (
	MaxEntries    = 50
	MaxCandidates = 200
)

type Meta struct {
	Title  string `json:"title"`
	Author string `json:"author"`
}

type Ranked struct {
	ProjectID string
	Visits    int64
}

type Counters struct {
	Exposure int64
	Clicks   int64
}

type Store struct {
	client *redis.Client
	prefix string
}

func NewStore(client *redis.Client, prefix string) *Store {
	return &Store{client: client, prefix: prefix}
}

func (s *Store) pvKey() string       { return s.prefix + ":leaderboard:pv" }
func (s *Store) hiddenKey() string   { return s.prefix + ":leaderboard:hidden" }
func (s *Store) exposureKey() string { return s.prefix + ":leaderboard:exposure" }
func (s *Store) clicksKey() string   { return s.prefix + ":leaderboard:clicks" }
func (s *Store) metaKey() string     { return s.prefix + ":leaderboard:meta" }

func (s *Store) RecordView(ctx context.Context, projectID string) error {
	pipe := s.client.TxPipeline()
	pipe.ZIncrBy(ctx, s.pvKey(), 1, projectID)
	pipe.ZRemRangeByRank(ctx, s.pvKey(), 0, -(MaxCandidates + 1))
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) RecordClick(ctx context.Context, projectID string) error {
	return s.client.HIncrBy(ctx, s.clicksKey(), projectID, 1).Err()
}

func (s *Store) RecordExposure(ctx context.Context, projectIDs []string) error {
	if len(projectIDs) == 0 {
		return nil
	}
	pipe := s.client.TxPipeline()
	for _, id := range projectIDs {
		pipe.HIncrBy(ctx, s.exposureKey(), id, 1)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) Top(ctx context.Context, limit int) ([]Ranked, error) {
	if limit <= 0 {
		limit = MaxEntries
	}
	rows, err := s.client.ZRevRangeWithScores(ctx, s.pvKey(), 0, int64(limit-1)).Result()
	if err != nil {
		return nil, err
	}
	out := make([]Ranked, 0, len(rows))
	for _, row := range rows {
		member, ok := row.Member.(string)
		if !ok {
			continue
		}
		out = append(out, Ranked{ProjectID: member, Visits: int64(row.Score)})
	}
	return out, nil
}

func (s *Store) Counters(ctx context.Context, projectIDs []string) (map[string]Counters, error) {
	out := make(map[string]Counters, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}
	exposure, err := s.client.HMGet(ctx, s.exposureKey(), projectIDs...).Result()
	if err != nil {
		return nil, err
	}
	clicks, err := s.client.HMGet(ctx, s.clicksKey(), projectIDs...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range projectIDs {
		out[id] = Counters{Exposure: parseInt64(exposure[i]), Clicks: parseInt64(clicks[i])}
	}
	return out, nil
}

func (s *Store) Meta(ctx context.Context, projectIDs []string) (map[string]Meta, error) {
	out := make(map[string]Meta, len(projectIDs))
	if len(projectIDs) == 0 {
		return out, nil
	}
	values, err := s.client.HMGet(ctx, s.metaKey(), projectIDs...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range projectIDs {
		raw, ok := values[i].(string)
		if !ok || raw == "" {
			continue
		}
		var meta Meta
		if err := json.Unmarshal([]byte(raw), &meta); err != nil {
			continue
		}
		out[id] = meta
	}
	return out, nil
}

func (s *Store) PutMeta(ctx context.Context, projectID string, meta Meta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return s.client.HSet(ctx, s.metaKey(), projectID, data).Err()
}

func (s *Store) HiddenSet(ctx context.Context) (map[string]bool, error) {
	ids, err := s.client.SMembers(ctx, s.hiddenKey()).Result()
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

func (s *Store) Hide(ctx context.Context, projectID string, hidden bool) error {
	if hidden {
		return s.client.SAdd(ctx, s.hiddenKey(), projectID).Err()
	}
	return s.client.SRem(ctx, s.hiddenKey(), projectID).Err()
}

func (s *Store) Delete(ctx context.Context, projectID string) error {
	pipe := s.client.TxPipeline()
	pipe.ZRem(ctx, s.pvKey(), projectID)
	pipe.HDel(ctx, s.exposureKey(), projectID)
	pipe.HDel(ctx, s.clicksKey(), projectID)
	pipe.HDel(ctx, s.metaKey(), projectID)
	pipe.SRem(ctx, s.hiddenKey(), projectID)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) PruneRanking(ctx context.Context, projectIDs []string) error {
	if len(projectIDs) == 0 {
		return nil
	}
	pipe := s.client.TxPipeline()
	for _, id := range projectIDs {
		pipe.ZRem(ctx, s.pvKey(), id)
		pipe.HDel(ctx, s.metaKey(), id)
		pipe.SRem(ctx, s.hiddenKey(), id)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func parseInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd packages/backend && go test ./internal/leaderboard -v`

Expected: PASS.

---

### Task 2: 批量用户名查询

**Files:**
- Modify: `packages/backend/internal/dao/identity_repository.go`
- Test: `packages/backend/internal/dao/identity_repository_test.go`

**Interfaces:**
- Produces: `func (r *IdentityRepository) Usernames(ctx context.Context, ids []int64) (map[int64]string, error)`

- [ ] **Step 1: Write the failing test**

Append to `packages/backend/internal/dao/identity_repository_test.go`:

```go
func TestUsernames(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)

	mock.ExpectQuery("SELECT .* FROM `user` WHERE id IN").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).
			AddRow(1, "alice").
			AddRow(2, "bob"))

	names, err := repo.Usernames(context.Background(), []int64{1, 2})
	if err != nil {
		t.Fatalf("usernames: %v", err)
	}
	if names[1] != "alice" || names[2] != "bob" {
		t.Fatalf("names = %+v", names)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/backend && go test ./internal/dao -run TestUsernames -v`

Expected: FAIL because `Usernames` is undefined.

- [ ] **Step 3: Implement the method**

Add to `packages/backend/internal/dao/identity_repository.go` after `FindUserByIdentifier`:

```go
func (r *IdentityRepository) Usernames(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	var rows []model.User
	if err := r.db.WithContext(ctx).Select("id", "username").Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("usernames: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Username
	}
	return out, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd packages/backend && go test ./internal/dao -run TestUsernames -v`

Expected: PASS.

---

### Task 3: 排行榜 Service

**Files:**
- Create: `packages/backend/internal/leaderboard/service.go`
- Test: `packages/backend/internal/leaderboard/service_test.go`

**Interfaces:**
- Consumes: Task 1 Store; `publish.Published`; `model.Project`.
- Produces:
  - `type PublishedLister interface { List(context.Context) ([]publish.Published, error) }`
  - `type ProjectLister interface { List(context.Context) []*model.Project }`
  - `type UserNameResolver interface { Usernames(context.Context, []int64) (map[int64]string, error) }`
  - `type Item struct { Rank int; ProjectID, Name, Author string; Visits int64 }`
  - `type AdminRow struct { ProjectID, Name, Author string; Visits, Exposure, Clicks int64; Hidden bool; PublishedAt time.Time }`
  - `type StatInput struct { ProjectID, Name string }`
  - `type ProjectStat struct { ProjectID, Name string; Exposure, Clicks, DwellSeconds, Spend int64 }`
  - `func NewService(store *Store, published PublishedLister, projects ProjectLister, users UserNameResolver) *Service`
  - `func (s *Service) ListPublic(ctx context.Context, limit int) ([]Item, error)`
  - `func (s *Service) ListAdmin(ctx context.Context) ([]AdminRow, error)`
  - `func (s *Service) Stats(ctx context.Context, inputs []StatInput) ([]ProjectStat, error)`
  - `func (s *Service) UpsertMeta(ctx context.Context, project *model.Project, title string) error`
  - `func (s *Service) RecordView(ctx context.Context, projectID string) error`
  - `func (s *Service) RecordClick(ctx context.Context, projectID string) error`
  - `func (s *Service) Hide(ctx context.Context, projectID string, hidden bool) error`
  - `func (s *Service) Delete(ctx context.Context, projectID string) error`

- [ ] **Step 1: Write the failing tests**

Create `packages/backend/internal/leaderboard/service_test.go`:

```go
package leaderboard

import (
	"context"
	"testing"
	"time"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type fakePublished struct{ items []publish.Published }

func (f fakePublished) List(context.Context) ([]publish.Published, error) { return f.items, nil }

type fakeProjects struct{ items []*model.Project }

func (f fakeProjects) List(context.Context) []*model.Project { return f.items }

type fakeUsers struct{ names map[int64]string }

func (f fakeUsers) Usernames(context.Context, []int64) (map[int64]string, error) { return f.names, nil }

func newTestService(t *testing.T) (*Service, *Store) {
	t.Helper()
	store, _ := newTestStore(t)
	svc := NewService(
		store,
		fakePublished{items: []publish.Published{
			{ProjectID: "p1", Title: "小游戏", PublishedAt: time.Now()},
			{ProjectID: "p2", Title: "答题", PublishedAt: time.Now()},
		}},
		fakeProjects{items: []*model.Project{
			{ID: "p1", Name: "Demo1", OwnerUserID: 1},
			{ID: "p2", Name: "Demo2", OwnerUserID: 2},
			{ID: "p3", Name: "HiddenProject", OwnerUserID: 1, IsHidden: true},
		}},
		fakeUsers{names: map[int64]string{1: "alice", 2: "bob"}},
	)
	return svc, store
}

func TestListPublicFiltersStaleAndTracksExposure(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	for _, id := range []string{"p1", "p1", "p2", "p3"} {
		if err := store.RecordView(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2", items)
	}
	if items[0].ProjectID != "p1" || items[0].Rank != 1 || items[0].Visits != 2 ||
		items[0].Name != "小游戏" || items[0].Author != "alice" {
		t.Fatalf("item[0] = %+v", items[0])
	}
	if items[1].ProjectID != "p2" || items[1].Rank != 2 {
		t.Fatalf("item[1] = %+v", items[1])
	}
	counters, _ := store.Counters(ctx, []string{"p1", "p2", "p3"})
	if counters["p1"].Exposure != 1 || counters["p2"].Exposure != 1 || counters["p3"].Exposure != 0 {
		t.Fatalf("counters = %+v", counters)
	}
	ranked, _ := store.Top(ctx, 10)
	for _, item := range ranked {
		if item.ProjectID == "p3" {
			t.Fatal("stale p3 should be pruned from ranking")
		}
	}

	if _, err := svc.ListPublic(ctx, 50); err != nil {
		t.Fatal(err)
	}
	counters, _ = store.Counters(ctx, []string{"p1"})
	if counters["p1"].Exposure != 2 {
		t.Fatalf("exposure = %d, want 2", counters["p1"].Exposure)
	}
}

func TestListPublicHonorsHiddenAndAdminDoesNotTrackExposure(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	_ = store.RecordView(ctx, "p1")
	_ = store.RecordView(ctx, "p1")
	_ = store.RecordView(ctx, "p2")
	_ = store.Hide(ctx, "p1", true)

	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ProjectID != "p2" {
		t.Fatalf("items = %+v", items)
	}

	rows, err := svc.ListAdmin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Hidden {
		t.Fatalf("rows = %+v", rows)
	}
	counters, _ := store.Counters(ctx, []string{"p1", "p2"})
	if counters["p1"].Exposure != 0 || counters["p2"].Exposure != 1 {
		t.Fatalf("admin must not track exposure: %+v", counters)
	}
}

func TestStatsAndUpsertMeta(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	_ = store.RecordExposure(ctx, []string{"p1", "p1"})
	_ = store.RecordClick(ctx, "p1")

	stats, err := svc.Stats(ctx, []StatInput{{ProjectID: "p1", Name: "小游戏"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Exposure != 2 || stats[0].Clicks != 1 ||
		stats[0].DwellSeconds != 0 || stats[0].Spend != 0 {
		t.Fatalf("stats = %+v", stats)
	}

	if err := svc.UpsertMeta(ctx, &model.Project{ID: "p1", Name: "Demo", OwnerUserID: 1}, "小游戏"); err != nil {
		t.Fatal(err)
	}
	meta, _ := store.Meta(ctx, []string{"p1"})
	if meta["p1"].Title != "小游戏" || meta["p1"].Author != "alice" {
		t.Fatalf("meta = %+v", meta["p1"])
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd packages/backend && go test ./internal/leaderboard -run 'TestListPublic|TestStats' -v`

Expected: FAIL because `Service` is undefined.

- [ ] **Step 3: Implement the service**

Create `packages/backend/internal/leaderboard/service.go`:

```go
package leaderboard

import (
	"context"
	"log/slog"
	"time"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type PublishedLister interface {
	List(ctx context.Context) ([]publish.Published, error)
}

type ProjectLister interface {
	List(ctx context.Context) []*model.Project
}

type UserNameResolver interface {
	Usernames(ctx context.Context, ids []int64) (map[int64]string, error)
}

type Item struct {
	Rank      int
	ProjectID string
	Name      string
	Author    string
	Visits    int64
}

type AdminRow struct {
	ProjectID   string
	Name        string
	Author      string
	Visits      int64
	Exposure    int64
	Clicks      int64
	Hidden      bool
	PublishedAt time.Time
}

type StatInput struct {
	ProjectID string
	Name      string
}

type ProjectStat struct {
	ProjectID    string `json:"projectId"`
	Name         string `json:"name"`
	Exposure     int64  `json:"exposure"`
	Clicks       int64  `json:"clicks"`
	DwellSeconds int64  `json:"dwellSeconds"`
	Spend        int64  `json:"spend"`
}

type Service struct {
	Store     *Store
	Published PublishedLister
	Projects  ProjectLister
	Users     UserNameResolver
}

func NewService(store *Store, published PublishedLister, projects ProjectLister, users UserNameResolver) *Service {
	return &Service{Store: store, Published: published, Projects: projects, Users: users}
}

type view struct {
	ranked    []Ranked
	published map[string]publish.Published
	projects  map[string]*model.Project
	hidden    map[string]bool
}

func (s *Service) loadView(ctx context.Context) (*view, error) {
	ranked, err := s.Store.Top(ctx, MaxEntries)
	if err != nil {
		return nil, err
	}
	published, err := s.Published.List(ctx)
	if err != nil {
		return nil, err
	}
	publishedByID := make(map[string]publish.Published, len(published))
	for _, item := range published {
		publishedByID[item.ProjectID] = item
	}
	projectsByID := make(map[string]*model.Project)
	for _, item := range s.Projects.List(ctx) {
		projectsByID[item.ID] = item
	}
	hidden, err := s.Store.HiddenSet(ctx)
	if err != nil {
		return nil, err
	}
	return &view{ranked: ranked, published: publishedByID, projects: projectsByID, hidden: hidden}, nil
}

func (s *Service) visible(ctx context.Context, v *view, includeHidden bool) []Ranked {
	out := make([]Ranked, 0, len(v.ranked))
	stale := make([]string, 0)
	for _, item := range v.ranked {
		project, ok := v.projects[item.ProjectID]
		_, published := v.published[item.ProjectID]
		if !ok || !published || project.IsHidden {
			stale = append(stale, item.ProjectID)
			continue
		}
		if v.hidden[item.ProjectID] && !includeHidden {
			continue
		}
		out = append(out, item)
	}
	if len(stale) > 0 {
		_ = s.Store.PruneRanking(ctx, stale)
	}
	return out
}

func (s *Service) ensureMeta(ctx context.Context, v *view, ids []string) (map[string]Meta, error) {
	meta, err := s.Store.Meta(ctx, ids)
	if err != nil {
		return nil, err
	}
	ownerIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, id := range ids {
		if _, ok := meta[id]; ok {
			continue
		}
		project := v.projects[id]
		if project == nil || seen[project.OwnerUserID] {
			continue
		}
		seen[project.OwnerUserID] = true
		ownerIDs = append(ownerIDs, project.OwnerUserID)
	}
	names := make(map[int64]string)
	if len(ownerIDs) > 0 && s.Users != nil {
		resolved, err := s.Users.Usernames(ctx, ownerIDs)
		if err == nil {
			names = resolved
		} else {
			slog.Default().Error("leaderboard resolve authors failed", "error", err)
		}
	}
	for _, id := range ids {
		if _, ok := meta[id]; ok {
			continue
		}
		project := v.projects[id]
		if project == nil {
			continue
		}
		title := v.published[id].Title
		if title == "" {
			title = project.Name
		}
		item := Meta{Title: title, Author: names[project.OwnerUserID]}
		meta[id] = item
		if err := s.Store.PutMeta(ctx, id, item); err != nil {
			slog.Default().Error("leaderboard save meta failed", "project_id", id, "error", err)
		}
	}
	return meta, nil
}

func (s *Service) ListPublic(ctx context.Context, limit int) ([]Item, error) {
	if limit <= 0 || limit > MaxEntries {
		limit = MaxEntries
	}
	v, err := s.loadView(ctx)
	if err != nil {
		return nil, err
	}
	visible := s.visible(ctx, v, false)
	if len(visible) > limit {
		visible = visible[:limit]
	}
	ids := make([]string, 0, len(visible))
	for _, item := range visible {
		ids = append(ids, item.ProjectID)
	}
	meta, err := s.ensureMeta(ctx, v, ids)
	if err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(visible))
	for index, item := range visible {
		m := meta[item.ProjectID]
		items = append(items, Item{
			Rank: index + 1, ProjectID: item.ProjectID,
			Name: m.Title, Author: m.Author, Visits: item.Visits,
		})
	}
	if err := s.Store.RecordExposure(ctx, ids); err != nil {
		slog.Default().Error("leaderboard record exposure failed", "error", err)
	}
	return items, nil
}

func (s *Service) ListAdmin(ctx context.Context) ([]AdminRow, error) {
	v, err := s.loadView(ctx)
	if err != nil {
		return nil, err
	}
	visible := s.visible(ctx, v, true)
	ids := make([]string, 0, len(visible))
	for _, item := range visible {
		ids = append(ids, item.ProjectID)
	}
	meta, err := s.ensureMeta(ctx, v, ids)
	if err != nil {
		return nil, err
	}
	counters, err := s.Store.Counters(ctx, ids)
	if err != nil {
		return nil, err
	}
	rows := make([]AdminRow, 0, len(visible))
	for _, item := range visible {
		m := meta[item.ProjectID]
		c := counters[item.ProjectID]
		rows = append(rows, AdminRow{
			ProjectID: item.ProjectID, Name: m.Title, Author: m.Author,
			Visits: item.Visits, Exposure: c.Exposure, Clicks: c.Clicks,
			Hidden: v.hidden[item.ProjectID], PublishedAt: v.published[item.ProjectID].PublishedAt,
		})
	}
	return rows, nil
}

func (s *Service) Stats(ctx context.Context, inputs []StatInput) ([]ProjectStat, error) {
	if len(inputs) == 0 {
		return []ProjectStat{}, nil
	}
	ids := make([]string, 0, len(inputs))
	for _, input := range inputs {
		ids = append(ids, input.ProjectID)
	}
	counters, err := s.Store.Counters(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ProjectStat, 0, len(inputs))
	for _, input := range inputs {
		c := counters[input.ProjectID]
		out = append(out, ProjectStat{
			ProjectID: input.ProjectID, Name: input.Name,
			Exposure: c.Exposure, Clicks: c.Clicks, DwellSeconds: 0, Spend: 0,
		})
	}
	return out, nil
}

func (s *Service) UpsertMeta(ctx context.Context, project *model.Project, title string) error {
	if s == nil || s.Store == nil || project == nil {
		return nil
	}
	author := ""
	if s.Users != nil {
		names, err := s.Users.Usernames(ctx, []int64{project.OwnerUserID})
		if err == nil {
			author = names[project.OwnerUserID]
		}
	}
	if title == "" {
		title = project.Name
	}
	return s.Store.PutMeta(ctx, project.ID, Meta{Title: title, Author: author})
}

func (s *Service) RecordView(ctx context.Context, projectID string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	err := s.Store.RecordView(ctx, projectID)
	if err != nil {
		slog.Default().Error("leaderboard record view failed", "project_id", projectID, "error", err)
	}
	return err
}

func (s *Service) RecordClick(ctx context.Context, projectID string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	err := s.Store.RecordClick(ctx, projectID)
	if err != nil {
		slog.Default().Error("leaderboard record click failed", "project_id", projectID, "error", err)
	}
	return err
}

func (s *Service) Hide(ctx context.Context, projectID string, hidden bool) error {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.Hide(ctx, projectID, hidden)
}

func (s *Service) Delete(ctx context.Context, projectID string) error {
	if s == nil || s.Store == nil {
		return nil
	}
	return s.Store.Delete(ctx, projectID)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd packages/backend && go test ./internal/leaderboard -v`

Expected: PASS for all store and service tests.

---

### Task 4: 排行榜 HTTP Handler 与路由

**Files:**
- Create: `packages/backend/internal/handler/leaderboard.go`
- Test: `packages/backend/internal/handler/leaderboard_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go:284`
- Modify: `packages/backend/internal/router/span_test.go:45`

**Interfaces:**
- Consumes: Task 3 `*leaderboard.Service`.
- Produces:
  - `func NewLeaderboardHandler(service *leaderboard.Service, publicBaseURL string) *LeaderboardHandler`
  - `func (h *LeaderboardHandler) ListPublic(c echo.Context) error`
  - `func (h *LeaderboardHandler) ListAdmin(c echo.Context) error`
  - `func (h *LeaderboardHandler) Hide(c echo.Context) error`
  - `func (h *LeaderboardHandler) Delete(c echo.Context) error`

- [ ] **Step 1: Write the failing tests**

Create `packages/backend/internal/handler/leaderboard_test.go`:

```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/leaderboard"
	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type leaderboardPublished struct{ items []publish.Published }

func (f leaderboardPublished) List(context.Context) ([]publish.Published, error) { return f.items, nil }

type leaderboardProjects struct{ items []*model.Project }

func (f leaderboardProjects) List(context.Context) []*model.Project { return f.items }

type leaderboardUsers struct{}

func (leaderboardUsers) Usernames(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{1: "alice"}, nil
}

func newLeaderboardHandlerForTest(t *testing.T) (*echo.Echo, *leaderboard.Service) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := leaderboard.NewStore(client, "vibebiz")
	_ = store.RecordView(context.Background(), "p1")
	svc := leaderboard.NewService(
		store,
		leaderboardPublished{items: []publish.Published{{ProjectID: "p1", Title: "小游戏", PublishedAt: time.Now()}}},
		leaderboardProjects{items: []*model.Project{{ID: "p1", Name: "Demo", OwnerUserID: 1}}},
		leaderboardUsers{},
	)
	h := NewLeaderboardHandler(svc, "https://play.example.com")
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/leaderboard", h.ListPublic, sessionForTest(3))
	e.GET("/cp/api/v1/admin/leaderboard", h.ListAdmin)
	e.PUT("/cp/api/v1/admin/leaderboard/:projectId/hidden", h.Hide)
	e.DELETE("/cp/api/v1/admin/leaderboard/:projectId", h.Delete)
	return e, svc
}

func TestLeaderboardPublicAndAdminFlow(t *testing.T) {
	e, _ := newLeaderboardHandlerForTest(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/leaderboard", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "小游戏") ||
		!strings.Contains(rec.Body.String(), "https://play.example.com/p/p1/") {
		t.Fatalf("public status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/leaderboard", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("admin status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/admin/leaderboard/p1/hidden", strings.NewReader(`{"hidden":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hidden":true`) {
		t.Fatalf("hide status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/leaderboard", nil))
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("hidden public body = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/admin/leaderboard/p1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/leaderboard", nil))
	if !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Fatalf("after delete body = %s", rec.Body.String())
	}
}

func TestLeaderboardHideRejectsInvalidBody(t *testing.T) {
	e, _ := newLeaderboardHandlerForTest(t)
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/admin/leaderboard/p1/hidden", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd packages/backend && go test ./internal/handler -run TestLeaderboard -v`

Expected: FAIL because `NewLeaderboardHandler` is undefined.

- [ ] **Step 3: Implement the handler**

Create `packages/backend/internal/handler/leaderboard.go`:

```go
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/leaderboard"
	"vibebiz/internal/response"
)

type LeaderboardHandler struct {
	Service       *leaderboard.Service
	PublicBaseURL string
}

func NewLeaderboardHandler(service *leaderboard.Service, publicBaseURL string) *LeaderboardHandler {
	return &LeaderboardHandler{Service: service, PublicBaseURL: publicBaseURL}
}

type publicLeaderboardItem struct {
	Rank      int    `json:"rank"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Author    string `json:"author"`
	Visits    int64  `json:"visits"`
	URL       string `json:"url"`
}

type adminLeaderboardItem struct {
	Rank        int    `json:"rank"`
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Visits      int64  `json:"visits"`
	Exposure    int64  `json:"exposure"`
	Clicks      int64  `json:"clicks"`
	Hidden      bool   `json:"hidden"`
	PublishedAt string `json:"publishedAt"`
}

func (h *LeaderboardHandler) ListPublic(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	items, err := h.Service.ListPublic(c.Request().Context(), limit)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50013, "load leaderboard failed")
	}
	base := leaderboardPublicBase(c, h.PublicBaseURL)
	out := make([]publicLeaderboardItem, 0, len(items))
	for _, item := range items {
		out = append(out, publicLeaderboardItem{
			Rank: item.Rank, ProjectID: item.ProjectID, Name: item.Name,
			Author: item.Author, Visits: item.Visits,
			URL: base + "/p/" + item.ProjectID + "/",
		})
	}
	return response.OK(c, map[string]any{"items": out})
}

func (h *LeaderboardHandler) ListAdmin(c echo.Context) error {
	rows, err := h.Service.ListAdmin(c.Request().Context())
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50013, "load leaderboard failed")
	}
	items := make([]adminLeaderboardItem, 0, len(rows))
	for index, row := range rows {
		items = append(items, adminLeaderboardItem{
			Rank: index + 1, ProjectID: row.ProjectID, Name: row.Name, Author: row.Author,
			Visits: row.Visits, Exposure: row.Exposure, Clicks: row.Clicks,
			Hidden: row.Hidden, PublishedAt: row.PublishedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	query := adminListQuery(c)
	filtered := make([]adminLeaderboardItem, 0, len(items))
	for _, item := range items {
		if admin.MatchesKeyword(query.Keyword, item.Name, item.Author, item.ProjectID) {
			filtered = append(filtered, item)
		}
	}
	page := admin.Paginate(filtered, query)
	return response.OK(c, map[string]any{
		"items": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func (h *LeaderboardHandler) Hide(c echo.Context) error {
	projectID := strings.TrimSpace(c.Param("projectId"))
	if projectID == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid project id")
	}
	var req struct {
		Hidden *bool `json:"hidden"`
	}
	if err := c.Bind(&req); err != nil || req.Hidden == nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	if err := h.Service.Hide(c.Request().Context(), projectID, *req.Hidden); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "update leaderboard failed")
	}
	return response.OK(c, map[string]any{"projectId": projectID, "hidden": *req.Hidden})
}

func (h *LeaderboardHandler) Delete(c echo.Context) error {
	projectID := strings.TrimSpace(c.Param("projectId"))
	if projectID == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid project id")
	}
	if err := h.Service.Delete(c.Request().Context(), projectID); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "delete leaderboard failed")
	}
	return response.OK(c, map[string]any{"projectId": projectID})
}

func leaderboardPublicBase(c echo.Context, configured string) string {
	if base := strings.TrimRight(strings.TrimSpace(configured), "/"); base != "" {
		return base
	}
	proto := strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
		if c.Request().TLS != nil {
			proto = "https"
		}
	}
	host := strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = c.Request().Host
	}
	return proto + "://" + host
}
```

- [ ] **Step 4: Register routes**

Modify `packages/backend/internal/router/control.go`:

1. Add parameter at the end of `RegisterControl`:

```go
leaderboardHandler *handler.LeaderboardHandler,
```

2. Inside `if publishApp != nil { ... }` where `meAPI` is used, add:

```go
meAPI.GET("/published-projects/stats", publishApp.PublishedStats)
```

3. Next to the public preview routes, add:

```go
e.POST("/p/:slug/track/click", publishApp.TrackClick)
```

4. After `projectsAPI` block, add:

```go
if leaderboardHandler != nil {
	api.GET("/leaderboard", leaderboardHandler.ListPublic, authn.RequireUser)
}
```

5. Inside the `adminProtected` block, add:

```go
if leaderboardHandler != nil {
	adminProtected.GET("/leaderboard", leaderboardHandler.ListAdmin)
	adminProtected.PUT("/leaderboard/:projectId/hidden", leaderboardHandler.Hide)
	adminProtected.DELETE("/leaderboard/:projectId", leaderboardHandler.Delete)
}
```

- [ ] **Step 5: Update router test calls**

In `packages/backend/internal/router/helpers_test.go:284` and `packages/backend/internal/router/span_test.go:45`, append `nil` as the final argument to `RegisterControl(...)` so the new parameter is satisfied.

- [ ] **Step 6: Run tests**

Run: `cd packages/backend && go test ./internal/handler ./internal/router -run Leaderboard -v`

Expected: PASS. `go test ./internal/router` may run all router tests; they must still compile.

---

### Task 5: 发布链路接入 PV / 点击 / 元数据 / 统计

**Files:**
- Modify: `packages/backend/internal/handler/publish.go`
- Modify: `packages/backend/internal/publish/share.html`
- Test: `packages/backend/internal/handler/publish_test.go`

**Interfaces:**
- Consumes: Task 3 `*leaderboard.Service`.
- Produces:
  - `PublishHandler.Leaderboard *leaderboard.Service`
  - `func (h *PublishHandler) TrackClick(c echo.Context) error`
  - `func (h *PublishHandler) PublishedStats(c echo.Context) error`

- [ ] **Step 1: Write the failing tests**

Append to `packages/backend/internal/handler/publish_test.go`:

```go
func newPublishedHandlerWithLeaderboard(t *testing.T) (*echo.Echo, *leaderboard.Service) {
	t.Helper()
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	source := filepath.Join(workspaces, "p1")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("<h1>game</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{
		ID: "p1", Name: "Demo", WorkspaceID: 3, OwnerUserID: 1,
	}); err != nil {
		t.Fatal(err)
	}
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	svc := leaderboard.NewService(
		leaderboard.NewStore(client, "vibebiz"),
		publish.NewService(workspaces, published),
		projects,
		leaderboardUsers{},
	)
	h := NewPublishHandler(publish.NewService(workspaces, published), projects, "https://play.example.com")
	h.Leaderboard = svc
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/publish", h.Publish, sessionForTest(3))
	routeWithSession(e, http.MethodGet, "/cp/api/v1/me/published-projects/stats", h.PublishedStats, sessionForTest(3))
	e.GET("/p/:slug/*", h.Preview)
	e.POST("/p/:slug/track/click", h.TrackClick)
	return e, svc
}

func TestPublishTracksViewAndClick(t *testing.T) {
	e, svc := newPublishedHandlerWithLeaderboard(t)
	ctx := context.Background()

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, newPublishRequest("/cp/api/v1/projects/p1/publish", `{"title":"小游戏"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}
	meta, _ := svc.Store.Meta(ctx, []string{"p1"})
	if meta["p1"].Title != "小游戏" || meta["p1"].Author != "alice" {
		t.Fatalf("meta = %+v", meta["p1"])
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/p1/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status = %d", rec.Code)
	}
	ranked, _ := svc.Store.Top(ctx, 10)
	if len(ranked) != 1 || ranked[0].Visits != 1 {
		t.Fatalf("ranked = %+v", ranked)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/p1/game/index.html", nil))
	ranked, _ = svc.Store.Top(ctx, 10)
	if ranked[0].Visits != 1 {
		t.Fatalf("asset request must not add PV: %+v", ranked)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/p/p1/track/click", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("click status = %d body=%s", rec.Code, rec.Body.String())
	}
	counters, _ := svc.Store.Counters(ctx, []string{"p1"})
	if counters["p1"].Clicks != 1 {
		t.Fatalf("counters = %+v", counters["p1"])
	}

	if _, err := svc.ListPublic(ctx, 50); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/me/published-projects/stats", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"exposure":1`) ||
		!strings.Contains(rec.Body.String(), `"clicks":1`) {
		t.Fatalf("stats status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestTrackClickRejectsUnknownSlug(t *testing.T) {
	e, _ := newPublishedHandlerWithLeaderboard(t)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/p/missing/track/click", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}
```

Add these imports to `publish_test.go`:

```go
"github.com/alicebob/miniredis/v2"
"github.com/redis/go-redis/v9"

"vibebiz/internal/leaderboard"
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd packages/backend && go test ./internal/handler -run 'TestPublishTracks|TestTrackClick' -v`

Expected: FAIL because `Leaderboard`, `TrackClick`, `PublishedStats` are undefined.

- [ ] **Step 3: Modify the publish handler**

In `packages/backend/internal/handler/publish.go`:

1. Add imports:

```go
"vibebiz/internal/leaderboard"
```

2. Add field to `PublishHandler`:

```go
Leaderboard *leaderboard.Service
```

3. In `Publish`, after `result, err := h.Service.Publish(...)` succeeds, add:

```go
if h.Leaderboard != nil {
	_ = h.Leaderboard.UpsertMeta(c.Request().Context(), project, req.Title)
}
```

4. In `DeletePublished`, after `h.Service.Delete(...)` succeeds, add:

```go
if h.Leaderboard != nil {
	_ = h.Leaderboard.Delete(c.Request().Context(), project.ID)
}
```

5. In `Preview`, after `path, err := h.Service.FilePath(...)` and the error check, before setting headers, add:

```go
if h.Leaderboard != nil && isEntryDocument(c.Param("*")) {
	_ = h.Leaderboard.RecordView(c.Request().Context(), c.Param("slug"))
}
```

6. Add helpers and handlers:

```go
func isEntryDocument(wildcard string) bool {
	path := strings.Trim(strings.TrimSpace(wildcard), "/")
	return path == "" || path == "index.html"
}

func (h *PublishHandler) TrackClick(c echo.Context) error {
	slug := c.Param("slug")
	if _, err := h.Service.FilePath(slug, ""); err != nil {
		return publishError(c, err)
	}
	if h.Leaderboard != nil {
		_ = h.Leaderboard.RecordClick(c.Request().Context(), slug)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *PublishHandler) PublishedStats(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	ctx := c.Request().Context()
	published, err := h.Service.List(ctx)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50011, "list published projects failed")
	}
	allowed := make(map[string]*model.Project)
	for _, project := range h.Projects.ListByWorkspaces(ctx, session.WorkspaceIDs) {
		if project.IsHidden {
			continue
		}
		allowed[project.ID] = project
	}
	inputs := make([]leaderboard.StatInput, 0, len(published))
	for _, item := range published {
		project, exists := allowed[item.ProjectID]
		if !exists {
			continue
		}
		name := item.Title
		if name == "" {
			name = project.Name
		}
		inputs = append(inputs, leaderboard.StatInput{ProjectID: item.ProjectID, Name: name})
	}
	stats := make([]leaderboard.ProjectStat, 0, len(inputs))
	if h.Leaderboard != nil {
		stats, err = h.Leaderboard.Stats(ctx, inputs)
		if err != nil {
			return response.Fail(c, http.StatusInternalServerError, 50015, "load published stats failed")
		}
	} else {
		for _, input := range inputs {
			stats = append(stats, leaderboard.ProjectStat{ProjectID: input.ProjectID, Name: input.Name})
		}
	}
	var totals leaderboard.ProjectStat
	for _, item := range stats {
		totals.Exposure += item.Exposure
		totals.Clicks += item.Clicks
	}
	return response.OK(c, map[string]any{
		"exposure": totals.Exposure, "clicks": totals.Clicks,
		"dwellSeconds": 0, "spend": 0, "projects": stats,
	})
}
```

- [ ] **Step 4: Add the click beacon to the share page**

In `packages/backend/internal/publish/share.html`, replace the `play` function block with:

```html
      (function () {
        var frame = document.getElementById('game-frame');
        var start = document.getElementById('start');
        var exit = document.getElementById('exit');
        function trackClick() {
          if (frame.dataset.tracked === '1') return;
          frame.dataset.tracked = '1';
          var base = window.location.pathname.replace(/\/index\.html$/, '/').replace(/\/?$/, '/');
          var endpoint = base + 'track/click';
          var payload = new Blob([], { type: 'text/plain' });
          if (navigator.sendBeacon) {
            navigator.sendBeacon(endpoint, payload);
          } else {
            fetch(endpoint, { method: 'POST', keepalive: true });
          }
        }
        function play() {
          if (!frame.src) frame.src = frame.dataset.src;
          trackClick();
          document.body.classList.add('playing');
        }
        start.addEventListener('click', play);
        exit.addEventListener('click', function () { window.location.reload(); });
        if (new URLSearchParams(window.location.search).get('play') === '1') play();
      })();
```

- [ ] **Step 5: Run tests**

Run: `cd packages/backend && go test ./internal/handler -run 'TestPublish|TestTrackClick|TestDeletePublished|TestListPublished|TestPublishedPreview' -v`

Expected: PASS, including existing publish tests.

---

### Task 6: CP 装配

**Files:**
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/internal/router/control.go` (already changed in Task 4)
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`

**Interfaces:**
- Consumes: Task 3 `leaderboard.NewService`, Task 4 `handler.NewLeaderboardHandler`.
- Produces: running CP with wired leaderboard.

- [ ] **Step 1: Add wiring in `cmd/cp/main.go`**

After `publishHandler := ...` and its cover fields, add:

```go
leaderboardService := leaderboard.NewService(
	leaderboard.NewStore(redisClient, redisPrefix),
	publishService,
	projectStore,
	identity,
)
publishHandler.Leaderboard = leaderboardService
leaderboardHandler := handler.NewLeaderboardHandler(leaderboardService, cfg.PublicBaseURL)
```

Add import:

```go
"vibebiz/internal/leaderboard"
```

- [ ] **Step 2: Pass the handler to the router**

At the call to `router.RegisterControl(...)`, append `leaderboardHandler` as the final argument.

- [ ] **Step 3: Verify compile and tests**

Run:

```bash
cd packages/backend && go build ./... && go test ./...
```

Expected: PASS. Fix any positional argument mismatch in `router/helpers_test.go` and `router/span_test.go` by appending `nil` if not already done in Task 4.

---

### Task 7: Web API 与类型

**Files:**
- Create: `packages/web/src/api/leaderboard.ts`
- Create: `packages/web/src/api/leaderboard.test.ts`
- Modify: `packages/web/src/api/publish.ts`
- Modify: `packages/web/src/api/publish.test.ts`
- Modify: `packages/web/src/types/index.ts`

**Interfaces:**
- Produces:
  - `getLeaderboard(limit?: number): Promise<LeaderboardEntry[]>`
  - `getPublishedStats(): Promise<PublishedStats>`
  - types `LeaderboardEntry`, `PublishedProjectStats`, `PublishedStats`

- [ ] **Step 1: Add types**

In `packages/web/src/types/index.ts`, after `PublishedProject`:

```ts
export interface LeaderboardEntry {
  rank: number
  projectId: string
  name: string
  author: string
  visits: number
  url: string
}

export interface PublishedProjectStats {
  projectId: string
  name: string
  exposure: number
  clicks: number
  dwellSeconds: number
  spend: number
}

export interface PublishedStats {
  exposure: number
  clicks: number
  dwellSeconds: number
  spend: number
  projects: PublishedProjectStats[]
}
```

- [ ] **Step 2: Write the failing API tests**

Create `packages/web/src/api/leaderboard.test.ts`:

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import { getLeaderboard } from './leaderboard'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
})

describe('leaderboard api', () => {
  it('unwraps leaderboard items', async () => {
    requestMock.mockResolvedValue({
      items: [
        {
          rank: 1,
          projectId: 'p1',
          name: '小游戏',
          author: 'alice',
          visits: 12,
          url: 'https://play.example.com/p/p1/',
        },
      ],
    })
    await expect(getLeaderboard()).resolves.toEqual([
      expect.objectContaining({ projectId: 'p1', visits: 12 }),
    ])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/leaderboard',
      params: { limit: 50 },
    })
  })
})
```

Append to `packages/web/src/api/publish.test.ts`:

```ts
  it('loads published stats', async () => {
    requestMock.mockResolvedValue({
      exposure: 9,
      clicks: 3,
      dwellSeconds: 0,
      spend: 0,
      projects: [],
    })
    await expect(getPublishedStats()).resolves.toMatchObject({ exposure: 9, clicks: 3 })
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/me/published-projects/stats',
    })
  })
```

Update the import line in `publish.test.ts`:

```ts
import { deletePublishedProject, getPublishedStats, listPublishedProjects, publishProject } from './publish'
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd packages/web && npx vitest run src/api/leaderboard.test.ts src/api/publish.test.ts`

Expected: FAIL because `leaderboard.ts` and `getPublishedStats` are undefined.

- [ ] **Step 4: Implement the APIs**

Create `packages/web/src/api/leaderboard.ts`:

```ts
import { cpHttp, request } from './client'
import type { LeaderboardEntry } from '@/types'

export async function getLeaderboard(limit = 50): Promise<LeaderboardEntry[]> {
  const result = await request<{ items: LeaderboardEntry[] }>(cpHttp, {
    method: 'get',
    url: '/leaderboard',
    params: { limit },
  })
  return result.items ?? []
}
```

In `packages/web/src/api/publish.ts`, add the import and function:

```ts
import type { PublishedProject, PublishedStats } from '@/types'
```

```ts
export async function getPublishedStats(): Promise<PublishedStats> {
  return request<PublishedStats>(cpHttp, {
    method: 'get',
    url: '/me/published-projects/stats',
  })
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd packages/web && npx vitest run src/api/leaderboard.test.ts src/api/publish.test.ts`

Expected: PASS.

---

### Task 8: Web 排行榜页面与导航

**Files:**
- Create: `packages/web/src/components/home/LeaderboardPanel.vue`
- Create: `packages/web/src/components/home/LeaderboardPanel.test.ts`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/views/HomeView.vue`
- Modify: `packages/web/src/router/index.ts`

**Interfaces:**
- Consumes: Task 7 `getLeaderboard`, `LeaderboardEntry`.
- Produces: `/leaderboard` page and sidebar entry.

- [ ] **Step 1: Write the failing component test**

Create `packages/web/src/components/home/LeaderboardPanel.test.ts`:

```ts
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { getLeaderboard } from '@/api/leaderboard'
import LeaderboardPanel from './LeaderboardPanel.vue'

vi.mock('@/api/leaderboard', () => ({ getLeaderboard: vi.fn() }))

const getLeaderboardMock = vi.mocked(getLeaderboard)
let app: App | null = null
let host: HTMLElement

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(LeaderboardPanel)
  app.mount(host)
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('LeaderboardPanel', () => {
  it('renders ranked cards', async () => {
    getLeaderboardMock.mockResolvedValue([
      {
        rank: 1,
        projectId: 'p1',
        name: '小游戏',
        author: 'alice',
        visits: 12,
        url: 'https://play.example.com/p/p1/',
      },
      {
        rank: 2,
        projectId: 'p2',
        name: '答题',
        author: 'bob',
        visits: 8,
        url: 'https://play.example.com/p/p2/',
      },
    ])
    mount()
    await flush()
    expect(host.querySelectorAll('.leaderboard-card')).toHaveLength(2)
    expect(host.textContent).toContain('小游戏')
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('12')
  })

  it('shows the empty state', async () => {
    getLeaderboardMock.mockResolvedValue([])
    mount()
    await flush()
    expect(host.textContent).toContain('暂无排行榜数据')
  })

  it('shows the error state', async () => {
    getLeaderboardMock.mockRejectedValue(new Error('network down'))
    mount()
    await flush()
    expect(host.textContent).toContain('network down')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npx vitest run src/components/home/LeaderboardPanel.test.ts`

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the component**

Create `packages/web/src/components/home/LeaderboardPanel.vue`:

```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { getLeaderboard } from '@/api/leaderboard'
import type { LeaderboardEntry } from '@/types'

const entries = ref<LeaderboardEntry[]>([])
const loading = ref(true)
const error = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    entries.value = await getLeaderboard()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '排行榜加载失败'
    entries.value = []
  } finally {
    loading.value = false
  }
}

function open(entry: LeaderboardEntry) {
  window.open(entry.url, '_blank', 'noopener,noreferrer')
}

onMounted(load)
</script>

<template>
  <div class="projects-page leaderboard-page">
    <header class="projects-head">
      <h1>发布作品排行榜</h1>
      <span>{{ entries.length }} 个作品</span>
    </header>

    <p v-if="loading" class="discovery-empty">加载中...</p>
    <p v-else-if="error" class="error-text" role="alert">{{ error }}</p>
    <p v-else-if="entries.length === 0" class="discovery-empty">暂无排行榜数据</p>
    <div v-else class="leaderboard-grid">
      <article
        v-for="entry in entries"
        :key="entry.projectId"
        class="leaderboard-card"
        role="button"
        tabindex="0"
        @click="open(entry)"
        @keydown.enter="open(entry)"
      >
        <span class="leaderboard-rank" :class="{ top: entry.rank <= 3 }">#{{ entry.rank }}</span>
        <div class="leaderboard-body">
          <strong>{{ entry.name }}</strong>
          <span class="leaderboard-author">作者：{{ entry.author || '未知作者' }}</span>
          <span class="leaderboard-visits">{{ entry.visits }} 次访问</span>
        </div>
      </article>
    </div>
  </div>
</template>

<style scoped>
.leaderboard-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.leaderboard-card {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
  padding: 18px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    transform 0.15s ease;
}

.leaderboard-card:hover {
  border-color: var(--line-strong);
  transform: translateY(-1px);
}

.leaderboard-rank {
  display: grid;
  flex: 0 0 auto;
  width: 44px;
  height: 44px;
  place-items: center;
  background: var(--soft);
  border-radius: 12px;
  color: var(--muted);
  font-size: 13px;
  font-weight: 700;
}

.leaderboard-rank.top {
  background: var(--blue-soft);
  color: var(--blue);
}

.leaderboard-body {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 6px;
}

.leaderboard-body strong {
  overflow: hidden;
  color: var(--ink);
  font-size: 15px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.leaderboard-author,
.leaderboard-visits {
  color: var(--muted);
  font-size: 12px;
}

@media (max-width: 720px) {
  .leaderboard-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
```

- [ ] **Step 4: Wire the sidebar, route and view**

In `packages/web/src/components/home/HomeSidebar.vue`, add to `navigation`:

```ts
  { label: '排行榜', icon: 'pi pi-trophy', route: 'leaderboard', visible: true },
```

Place it after the `首页` entry.

In `packages/web/src/router/index.ts`, add:

```ts
    { path: '/leaderboard', name: 'leaderboard', component: HomeView },
```

Place it after the `home` route. In the `afterEach` title chain, add a branch:

```ts
          : name === 'leaderboard'
            ? 'Vibebiz · 排行榜'
```

Put it before the final `: 'Vibebiz'`.

In `packages/web/src/views/HomeView.vue`:

1. Import the panel:

```ts
import LeaderboardPanel from '@/components/home/LeaderboardPanel.vue'
```

2. Add the computed:

```ts
const isLeaderboardPage = computed(() => route.name === 'leaderboard')
```

3. Add the branch before the default home content:

```vue
      <LeaderboardPanel v-else-if="isLeaderboardPage" />
```

Place it after the `ScheduledTasksPanel` branch and before the default `<div v-else class="home-main-inner">`.

- [ ] **Step 5: Run tests and build**

Run:

```bash
cd packages/web && npx vitest run src/components/home/LeaderboardPanel.test.ts && npm run build
```

Expected: PASS and build succeeds.

---

### Task 9: Web 账户设置发布数据面板

**Files:**
- Create: `packages/web/src/components/settings/PublishedStatsPanel.vue`
- Create: `packages/web/src/components/settings/PublishedStatsPanel.test.ts`
- Modify: `packages/web/src/views/settings/ProfileView.vue`

**Interfaces:**
- Consumes: Task 7 `getPublishedStats`, `PublishedStats`.
- Produces: `PublishedStatsPanel` component and settings integration.

- [ ] **Step 1: Write the failing component test**

Create `packages/web/src/components/settings/PublishedStatsPanel.test.ts`:

```ts
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import type { PublishedStats } from '@/types'
import PublishedStatsPanel from './PublishedStatsPanel.vue'

let app: App | null = null
let host: HTMLElement

function mount(stats: PublishedStats | null, loading = false, error = '') {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(PublishedStatsPanel, { stats, loading, error })
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('PublishedStatsPanel', () => {
  it('renders totals and project rows', () => {
    mount({
      exposure: 9,
      clicks: 3,
      dwellSeconds: 0,
      spend: 0,
      projects: [
        {
          projectId: 'p1',
          name: '小游戏',
          exposure: 9,
          clicks: 3,
          dwellSeconds: 0,
          spend: 0,
        },
      ],
    })
    expect(host.textContent).toContain('曝光量')
    expect(host.textContent).toContain('9')
    expect(host.textContent).toContain('小游戏')
    expect(host.textContent).toContain('0 秒')
  })

  it('shows the empty state', () => {
    mount({ exposure: 0, clicks: 0, dwellSeconds: 0, spend: 0, projects: [] })
    expect(host.textContent).toContain('暂无发布数据')
  })

  it('shows loading and error states', () => {
    mount(null, true)
    expect(host.textContent).toContain('加载中')
    app?.unmount()
    host.remove()
    mount(null, false, '加载失败')
    expect(host.textContent).toContain('加载失败')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npx vitest run src/components/settings/PublishedStatsPanel.test.ts`

Expected: FAIL because the component does not exist.

- [ ] **Step 3: Implement the panel**

Create `packages/web/src/components/settings/PublishedStatsPanel.vue`:

```vue
<script setup lang="ts">
import type { PublishedStats } from '@/types'

const props = withDefaults(
  defineProps<{
    stats: PublishedStats | null
    loading?: boolean
    error?: string
  }>(),
  {
    loading: false,
    error: '',
  },
)
</script>

<template>
  <section class="settings-published-stats">
    <header class="stats-head">
      <div>
        <h2>发布数据</h2>
        <p>统计已发布作品的曝光与点击。</p>
      </div>
    </header>

    <p v-if="props.loading" class="settings-state">加载中…</p>
    <p v-else-if="props.error" class="settings-error" role="alert">{{ props.error }}</p>
    <template v-else>
      <div class="stats-grid">
        <article class="stat-card">
          <span>曝光量</span>
          <strong>{{ props.stats?.exposure ?? 0 }}</strong>
        </article>
        <article class="stat-card">
          <span>点击数</span>
          <strong>{{ props.stats?.clicks ?? 0 }}</strong>
        </article>
        <article class="stat-card">
          <span>停留时长</span>
          <strong>{{ props.stats?.dwellSeconds ?? 0 }} 秒</strong>
        </article>
        <article class="stat-card">
          <span>消费额度</span>
          <strong>{{ props.stats?.spend ?? 0 }}</strong>
        </article>
      </div>

      <p v-if="!props.stats || props.stats.projects.length === 0" class="settings-state">
        暂无发布数据
      </p>
      <table v-else class="stats-table">
        <thead>
          <tr>
            <th>项目</th>
            <th>曝光量</th>
            <th>点击数</th>
            <th>停留时长</th>
            <th>消费额度</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="item in props.stats.projects" :key="item.projectId">
            <td>{{ item.name }}</td>
            <td>{{ item.exposure }}</td>
            <td>{{ item.clicks }}</td>
            <td>{{ item.dwellSeconds }} 秒</td>
            <td>{{ item.spend }}</td>
          </tr>
        </tbody>
      </table>
    </template>
  </section>
</template>

<style scoped>
.settings-published-stats {
  margin-top: 32px;
}

.stats-head h2 {
  margin: 0;
  color: var(--ink);
  font-size: 16px;
}

.stats-head p {
  margin: 6px 0 16px;
  color: var(--muted);
  font-size: 12px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.stat-card {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.stat-card span {
  color: var(--muted);
  font-size: 12px;
}

.stat-card strong {
  color: var(--ink);
  font-size: 22px;
}

.stats-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.stats-table th,
.stats-table td {
  padding: 10px 12px;
  border-top: 1px solid var(--line);
  text-align: left;
}

.stats-table th {
  background: var(--soft);
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}

.stats-table td {
  color: var(--text);
}

@media (max-width: 720px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
```

- [ ] **Step 4: Integrate into ProfileView**

In `packages/web/src/views/settings/ProfileView.vue`:

1. Import:

```ts
import { deletePublishedProject, getPublishedStats, listPublishedProjects } from '@/api/publish'
import PublishedStatsPanel from '@/components/settings/PublishedStatsPanel.vue'
import type { Presign, PublishedProject, PublishedStats } from '@/types'
```

2. Add state near the published state:

```ts
const stats = ref<PublishedStats | null>(null)
const statsLoading = ref(true)
const statsError = ref('')
```

3. Add a loader:

```ts
async function loadStats() {
  statsLoading.value = true
  statsError.value = ''
  try {
    stats.value = await getPublishedStats()
  } catch (err) {
    statsError.value = err instanceof Error ? err.message : '发布数据加载失败'
  } finally {
    statsLoading.value = false
  }
}
```

4. In `onMounted`, add `void loadStats()` next to `void loadPublished()`.

5. In the template, before `<section class="settings-published">`, add:

```vue
    <PublishedStatsPanel :stats="stats" :loading="statsLoading" :error="statsError" />
```

- [ ] **Step 5: Run tests and build**

Run:

```bash
cd packages/web && npx vitest run src/components/settings/PublishedStatsPanel.test.ts && npm run build
```

Expected: PASS and build succeeds.

---

### Task 10: ops-web API 与类型

**Files:**
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/api/leaderboard.test.ts`
- Modify: `packages/ops-web/src/types/index.ts`

**Interfaces:**
- Produces:
  - `type AdminLeaderboardRow`
  - `getLeaderboard(query?: PageQuery): Promise<PageResult<AdminLeaderboardRow>>`
  - `updateLeaderboardHidden(projectId: string, hidden: boolean): Promise<void>`
  - `deleteLeaderboardEntry(projectId: string): Promise<void>`

- [ ] **Step 1: Add the type**

In `packages/ops-web/src/types/index.ts`, after `AdminProject`:

```ts
export interface AdminLeaderboardRow {
  rank: number
  projectId: string
  name: string
  author: string
  visits: number
  exposure: number
  clicks: number
  hidden: boolean
  publishedAt: string
}
```

- [ ] **Step 2: Write the failing API test**

Create `packages/ops-web/src/api/leaderboard.test.ts`:

```ts
import { describe, expect, it, vi } from 'vitest'

import {
  deleteLeaderboardEntry,
  getLeaderboard,
  updateLeaderboardHidden,
} from './admin'
import { http } from './client'

describe('leaderboard admin api', () => {
  it('loads a page of leaderboard rows', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: {
        code: 0,
        message: 'ok',
        result: {
          items: [
            {
              rank: 1,
              projectId: 'p1',
              name: '小游戏',
              author: 'alice',
              visits: 12,
              exposure: 24,
              clicks: 3,
              hidden: false,
              publishedAt: '2026-11-07T10:00:00Z',
            },
          ],
          total: 1,
          page: 1,
          pageSize: 10,
        },
      },
    })
    const page = await getLeaderboard({ page: 1, pageSize: 10, keyword: '小' })
    expect(page.items).toHaveLength(1)
    expect(page.total).toBe(1)
    expect(spy).toHaveBeenCalledWith({
      method: 'get',
      url: '/leaderboard',
      params: { page: 1, pageSize: 10, keyword: '小' },
    })
  })

  it('hides and deletes leaderboard entries', async () => {
    const hideSpy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { projectId: 'p1', hidden: true } },
    })
    await updateLeaderboardHidden('p1', true)
    expect(hideSpy).toHaveBeenCalledWith({
      method: 'put',
      url: '/leaderboard/p1/hidden',
      data: { hidden: true },
    })

    const deleteSpy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { projectId: 'p1' } },
    })
    await deleteLeaderboardEntry('p1')
    expect(deleteSpy).toHaveBeenCalledWith({
      method: 'delete',
      url: '/leaderboard/p1',
    })
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd packages/ops-web && npx vitest run src/api/leaderboard.test.ts`

Expected: FAIL because the functions are undefined.

- [ ] **Step 4: Implement the API**

In `packages/ops-web/src/api/admin.ts`:

1. Add `AdminLeaderboardRow` to the type import list.
2. Add after `getProjects`/`updateProjectHidden`:

```ts
export async function getLeaderboard(
  query: PageQuery = {},
): Promise<PageResult<AdminLeaderboardRow>> {
  const result = await request<Record<string, unknown>>({
    method: 'get',
    url: '/leaderboard',
    params: pageParams(query),
  })
  return toPage<AdminLeaderboardRow>(result, 'items')
}

export async function updateLeaderboardHidden(projectId: string, hidden: boolean): Promise<void> {
  await request<{ projectId: string; hidden: boolean }>({
    method: 'put',
    url: `/leaderboard/${encodeURIComponent(projectId)}/hidden`,
    data: { hidden },
  })
}

export async function deleteLeaderboardEntry(projectId: string): Promise<void> {
  await request<{ projectId: string }>({
    method: 'delete',
    url: `/leaderboard/${encodeURIComponent(projectId)}`,
  })
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/ops-web && npx vitest run src/api/leaderboard.test.ts`

Expected: PASS.

---

### Task 11: ops-web 排行榜页面与导航

**Files:**
- Create: `packages/ops-web/src/views/LeaderboardView.vue`
- Create: `packages/ops-web/src/views/LeaderboardView.test.ts`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: Task 10 APIs and `AdminLeaderboardRow`.
- Produces: `/leaderboard` ops page.

- [ ] **Step 1: Update the navigation test**

In `packages/ops-web/src/router/navigation.test.ts`, update the expected path array to insert `'/leaderboard'` after `'/projects'`:

```ts
      '/projects',
      '/leaderboard',
      '/sandboxes',
```

Update the label array to insert `'排行榜'` after `'项目'`:

```ts
      '项目',
      '排行榜',
      '沙箱节点',
```

- [ ] **Step 2: Write the failing view test**

Create `packages/ops-web/src/views/LeaderboardView.test.ts`:

```ts
import { createApp, type App } from 'vue'
import PrimeVue from 'primevue/config'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  deleteLeaderboardEntry,
  getLeaderboard,
  updateLeaderboardHidden,
} from '@/api/admin'
import LeaderboardView from './LeaderboardView.vue'

vi.mock('@/api/admin', () => ({
  getLeaderboard: vi.fn(),
  updateLeaderboardHidden: vi.fn(),
  deleteLeaderboardEntry: vi.fn(),
}))

const getLeaderboardMock = vi.mocked(getLeaderboard)
const hideMock = vi.mocked(updateLeaderboardHidden)
const deleteMock = vi.mocked(deleteLeaderboardEntry)

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

beforeEach(() => {
  getLeaderboardMock.mockResolvedValue({
    items: [
      {
        rank: 1,
        projectId: 'p1',
        name: '小游戏',
        author: 'alice',
        visits: 12,
        exposure: 24,
        clicks: 3,
        hidden: false,
        publishedAt: '2026-11-07T10:00:00Z',
      },
    ],
    total: 1,
    page: 1,
    pageSize: 10,
  })
  hideMock.mockResolvedValue(undefined)
  deleteMock.mockResolvedValue(undefined)
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(LeaderboardView)
  app.use(PrimeVue)
  app.mount(host)
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  vi.restoreAllMocks()
})

describe('LeaderboardView', () => {
  it('renders leaderboard rows', async () => {
    await flush()
    expect(host.textContent).toContain('小游戏')
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('12')
  })

  it('hides a row', async () => {
    await flush()
    const button = Array.from(host.querySelectorAll('button')).find((item) =>
      item.textContent?.includes('隐藏'),
    )
    expect(button).toBeTruthy()
    button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flush()
    expect(hideMock).toHaveBeenCalledWith('p1', true)
  })

  it('deletes a row after confirmation', async () => {
    await flush()
    const button = Array.from(host.querySelectorAll('button')).find((item) =>
      item.textContent?.includes('删除'),
    )
    expect(button).toBeTruthy()
    button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flush()
    expect(deleteMock).toHaveBeenCalledWith('p1')
  })
})
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd packages/ops-web && npx vitest run src/views/LeaderboardView.test.ts`

Expected: FAIL because the view does not exist.

- [ ] **Step 4: Implement the view**

Create `packages/ops-web/src/views/LeaderboardView.vue`:

```vue
<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'

import {
  deleteLeaderboardEntry,
  getLeaderboard,
  updateLeaderboardHidden,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminLeaderboardRow, TableColumn } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getLeaderboard)

const columns: TableColumn[] = [
  { key: 'rank', label: '排名', width: '70px' },
  { key: 'name', label: '项目名' },
  { key: 'author', label: '作者', width: '140px' },
  { key: 'visits', label: '访问数', width: '100px' },
  { key: 'exposure', label: '曝光量', width: '100px' },
  { key: 'clicks', label: '点击数', width: '100px' },
  { key: 'hidden', label: '状态', width: '100px' },
  { key: 'actions', label: '操作', width: '220px' },
]

const notice = ref('')
const actionError = ref('')
const busyId = ref('')

async function toggleHidden(row: unknown) {
  const item = row as AdminLeaderboardRow
  busyId.value = item.projectId
  actionError.value = ''
  try {
    await updateLeaderboardHidden(item.projectId, !item.hidden)
    notice.value = item.hidden ? '已取消隐藏' : '已隐藏'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '更新排行榜失败'
  } finally {
    busyId.value = ''
  }
}

async function remove(row: unknown) {
  const item = row as AdminLeaderboardRow
  if (!window.confirm(`确定删除「${item.name}」的榜单数据吗？已发布应用不会被删除。`)) {
    return
  }
  busyId.value = item.projectId
  actionError.value = ''
  try {
    await deleteLeaderboardEntry(item.projectId)
    notice.value = '榜单数据已删除'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除榜单数据失败'
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>排行榜</h1>
        <p class="page-desc">查看已发布作品热度，隐藏或删除榜单条目。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>
    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无排行榜数据"
      @retry="load"
    >
      <template #cell-hidden="{ value }">{{ value ? '已隐藏' : '正常' }}</template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            :label="(row as AdminLeaderboardRow).hidden ? '取消隐藏' : '隐藏'"
            size="small"
            text
            :loading="busyId === (row as AdminLeaderboardRow).projectId"
            @click="toggleHidden(row)"
          />
          <Button
            label="删除"
            size="small"
            text
            :loading="busyId === (row as AdminLeaderboardRow).projectId"
            @click="remove(row)"
          />
        </div>
      </template>
    </DataTable>

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />
  </div>
</template>

<style scoped>
.inline-notice {
  margin-bottom: 12px;
  color: var(--blue);
  font-size: 13px;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
</style>
```

- [ ] **Step 5: Add the route and navigation item**

In `packages/ops-web/src/router/navigation.ts`, insert after the projects entry:

```ts
  { name: 'leaderboard', label: '排行榜', path: '/leaderboard', icon: 'pi pi-trophy' },
```

In `packages/ops-web/src/router/routes.ts`, import the view and insert the route after projects, matching the navigation order:

```ts
import LeaderboardView from '@/views/LeaderboardView.vue'
```

```ts
  { path: 'leaderboard', name: 'leaderboard', component: LeaderboardView, meta: { title: '排行榜' } },
```

- [ ] **Step 6: Run tests**

Run:

```bash
cd packages/ops-web && npx vitest run src/api/leaderboard.test.ts src/views/LeaderboardView.test.ts src/router/navigation.test.ts && npm run build
```

Expected: PASS and build succeeds.

---

### Task 12: 全量验证与验收

**Files:** no new files.

- [ ] **Step 1: Run backend tests**

```bash
cd packages/backend && gofmt -w internal/leaderboard/*.go internal/handler/leaderboard.go internal/handler/publish.go internal/dao/identity_repository.go cmd/cp/main.go && go vet ./... && go test ./...
```

Expected: PASS.

- [ ] **Step 2: Run web tests, build and lint**

```bash
cd packages/web && npm test -- --run && npm run build && npm run lint
```

Expected: PASS.

- [ ] **Step 3: Run ops-web tests, build and lint**

```bash
cd packages/ops-web && npm test -- --run && npm run build && npm run lint
```

Expected: PASS.

- [ ] **Step 4: Run repository validation**

```bash
make validate
```

Expected: PASS.

- [ ] **Step 5: Manual acceptance checklist**

1. 启动 `make dev`，登录 web。
2. 左侧菜单出现「排行榜」，进入后每行 2 个卡片，展示名次、项目名、作者、访问数。
3. 打开一个已发布分享页根地址，刷新排行榜，访问数增加；打开 `game/index.html` 不增加。
4. 在分享页点击「开始体验」，设置页点击数增加。
5. 刷新排行榜页，设置页曝光量增加；ops-web 查看排行榜不增加曝光。
6. ops-web「排行榜」页表格展示排名、项目名、作者、访问数、曝光量、点击数、状态；隐藏后用户榜消失，取消隐藏后恢复。
7. ops-web 删除榜单数据后，用户榜不再展示该项目；分享页仍可访问；再次访问后重新上榜。
8. 账户设置「我发布的项目」上方显示 4 个指标和项目明细，停留时长与消费额度为 0。

---

## Self-Review Notes

- Spec coverage: 统计口径（Task 1/3/5）、Redis 结构（Task 1）、用户榜（Task 3/4/8）、ops 隐藏删除（Task 3/4/11）、设置统计（Task 5/9）、发布页埋点（Task 5）、无 schema 变更（Global Constraints）。
- Type consistency: `leaderboard.Item`/`AdminRow`/`ProjectStat` 在 Task 3 定义，Task 4/5 使用；web/ops 类型在 Task 7/10 定义，Task 8/9/11 使用。
- 无提交步骤，避免在用户未授权时执行 git commit。
