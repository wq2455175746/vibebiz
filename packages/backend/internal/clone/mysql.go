package clone

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	daomodel "vibebiz/internal/dao/model"
)

type MySQLStore struct {
	db *gorm.DB
}

func NewMySQLStore(db *gorm.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func (s *MySQLStore) CreateSource(ctx context.Context, source Source) error {
	row := &daomodel.ProjectCloneSource{
		ProjectPublicID: source.ProjectID,
		SourceURL:       source.SourceURL,
		Status:          source.Status,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create clone source: %w", err)
	}
	return nil
}

func (s *MySQLStore) GetSource(ctx context.Context, projectID string) (Source, error) {
	var row daomodel.ProjectCloneSource
	if err := s.db.WithContext(ctx).Where("project_public_id = ?", projectID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Source{}, ErrNotFound
		}
		return Source{}, fmt.Errorf("get clone source: %w", err)
	}
	return Source{
		ProjectID: row.ProjectPublicID, SourceURL: row.SourceURL, Status: row.Status,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (s *MySQLStore) ListAssets(ctx context.Context, projectID string) ([]Asset, error) {
	var rows []daomodel.ProjectCloneAsset
	if err := s.db.WithContext(ctx).
		Where("project_public_id = ?", projectID).
		Order("sort_order ASC, id ASC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list clone assets: %w", err)
	}
	out := make([]Asset, 0, len(rows))
	for index := range rows {
		out = append(out, Asset{
			ProjectID: rows[index].ProjectPublicID, ResourceID: rows[index].ResourceID,
			Role: rows[index].Role, SortOrder: rows[index].SortOrder, CreatedAt: rows[index].CreatedAt,
		})
	}
	return out, nil
}

func (s *MySQLStore) CreateAsset(ctx context.Context, asset Asset) error {
	row := &daomodel.ProjectCloneAsset{
		ProjectPublicID: asset.ProjectID,
		ResourceID:      asset.ResourceID,
		Role:            asset.Role,
		SortOrder:       asset.SortOrder,
	}
	if err := s.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("create clone asset: %w", err)
	}
	return nil
}

func (s *MySQLStore) DeleteAsset(ctx context.Context, projectID, resourceID string) error {
	result := s.db.WithContext(ctx).Unscoped().
		Where("project_public_id = ? AND resource_id = ?", projectID, resourceID).
		Delete(&daomodel.ProjectCloneAsset{})
	if result.Error != nil {
		return fmt.Errorf("delete clone asset: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) SetCover(ctx context.Context, projectID, resourceID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&daomodel.ProjectCloneAsset{}).
			Where("project_public_id = ? AND role = ?", projectID, RoleCover).
			Update("role", RoleGallery).Error; err != nil {
			return fmt.Errorf("demote clone cover: %w", err)
		}
		result := tx.Model(&daomodel.ProjectCloneAsset{}).
			Where("project_public_id = ? AND resource_id = ?", projectID, resourceID).
			Update("role", RoleCover)
		if result.Error != nil {
			return fmt.Errorf("promote clone cover: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *MySQLStore) DeleteByProject(ctx context.Context, projectID string) error {
	if err := s.db.WithContext(ctx).Unscoped().
		Where("project_public_id = ?", projectID).
		Delete(&daomodel.ProjectCloneAsset{}).Error; err != nil {
		return fmt.Errorf("delete clone assets: %w", err)
	}
	if err := s.db.WithContext(ctx).Unscoped().
		Where("project_public_id = ?", projectID).
		Delete(&daomodel.ProjectCloneSource{}).Error; err != nil {
		return fmt.Errorf("delete clone source: %w", err)
	}
	return nil
}
