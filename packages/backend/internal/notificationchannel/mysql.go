package notificationchannel

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) List(ctx context.Context, userID int64) ([]model.NotificationChannel, error) {
	var items []model.NotificationChannel
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list notification channels: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) Get(ctx context.Context, userID, id int64) (*model.NotificationChannel, error) {
	var item model.NotificationChannel
	err := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Take(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get notification channel: %w", err)
	}
	return &item, nil
}

func (r *MySQLRepository) Create(ctx context.Context, item *model.NotificationChannel) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create notification channel: %w", err)
	}
	return nil
}

func (r *MySQLRepository) Update(ctx context.Context, item *model.NotificationChannel) error {
	result := r.db.WithContext(ctx).Model(&model.NotificationChannel{}).
		Where("user_id = ? AND id = ?", item.UserID, item.ID).
		Updates(map[string]any{"name": item.Name, "type": item.Type, "config_cipher": item.ConfigCipher, "enabled": item.Enabled})
	if result.Error != nil {
		return fmt.Errorf("update notification channel: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) Delete(ctx context.Context, userID, id int64) error {
	result := r.db.WithContext(ctx).Where("user_id = ? AND id = ?", userID, id).Delete(&model.NotificationChannel{})
	if result.Error != nil {
		return fmt.Errorf("delete notification channel: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) NameExists(ctx context.Context, userID int64, name string, excludeID int64) (bool, error) {
	query := r.db.WithContext(ctx).Model(&model.NotificationChannel{}).Where("user_id = ? AND name = ?", userID, name)
	if excludeID > 0 {
		query = query.Where("id <> ?", excludeID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, fmt.Errorf("count notification channel name: %w", err)
	}
	return count > 0, nil
}

func (r *MySQLRepository) UpdateTest(ctx context.Context, userID, id int64, at time.Time, status, message string) error {
	result := r.db.WithContext(ctx).Model(&model.NotificationChannel{}).
		Where("user_id = ? AND id = ?", userID, id).
		Updates(map[string]any{"last_test_at": at, "last_test_status": status, "last_test_error": message})
	if result.Error != nil {
		return fmt.Errorf("update notification channel test: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
