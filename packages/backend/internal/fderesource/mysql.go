package fderesource

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type MySQLStore struct {
	db *gorm.DB
}

func NewMySQLStore(db *gorm.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) List(ctx context.Context, enabledOnly bool) ([]model.FdeResource, error) {
	query := s.db.WithContext(ctx).Model(&model.FdeResource{})
	if enabledOnly {
		query = query.Where("enabled = ?", true)
	}
	items := make([]model.FdeResource, 0)
	if err := query.Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list fde resources: %w", err)
	}
	return items, nil
}

func (s *MySQLStore) Get(ctx context.Context, id int64) (*model.FdeResource, error) {
	var item model.FdeResource
	if err := s.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get fde resource: %w", err)
	}
	return &item, nil
}

func (s *MySQLStore) Create(ctx context.Context, item *model.FdeResource) error {
	if err := s.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create fde resource: %w", err)
	}
	return nil
}

func (s *MySQLStore) Update(ctx context.Context, item *model.FdeResource) error {
	result := s.db.WithContext(ctx).Model(&model.FdeResource{}).
		Where("id = ?", item.ID).
		Updates(map[string]any{
			"name": item.Name, "contact": item.Contact, "skills": item.Skills, "region": item.Region,
			"note": item.Note, "enabled": item.Enabled, "updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update fde resource: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if _, err := s.Get(ctx, item.ID); err != nil {
			return ErrNotFound
		}
	}
	return nil
}
