package storagesync

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newStorageMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestMySQLRepositorySaveUpdatesUserQuota(t *testing.T) {
	db, mock := newStorageMockDB(t)
	repo := NewMySQLRepository(db)
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project_storage`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("SELECT COALESCE\\(SUM\\(file_size_bytes\\), 0\\) FROM `project_storage`").
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"used"}).AddRow(123))
	mock.ExpectExec("UPDATE `user_quota` SET").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := repo.Save(context.Background(), Record{
		ProjectID: 1, WorkspaceID: 5, UserID: 7,
		FileSizeBytes: 123, FileCount: 4, DirCount: 2, LastScannedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryListByWorkspace(t *testing.T) {
	db, mock := newStorageMockDB(t)
	repo := NewMySQLRepository(db)
	modified := time.UnixMilli(1793191707598).UTC()
	scanned := time.Now().UTC()

	mock.ExpectQuery("SELECT \\* FROM `project_storage` WHERE workspace_id = \\?").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "workspace_id", "project_id", "user_id", "file_size_bytes", "file_count", "dir_count", "last_modified_at", "last_scanned_at", "status"}).
			AddRow(1, 5, 10, 7, 123, 4, 2, modified, scanned, 1))

	records, err := repo.ListByWorkspace(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].ProjectID != 10 || records[0].FileSizeBytes != 123 {
		t.Fatalf("records = %+v", records)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
