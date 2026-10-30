package agentrole

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	gormDB, err := gorm.Open(mysql.New(mysql.Config{Conn: db, SkipInitializeWithVersion: true}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	return gormDB, mock
}

func TestMySQLListRoles(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "avatar_url", "is_builtin", "status", "sort_order", "version", "created_by", "created_at", "updated_at"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "desc", "prompt", "", 1, 1, 10, 1, "", now, now))

	repo := NewMySQLRepository(db)
	roles, err := repo.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].RoleCode != "alex" || !roles[0].IsBuiltin {
		t.Fatalf("roles = %+v", roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLListPageRoles(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `agent_role`").
		WithArgs("%alex%", "%alex%", "%alex%").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WithArgs("%alex%", "%alex%", "%alex%", 10).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "avatar_url", "is_builtin", "status", "sort_order", "version", "created_by", "created_at", "updated_at"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "desc", "prompt", "", 1, 1, 10, 1, "", now, now))

	page, err := NewMySQLRepository(db).ListPage(context.Background(), ListQuery{Keyword: "alex", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].RoleCode != "alex" {
		t.Fatalf("page = %+v", page)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLGetActiveRole(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WithArgs(1, int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "status", "sort_order"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "desc", "prompt", 1, 10))

	role, err := NewMySQLRepository(db).GetActive(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if role.RoleCode != "alex" || role.SystemPrompt != "prompt" {
		t.Fatalf("role = %+v", role)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLGetActiveRoleNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WithArgs(1, int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := NewMySQLRepository(db).GetActive(context.Background(), 1)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLUpdateRole(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `agent_role`").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WithArgs(int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "avatar_url", "is_builtin", "status", "sort_order", "version", "created_by", "created_at", "updated_at"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "新描述", "新提示词", "", 1, 0, 10, 2, "", now, now))

	role, err := NewMySQLRepository(db).Update(context.Background(), 1, UpdateInput{
		Description: "新描述", SystemPrompt: "新提示词", Status: 0,
	}, "ops")
	if err != nil {
		t.Fatal(err)
	}
	if role.SystemPrompt != "新提示词" || role.Status != 0 || role.Version != 2 {
		t.Fatalf("role = %+v", role)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLUpdateRoleNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `agent_role`").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT \\* FROM `agent_role`").
		WithArgs(int64(999), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	_, err := NewMySQLRepository(db).Update(context.Background(), 999, UpdateInput{
		Description: "d", SystemPrompt: "p", Status: 1,
	}, "ops")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLListActiveRoles(t *testing.T) {
	db, mock := newMockDB(t)
	now := time.Now()
	mock.ExpectQuery("SELECT \\* FROM `agent_role` WHERE status = \\? AND `agent_role`\\.`deleted_at` IS NULL ORDER BY sort_order ASC, id ASC").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "role_code", "role_name_en", "role_name_zh", "description", "system_prompt", "avatar_url", "is_builtin", "status", "sort_order", "version", "created_by", "created_at", "updated_at"}).
			AddRow(1, "alex", "Alex", "全栈工程师", "desc", "prompt", "", 1, 1, 10, 1, "", now, now))

	roles, err := NewMySQLRepository(db).ListActive(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || roles[0].RoleCode != "alex" {
		t.Fatalf("roles = %+v", roles)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
