package scheduledtask

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestMySQLDueAndClaim(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT \\* FROM `scheduled_task` WHERE .*enabled = \\?.*next_run_at <= \\?.*").
		WithArgs(1, now, 100).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "workspace_id", "project_id", "name", "task_type", "cron_expr",
			"timezone", "prompt", "enabled", "next_run_at", "last_run_at", "last_status",
			"last_error", "run_count", "max_runs", "created_at", "updated_at", "deleted_at", "extra_p",
		}).AddRow(3, 7, 5, 11, "任务", "agent", "0 9 * * *", "Asia/Shanghai", "检查", 1,
			now.Add(-time.Minute), nil, "pending", "", 2, 0, now, now, nil, []byte(`{}`)))
	items, err := repo.Due(context.Background(), now, 100)
	if err != nil || len(items) != 1 || items[0].ID != 3 {
		t.Fatalf("items = %+v err = %v", items, err)
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `scheduled_task` SET .*next_run_at.*run_count.*").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ok, err := repo.Claim(context.Background(), 3, now.Add(-time.Minute), now.Add(time.Hour), now)
	if err != nil || !ok {
		t.Fatalf("ok = %v err = %v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLUserNames(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT id, username FROM `user` WHERE id IN \\(\\?,\\?\\)").
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).
			AddRow(1, "alice").AddRow(2, "bob"))
	names, err := repo.UserNames(context.Background(), []int64{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	if names[1] != "alice" || names[2] != "bob" {
		t.Fatalf("names = %+v", names)
	}
}
