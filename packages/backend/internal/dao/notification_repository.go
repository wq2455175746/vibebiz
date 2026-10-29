package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type NotificationFilter struct {
	Type   string
	Status string
}

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) List(ctx context.Context, filter NotificationFilter, offset, limit int) ([]model.OpsNotification, int64, error) {
	query := r.db.WithContext(ctx).Model(&model.OpsNotification{})
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count notifications: %w", err)
	}
	items := make([]model.OpsNotification, 0)
	if err := query.Order("created_at DESC, id DESC").Offset(offset).Limit(limit).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	return items, total, nil
}

func (r *NotificationRepository) Get(ctx context.Context, id int64) (*model.OpsNotification, error) {
	var item model.OpsNotification
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get notification: %w", err)
	}
	return &item, nil
}

func (r *NotificationRepository) Create(ctx context.Context, item *model.OpsNotification) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) Update(ctx context.Context, item *model.OpsNotification) error {
	result := r.db.WithContext(ctx).Model(&model.OpsNotification{}).
		Where("id = ?", item.ID).
		Updates(map[string]any{
			"title": item.Title, "content": item.Content, "type": item.Type,
			"version": item.Version, "link": item.Link, "expire_at": item.ExpireAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if _, err := r.Get(ctx, item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (r *NotificationRepository) UpdateStatus(ctx context.Context, id int64, status string, publishedAt *time.Time) error {
	updates := map[string]any{"status": status}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	result := r.db.WithContext(ctx).Model(&model.OpsNotification{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update notification status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		if _, err := r.Get(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (r *NotificationRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&model.OpsNotification{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) Published(ctx context.Context, now time.Time) ([]model.OpsNotification, error) {
	items := make([]model.OpsNotification, 0)
	err := r.db.WithContext(ctx).
		Where("status = ? AND expire_at > ?", model.NotificationPublished, now).
		Order("published_at DESC, id DESC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list published notifications: %w", err)
	}
	return items, nil
}
