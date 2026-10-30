package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newAdminMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

type fixedCounter int64

func (c fixedCounter) Today(context.Context) (int64, error) {
	return int64(c), nil
}

func TestMySQLRepositoryUsers(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	created := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM user AS u LEFT JOIN user_quota AS q").
		WithArgs("%wang%", "%wang%", "%wang%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT .* FROM user AS u LEFT JOIN user_quota AS q").
		WithArgs("%wang%", "%wang%", "%wang%", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "nickname", "email", "plan_code", "status", "created_at"}).
			AddRow(1, "wanghg11", "", "wang@example.com", "free", 1, created))

	users, err := repo.Users(context.Background(), ListQuery{Keyword: "wang", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if users.Total != 1 || len(users.Items) != 1 || users.Items[0].Name != "wanghg11" || users.Items[0].Plan != "free" || users.Items[0].Status != "active" {
		t.Fatalf("users = %+v", users)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryQuotas(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	mock.ExpectQuery("SELECT user_id, plan_code, llm_token_quota_month").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "plan_code", "llm_token_quota_month", "llm_tokens_used_month", "sandbox_minutes_quota_month", "sandbox_minutes_used_month", "storage_quota_bytes", "storage_used_bytes"}).
			AddRow(1, "free", 500000, 12000, 600, 45, 1024*1024*1024, 12*1024*1024))

	quotas, err := repo.Quotas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(quotas) != 1 || quotas[0].UserID != "1" || quotas[0].LLMTokensQuota != 500000 || quotas[0].LLMTokensUsed != 12000 || quotas[0].StorageQuotaMB != 1024 || quotas[0].StorageUsedMB != 12 {
		t.Fatalf("quotas = %+v", quotas)
	}
}

func TestMySQLRepositoryAuditAndRecord(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	created := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `ops_audit_log`").
		WithArgs("%admin%", "%admin%", "%admin%", "%admin%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `ops_audit_log`").
		WithArgs("%admin%", "%admin%", "%admin%", "%admin%", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_name", "action", "resource_id", "request_path", "created_at"}).
			AddRow(9, "admin", "notification.publish", "12", "/cp/api/v1/admin/notifications/12/publish", created))

	events, err := repo.Audit(context.Background(), ListQuery{Keyword: "admin", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if events.Total != 1 || len(events.Items) != 1 || events.Items[0].ID != "9" || events.Items[0].Target != "12" || events.Items[0].Action != "notification.publish" {
		t.Fatalf("events = %+v", events)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `ops_audit_log`").
		WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectCommit()
	err = repo.RecordAudit(context.Background(), AuditRecord{
		OpsUserID: 3, ActorName: "admin", Action: "notification.publish",
		ResourceType: "notification", ResourceID: "12", RequestMethod: "POST",
		RequestPath: "/cp/api/v1/admin/notifications/12/publish", ResultStatus: "success",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdateUserStatus(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `status`=\\?,`updated_at`=\\? WHERE id = \\? AND `user`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateUserStatus(context.Background(), 1, 0); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdateUserStatusNotFound(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `status`=\\?,`updated_at`=\\? WHERE id = \\? AND `user`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if err := repo.UpdateUserStatus(context.Background(), 99, 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdateQuota(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_quota` SET .*WHERE user_id = \\? AND `user_quota`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateQuota(context.Background(), 1, QuotaUpdateInput{
		LLMTokensQuota: 500000, SandboxMinutesQuota: 600, StorageQuotaMB: 1024,
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdateQuotaNotFound(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user_quota` SET .*WHERE user_id = \\? AND `user_quota`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_quota`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if err := repo.UpdateQuota(context.Background(), 99, QuotaUpdateInput{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryOverview(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(37))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `sandbox_instance`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	overview, err := repo.Overview(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if overview.Projects != 5 || overview.Users != 2 || overview.RunningSandboxes != 1 || overview.TodayRequests != 37 {
		t.Fatalf("overview = %+v", overview)
	}
}

func TestMySQLRepositorySandboxes(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	now := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `sandbox_instance`").
		WithArgs("%sandbox%", "%sandbox%", "%sandbox%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `sandbox_instance` WHERE").
		WithArgs("%sandbox%", "%sandbox%", "%sandbox%", 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "name", "scheme", "host", "port", "status", "health_status",
			"active_requests", "max_concurrency", "last_heartbeat_at", "last_health_at", "health_error",
		}).
			AddRow(1, "sb-1", "sandbox-a", "http", "10.0.0.11", 8000, "online", "healthy", 2, 8, now, now, ""))

	sandboxes, err := repo.Sandboxes(context.Background(), ListQuery{Keyword: "sandbox", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if sandboxes.Total != 1 || len(sandboxes.Items) != 1 || sandboxes.Items[0].Name != "sandbox-a" || sandboxes.Items[0].Status != "online" || sandboxes.Items[0].ActiveRequests != 2 {
		t.Fatalf("sandboxes = %+v", sandboxes)
	}
}
