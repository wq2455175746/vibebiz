package dao

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"vibebiz/internal/dao/model"
)

func TestChannelGuideRepositoryCreate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `channel_guide`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.Create(context.Background(), &model.ChannelGuide{Channel: "taptap_mobile", Title: "TapTap"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideRepositoryGetNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectQuery("SELECT \\* FROM `channel_guide` WHERE channel = \\?").
		WithArgs("missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "channel"}))
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestChannelGuideRepositoryListEnabled(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	rows := sqlmock.NewRows([]string{"id", "channel", "title", "enabled", "sort_order"}).
		AddRow(1, "wechat_minigame", "微信小游戏", 1, 10)
	mock.ExpectQuery("SELECT \\* FROM `channel_guide` WHERE enabled = \\? ORDER BY sort_order ASC, channel ASC").
		WithArgs(true).
		WillReturnRows(rows)
	items, err := repo.List(context.Background(), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Channel != "wechat_minigame" {
		t.Fatalf("items = %+v", items)
	}
}

func TestChannelGuideRepositoryUpdate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `channel_guide` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	item := &model.ChannelGuide{ID: 1, Channel: "wechat_minigame", Title: "微信小游戏"}
	if err := repo.Update(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideRepositoryDelete(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `channel_guide` WHERE channel = \\?").
		WithArgs("taptap_mobile").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), "taptap_mobile"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideRepositoryDeleteNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `channel_guide` WHERE channel = \\?").
		WithArgs("missing").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	if err := repo.Delete(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideRepositoryListKeyword(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	rows := sqlmock.NewRows([]string{"id", "channel", "title", "enabled", "sort_order"}).
		AddRow(1, "taptap_mobile", "TapTap", 1, 40)
	mock.ExpectQuery("SELECT \\* FROM `channel_guide` WHERE \\(channel LIKE \\? OR title LIKE \\? OR summary LIKE \\?\\) ORDER BY sort_order ASC, channel ASC").
		WithArgs("%tap%", "%tap%", "%tap%").
		WillReturnRows(rows)
	items, err := repo.List(context.Background(), true, "tap")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Channel != "taptap_mobile" {
		t.Fatalf("items = %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideRepositorySeedDefaults(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `channel_guide`").
		WillReturnResult(sqlmock.NewResult(1, 3))
	mock.ExpectCommit()
	items := []model.ChannelGuide{{Channel: "wechat_minigame"}, {Channel: "douyin_minigame"}, {Channel: "steam_desktop"}}
	if err := repo.SeedDefaults(context.Background(), items); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
