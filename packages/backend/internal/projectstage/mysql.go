package projectstage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
)

type MySQLStore struct {
	db *gorm.DB
}

func NewMySQLStore(db *gorm.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) CreateInitial(ctx context.Context, projectID string) error {
	return s.CreateInitialForMode(ctx, projectID, model.ProjectModeEnterprise)
}

func (s *MySQLStore) CreateInitialForMode(ctx context.Context, projectID, mode string) error {
	if mode == "" {
		mode = model.ProjectModeEnterprise
	}
	stages := InitialStagesForMode(projectID, mode)
	rows := make([]daomodel.ProjectStage, 0, len(stages))
	for _, stage := range stages {
		rows = append(rows, daomodel.ProjectStage{
			ProjectID: stage.ProjectID, StageKey: stage.StageKey, Seq: stage.Seq,
			Status: stage.Status, ArtifactPath: stage.ArtifactPath,
		})
	}
	if err := s.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return fmt.Errorf("create project stages: %w", err)
	}
	return nil
}

func (s *MySQLStore) DeleteByProject(ctx context.Context, projectID string) error {
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Delete(&daomodel.ProjectStage{}).Error; err != nil {
		return fmt.Errorf("delete project stages: %w", err)
	}
	return nil
}

func (s *MySQLStore) List(ctx context.Context, projectID string) ([]model.ProjectStage, error) {
	var rows []daomodel.ProjectStage
	if err := s.db.WithContext(ctx).Where("project_id = ?", projectID).Order("seq ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list project stages: %w", err)
	}
	out := make([]model.ProjectStage, 0, len(rows))
	for index := range rows {
		out = append(out, toStage(&rows[index]))
	}
	return out, nil
}

func (s *MySQLStore) Get(ctx context.Context, projectID, stageKey string) (*model.ProjectStage, error) {
	var row daomodel.ProjectStage
	err := s.db.WithContext(ctx).Where("project_id = ? AND stage_key = ?", projectID, stageKey).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrStageNotFound
		}
		return nil, fmt.Errorf("get project stage: %w", err)
	}
	stage := toStage(&row)
	return &stage, nil
}

func (s *MySQLStore) Start(ctx context.Context, projectID, stageKey string, now time.Time) (bool, error) {
	result := s.db.WithContext(ctx).
		Model(&daomodel.ProjectStage{}).
		Where("project_id = ? AND stage_key = ? AND status = ? AND started_at IS NULL", projectID, stageKey, model.ProjectStageActive).
		Update("started_at", now)
	if result.Error != nil {
		return false, fmt.Errorf("start project stage: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (s *MySQLStore) Complete(ctx context.Context, projectID, stageKey string, now time.Time) (string, error) {
	nextKey := ""
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current daomodel.ProjectStage
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("project_id = ? AND stage_key = ?", projectID, stageKey).
			Take(&current).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrStageNotFound
			}
			return err
		}
		if current.Status != model.ProjectStageActive {
			return ErrStageConflict
		}
		if err := tx.Model(&daomodel.ProjectStage{}).
			Where("id = ?", current.ID).
			Updates(map[string]any{"status": model.ProjectStageDone, "completed_at": now}).Error; err != nil {
			return err
		}

		var next daomodel.ProjectStage
		err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("project_id = ? AND seq > ? AND status = ?", projectID, current.Seq, model.ProjectStageLocked).
			Order("seq ASC").Take(&next).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := tx.Model(&daomodel.ProjectStage{}).
			Where("id = ?", next.ID).
			Updates(map[string]any{"status": model.ProjectStageActive, "started_at": nil}).Error; err != nil {
			return err
		}
		nextKey = next.StageKey
		return nil
	})
	if err != nil {
		return "", err
	}
	return nextKey, nil
}

func toStage(row *daomodel.ProjectStage) model.ProjectStage {
	return model.ProjectStage{
		InternalID: row.ID, ProjectID: row.ProjectID, StageKey: row.StageKey, Seq: row.Seq,
		Status: row.Status, ArtifactPath: row.ArtifactPath, StartedAt: row.StartedAt, CompletedAt: row.CompletedAt,
	}
}
