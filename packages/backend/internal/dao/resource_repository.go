package dao

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type ProjectResourceRepository struct {
	db *gorm.DB
}

func NewProjectResourceRepository(db *gorm.DB) *ProjectResourceRepository {
	return &ProjectResourceRepository{db: db}
}

func (r *ProjectResourceRepository) Create(ctx context.Context, item *model.ProjectResource) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create project resource: %w", err)
	}
	return nil
}

func (r *ProjectResourceRepository) GetByResourceID(ctx context.Context, resourceID string) (*model.ProjectResource, error) {
	var item model.ProjectResource
	if err := r.db.WithContext(ctx).Where("resource_id = ?", resourceID).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project resource: %w", err)
	}
	return &item, nil
}

func (r *ProjectResourceRepository) ListReadyByProject(ctx context.Context, projectID string) ([]model.ProjectResource, error) {
	var items []model.ProjectResource
	if err := r.db.WithContext(ctx).
		Where("project_public_id = ? AND status = ?", projectID, "ready").
		Order("created_at ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list project resources: %w", err)
	}
	return items, nil
}

func (r *ProjectResourceRepository) ListByProject(ctx context.Context, projectID string) ([]model.ProjectResource, error) {
	var items []model.ProjectResource
	if err := r.db.WithContext(ctx).
		Where("project_public_id = ? AND status <> ?", projectID, "deleted").
		Order("created_at ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list project resources: %w", err)
	}
	return items, nil
}

func (r *ProjectResourceRepository) SumReadyByProject(ctx context.Context, projectID string) (int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).
		Model(&model.ProjectResource{}).
		Where("project_public_id = ? AND status = ?", projectID, "ready").
		Select("COALESCE(SUM(size_bytes), 0)").
		Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("sum project resources: %w", err)
	}
	return total, nil
}

func (r *ProjectResourceRepository) Update(ctx context.Context, item *model.ProjectResource) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update project resource: %w", err)
	}
	return nil
}

func (r *ProjectResourceRepository) SoftDelete(ctx context.Context, resourceID string) error {
	result := r.db.WithContext(ctx).
		Model(&model.ProjectResource{}).
		Where("resource_id = ?", resourceID).
		Update("status", "deleted")
	if result.Error != nil {
		return fmt.Errorf("delete project resource: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	if err := r.db.WithContext(ctx).Where("resource_id = ?", resourceID).Delete(&model.ProjectResource{}).Error; err != nil {
		return fmt.Errorf("soft delete project resource: %w", err)
	}
	return nil
}
