package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/dao/model"
)

func newTestCache(t *testing.T) (*Cache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewCache(client, "vibebiz"), mr
}

func publishedItem(id int64, publishedAt time.Time, expireAt time.Time) model.OpsNotification {
	return model.OpsNotification{
		Base:        model.Base{ID: id, CreatedAt: publishedAt},
		Title:       "v1.2.0 发布",
		Content:     "## 更新",
		Type:        model.NotificationVersionRelease,
		Version:     "v1.2.0",
		Link:        "https://help.atoms.dev/zh",
		Status:      model.NotificationPublished,
		ExpireAt:    expireAt,
		PublishedAt: &publishedAt,
	}
}

func TestCachePutListAndTTL(t *testing.T) {
	cache, mr := newTestCache(t)
	now := time.Now().UTC().Truncate(time.Second)
	item := publishedItem(12, now, now.Add(7*24*time.Hour))

	if err := cache.Put(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	list, err := cache.List(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != 12 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	if list[0].Title != item.Title || list[0].Type != item.Type || list[0].Version != item.Version {
		t.Fatalf("list item = %+v, want fields from %+v", list[0], item)
	}
	ttl := mr.TTL("vibebiz:notification:item:12")
	if ttl <= 0 || ttl > 7*24*time.Hour {
		t.Fatalf("ttl = %v", ttl)
	}
}

func TestCacheListNewestFirst(t *testing.T) {
	cache, _ := newTestCache(t)
	now := time.Now().UTC().Truncate(time.Second)
	expireAt := now.Add(24 * time.Hour)

	for _, id := range []int64{1, 2, 3} {
		publishedAt := now.Add(time.Duration(id) * time.Minute)
		if err := cache.Put(context.Background(), publishedItem(id, publishedAt, expireAt)); err != nil {
			t.Fatal(err)
		}
	}
	list, err := cache.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != 3 || list[1].ID != 2 || list[2].ID != 1 {
		t.Fatalf("list = %+v, want ids [3 2 1]", list)
	}
}

func TestCacheExpiredItemIsFiltered(t *testing.T) {
	cache, mr := newTestCache(t)
	now := time.Now().UTC().Truncate(time.Second)
	item := publishedItem(5, now, now.Add(time.Hour))
	if err := cache.Put(context.Background(), item); err != nil {
		t.Fatal(err)
	}

	mr.FastForward(2 * time.Hour)

	list, err := cache.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("list = %+v, want empty", list)
	}
	if mr.Exists("vibebiz:notification:item:5") {
		t.Fatal("expired item key still exists")
	}
	if mr.Exists("vibebiz:notification:index") {
		members, err := mr.ZMembers("vibebiz:notification:index")
		if err != nil {
			t.Fatal(err)
		}
		if len(members) != 0 {
			t.Fatalf("index members = %v, want stale id removed", members)
		}
	}
}

func TestCacheRemove(t *testing.T) {
	cache, mr := newTestCache(t)
	now := time.Now().UTC().Truncate(time.Second)
	expireAt := now.Add(time.Hour)
	if err := cache.Put(context.Background(), publishedItem(7, now, expireAt)); err != nil {
		t.Fatal(err)
	}
	if err := cache.Put(context.Background(), publishedItem(8, now.Add(time.Minute), expireAt)); err != nil {
		t.Fatal(err)
	}

	if err := cache.Remove(context.Background(), 7); err != nil {
		t.Fatal(err)
	}
	if mr.Exists("vibebiz:notification:item:7") {
		t.Fatal("item key still exists")
	}
	list, err := cache.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != 8 {
		t.Fatalf("list = %+v, want id 8 only", list)
	}
}

func TestCacheListReportsMissingIndex(t *testing.T) {
	cache, _ := newTestCache(t)

	list, err := cache.List(context.Background())
	if !errors.Is(err, ErrIndexMissing) {
		t.Fatalf("err = %v, want ErrIndexMissing", err)
	}
	if list != nil {
		t.Fatalf("list = %+v, want nil", list)
	}
}

func TestCacheRebuild(t *testing.T) {
	cache, _ := newTestCache(t)
	now := time.Now().UTC().Truncate(time.Second)
	expireAt := now.Add(24 * time.Hour)

	if err := cache.Put(context.Background(), publishedItem(1, now, expireAt)); err != nil {
		t.Fatal(err)
	}
	if err := cache.Rebuild(context.Background(), []model.OpsNotification{
		publishedItem(2, now.Add(time.Minute), expireAt),
		publishedItem(3, now.Add(2*time.Minute), expireAt),
	}); err != nil {
		t.Fatal(err)
	}

	list, err := cache.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].ID != 3 || list[1].ID != 2 {
		t.Fatalf("list = %+v, want ids [3 2]", list)
	}
}
