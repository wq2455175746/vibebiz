package dao

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"vibebiz/internal/dao/model"
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

func TestRepositoryCountAndDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRepository[model.User](db)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user` WHERE `user`\\.`deleted_at` IS NULL").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	count, err := repo.Count(context.Background())
	if err != nil || count != 2 {
		t.Fatalf("count = %d, err = %v", count, err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `deleted_at`=\\? WHERE `user`\\.`id` = \\? AND `user`\\.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), 7); err != nil {
		t.Fatalf("delete: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET `deleted_at`=\\? WHERE `user`\\.`id` = \\? AND `user`\\.`deleted_at` IS NULL").
		WithArgs(sqlmock.AnyArg(), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), 8); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing err = %v, want ErrNotFound", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNewRepositories(t *testing.T) {
	db, _ := newMockDB(t)
	repos := NewRepositories(db)
	if repos.Users == nil || repos.Projects == nil || repos.ChatMessages == nil || repos.ProjectRAGs == nil || repos.OpsNotifications == nil {
		t.Fatal("repositories not initialized")
	}
}

func TestRepositoryGetNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewRepository[model.User](db)

	mock.ExpectQuery("SELECT \\* FROM `user` WHERE `user`\\.`id` = \\? AND `user`\\.`deleted_at` IS NULL ORDER BY `user`\\.`id` LIMIT \\?").
		WithArgs(int64(99), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username"}))

	if _, err := repo.Get(context.Background(), 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get missing err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
