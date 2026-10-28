package dao

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestFindUserByIdentifier(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `user` WHERE \\(\\(username = \\? OR email = \\?\\)\\) AND `user`\\.`deleted_at` IS NULL ORDER BY `user`\\.`id` LIMIT \\?").
		WithArgs("wanghg11", "wanghg11", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "password_hash", "status"}).
			AddRow(7, "wang@example.com", "wanghg11", "hash", 1))

	user, err := repo.FindUserByIdentifier(context.Background(), "wanghg11")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if user.ID != 7 || user.Username != "wanghg11" || user.Email != "wang@example.com" {
		t.Fatalf("user = %+v", user)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindUserByIdentifierNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `user` WHERE").
		WithArgs("missing", "missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.FindUserByIdentifier(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestFindOpsByIdentifier(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `ops_user` WHERE \\(\\(username = \\? OR email = \\?\\)\\) AND `ops_user`\\.`deleted_at` IS NULL ORDER BY `ops_user`\\.`id` LIMIT \\?").
		WithArgs("admin", "admin", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "email", "username", "role_code", "status"}).
			AddRow(9, "admin@example.com", "admin", "super_admin", 1))

	user, err := repo.FindOpsByIdentifier(context.Background(), "admin")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if user.ID != 9 || user.RoleCode != "super_admin" {
		t.Fatalf("ops user = %+v", user)
	}
}

func TestFindOpsByIdentifierNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)

	mock.ExpectQuery("SELECT \\* FROM `ops_user` WHERE").
		WithArgs("missing", "missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	if _, err := repo.FindOpsByIdentifier(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestWorkspaceIDs(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)

	query := regexp.QuoteMeta("SELECT workspace_user.workspace_id FROM `workspace_user` JOIN workspace ON workspace.id = workspace_user.workspace_id WHERE (workspace_user.user_id = ? AND workspace_user.status = 1 AND workspace_user.deleted_at IS NULL) AND (workspace.status = 1 AND workspace.deleted_at IS NULL) ORDER BY workspace_user.workspace_id ASC")
	mock.ExpectQuery(query).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"workspace_id"}).AddRow(1).AddRow(2))

	ids, err := repo.WorkspaceIDs(context.Background(), 7)
	if err != nil {
		t.Fatalf("workspace ids: %v", err)
	}
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("ids = %v", ids)
	}
}

func TestTouchUserLogin(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)
	at := time.Unix(1700000000, 0).UTC()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `user` SET .*WHERE id = \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.TouchUserLogin(context.Background(), 7, "10.0.0.1", at); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTouchOpsLogin(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewIdentityRepository(db)
	at := time.Unix(1700000000, 0).UTC()

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `ops_user` SET .*WHERE id = \\?").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.TouchOpsLogin(context.Background(), 9, "10.0.0.1", at); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
