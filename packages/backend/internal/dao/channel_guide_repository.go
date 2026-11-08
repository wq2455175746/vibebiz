package dao

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type ChannelGuideRepository struct {
	db *gorm.DB
}

func NewChannelGuideRepository(db *gorm.DB) *ChannelGuideRepository {
	return &ChannelGuideRepository{db: db}
}

func (r *ChannelGuideRepository) List(ctx context.Context, includeDisabled bool, keyword string) ([]model.ChannelGuide, error) {
	query := r.db.WithContext(ctx).Model(&model.ChannelGuide{})
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(channel LIKE ? OR title LIKE ? OR summary LIKE ?)", like, like, like)
	}
	items := make([]model.ChannelGuide, 0)
	if err := query.Order("sort_order ASC, channel ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list channel guides: %w", err)
	}
	return items, nil
}

func (r *ChannelGuideRepository) Get(ctx context.Context, channel string) (*model.ChannelGuide, error) {
	var item model.ChannelGuide
	if err := r.db.WithContext(ctx).Where("channel = ?", channel).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get channel guide: %w", err)
	}
	return &item, nil
}

func (r *ChannelGuideRepository) Create(ctx context.Context, item *model.ChannelGuide) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create channel guide: %w", err)
	}
	return nil
}

func (r *ChannelGuideRepository) Update(ctx context.Context, item *model.ChannelGuide) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update channel guide: %w", err)
	}
	return nil
}

func (r *ChannelGuideRepository) Delete(ctx context.Context, channel string) error {
	result := r.db.WithContext(ctx).Where("channel = ?", channel).Delete(&model.ChannelGuide{})
	if result.Error != nil {
		return fmt.Errorf("delete channel guide: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ChannelGuideRepository) SeedDefaults(ctx context.Context, items []model.ChannelGuide) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "channel"}}, DoNothing: true}).
		Create(&items).Error; err != nil {
		return fmt.Errorf("seed channel guides: %w", err)
	}
	return nil
}
