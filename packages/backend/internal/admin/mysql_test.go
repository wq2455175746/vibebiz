package admin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"
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
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "plan_code", "llm_token_quota_month", "llm_tokens_used_month", "sandbox_minutes_quota_month", "sandbox_minutes_used_month", "storage_quota_bytes", "storage_used_bytes", "project_daily_quota", "llm_token_daily_quota", "upload_daily_count_quota", "upload_daily_bytes_quota"}).
			AddRow(1, "free", 500000, 12000, 600, 45, 1024*1024*1024, 12*1024*1024, 3, 1000, 5, 2000))

	quotas, err := repo.Quotas(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(quotas) != 1 || quotas[0].UserID != "1" || quotas[0].LLMTokensQuota != 500000 || quotas[0].LLMTokensUsed != 12000 || quotas[0].StorageQuotaMB != 1024 || quotas[0].StorageUsedMB != 12 || quotas[0].ProjectDailyQuota != 3 || quotas[0].LLMTokenDailyQuota != 1000 {
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
	mock.ExpectExec("UPDATE `user` SET .*disabled_at.*disabled_by.*disabled_reason.*WHERE id = \\? AND `user`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateUserStatus(context.Background(), 1, UserStatusUpdate{
		Status: 0, Reason: "违规", Operator: "admin", At: time.Now(),
	}); err != nil {
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
	mock.ExpectExec("UPDATE `user` SET .*WHERE id = \\? AND `user`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if err := repo.UpdateUserStatus(context.Background(), 99, UserStatusUpdate{Status: 1}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUserDetail(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	created := time.Date(2026, 10, 29, 8, 0, 0, 0, time.UTC)
	disabledAt := created.Add(time.Hour)
	lastLogin := created.Add(-time.Hour)
	mock.ExpectQuery("SELECT u.id, u.username, u.nickname").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "nickname", "email", "phone_number", "avatar_url", "avatar_s3_path",
			"plan_code", "source", "status", "email_verified_at", "last_login_time", "last_login_ip",
			"disabled_reason", "disabled_at", "disabled_by", "created_at", "updated_at",
		}).AddRow(
			1, "wanghg11", "老王", "wang@example.com", "13800000000", "https://old/avatar.png", "avatar/1/a.png",
			"pro", "CUSTOM", 0, created, lastLogin, "10.0.0.1",
			"违规内容", disabledAt, "admin", created, created,
		))

	detail, err := repo.User(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "1" || detail.Name != "老王" || detail.Username != "wanghg11" || detail.Plan != "pro" ||
		detail.Status != "disabled" || detail.AvatarS3Path != "avatar/1/a.png" ||
		detail.DisabledReason != "违规内容" || detail.DisabledBy != "admin" || detail.DisabledAt == nil {
		t.Fatalf("detail = %+v", detail)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdateUserPassword(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `password_hash`=\\?,`updated_at`=\\? WHERE id = \\? AND `user`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateUserPassword(context.Background(), 1, "hashed"); err != nil {
		t.Fatal(err)
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

	llm := int64(500000)
	sandbox := int64(600)
	storage := int64(1024)
	if err := repo.UpdateQuota(context.Background(), 1, QuotaUpdateInput{
		LLMTokensQuota: &llm, SandboxMinutesQuota: &sandbox, StorageQuotaMB: &storage,
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

	missing := int64(1)
	if err := repo.UpdateQuota(context.Background(), 99, QuotaUpdateInput{LLMTokensQuota: &missing}); !errors.Is(err, ErrNotFound) {
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

func TestMySQLRepositoryCreateUser(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))
	created := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO `workspace`").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO `workspace_user`").WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("INSERT INTO `user_quota`").WillReturnResult(sqlmock.NewResult(13, 1))
	mock.ExpectQuery("SELECT u.id, u.username, u.nickname").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "username", "nickname", "email", "phone_number", "avatar_url", "avatar_s3_path",
			"plan_code", "source", "status", "email_verified_at", "last_login_time", "last_login_ip",
			"disabled_reason", "disabled_at", "disabled_by", "created_at", "updated_at",
		}).AddRow(
			7, "tester01", "Tester", "tester01@example.com", "", "", "",
			"pro", "ADMIN", 1, created, nil, "",
			"", nil, "", created, created,
		))
	mock.ExpectCommit()

	detail, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "tester01", Email: "tester01@example.com", Nickname: "Tester",
		PasswordHash: "hash", Plan: "pro", Status: 1, Operator: "admin", At: created,
	})
	if err != nil {
		t.Fatal(err)
	}
	if detail.ID != "7" || detail.Username != "tester01" || detail.Plan != "pro" || detail.Status != "active" {
		t.Fatalf("detail = %+v", detail)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCreateUserConflict(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user`").
		WillReturnError(&mysqldriver.MySQLError{Number: 1062})
	mock.ExpectRollback()

	_, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "tester01", Email: "tester01@example.com", Plan: "free", Status: 1,
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryCreateUserRollsBackOnLaterFailure(t *testing.T) {
	db, mock := newAdminMockDB(t)
	repo := NewMySQLRepository(db, fixedCounter(0))

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user`").WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO `workspace`").WillReturnError(errors.New("workspace insert failed"))
	mock.ExpectRollback()

	_, err := repo.CreateUser(context.Background(), CreateUserInput{
		Username: "tester01", Email: "tester01@example.com", Plan: "free", Status: 1,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCreateUserRecords(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	user, workspace, member, quota := buildCreateUserRecords(CreateUserInput{
		Username: "tester01", Email: "tester01@example.com", Nickname: "Tester",
		PasswordHash: "hash", Plan: "pro", Status: 1, Operator: "admin",
	}, now)

	if user.Email != "tester01@example.com" || user.Source != "ADMIN" || user.Username != "tester01" ||
		user.Nickname != "Tester" || user.Status != 1 {
		t.Fatalf("user = %+v", user)
	}
	if user.PasswordHash == nil || *user.PasswordHash != "hash" {
		t.Fatalf("user.PasswordHash = %v", user.PasswordHash)
	}
	if user.EmailVerifiedAt == nil || !user.EmailVerifiedAt.Equal(now) {
		t.Fatalf("user.EmailVerifiedAt = %v", user.EmailVerifiedAt)
	}

	if workspace.Name != "tester01 的个人工作区" || workspace.Description != "默认个人工作区" ||
		workspace.WorkspaceType != "personal" || workspace.Visibility != "private" ||
		workspace.Status != 1 || workspace.CreatedBy != "admin" {
		t.Fatalf("workspace = %+v", workspace)
	}

	if member.Role != "owner" || member.Status != 1 ||
		member.JoinedAt == nil || !member.JoinedAt.Equal(now) {
		t.Fatalf("member = %+v", member)
	}

	if quota.PlanCode != "pro" || quota.Status != 1 {
		t.Fatalf("quota = %+v", quota)
	}
	if quota.ConcurrentAgentRuns != 1 {
		t.Fatalf("quota.ConcurrentAgentRuns = %d", quota.ConcurrentAgentRuns)
	}
}

func TestBuildCreateUserRecordsNoEmail(t *testing.T) {
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	user, _, _, _ := buildCreateUserRecords(CreateUserInput{Username: "tester02", Status: 1}, now)
	if user.EmailVerifiedAt != nil {
		t.Fatalf("user.EmailVerifiedAt = %v", user.EmailVerifiedAt)
	}
}
