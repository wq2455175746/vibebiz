package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
)

type MySQLProjectStore struct {
	db            *gorm.DB
	commonTopicID int64
}

func NewMySQLProjectStore(ctx context.Context, db *gorm.DB) (*MySQLProjectStore, error) {
	var topicID int64
	err := db.WithContext(ctx).
		Table("project_topic").
		Select("id").
		Where("topic_code = ? AND status = ?", "common", 1).
		Take(&topicID).Error
	if err != nil {
		return nil, fmt.Errorf("resolve common project topic: %w", err)
	}
	return &MySQLProjectStore{db: db, commonTopicID: topicID}, nil
}

func (s *MySQLProjectStore) Create(ctx context.Context, project *model.Project) error {
	status := project.State
	if status == "" {
		status = model.ProjectStateReady
	}
	row := &daomodel.Project{
		PublicID:    project.ID,
		WorkspaceID: project.WorkspaceID,
		UserID:      project.OwnerUserID,
		Name:        project.Name,
		Prompt:      project.Prompt,
		SandboxID:   project.SandboxID,
		TopicID:     s.commonTopicID,
		Status:      status,
		Base: daomodel.Base{
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		},
	}
	return s.db.WithContext(ctx).Create(row).Error
}

func (s *MySQLProjectStore) Get(ctx context.Context, id string) (*model.Project, error) {
	var row daomodel.Project
	err := s.db.WithContext(ctx).Where("public_id = ?", id).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return toProject(&row), nil
}

func (s *MySQLProjectStore) List(ctx context.Context) []*model.Project {
	var rows []daomodel.Project
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&rows).Error; err != nil {
		return []*model.Project{}
	}
	out := make([]*model.Project, 0, len(rows))
	for i := range rows {
		out = append(out, toProject(&rows[i]))
	}
	return out
}

func (s *MySQLProjectStore) ListByWorkspaces(ctx context.Context, workspaceIDs []int64) []*model.Project {
	if len(workspaceIDs) == 0 {
		return []*model.Project{}
	}
	var rows []daomodel.Project
	if err := s.db.WithContext(ctx).
		Where("workspace_id IN ?", workspaceIDs).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return []*model.Project{}
	}
	out := make([]*model.Project, 0, len(rows))
	for i := range rows {
		out = append(out, toProject(&rows[i]))
	}
	return out
}

func (s *MySQLProjectStore) Update(ctx context.Context, project *model.Project) error {
	result := s.db.WithContext(ctx).
		Model(&daomodel.Project{}).
		Where("public_id = ?", project.ID).
		Updates(map[string]any{
			"name":         project.Name,
			"prompt":       project.Prompt,
			"sandbox_id":   project.SandboxID,
			"status":       project.State,
			"workspace_id": project.WorkspaceID,
			"user_id":      project.OwnerUserID,
			"updated_at":   time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("update project: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLProjectStore) Delete(ctx context.Context, id string) error {
	result := s.db.WithContext(ctx).
		Where("public_id = ?", id).
		Delete(&daomodel.Project{})
	if result.Error != nil {
		return fmt.Errorf("delete project: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func toProject(row *daomodel.Project) *model.Project {
	return &model.Project{
		ID:          row.PublicID,
		Name:        row.Name,
		Prompt:      row.Prompt,
		State:       row.Status,
		WorkspaceID: row.WorkspaceID,
		OwnerUserID: row.UserID,
		SandboxID:   row.SandboxID,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}
