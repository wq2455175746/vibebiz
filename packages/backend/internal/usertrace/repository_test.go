package usertrace

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTraceMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestMySQLRepositoryWrite(t *testing.T) {
	db, mock := newTraceMockDB(t)
	repo := NewMySQLRepository(db)
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_access_log`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.Write(context.Background(), []Event{{
		EventID: "e1", EventType: EventRequest, UserID: 7, Username: "wang",
		RequestPath: "/cp/api/v1/projects", StatusCode: 200, EventAt: at,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryList(t *testing.T) {
	db, mock := newTraceMockDB(t)
	repo := NewMySQLRepository(db)
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_access_log` WHERE event_type = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `user_access_log` WHERE event_type = \\?").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "user_id", "username", "ip",
			"device_id", "device_type", "os", "browser", "request_method",
			"request_path", "request_query", "route", "status_code",
			"duration_ms", "event_at",
		}).AddRow(1, "e1", "request", 7, "wang", "1.2.3.4", "d1", "desktop", "macOS", "Chrome", "GET", "/cp/api/v1/projects", "", "/cp/api/v1/projects", 200, 12, at))

	page, err := repo.List(context.Background(), Query{EventType: "request", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].ID != "1" || page.Items[0].EventType != "request" || page.Items[0].Path != "/cp/api/v1/projects" || page.Items[0].DurationMs != 12 {
		t.Fatalf("log = %+v", page.Items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
