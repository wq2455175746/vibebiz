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
			"id", "public_id", "workspace_id", "user_id", "name", "prompt", "mode", "status",
			"sandbox_id", "created_at", "updated_at",
		}).AddRow(42, "d5f5e1d0-0000-4000-8000-000000000001", 3, 7, "demo", "make a game", "enterprise", "ready", 9, now, now))
	mock.ExpectQuery("SELECT agent_role_id FROM `project_agent_role`").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"agent_role_id"}))

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
	if got.Mode != model.ProjectModeEnterprise {
		t.Fatalf("mode = %q", got.Mode)
	}
	if got.SandboxID == nil || *got.SandboxID != 9 {
		t.Fatalf("sandbox id = %+v, want 9", got.SandboxID)
	}
}

func TestMySQLProjectStoreGetWithAgentRoles(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT \\* FROM `project` WHERE public_id = \\? AND `project`\\.`deleted_at` IS NULL").
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "workspace_id", "user_id", "name", "prompt", "status",
			"sandbox_id", "default_agent_role_id", "created_at", "updated_at",
		}).AddRow(42, "p1", 1, 7, "demo", "prompt", "ready", nil, 2, now, now))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT agent_role_id FROM `project_agent_role` WHERE project_id = ? ORDER BY sort_order ASC,id ASC")).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"agent_role_id"}).AddRow(2).AddRow(4))

	got, err := s.Get(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AgentRoleIDs) != 2 || got.AgentRoleIDs[0] != 2 || got.AgentRoleIDs[1] != 4 {
		t.Fatalf("role ids = %v", got.AgentRoleIDs)
	}
	if got.DefaultAgentRoleID == nil || *got.DefaultAgentRoleID != 2 {
		t.Fatalf("default role = %v", got.DefaultAgentRoleID)
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
	mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProjectStoreCreateWithAgentRoles(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	first := int64(2)
	project := &model.Project{
		ID: "p1", Name: "demo", WorkspaceID: 1, OwnerUserID: 7,
		DefaultAgentRoleID: &first, AgentRoleIDs: []int64{2, 4},
		State: model.ProjectStateReady,
	}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `project`").WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO `project_agent_role`").
		WithArgs(int64(2), int64(42), 0, int64(4), int64(42), 1).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProjectStoreSetAgentRoles(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM `project` WHERE public_id = ?").WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectExec("UPDATE `project` SET").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("INSERT INTO `project_agent_role`").
		WithArgs(int64(2), int64(42), 0, int64(4), int64(42), 1).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	if err := s.SetAgentRoles(context.Background(), "p1", []int64{2, 4}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProjectStoreSetAgentRolesClearsDefaults(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id FROM `project` WHERE public_id = ?").WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42))
	mock.ExpectExec("UPDATE `project` SET").WithArgs(nil, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `project_agent_role`").WithArgs(int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectCommit()
	if err := s.SetAgentRoles(context.Background(), "p1", nil); err != nil {
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

func TestMySQLProjectStoreUpdateSandboxID(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	sandboxID := int64(9)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project` SET .*sandbox_id.*").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := s.Update(context.Background(), &model.Project{ID: "p1", Name: "demo", SandboxID: &sandboxID}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLProjectStoreSetCover(t *testing.T) {
	db, mock := newMockProjectDB(t)
	s := newMySQLProjectStore(t, db, mock)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project` SET .*cover_s3_path.*").
		WithArgs("projects/p1/cover.png", sqlmock.AnyArg(), "p1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := s.SetCover(context.Background(), "p1", "projects/p1/cover.png"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
