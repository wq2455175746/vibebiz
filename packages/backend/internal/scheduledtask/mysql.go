package scheduledtask

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Create(ctx context.Context, item *daomodel.ScheduledTask) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create scheduled task: %w", err)
	}
	return nil
}

func (r *MySQLRepository) Get(ctx context.Context, id int64) (*daomodel.ScheduledTask, error) {
	var item daomodel.ScheduledTask
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get scheduled task: %w", err)
	}
	return &item, nil
}

func (r *MySQLRepository) ListByUser(ctx context.Context, userID int64, workspaceIDs []int64) ([]daomodel.ScheduledTask, error) {
	var items []daomodel.ScheduledTask
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if len(workspaceIDs) > 0 {
		query = query.Where("workspace_id IN ?", workspaceIDs)
	}
	if err := query.Order("created_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list user scheduled tasks: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) List(ctx context.Context, filter ListFilter) ([]daomodel.ScheduledTask, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	query := r.db.WithContext(ctx).Model(&daomodel.ScheduledTask{})
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR prompt LIKE ?", like, like)
	}
	if filter.Enabled != nil {
		value := 0
		if *filter.Enabled {
			value = 1
		}
		query = query.Where("enabled = ?", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count scheduled tasks: %w", err)
	}
	var items []daomodel.ScheduledTask
	if err := query.Order("created_at DESC, id DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list scheduled tasks: %w", err)
	}
	return items, total, nil
}

func (r *MySQLRepository) Update(ctx context.Context, id int64, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&daomodel.ScheduledTask{}).Where("id = ?", id).Updates(fields)
	if result.Error != nil {
		return fmt.Errorf("update scheduled task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&daomodel.ScheduledTask{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete scheduled task: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) Due(ctx context.Context, now time.Time, limit int) ([]daomodel.ScheduledTask, error) {
	var items []daomodel.ScheduledTask
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND deleted_at IS NULL AND next_run_at IS NOT NULL AND next_run_at <= ?", 1, now).
		Order("next_run_at ASC, id ASC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list due scheduled tasks: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) Claim(ctx context.Context, id int64, oldNext, next, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&daomodel.ScheduledTask{}).
		Where("id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?", id, 1, oldNext).
		Updates(map[string]any{
			"next_run_at": next,
			"last_run_at": now,
			"run_count":   gorm.Expr("run_count + 1"),
			"updated_at":  now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("claim scheduled task: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

func (r *MySQLRepository) TopicByCode(ctx context.Context, code string) (*daomodel.ProjectTopic, error) {
	var topic daomodel.ProjectTopic
	if err := r.db.WithContext(ctx).Where("topic_code = ?", code).Take(&topic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalid
		}
		return nil, fmt.Errorf("get scheduled topic: %w", err)
	}
	return &topic, nil
}

func (r *MySQLRepository) UserNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	var rows []struct {
		ID       int64  `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("user").Select("id, username").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load scheduled task usernames: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Username
	}
	return out, nil
}
