package dao

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"vibebiz/internal/dao/model"
)

func TestChannelPackageRepositoryCreate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project_channel_package`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	item := &model.ProjectChannelPackage{PublicID: "pkg-1", ProjectPublicID: "p1", Channel: "wechat_minigame"}
	if err := repo.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPackageRepositoryUpdate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project_channel_package` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	item := &model.ProjectChannelPackage{Base: model.Base{ID: 1}, PublicID: "pkg-1", Status: model.ChannelPackageStatusReady}
	if err := repo.Update(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPackageRepositoryGetByPublicID(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	rows := sqlmock.NewRows([]string{"id", "public_id", "project_public_id", "channel", "status"}).
		AddRow(1, "pkg-1", "p1", "wechat_minigame", "ready")
	mock.ExpectQuery("SELECT \\* FROM `project_channel_package` WHERE public_id = \\?").
		WithArgs("pkg-1", 1).
		WillReturnRows(rows)
	item, err := repo.GetByPublicID(context.Background(), "pkg-1")
	if err != nil {
		t.Fatal(err)
	}
	if item.PublicID != "pkg-1" || item.Channel != "wechat_minigame" {
		t.Fatalf("item = %+v", item)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPackageRepositoryGetByPublicIDNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	mock.ExpectQuery("SELECT \\* FROM `project_channel_package` WHERE public_id = \\?").
		WithArgs("missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id"}))
	if _, err := repo.GetByPublicID(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPackageRepositoryListByProject(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	rows := sqlmock.NewRows([]string{"id", "public_id", "project_public_id", "channel"}).
		AddRow(2, "pkg-2", "p1", "douyin_minigame").
		AddRow(1, "pkg-1", "p1", "wechat_minigame")
	mock.ExpectQuery("SELECT \\* FROM `project_channel_package` WHERE project_public_id = \\?").
		WithArgs("p1").
		WillReturnRows(rows)
	items, err := repo.ListByProject(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].PublicID != "pkg-2" {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPackageRepositoryDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project_channel_package` SET `deleted_at`=").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), "pkg-1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelPackageRepositoryDeleteNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelPackageRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project_channel_package` SET `deleted_at`=").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
