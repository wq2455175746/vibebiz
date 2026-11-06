package usage

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepo struct {
	quota  Quota
	usage  Usage
	deltas []Usage
	rows   []DailyRow
}

func (f *fakeRepo) Quota(context.Context, int64) (Quota, error) { return f.quota, nil }

func (f *fakeRepo) Usage(context.Context, int64, time.Time) (Usage, error) { return f.usage, nil }

func (f *fakeRepo) Increment(_ context.Context, _ int64, _ time.Time, delta Usage) error {
	f.deltas = append(f.deltas, delta)
	return nil
}

func (f *fakeRepo) Series(context.Context, Filter) ([]DailyRow, error) { return f.rows, nil }

func newTestService(repo *fakeRepo, now time.Time) *Service {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	service := NewService(repo, loc)
	service.Now = func() time.Time { return now }
	return service
}

func TestCheckProjectCreateUnlimited(t *testing.T) {
	service := newTestService(&fakeRepo{}, time.Now())
	if err := service.CheckProjectCreate(context.Background(), 1); err != nil {
		t.Fatalf("err = %v", err)
	}
}

func TestCheckProjectCreateExceeded(t *testing.T) {
	service := newTestService(&fakeRepo{
		quota: Quota{ProjectDailyQuota: 3},
		usage: Usage{ProjectsCreated: 3},
	}, time.Now())
	err := service.CheckProjectCreate(context.Background(), 1)
	var exceeded *QuotaExceededError
	if !errors.As(err, &exceeded) || exceeded.QuotaType != "project" || exceeded.Used != 3 || exceeded.Limit != 3 {
		t.Fatalf("err = %+v", err)
	}
}

func TestCheckChatExceeded(t *testing.T) {
	service := newTestService(&fakeRepo{
		quota: Quota{LLMTokenDailyQuota: 100},
		usage: Usage{InputTokens: 60, OutputTokens: 40},
	}, time.Now())
	err := service.CheckChat(context.Background(), 1)
	var exceeded *QuotaExceededError
	if !errors.As(err, &exceeded) || exceeded.QuotaType != "token" || exceeded.Used != 100 {
		t.Fatalf("err = %+v", err)
	}
}

func TestCheckUploadCountExceeded(t *testing.T) {
	service := newTestService(&fakeRepo{
		quota: Quota{UploadDailyCountQuota: 5},
		usage: Usage{UploadCount: 5},
	}, time.Now())
	err := service.CheckUpload(context.Background(), 1, 1, 0)
	var exceeded *QuotaExceededError
	if !errors.As(err, &exceeded) || exceeded.QuotaType != "upload_count" {
		t.Fatalf("err = %+v", err)
	}
}

func TestCheckUploadBytesExceeded(t *testing.T) {
	service := newTestService(&fakeRepo{
		quota: Quota{UploadDailyBytesQuota: 100},
		usage: Usage{UploadBytes: 90},
	}, time.Now())
	err := service.CheckUpload(context.Background(), 1, 1, 20)
	var exceeded *QuotaExceededError
	if !errors.As(err, &exceeded) || exceeded.QuotaType != "upload_bytes" {
		t.Fatalf("err = %+v", err)
	}
}

func TestRecordMethodsIncrement(t *testing.T) {
	repo := &fakeRepo{}
	service := newTestService(repo, time.Now())
	ctx := context.Background()
	if err := service.RecordProjectCreate(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordChatUsage(ctx, 1, 10, 5, 2); err != nil {
		t.Fatal(err)
	}
	if err := service.RecordUpload(ctx, 1, 1, 100); err != nil {
		t.Fatal(err)
	}
	if len(repo.deltas) != 3 {
		t.Fatalf("deltas = %+v", repo.deltas)
	}
	if repo.deltas[0] != (Usage{ProjectsCreated: 1}) {
		t.Fatalf("project delta = %+v", repo.deltas[0])
	}
	if repo.deltas[1] != (Usage{InputTokens: 10, OutputTokens: 5, ModelRequests: 2}) {
		t.Fatalf("chat delta = %+v", repo.deltas[1])
	}
	if repo.deltas[2] != (Usage{UploadCount: 1, UploadBytes: 100}) {
		t.Fatalf("upload delta = %+v", repo.deltas[2])
	}
}

func TestTodayUsesConfiguredLocation(t *testing.T) {
	service := newTestService(&fakeRepo{}, time.Date(2026, 11, 5, 17, 0, 0, 0, time.UTC))
	got := service.Today().Format("2006-01-02")
	if got != "2026-11-06" {
		t.Fatalf("today = %s", got)
	}
}
