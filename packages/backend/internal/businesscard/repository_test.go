package businesscard

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

func TestGetOrCreateCreatesMissingCard(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)

	mock.ExpectQuery("SELECT bc\\.\\*, u\\.username AS owner_username").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `business_card`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT bc\\.\\*, u\\.username AS owner_username").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "user_id", "headline", "tags", "featured_type",
			"featured_project_public_id", "featured_s3_key", "featured_file_name",
			"featured_file_size", "published", "published_at", "comment_count", "is_hidden",
			"created_at", "updated_at", "owner_username", "owner_nickname", "owner_avatar_s3_path",
		}).AddRow(1, "card-1", 7, "", []byte("[]"), "", "", "", "", 0, 0, nil, 0, 0, time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), "alice", "alice", ""))
	mock.ExpectQuery("SELECT \\* FROM `business_card_item`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "project_public_id", "sort_order"}))

	card, err := repo.GetOrCreate(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if card.PublicID != "card-1" || card.OwnerName != "alice" {
		t.Fatalf("card = %+v", card)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveItemHardDeletes(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `business_card_item` WHERE card_id = \\? AND project_public_id = \\?").
		WithArgs(int64(1), "p1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.RemoveItem(context.Background(), 1, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteReturnsFeaturedKey(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT `id`,`featured_s3_key` FROM `business_card` WHERE `business_card`\\.`id` = \\?").
		WithArgs(int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "featured_s3_key"}).AddRow(1, "cards/7/a.pdf"))
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `business_card_item` WHERE card_id = \\?").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `business_card` WHERE `business_card`\\.`id` = \\?").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	key, err := repo.Delete(context.Background(), 1)
	if err != nil || key != "cards/7/a.pdf" {
		t.Fatalf("key = %q err = %v", key, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindByProjectNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT bc\\.\\*, u\\.username AS owner_username").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := repo.FindByProject(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
