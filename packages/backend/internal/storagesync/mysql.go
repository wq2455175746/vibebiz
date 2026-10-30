package storagesync

import (
	"context"
	"fmt"
	"time"

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

func (r *MySQLRepository) Save(ctx context.Context, record Record) error {
	row := &model.ProjectStorage{
		WorkspaceID:    record.WorkspaceID,
		ProjectID:      record.ProjectID,
		UserID:         record.UserID,
		StoragePath:    record.StoragePath,
		FileSizeBytes:  record.FileSizeBytes,
		FileCount:      record.FileCount,
		DirCount:       record.DirCount,
		LastModifiedAt: record.LastModifiedAt,
		LastScannedAt:  &record.LastScannedAt,
		Status:         1,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "project_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"workspace_id", "user_id", "storage_path", "file_size_bytes", "file_count",
				"dir_count", "last_modified_at", "last_scanned_at", "status", "updated_at", "deleted_at",
			}),
		}).Create(row).Error
		if err != nil {
			return fmt.Errorf("save project storage: %w", err)
		}
		var used int64
		if err := tx.Model(&model.ProjectStorage{}).
			Where("user_id = ?", record.UserID).
			Select("COALESCE(SUM(file_size_bytes), 0)").
			Scan(&used).Error; err != nil {
			return fmt.Errorf("sum project storage: %w", err)
		}
		if err := tx.Model(&model.UserQuota{}).
			Where("user_id = ?", record.UserID).
			Update("storage_used_bytes", used).Error; err != nil {
			return fmt.Errorf("update user storage: %w", err)
		}
		return nil
	})
}

func (r *MySQLRepository) ListByWorkspace(ctx context.Context, workspaceID int64) ([]Record, error) {
	var rows []model.ProjectStorage
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ?", workspaceID).
		Order("project_id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list project storage: %w", err)
	}
	out := make([]Record, 0, len(rows))
	for index := range rows {
		row := rows[index]
		out = append(out, Record{
			ProjectID:      row.ProjectID,
			WorkspaceID:    row.WorkspaceID,
			UserID:         row.UserID,
			StoragePath:    row.StoragePath,
			FileSizeBytes:  row.FileSizeBytes,
			FileCount:      row.FileCount,
			DirCount:       row.DirCount,
			LastModifiedAt: row.LastModifiedAt,
			LastScannedAt:  valueOrZero(row.LastScannedAt),
		})
	}
	return out, nil
}

func valueOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}
