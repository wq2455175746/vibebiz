package sandboxregistry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysqldriver "github.com/go-sql-driver/mysql"
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
	leaseUntil := time.Date(2026, 11, 7, 10, 0, 30, 0, time.UTC)

	mock.ExpectQuery("SELECT project.id, project.public_id, project.sandbox_id AS project_sandbox, project_leases.sandbox_id AS lease_sandbox, project_leases.lease_until, project_leases.epoch FROM `project` LEFT JOIN project_leases ON project_leases.project_id = project.id WHERE .*public_id = \\?.*deleted_at IS NULL").
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "project_sandbox", "lease_sandbox", "lease_until", "epoch"}).
			AddRow(42, "p1", 7, 8, leaseUntil, 4))
	mock.ExpectQuery("SELECT .* FROM `sandbox_instance` WHERE id = \\?").
		WithArgs(8, 1).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "name", "scheme", "host", "port", "status", "health_status",
			"active_requests", "max_concurrency", "last_heartbeat_at", "last_health_at", "health_error",
		}).AddRow(8, "sb-8", "sandbox-b", "http", "10.0.0.12", 8000, StatusOnline, HealthHealthy, 1, 8, time.Now(), time.Now(), ""))

	got, err := repo.GetProjectBinding(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectInternalID != 42 || got.SandboxID == nil || *got.SandboxID != 8 || got.Instance == nil || got.Instance.Name != "sandbox-b" {
		t.Fatalf("binding = %+v", got)
	}
	if !got.LeaseUntil.Equal(leaseUntil) || got.Epoch != 4 {
		t.Fatalf("lease = %v epoch = %d", got.LeaseUntil, got.Epoch)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryGetProjectBindingWithoutSandbox(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)

	mock.ExpectQuery("SELECT project.id, project.public_id, project.sandbox_id AS project_sandbox, project_leases.sandbox_id AS lease_sandbox, project_leases.lease_until, project_leases.epoch FROM `project` LEFT JOIN project_leases ON project_leases.project_id = project.id WHERE .*public_id = \\?.*deleted_at IS NULL").
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "project_sandbox", "lease_sandbox", "lease_until", "epoch"}).
			AddRow(42, "p1", nil, nil, nil, nil))

	got, err := repo.GetProjectBinding(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectInternalID != 42 || got.SandboxID != nil || got.Instance != nil || got.Epoch != 0 || !got.LeaseUntil.IsZero() {
		t.Fatalf("binding = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryGetProjectBindingWithoutInstance(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	leaseUntil := time.Date(2026, 11, 7, 10, 0, 30, 0, time.UTC)

	mock.ExpectQuery("SELECT project.id, project.public_id, project.sandbox_id AS project_sandbox, project_leases.sandbox_id AS lease_sandbox, project_leases.lease_until, project_leases.epoch FROM `project` LEFT JOIN project_leases ON project_leases.project_id = project.id WHERE .*public_id = \\?.*deleted_at IS NULL").
		WithArgs("p1", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "public_id", "project_sandbox", "lease_sandbox", "lease_until", "epoch"}).
			AddRow(42, "p1", 7, 7, leaseUntil, 4))
	mock.ExpectQuery("SELECT .* FROM `sandbox_instance` WHERE id = \\?").
		WithArgs(7, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := repo.GetProjectBinding(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ProjectInternalID != 42 || got.SandboxID == nil || *got.SandboxID != 7 || got.Instance != nil || got.Epoch != 4 || !got.LeaseUntil.Equal(leaseUntil) {
		t.Fatalf("binding = %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseCreates(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	expires := now.UTC().Add(30 * time.Second)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
	mock.ExpectExec("INSERT INTO `project_leases`").
		WithArgs(int64(2), expires, int64(1), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `project` SET `sandbox_id`=\\?,`updated_at`=\\? WHERE id = \\? AND `project`.`deleted_at` IS NULL").
		WithArgs(int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	lease, err := repo.AcquireProjectLease(context.Background(), 42, nil, 2, now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lease.ProjectInternalID != 42 || lease.SandboxID != 2 || lease.Epoch != 1 || !lease.LeaseUntil.Equal(expires) || lease.LeaseUntil.Location() != time.UTC {
		t.Fatalf("lease = %+v", lease)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseExpiredNewOwner(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	expires := now.Add(30 * time.Second)
	expired := now.Add(-time.Second)
	oldID := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "sandbox_id", "lease_until", "epoch"}).
			AddRow(42, 1, expired, 2))
	mock.ExpectExec("UPDATE `project_leases` SET `epoch`=\\?,`lease_until`=\\?,`sandbox_id`=\\?,`updated_at`=\\? WHERE `project_id` = \\?").
		WithArgs(int64(3), expires, int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `project` SET `sandbox_id`=\\?,`updated_at`=\\? WHERE id = \\? AND `project`.`deleted_at` IS NULL").
		WithArgs(int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	lease, err := repo.AcquireProjectLease(context.Background(), 42, &oldID, 2, now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lease.SandboxID != 2 || lease.Epoch != 3 || !lease.LeaseUntil.Equal(expires) {
		t.Fatalf("lease = %+v", lease)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseRenewsSameOwner(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	expires := now.Add(30 * time.Second)
	future := now.Add(20 * time.Second)
	owner := int64(2)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "sandbox_id", "lease_until", "epoch"}).
			AddRow(42, 2, future, 3))
	mock.ExpectExec("UPDATE `project_leases` SET `epoch`=\\?,`lease_until`=\\?,`sandbox_id`=\\?,`updated_at`=\\? WHERE `project_id` = \\?").
		WithArgs(int64(3), expires, int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	lease, err := repo.AcquireProjectLease(context.Background(), 42, &owner, 2, now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lease.SandboxID != 2 || lease.Epoch != 3 || !lease.LeaseUntil.Equal(expires) {
		t.Fatalf("lease = %+v", lease)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseExpiredSameOwner(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	expires := now.Add(30 * time.Second)
	expired := now.Add(-time.Second)
	owner := int64(2)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "sandbox_id", "lease_until", "epoch"}).
			AddRow(42, 2, expired, 3))
	mock.ExpectExec("UPDATE `project_leases` SET `epoch`=\\?,`lease_until`=\\?,`sandbox_id`=\\?,`updated_at`=\\? WHERE `project_id` = \\?").
		WithArgs(int64(4), expires, int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	lease, err := repo.AcquireProjectLease(context.Background(), 42, &owner, 2, now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lease.SandboxID != 2 || lease.Epoch != 4 || !lease.LeaseUntil.Equal(expires) {
		t.Fatalf("lease = %+v", lease)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseConflict(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	future := now.Add(20 * time.Second)
	oldID := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "sandbox_id", "lease_until", "epoch"}).
			AddRow(42, 1, future, 2))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `sandbox_instance` WHERE id = \\?.*deleted_at.*IS NULL").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectRollback()

	_, err := repo.AcquireProjectLease(context.Background(), 42, &oldID, 2, now, 30*time.Second)
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("err = %v, want ErrBindingConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseTakesOverMissingOwner(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	expires := now.Add(30 * time.Second)
	future := now.Add(20 * time.Second)
	oldID := int64(1)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
		WithArgs(int64(42), 1).
		WillReturnRows(sqlmock.NewRows([]string{"project_id", "sandbox_id", "lease_until", "epoch"}).
			AddRow(42, 1, future, 2))
	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `sandbox_instance` WHERE id = \\?.*deleted_at.*IS NULL").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE `project_leases` SET `epoch`=\\?,`lease_until`=\\?,`sandbox_id`=\\?,`updated_at`=\\? WHERE `project_id` = \\?").
		WithArgs(int64(3), expires, int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE `project` SET `sandbox_id`=\\?,`updated_at`=\\? WHERE id = \\? AND `project`.`deleted_at` IS NULL").
		WithArgs(int64(2), sqlmock.AnyArg(), int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	lease, err := repo.AcquireProjectLease(context.Background(), 42, &oldID, 2, now, 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if lease.SandboxID != 2 || lease.Epoch != 3 || !lease.LeaseUntil.Equal(expires) {
		t.Fatalf("lease = %+v", lease)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryAcquireProjectLeaseRetryableErrors(t *testing.T) {
	for _, number := range []uint16{mysqlDuplicateEntry, mysqlDeadlock} {
		t.Run(fmt.Sprintf("%d", number), func(t *testing.T) {
			db, mock := newMockRegistryDB(t)
			repo := NewMySQLRepository(db)
			now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

			mock.ExpectBegin()
			mock.ExpectQuery("SELECT .* FROM `project_leases` WHERE project_id = \\? .*FOR UPDATE").
				WithArgs(int64(42), 1).
				WillReturnRows(sqlmock.NewRows([]string{"project_id"}))
			mock.ExpectExec("INSERT INTO `project_leases`").
				WillReturnError(&mysqldriver.MySQLError{Number: number})
			mock.ExpectRollback()

			_, err := repo.AcquireProjectLease(context.Background(), 42, nil, 2, now, 30*time.Second)
			if !errors.Is(err, ErrBindingConflict) {
				t.Fatalf("err = %v, want ErrBindingConflict", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMySQLRepositoryRenewProjectLease(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	expires := now.Add(30 * time.Second)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project_leases` SET `lease_until`=\\?,`updated_at`=\\? WHERE project_id = \\? AND sandbox_id = \\? AND epoch = \\? AND lease_until > \\?").
		WithArgs(expires, sqlmock.AnyArg(), int64(42), int64(2), int64(3), now).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := repo.RenewProjectLease(context.Background(), 42, 2, 3, now, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryRenewProjectLeaseConflict(t *testing.T) {
	db, mock := newMockRegistryDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `project_leases` SET `lease_until`=\\?,`updated_at`=\\? WHERE project_id = \\? AND sandbox_id = \\? AND epoch = \\? AND lease_until > \\?").
		WithArgs(now.Add(30*time.Second), sqlmock.AnyArg(), int64(42), int64(2), int64(3), now).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := repo.RenewProjectLease(context.Background(), 42, 2, 3, now, 30*time.Second)
	if !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("err = %v, want ErrBindingConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
