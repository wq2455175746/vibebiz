package clone

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockCloneDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestMySQLStoreCreateSourceAndAsset(t *testing.T) {
	db, mock := newMockCloneDB(t)
	store := NewMySQLStore(db)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project_clone_source`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := store.CreateSource(context.Background(), Source{ProjectID: "p1", SourceURL: "https://x", Status: StatusReady}); err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project_clone_asset`").WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	if err := store.CreateAsset(context.Background(), Asset{ProjectID: "p1", ResourceID: "r1", Role: RoleCover, SortOrder: 0}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLStoreListAssets(t *testing.T) {
	db, mock := newMockCloneDB(t)
	store := NewMySQLStore(db)
	mock.ExpectQuery("SELECT \\* FROM `project_clone_asset` WHERE project_public_id = \\? AND `project_clone_asset`.`deleted_at` IS NULL ORDER BY sort_order ASC, id ASC").
		WithArgs("p1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_public_id", "resource_id", "role", "sort_order"}).
			AddRow(1, "p1", "r1", RoleCover, 0))
	assets, err := store.ListAssets(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 || assets[0].ResourceID != "r1" || assets[0].Role != RoleCover {
		t.Fatalf("assets = %+v", assets)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
