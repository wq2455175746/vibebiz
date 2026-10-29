package store

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"vibebiz/internal/model"
)

func newMockProjectDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func newMySQLProjectStore(t *testing.T, db *gorm.DB, mock sqlmock.Sqlmock) *MySQLProjectStore {
	t.Helper()
	mock.ExpectQuery("SELECT id FROM `project_topic`").
		WithArgs("common", 1, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(5))
	s, err := NewMySQLProjectStore(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMySQLProjectStoreGetMapsPublicFields(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT \\* FROM `project` WHERE public_id = \\? AND `project`\\.`deleted_at` IS NULL").
		WithArgs("d5f5e1d0-0000-4000-8000-000000000001", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "workspace_id", "user_id", "name", "prompt", "status",
			"created_at", "updated_at",
		}).AddRow(42, "d5f5e1d0-0000-4000-8000-000000000001", 3, 7, "demo", "make a game", "ready", now, now))

	got, err := s.Get(context.Background(), "d5f5e1d0-0000-4000-8000-000000000001")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "d5f5e1d0-0000-4000-8000-000000000001" || got.Prompt != "make a game" || got.State != "ready" {
		t.Fatalf("project = %+v", got)
	}
	if got.WorkspaceID != 3 || got.OwnerUserID != 7 || got.Name != "demo" {
		t.Fatalf("mapped project = %+v", got)
	}
}

func TestMySQLProjectStoreCreate(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	project := &model.Project{
		ID: "d5f5e1d0-0000-4000-8000-000000000001", Name: "demo", Prompt: "make a game",
		State: model.ProjectStateReady, WorkspaceID: 3, OwnerUserID: 7, CreatedAt: now, UpdatedAt: now,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project`").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectCommit()

	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProjectStoreListByWorkspaces(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	query := regexp.QuoteMeta("SELECT * FROM `project` WHERE workspace_id IN (?,?) AND `project`.`deleted_at` IS NULL ORDER BY created_at DESC")
	mock.ExpectQuery(query).
		WithArgs(int64(1), int64(2)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "workspace_id", "user_id", "name", "prompt", "status",
			"created_at", "updated_at",
		}).
			AddRow(42, "p-new", 1, 7, "new", "prompt", "ready", now, now).
			AddRow(41, "p-old", 2, 8, "old", "prompt", "ready", now.Add(-time.Hour), now.Add(-time.Hour)))

	list := s.ListByWorkspaces(context.Background(), []int64{1, 2})
	if len(list) != 2 || list[0].ID != "p-new" || list[1].ID != "p-old" {
		t.Fatalf("list = %+v", list)
	}
}

func TestMySQLProjectStoreUpdateNotFound(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project` SET").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := s.Update(context.Background(), &model.Project{ID: "missing", Name: "new"})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMySQLProjectStoreDeleteNotFound(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project` SET `deleted_at`=").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := s.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
