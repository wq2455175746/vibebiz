package dao

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"vibebiz/internal/dao/model"
)

func TestNotificationRepositoryListFilters(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(db)

	now := time.Unix(1700000000, 0).UTC()
	query := "SELECT \\* FROM `ops_notification` WHERE type = \\? AND status = \\? AND `ops_notification`.`deleted_at` IS NULL ORDER BY created_at DESC, id DESC"
	mock.ExpectQuery(query).
		WithArgs(model.NotificationVersionRelease, model.NotificationPublished).
		WillReturnRows(sqlmock.NewRows([]string{"id", "title", "type", "status", "expire_at", "created_at"}).
			AddRow(1, "v1.2.0", "version_release", "published", now, now))

	items, err := repo.List(context.Background(), NotificationFilter{
		Type:   string(model.NotificationVersionRelease),
		Status: string(model.NotificationPublished),
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != 1 || items[0].Title != "v1.2.0" {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationRepositoryListWithoutFilters(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `ops_notification` WHERE `ops_notification`.`deleted_at` IS NULL ORDER BY created_at DESC, id DESC").
		WillReturnRows(sqlmock.NewRows([]string{"id", "title"}).
			AddRow(2, "inbox"))

	items, err := repo.List(context.Background(), NotificationFilter{})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(items) != 1 || items[0].ID != 2 {
		t.Fatalf("items = %+v", items)
	}
}

func TestNotificationRepositoryGetNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `ops_notification` WHERE `ops_notification`.`id` = \\? AND `ops_notification`.`deleted_at` IS NULL ORDER BY `ops_notification`.`id` LIMIT \\?").
		WithArgs(int64(99), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.Get(context.Background(), 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationRepositoryUpdateStatus(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(db)

	publishedAt := time.Unix(1700000100, 0).UTC()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_notification` SET `published_at`=\\?,`status`=\\?,`updated_at`=\\? WHERE id = \\? AND `ops_notification`.`deleted_at` IS NULL").
		WithArgs(publishedAt, string(model.NotificationPublished), sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.UpdateStatus(context.Background(), 3, string(model.NotificationPublished), &publishedAt); err != nil {
		t.Fatalf("update status: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_notification` SET `status`=\\?,`updated_at`=\\? WHERE id = \\? AND `ops_notification`.`deleted_at` IS NULL").
		WithArgs(string(model.NotificationDraft), sqlmock.AnyArg(), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `ops_notification` WHERE `ops_notification`.`id` = \\? AND `ops_notification`.`deleted_at` IS NULL ORDER BY `ops_notification`.`id` LIMIT \\?").
		WithArgs(int64(99), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if err := repo.UpdateStatus(context.Background(), 99, string(model.NotificationDraft), nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_notification` SET `status`=\\?,`updated_at`=\\? WHERE id = \\? AND `ops_notification`.`deleted_at` IS NULL").
		WithArgs(string(model.NotificationDraft), sqlmock.AnyArg(), int64(3)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `ops_notification` WHERE `ops_notification`.`id` = \\? AND `ops_notification`.`deleted_at` IS NULL ORDER BY `ops_notification`.`id` LIMIT \\?").
		WithArgs(int64(3), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(3, "draft"))

	if err := repo.UpdateStatus(context.Background(), 3, string(model.NotificationDraft), nil); err != nil {
		t.Fatalf("no-op update status: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationRepositoryPublished(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(db)

	now := time.Unix(1700000000, 0).UTC()
	query := "SELECT \\* FROM `ops_notification` WHERE \\(status = \\? AND expire_at > \\?\\) AND `ops_notification`.`deleted_at` IS NULL ORDER BY published_at DESC, id DESC"
	mock.ExpectQuery(query).
		WithArgs(model.NotificationPublished, now).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status", "expire_at"}).
			AddRow(4, "published", now.Add(time.Hour)))

	items, err := repo.Published(context.Background(), now)
	if err != nil {
		t.Fatalf("published: %v", err)
	}
	if len(items) != 1 || items[0].ID != 4 {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNotificationRepositoryCreateUpdateDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewNotificationRepository(db)

	item := &model.OpsNotification{
		Title:   "v1.2.0",
		Content: "# release",
		Type:    model.NotificationVersionRelease,
		Status:  model.NotificationDraft,
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `ops_notification`").
		WillReturnResult(sqlmock.NewResult(5, 1))
	mock.ExpectCommit()
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatalf("create: %v", err)
	}

	item.ID = 5
	item.Title = "v1.2.1"
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_notification` SET .*WHERE id = \\? AND `ops_notification`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.Update(context.Background(), item); err != nil {
		t.Fatalf("update: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_notification` SET .*WHERE id = \\? AND `ops_notification`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `ops_notification` WHERE `ops_notification`.`id` = \\? AND `ops_notification`.`deleted_at` IS NULL ORDER BY `ops_notification`.`id` LIMIT \\?").
		WithArgs(int64(6), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(6))
	if err := repo.Update(context.Background(), &model.OpsNotification{Base: model.Base{ID: 6}}); err != nil {
		t.Fatalf("no-op update: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_notification` SET .*WHERE id = \\? AND `ops_notification`.`deleted_at` IS NULL").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `ops_notification` WHERE `ops_notification`.`id` = \\? AND `ops_notification`.`deleted_at` IS NULL ORDER BY `ops_notification`.`id` LIMIT \\?").
		WithArgs(int64(7), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if err := repo.Update(context.Background(), &model.OpsNotification{Base: model.Base{ID: 7}}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing err = %v, want ErrNotFound", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `ops_notification` SET `deleted_at`=? WHERE `ops_notification`.`id` = ? AND `ops_notification`.`deleted_at` IS NULL")).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), 5); err != nil {
		t.Fatalf("delete: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("UPDATE `ops_notification` SET `deleted_at`=? WHERE `ops_notification`.`id` = ? AND `ops_notification`.`deleted_at` IS NULL")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), 6); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
