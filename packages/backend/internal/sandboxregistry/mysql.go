package sandboxregistry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) UpsertHeartbeat(ctx context.Context, heartbeat Heartbeat, now time.Time) (Instance, error) {
	var result Instance
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row model.SandboxInstance
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("name = ?", heartbeat.Name).
			Take(&row).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			row = model.SandboxInstance{
				PublicID:        uuid.NewString(),
				Name:            heartbeat.Name,
				Scheme:          heartbeat.Scheme,
				Host:            heartbeat.Host,
				Port:            heartbeat.Port,
				Status:          StatusOffline,
				HealthStatus:    HealthUnknown,
				ActiveRequests:  heartbeat.ActiveRequests,
				MaxConcurrency:  heartbeat.MaxConcurrency,
				LastHeartbeatAt: &now,
				HealthError:     "",
			}
			if err := tx.Create(&row).Error; err != nil {
				return fmt.Errorf("create sandbox instance: %w", err)
			}
		case err != nil:
			return fmt.Errorf("lock sandbox instance: %w", err)
		default:
			status := row.Status
			if row.HealthStatus == HealthHealthy {
				status = StatusOnline
			}
			updates := map[string]any{
				"scheme":            heartbeat.Scheme,
				"host":              heartbeat.Host,
				"port":              heartbeat.Port,
				"active_requests":   heartbeat.ActiveRequests,
				"max_concurrency":   heartbeat.MaxConcurrency,
				"last_heartbeat_at": now,
				"status":            status,
			}
			if err := tx.Model(&row).Updates(updates).Error; err != nil {
				return fmt.Errorf("update sandbox heartbeat: %w", err)
			}
			row.Scheme = heartbeat.Scheme
			row.Host = heartbeat.Host
			row.Port = heartbeat.Port
			row.ActiveRequests = heartbeat.ActiveRequests
			row.MaxConcurrency = heartbeat.MaxConcurrency
			row.LastHeartbeatAt = &now
			row.Status = status
		}
		result = toInstance(&row)
		return nil
	})
	if err != nil {
		return Instance{}, err
	}
	return result, nil
}

func (r *MySQLRepository) List(ctx context.Context) ([]Instance, error) {
	var rows []model.SandboxInstance
	if err := r.db.WithContext(ctx).Order("name ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list sandbox instances: %w", err)
	}
	return toInstances(rows), nil
}

func (r *MySQLRepository) Get(ctx context.Context, id int64) (Instance, error) {
	var row model.SandboxInstance
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&row).Error; err != nil {
		return Instance{}, fmt.Errorf("get sandbox instance: %w", err)
	}
	return toInstance(&row), nil
}

func (r *MySQLRepository) Select(ctx context.Context, now time.Time, ttl time.Duration, exclude []int64) (Instance, error) {
	var result Instance
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ? AND health_status = ? AND active_requests < max_concurrency AND last_heartbeat_at >= ?",
				StatusOnline, HealthHealthy, now.Add(-ttl))
		if len(exclude) > 0 {
			query = query.Where("id NOT IN ?", exclude)
		}
		var row model.SandboxInstance
		err := query.
			Order("(active_requests * 1.0 / GREATEST(max_concurrency, 1)) ASC").
			Order("active_requests ASC").
			Order("COALESCE(last_selected_at, '1970-01-01 00:00:00') ASC").
			Order("id ASC").
			Take(&row).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoHealthySandbox
			}
			return fmt.Errorf("select sandbox instance: %w", err)
		}
		if err := tx.Model(&row).Update("last_selected_at", now).Error; err != nil {
			return fmt.Errorf("update sandbox selection: %w", err)
		}
		result = toInstance(&row)
		return nil
	})
	if err != nil {
		return Instance{}, err
	}
	return result, nil
}

func (r *MySQLRepository) MarkStaleOffline(ctx context.Context, now time.Time, ttl time.Duration) error {
	err := r.db.WithContext(ctx).
		Model(&model.SandboxInstance{}).
		Where("status = ? AND last_heartbeat_at < ?", StatusOnline, now.Add(-ttl)).
		Update("status", StatusOffline).Error
	if err != nil {
		return fmt.Errorf("mark stale sandboxes offline: %w", err)
	}
	return nil
}

func (r *MySQLRepository) ListPendingHealth(ctx context.Context, now time.Time, ttl time.Duration) ([]Instance, error) {
	var rows []model.SandboxInstance
	err := r.db.WithContext(ctx).
		Where("last_heartbeat_at >= ? OR status = ?", now.Add(-ttl), StatusOnline).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list pending sandbox health: %w", err)
	}
	return toInstances(rows), nil
}

func (r *MySQLRepository) UpdateHealth(ctx context.Context, id int64, healthy bool, at time.Time, errMessage string) error {
	healthStatus := HealthUnhealthy
	status := StatusOffline
	if healthy {
		healthStatus = HealthHealthy
		status = StatusOnline
	}
	err := r.db.WithContext(ctx).
		Model(&model.SandboxInstance{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"health_status":  healthStatus,
			"status":         status,
			"last_health_at": at,
			"health_error":   errMessage,
		}).Error
	if err != nil {
		return fmt.Errorf("update sandbox health: %w", err)
	}
	return nil
}

func (r *MySQLRepository) GetProjectBinding(ctx context.Context, publicProjectID string) (Binding, error) {
	var row struct {
		ID        int64
		PublicID  string
		SandboxID *int64
	}
	err := r.db.WithContext(ctx).
		Table("project").
		Select("id, public_id, sandbox_id").
		Where("public_id = ? AND deleted_at IS NULL", publicProjectID).
		Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Binding{}, ErrProjectNotFound
		}
		return Binding{}, fmt.Errorf("get project binding: %w", err)
	}
	binding := Binding{
		ProjectInternalID: row.ID,
		ProjectPublicID:   row.PublicID,
		SandboxID:         row.SandboxID,
	}
	if row.SandboxID == nil {
		return binding, nil
	}
	instance, err := r.Get(ctx, *row.SandboxID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return binding, nil
		}
		return Binding{}, err
	}
	binding.Instance = &instance
	return binding, nil
}

func (r *MySQLRepository) UpdateProjectBinding(ctx context.Context, projectInternalID int64, oldSandboxID *int64, newSandboxID int64) error {
	query := r.db.WithContext(ctx).
		Model(&model.Project{}).
		Where("id = ? AND deleted_at IS NULL", projectInternalID)
	if oldSandboxID == nil {
		query = query.Where("sandbox_id IS NULL")
	} else {
		query = query.Where("sandbox_id = ?", *oldSandboxID)
	}
	result := query.Update("sandbox_id", newSandboxID)
	if result.Error != nil {
		return fmt.Errorf("update project sandbox binding: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrBindingConflict
	}
	return nil
}

func toInstances(rows []model.SandboxInstance) []Instance {
	out := make([]Instance, 0, len(rows))
	for i := range rows {
		out = append(out, toInstance(&rows[i]))
	}
	return out
}

func toInstance(row *model.SandboxInstance) Instance {
	return Instance{
		ID:              row.ID,
		PublicID:        row.PublicID,
		Name:            row.Name,
		Scheme:          row.Scheme,
		Host:            row.Host,
		Port:            row.Port,
		Status:          row.Status,
		HealthStatus:    row.HealthStatus,
		ActiveRequests:  row.ActiveRequests,
		MaxConcurrency:  row.MaxConcurrency,
		LastHeartbeatAt: row.LastHeartbeatAt,
		LastHealthAt:    row.LastHealthAt,
		HealthError:     row.HealthError,
	}
}
