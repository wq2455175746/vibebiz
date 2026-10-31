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
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		topicID := project.TopicID
		if topicID <= 0 {
			topicID = s.commonTopicID
		}
		reviewStatus := project.ReviewStatus
		if reviewStatus == "" {
			reviewStatus = model.ReviewStatusPending
		}
		mode := project.Mode
		if mode == "" {
			mode = model.ProjectModeStandard
		}
		row := &daomodel.Project{
			PublicID:           project.ID,
			WorkspaceID:        project.WorkspaceID,
			UserID:             project.OwnerUserID,
			Name:               project.Name,
			Prompt:             project.Prompt,
			Mode:               mode,
			PreviewPath:        project.PreviewPath,
			SandboxID:          project.SandboxID,
			CoverS3Path:        project.CoverS3Path,
			TopicID:            topicID,
			TemplateID:         project.TemplateID,
			ReviewStatus:       reviewStatus,
			ReviewNote:         project.ReviewNote,
			ReviewedBy:         project.ReviewedBy,
			ReviewedAt:         project.ReviewedAt,
			Status:             status,
			DefaultAgentRoleID: project.DefaultAgentRoleID,
			Base: daomodel.Base{
				CreatedAt: project.CreatedAt,
				UpdatedAt: project.UpdatedAt,
			},
		}
		if err := tx.Create(row).Error; err != nil {
			return err
		}
		return replaceProjectAgentRoles(tx, row.ID, project.AgentRoleIDs)
	})
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
	project := toProject(&row)
	roleIDs, err := loadProjectAgentRoles(s.db.WithContext(ctx), row.ID)
	if err != nil {
		return nil, err
	}
	project.AgentRoleIDs = roleIDs
	return project, nil
}

func (s *MySQLProjectStore) SetCover(ctx context.Context, id, key string) error {
	result := s.db.WithContext(ctx).
		Model(&daomodel.Project{}).
		Where("public_id = ?", id).
		Update("cover_s3_path", key)
	if result.Error != nil {
		return fmt.Errorf("update project cover: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLProjectStore) SetPreviewPath(ctx context.Context, id, previewPath string) error {
	result := s.db.WithContext(ctx).
		Model(&daomodel.Project{}).
		Where("public_id = ?", id).
		Update("preview_path", previewPath)
	if result.Error != nil {
		return fmt.Errorf("update project preview path: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLProjectStore) SetAgentRoles(ctx context.Context, id string, roleIDs []int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var projectID int64
		if err := tx.Table("project").Select("id").Where("public_id = ?", id).Take(&projectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		var defaultID *int64
		if len(roleIDs) > 0 {
			first := roleIDs[0]
			defaultID = &first
		}
		if err := tx.Table("project").Where("id = ?", projectID).
			Update("default_agent_role_id", defaultID).Error; err != nil {
			return err
		}
		return replaceProjectAgentRoles(tx, projectID, roleIDs)
	})
}

func replaceProjectAgentRoles(tx *gorm.DB, projectID int64, roleIDs []int64) error {
	if err := tx.Exec("DELETE FROM `project_agent_role` WHERE project_id = ?", projectID).Error; err != nil {
		return err
	}
	if len(roleIDs) == 0 {
		return nil
	}
	rows := make([]map[string]any, 0, len(roleIDs))
	for index, roleID := range roleIDs {
		rows = append(rows, map[string]any{
			"project_id": projectID, "agent_role_id": roleID, "sort_order": index,
		})
	}
	return tx.Table("project_agent_role").Create(&rows).Error
}

func loadProjectAgentRoles(tx *gorm.DB, projectID int64) ([]int64, error) {
	var rows []struct {
		AgentRoleID int64 `gorm:"column:agent_role_id"`
	}
	if err := tx.Table("project_agent_role").
		Select("agent_role_id").
		Where("project_id = ?", projectID).
		Order("sort_order ASC,id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AgentRoleID)
	}
	return ids, nil
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
			"name":          project.Name,
			"prompt":        project.Prompt,
			"preview_path":  project.PreviewPath,
			"sandbox_id":    project.SandboxID,
			"status":        project.State,
			"review_status": project.ReviewStatus,
			"review_note":   project.ReviewNote,
			"reviewed_by":   project.ReviewedBy,
			"reviewed_at":   project.ReviewedAt,
			"workspace_id":  project.WorkspaceID,
			"topic_id":      project.TopicID,
			"template_id":   project.TemplateID,
			"user_id":       project.OwnerUserID,
			"updated_at":    time.Now().UTC(),
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
	mode := row.Mode
	if mode == "" {
		mode = model.ProjectModeStandard
	}
	return &model.Project{
		InternalID:         row.ID,
		ID:                 row.PublicID,
		Name:               row.Name,
		Prompt:             row.Prompt,
		Mode:               mode,
		PreviewPath:        row.PreviewPath,
		State:              row.Status,
		ReviewStatus:       row.ReviewStatus,
		ReviewNote:         row.ReviewNote,
		ReviewedBy:         row.ReviewedBy,
		ReviewedAt:         row.ReviewedAt,
		WorkspaceID:        row.WorkspaceID,
		TopicID:            row.TopicID,
		TemplateID:         row.TemplateID,
		OwnerUserID:        row.UserID,
		SandboxID:          row.SandboxID,
		DefaultAgentRoleID: row.DefaultAgentRoleID,
		CoverS3Path:        row.CoverS3Path,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}
