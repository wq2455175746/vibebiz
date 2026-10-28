package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

var ErrNotFound = errors.New("record not found")

type ListOptions struct {
	Offset int
	Limit  int
}

type Repository[T any] struct {
	db *gorm.DB
}

func NewRepository[T any](db *gorm.DB) *Repository[T] {
	return &Repository[T]{db: db}
}

func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

func (r *Repository[T]) Get(ctx context.Context, id int64) (*T, error) {
	var entity T
	if err := r.db.WithContext(ctx).First(&entity, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &entity, nil
}

func (r *Repository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

func (r *Repository[T]) Delete(ctx context.Context, id int64) error {
	var entity T
	result := r.db.WithContext(ctx).Delete(&entity, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository[T]) List(ctx context.Context, opts ListOptions) ([]T, error) {
	if opts.Offset < 0 {
		opts.Offset = 0
	}
	if opts.Limit <= 0 {
		opts.Limit = 100
	}
	if opts.Limit > 1000 {
		opts.Limit = 1000
	}

	var entities []T
	err := r.db.WithContext(ctx).
		Offset(opts.Offset).
		Limit(opts.Limit).
		Order("id DESC").
		Find(&entities).Error
	return entities, err
}

func (r *Repository[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(new(T)).Count(&count).Error
	return count, err
}

type Repositories struct {
	Users                    *Repository[model.User]
	OpsUsers                 *Repository[model.OpsUser]
	OpsAuditLogs             *Repository[model.OpsAuditLog]
	Workspaces               *Repository[model.Workspace]
	WorkspaceUsers           *Repository[model.WorkspaceUser]
	UserQuotas               *Repository[model.UserQuota]
	ProjectTopics            *Repository[model.ProjectTopic]
	ProjectTemplates         *Repository[model.ProjectTemplate]
	Projects                 *Repository[model.Project]
	ProjectStorages          *Repository[model.ProjectStorage]
	ExternalResources        *Repository[model.ExternalResource]
	ProjectExternalResources *Repository[model.ProjectExternalResource]
	LLMModels                *Repository[model.LLMModel]
	AgentRoles               *Repository[model.AgentRole]
	ScheduledTasks           *Repository[model.ScheduledTask]
	ChatConversations        *Repository[model.ChatConversation]
	ChatMessages             *Repository[model.ChatMessage]
	ChatMemories             *Repository[model.ChatMemory]
	RAGKnowledgeBases        *Repository[model.RagKnowledgeBase]
	ProjectRAGs              *Repository[model.ProjectRag]
}

func NewRepositories(db *gorm.DB) *Repositories {
	return &Repositories{
		Users:                    NewRepository[model.User](db),
		OpsUsers:                 NewRepository[model.OpsUser](db),
		OpsAuditLogs:             NewRepository[model.OpsAuditLog](db),
		Workspaces:               NewRepository[model.Workspace](db),
		WorkspaceUsers:           NewRepository[model.WorkspaceUser](db),
		UserQuotas:               NewRepository[model.UserQuota](db),
		ProjectTopics:            NewRepository[model.ProjectTopic](db),
		ProjectTemplates:         NewRepository[model.ProjectTemplate](db),
		Projects:                 NewRepository[model.Project](db),
		ProjectStorages:          NewRepository[model.ProjectStorage](db),
		ExternalResources:        NewRepository[model.ExternalResource](db),
		ProjectExternalResources: NewRepository[model.ProjectExternalResource](db),
		LLMModels:                NewRepository[model.LLMModel](db),
		AgentRoles:               NewRepository[model.AgentRole](db),
		ScheduledTasks:           NewRepository[model.ScheduledTask](db),
		ChatConversations:        NewRepository[model.ChatConversation](db),
		ChatMessages:             NewRepository[model.ChatMessage](db),
		ChatMemories:             NewRepository[model.ChatMemory](db),
		RAGKnowledgeBases:        NewRepository[model.RagKnowledgeBase](db),
		ProjectRAGs:              NewRepository[model.ProjectRag](db),
	}
}
