package sandboxregistry

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

func newMockRegistryDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
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

func TestMySQLRepositoryUpsertHeartbeatCreatesInstance(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `sandbox_instance` .*FOR UPDATE").
		WithArgs("sandbox-a", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT INTO `sandbox_instance`").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	got, err := repo.UpsertHeartbeat(context.Background(), Heartbeat{
		Name: "sandbox-a", Scheme: "http", Host: "10.0.0.11", Port: 8000,
		ActiveRequests: 2, MaxConcurrency: 8,
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 1 || got.Name != "sandbox-a" || got.Status != StatusOffline || got.HealthStatus != HealthUnknown {
		t.Fatalf("instance = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositorySelectLeastLoaded(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `sandbox_instance` .*FOR UPDATE").
		WithArgs(StatusOnline, HealthHealthy, now.Add(-15*time.Second), 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "name", "scheme", "host", "port", "status", "health_status",
			"active_requests", "max_concurrency", "last_heartbeat_at", "last_health_at", "health_error",
		}).AddRow(2, "sb-2", "sandbox-b", "http", "10.0.0.12", 8000, StatusOnline, HealthHealthy, 2, 8, heartbeat, heartbeat, ""))
	mock.ExpectExec("UPDATE `sandbox_instance` SET .*last_selected_at").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := repo.Select(context.Background(), now, 15*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 || got.BaseURL() != "http://10.0.0.12:8000" {
		t.Fatalf("instance = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryUpdateProjectBindingCAS(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	oldID := int64(1)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project` SET .*sandbox_id.*WHERE .*id = \\?.*sandbox_id = \\?").
		WithArgs(int64(2), sqlmock.AnyArg(), int64(42), oldID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.UpdateProjectBinding(context.Background(), 42, &oldID, 2)
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("err = %v, want ErrBindingConflict", err)
	}
}

func TestMySQLRepositoryGetProjectBinding(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)

	mock.ExpectQuery("SELECT id, public_id, sandbox_id FROM `project`").
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "sandbox_id"}).AddRow(42, "p1", 7))
	mock.ExpectQuery("SELECT .* FROM `sandbox_instance` WHERE id = \\?").
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "name", "scheme", "host", "port", "status", "health_status",
			"active_requests", "max_concurrency", "last_heartbeat_at", "last_health_at", "health_error",
		}).AddRow(7, "sb-7", "sandbox-a", "http", "10.0.0.11", 8000, StatusOnline, HealthHealthy, 1, 8, time.Now(), time.Now(), ""))

	got, err := repo.GetProjectBinding(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectInternalID != 42 || got.SandboxID == nil || *got.SandboxID != 7 || got.Instance == nil || got.Instance.Name != "sandbox-a" {
		t.Fatalf("binding = %+v", got)
	}
}

func TestMySQLRepositoryGetProjectBindingWithoutSandbox(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)

	mock.ExpectQuery("SELECT id, public_id, sandbox_id FROM `project`").
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "sandbox_id"}).AddRow(42, "p1", nil))

	got, err := repo.GetProjectBinding(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectInternalID != 42 || got.SandboxID != nil || got.Instance != nil {
		t.Fatalf("binding = %+v", got)
	}
}

func TestMySQLRepositoryMarkStaleOffline(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `sandbox_instance` SET `status`=\\?,`updated_at`=\\? WHERE .*status = \\? AND last_heartbeat_at < \\?.*deleted_at.*").
		WithArgs(StatusOffline, sqlmock.AnyArg(), StatusOnline, now.Add(-15*time.Second)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.MarkStaleOffline(context.Background(), now, 15*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
