package dao

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type ChannelPackageRepository struct {
	db *gorm.DB
}

func NewChannelPackageRepository(db *gorm.DB) *ChannelPackageRepository {
	return &ChannelPackageRepository{db: db}
}

func (r *ChannelPackageRepository) Create(ctx context.Context, item *model.ProjectChannelPackage) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create channel package: %w", err)
	}
	return nil
}

func (r *ChannelPackageRepository) Update(ctx context.Context, item *model.ProjectChannelPackage) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update channel package: %w", err)
	}
	return nil
}

func (r *ChannelPackageRepository) GetByPublicID(ctx context.Context, publicID string) (*model.ProjectChannelPackage, error) {
	var item model.ProjectChannelPackage
	if err := r.db.WithContext(ctx).Where("public_id = ?", publicID).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get channel package: %w", err)
	}
	return &item, nil
}

func (r *ChannelPackageRepository) ListByProject(ctx context.Context, projectPublicID string) ([]model.ProjectChannelPackage, error) {
	var items []model.ProjectChannelPackage
	if err := r.db.WithContext(ctx).
		Where("project_public_id = ?", projectPublicID).
		Order("created_at DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list channel packages: %w", err)
	}
	return items, nil
}

func (r *ChannelPackageRepository) Delete(ctx context.Context, publicID string) error {
	result := r.db.WithContext(ctx).Where("public_id = ?", publicID).Delete(&model.ProjectChannelPackage{})
	if result.Error != nil {
		return fmt.Errorf("delete channel package: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
