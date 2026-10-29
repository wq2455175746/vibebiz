package admin

import (
	"context"
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
	mock.ExpectQuery("SELECT .* FROM user AS u LEFT JOIN user_quota AS q").
		WillReturnRows(sqlmock.NewRows([]string{"id", "username", "nickname", "email", "plan_code", "status", "created_at"}).
			AddRow(1, "wanghg11", "", "wang@example.com", "free", 1, created).
			AddRow(2, "vistor", "Vistor", "vistor@example.com", "team", 0, created))

	users, err := repo.Users(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || users[0].Name != "wanghg11" || users[0].Plan != "free" || users[0].Status != "active" {
		t.Fatalf("users = %+v", users)
	}
	if users[1].Name != "Vistor" || users[1].Status != "disabled" {
		t.Fatalf("second user = %+v", users[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryQuotas(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	mock.ExpectQuery("SELECT user_id, plan_code, llm_tokens_used_month, sandbox_minutes_used_month, storage_used_bytes FROM `user_quota`").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "plan_code", "llm_tokens_used_month", "sandbox_minutes_used_month", "storage_used_bytes"}).
			AddRow(1, "free", 12000, 45, 12*1024*1024))

	quotas, err := repo.Quotas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(quotas) != 1 || quotas[0].UserID != "1" || quotas[0].LLMTokens != 12000 || quotas[0].StorageMB != 12 {
		t.Fatalf("quotas = %+v", quotas)
	}
}

func TestMySQLRepositoryAuditAndRecord(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	created := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT \\* FROM `ops_audit_log`").
		WithArgs(200).
		WillReturnRows(sqlmock.NewRows([]string{"id", "actor_name", "action", "resource_id", "request_path", "created_at"}).
			AddRow(9, "admin", "notification.publish", "12", "/cp/api/v1/admin/notifications/12/publish", created))

	events, err := repo.Audit(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].ID != "9" || events[0].Target != "12" || events[0].Action != "notification.publish" {
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
	mock.ExpectQuery("SELECT \\* FROM `sandbox_instance` WHERE `sandbox_instance`.`deleted_at` IS NULL ORDER BY name ASC").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "name", "scheme", "host", "port", "status", "health_status",
			"active_requests", "max_concurrency", "last_heartbeat_at", "last_health_at", "health_error",
		}).
			AddRow(1, "sb-1", "sandbox-a", "http", "10.0.0.11", 8000, "online", "healthy", 2, 8, now, now, "").
			AddRow(2, "sb-2", "sandbox-b", "http", "10.0.0.12", 8000, "offline", "unhealthy", 0, 8, now, now, "timeout"))

	sandboxes, err := repo.Sandboxes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(sandboxes) != 2 || sandboxes[0].Name != "sandbox-a" || sandboxes[0].Status != "online" || sandboxes[0].ActiveRequests != 2 {
		t.Fatalf("sandboxes = %+v", sandboxes)
	}
	if sandboxes[1].HealthStatus != "unhealthy" || sandboxes[1].HealthError != "timeout" {
		t.Fatalf("second sandbox = %+v", sandboxes[1])
	}
}
