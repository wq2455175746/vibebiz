package projecttemplate

import (
	"context"
	"errors"
	"fmt"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Create(ctx context.Context, item *model.ProjectTemplate) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create template: %w", err)
	}
	return nil
}

func (r *MySQLRepository) Get(ctx context.Context, id int64) (*model.ProjectTemplate, error) {
	var item model.ProjectTemplate
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *MySQLRepository) ListAdmin(ctx context.Context) ([]model.ProjectTemplate, error) {
	var items []model.ProjectTemplate
	if err := r.db.WithContext(ctx).
		Order("created_at DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list admin templates: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) Update(ctx context.Context, id int64, item *model.ProjectTemplate) error {
	result := r.db.WithContext(ctx).Model(&model.ProjectTemplate{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"template_name": item.TemplateName, "description": item.Description,
			"topic_id": item.TopicID, "recommend_score": item.RecommendScore,
			"status": item.Status, "updated_by": item.UpdatedBy,
		})
	if result.Error != nil {
		return fmt.Errorf("update template: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&model.ProjectTemplate{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete template: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) UpdateArchive(ctx context.Context, id int64, path string, fileTree []byte) error {
	if err := r.db.WithContext(ctx).Model(&model.ProjectTemplate{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"template_s3_path": path,
			"file_tree":        fileTree,
			"status":           1,
		}).Error; err != nil {
		return fmt.Errorf("update template archive: %w", err)
	}
	return nil
}

func (r *MySQLRepository) List(ctx context.Context) ([]model.ProjectTemplate, error) {
	var items []model.ProjectTemplate
	if err := r.db.WithContext(ctx).
		Where("status = ? AND template_s3_path <> ''", 1).
		Order("recommend_score DESC, id DESC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) Topics(ctx context.Context) ([]model.ProjectTopic, error) {
	var items []model.ProjectTopic
	if err := r.db.WithContext(ctx).
		Where("status = ?", 1).
		Order("parent_id ASC, sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list topics: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) AllTopics(ctx context.Context) ([]model.ProjectTopic, error) {
	var items []model.ProjectTopic
	if err := r.db.WithContext(ctx).
		Order("parent_id ASC, sort_order ASC, id ASC").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list all topics: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) CreateTopic(ctx context.Context, item *model.ProjectTopic) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		var mysqlErr *mysqldriver.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrConflict
		}
		return fmt.Errorf("create topic: %w", err)
	}
	return nil
}

func (r *MySQLRepository) UpdateTopic(ctx context.Context, id int64, item *model.ProjectTopic) error {
	result := r.db.WithContext(ctx).Model(&model.ProjectTopic{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"parent_id": item.ParentID, "topic_code": item.TopicCode, "topic_name": item.TopicName,
			"description": item.Description, "prompt_template": item.PromptTemplate,
			"sort_order": item.SortOrder, "status": item.Status, "updated_by": item.UpdatedBy,
		})
	if result.Error != nil {
		return fmt.Errorf("update topic: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) Topic(ctx context.Context, id int64) (*model.ProjectTopic, error) {
	var item model.ProjectTopic
	if err := r.db.WithContext(ctx).Where("id = ? AND status = ?", id, 1).Take(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}
