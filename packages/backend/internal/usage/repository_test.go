package usage

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newUsageMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func TestMySQLRepositoryQuota(t *testing.T) {
	db, mock := newUsageMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT project_daily_quota,.* FROM `user_quota` WHERE user_id = \\? AND `user_quota`.`deleted_at` IS NULL LIMIT \\?").
		WithArgs(int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"project_daily_quota", "llm_token_daily_quota", "upload_daily_count_quota", "upload_daily_bytes_quota",
		}).AddRow(3, 1000, 5, 2000))

	quota, err := repo.Quota(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if quota.ProjectDailyQuota != 3 || quota.LLMTokenDailyQuota != 1000 || quota.UploadDailyCountQuota != 5 || quota.UploadDailyBytesQuota != 2000 {
		t.Fatalf("quota = %+v", quota)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUsageMissingReturnsZero(t *testing.T) {
	db, mock := newUsageMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT projects_created,.* FROM `user_usage_daily` WHERE user_id = \\? AND usage_date = \\? LIMIT \\?").
		WithArgs(int64(1), "2026-11-06", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"projects_created", "input_tokens", "output_tokens", "model_requests", "upload_count", "upload_bytes",
		}))

	usage, err := repo.Usage(context.Background(), 1, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if usage != (Usage{}) {
		t.Fatalf("usage = %+v", usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUsageIncrementRowUsesUsageTimezoneDate(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	row := newUsageIncrementRow(1, time.Date(2026, 11, 6, 0, 0, 0, 0, loc), Usage{ProjectsCreated: 1})
	if row.UsageDate != "2026-11-06" {
		t.Fatalf("usage date = %q, want 2026-11-06", row.UsageDate)
	}
}

func TestMySQLRepositoryIncrement(t *testing.T) {
	db, mock := newUsageMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_usage_daily` .* ON DUPLICATE KEY UPDATE").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.Increment(context.Background(), 1, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC), Usage{
		ProjectsCreated: 1, InputTokens: 10, OutputTokens: 5, ModelRequests: 2, UploadCount: 1, UploadBytes: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositorySeries(t *testing.T) {
	db, mock := newUsageMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT user_id, DATE_FORMAT.* FROM `user_usage_daily` WHERE \\(usage_date >= \\? AND usage_date <= \\?\\) AND user_id = \\? GROUP BY user_id, usage_date ORDER BY usage_date ASC, user_id ASC").
		WithArgs("2026-11-01", "2026-11-06", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"user_id", "date", "projects_created", "input_tokens", "output_tokens", "model_requests", "upload_count", "upload_bytes",
		}).AddRow(7, "2026-11-06", 3, 10, 5, 2, 1, 100))

	rows, err := repo.Series(context.Background(), Filter{
		From: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC), UserID: 7,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UserID != 7 || rows[0].Date != "2026-11-06" || rows[0].ProjectsCreated != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
