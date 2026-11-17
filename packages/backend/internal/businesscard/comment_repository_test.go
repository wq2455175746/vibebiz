package businesscard

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestListRootsUsesCursorAndReturnsOneExtra(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	rows := sqlmock.NewRows([]string{
		"id", "public_id", "card_id", "user_id", "parent_id", "root_id", "depth",
		"content", "status", "edited_at", "reply_count", "created_at",
		"author_name", "author_avatar_s3_path", "reply_to_name",
	}).
		AddRow(12, "c-12", 1, 7, 0, 0, 1, "hello", 1, nil, 3, time.Now(), "alice", "", "").
		AddRow(11, "c-11", 1, 8, 0, 0, 1, "world", 1, nil, 0, time.Now(), "bob", "", "")
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WithArgs(int64(1), int64(10), int64(2)).
		WillReturnRows(rows)

	items, hasMore, err := repo.ListRoots(context.Background(), 1, 10, 1)
	if err != nil || !hasMore || len(items) != 1 || items[0].PublicID != "c-12" {
		t.Fatalf("items=%+v hasMore=%v err=%v", items, hasMore, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListRootsDefaultsAndOmitsCursor(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	rows := sqlmock.NewRows([]string{"id", "public_id"}).
		AddRow(12, "c-12").
		AddRow(11, "c-11")
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WithArgs(int64(1), int64(21)).
		WillReturnRows(rows)

	items, hasMore, err := repo.ListRoots(context.Background(), 1, 0, 0)
	if err != nil || hasMore || len(items) != 2 {
		t.Fatalf("items=%+v hasMore=%v err=%v", items, hasMore, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListRepliesUsesAscendingCursorAndReturnsOneExtra(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	rows := sqlmock.NewRows([]string{"id", "public_id"}).
		AddRow(20, "c-20").
		AddRow(21, "c-21").
		AddRow(22, "c-22")
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WithArgs(int64(12), int64(19), int64(3)).
		WillReturnRows(rows)

	items, hasMore, err := repo.ListReplies(context.Background(), 12, 19, 2)
	if err != nil || !hasMore || len(items) != 2 || items[0].PublicID != "c-20" {
		t.Fatalf("items=%+v hasMore=%v err=%v", items, hasMore, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetCommentByPublicIDNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.GetCommentByPublicID(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestCreateCommentIncrementsCounts(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(3, 1))
	mock.ExpectExec("INSERT INTO `business_card_comment`").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec("UPDATE `business_card` SET .*comment_count").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `business_card_comment` SET .*reply_count").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "card_id", "user_id", "status"}).
			AddRow(9, "c-9", 1, 7, 1))

	rec, err := repo.CreateComment(context.Background(), CommentInput{
		PublicID: "c-9", CardID: 1, UserID: 7, ParentID: 3, RootID: 3, Depth: 2, Content: "hi",
	})
	if err != nil || rec.PublicID != "c-9" {
		t.Fatalf("rec=%+v err=%v", rec, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateCommentRejectsTombstonedParent(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(3, 2))
	mock.ExpectRollback()

	_, err := repo.CreateComment(context.Background(), CommentInput{
		PublicID: "c-9", CardID: 1, UserID: 7, ParentID: 3, RootID: 3, Depth: 2, Content: "hi",
	})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreateCommentRootSkipsReplyCount(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `business_card_comment`").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectExec("UPDATE `business_card` SET .*comment_count").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id"}).AddRow(9, "c-9"))

	if _, err := repo.CreateComment(context.Background(), CommentInput{
		PublicID: "c-9", CardID: 1, UserID: 7, Depth: 1, Content: "root",
	}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCommentMarksEdited(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `business_card_comment` SET .*edited_at").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "content"}).AddRow(9, "c-9", "new"))

	rec, err := repo.UpdateComment(context.Background(), 1, 9, 7, "new", time.Now())
	if err != nil || rec.Content != "new" {
		t.Fatalf("rec=%+v err=%v", rec, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateCommentNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `business_card_comment` SET .*edited_at").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if _, err := repo.UpdateComment(context.Background(), 1, 9, 7, "new", time.Now()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestDeleteLeafCommentHardDeletesAndDecrements(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "user_id", "parent_id", "root_id", "status"}).
			AddRow(9, 1, 7, 3, 3, 1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `business_card_comment`").
		WithArgs(int64(9), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE `business_card` SET .*comment_count`=GREATEST\\(comment_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `business_card_comment`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `business_card_comment` SET .*reply_count`=GREATEST\\(reply_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "root_id", "status"}).
			AddRow(3, 0, 0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT `comment_count` FROM `business_card`").
		WillReturnRows(sqlmock.NewRows([]string{"comment_count"}).AddRow(8))

	result, err := repo.DeleteComment(context.Background(), 1, 9, 7)
	if err != nil || result.Tombstoned || result.CommentCount != 8 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCommentWithRepliesTombstones(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "user_id", "parent_id", "root_id", "status"}).
			AddRow(9, 1, 7, 0, 0, 1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `business_card_comment`").
		WithArgs(int64(9), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectExec("UPDATE `business_card` SET .*comment_count`=GREATEST\\(comment_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `business_card_comment` SET .*status").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT `comment_count` FROM `business_card`").
		WillReturnRows(sqlmock.NewRows([]string{"comment_count"}).AddRow(5))

	result, err := repo.DeleteComment(context.Background(), 1, 9, 7)
	if err != nil || !result.Tombstoned || result.CommentCount != 5 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCommentRemovesChildlessTombstoneAncestor(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "user_id", "parent_id", "root_id", "status"}).
			AddRow(2, 1, 7, 1, 1, 1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `business_card_comment`").
		WithArgs(int64(2), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectExec("UPDATE `business_card` SET .*comment_count`=GREATEST\\(comment_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `business_card_comment` SET .*status").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT `comment_count` FROM `business_card`").
		WillReturnRows(sqlmock.NewRows([]string{"comment_count"}).AddRow(3))

	result, err := repo.DeleteComment(context.Background(), 1, 2, 7)
	if err != nil || !result.Tombstoned || result.CommentCount != 3 {
		t.Fatalf("first delete result=%+v err=%v", result, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "user_id", "parent_id", "root_id", "status"}).
			AddRow(3, 1, 7, 2, 1, 1))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `business_card_comment`").
		WithArgs(int64(3), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE `business_card` SET .*comment_count`=GREATEST\\(comment_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `business_card_comment`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `business_card_comment` SET .*reply_count`=GREATEST\\(reply_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "root_id", "status"}).
			AddRow(2, 1, 1, 2))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `business_card_comment`").
		WithArgs(int64(2), int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("DELETE FROM `business_card_comment`").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `business_card_comment` SET .*reply_count`=GREATEST\\(reply_count, 1\\) - 1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "parent_id", "root_id", "status"}).
			AddRow(1, 0, 0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT `comment_count` FROM `business_card`").
		WillReturnRows(sqlmock.NewRows([]string{"comment_count"}).AddRow(2))

	result, err = repo.DeleteComment(context.Background(), 1, 3, 7)
	if err != nil || result.Tombstoned || result.CommentCount != 2 {
		t.Fatalf("second delete result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteCommentRejectsOtherUser(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `business_card_comment`.*FOR UPDATE").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "user_id", "parent_id", "root_id", "status"}).
			AddRow(9, 1, 8, 0, 0, 1))
	mock.ExpectRollback()

	if _, err := repo.DeleteComment(context.Background(), 1, 9, 7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
