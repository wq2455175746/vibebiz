package evolution

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type MySQLStore struct {
	db *gorm.DB
}

func NewMySQLStore(db *gorm.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func notFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *MySQLStore) ListVersions(ctx context.Context) ([]model.EvolutionVersion, error) {
	items := make([]model.EvolutionVersion, 0)
	err := s.db.WithContext(ctx).Order("updated_at DESC, id DESC").Find(&items).Error
	return items, err
}

func (s *MySQLStore) GetVersion(ctx context.Context, publicID string) (*model.EvolutionVersion, error) {
	var item model.EvolutionVersion
	if err := s.db.WithContext(ctx).Where("public_id = ?", publicID).First(&item).Error; err != nil {
		return nil, notFound(err)
	}
	return &item, nil
}

func (s *MySQLStore) CreateVersion(ctx context.Context, version *model.EvolutionVersion) error {
	return s.db.WithContext(ctx).Create(version).Error
}

func (s *MySQLStore) UpdateVersionStatus(ctx context.Context, publicID, status string) error {
	result := s.db.WithContext(ctx).Model(&model.EvolutionVersion{}).
		Where("public_id = ?", publicID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *MySQLStore) ListProposals(ctx context.Context, userID string) ([]model.EvolutionProposal, error) {
	items := make([]model.EvolutionProposal, 0)
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC, id DESC").
		Find(&items).Error
	return items, err
}

func (s *MySQLStore) ListAllProposals(ctx context.Context) ([]model.EvolutionProposal, error) {
	items := make([]model.EvolutionProposal, 0)
	err := s.db.WithContext(ctx).Order("created_at DESC, id DESC").Find(&items).Error
	return items, err
}

func (s *MySQLStore) CreateProposal(ctx context.Context, proposal *model.EvolutionProposal) error {
	return s.db.WithContext(ctx).Create(proposal).Error
}

func (s *MySQLStore) AcceptProposalsByVersion(ctx context.Context, versionID string) error {
	return s.db.WithContext(ctx).Model(&model.EvolutionProposal{}).
		Where("version_public_id = ?", versionID).
		Update("status", ProposalAccepted).Error
}

func (s *MySQLStore) ListMessages(ctx context.Context, userID string) ([]model.EvolutionMessage, error) {
	items := make([]model.EvolutionMessage, 0)
	err := s.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at ASC, id ASC").
		Find(&items).Error
	return items, err
}

func (s *MySQLStore) CreateMessage(ctx context.Context, message *model.EvolutionMessage) error {
	return s.db.WithContext(ctx).Create(message).Error
}

func (s *MySQLStore) ListTasks(ctx context.Context) ([]model.EvolutionTask, error) {
	items := make([]model.EvolutionTask, 0)
	err := s.db.WithContext(ctx).Order("created_at DESC, id DESC").Find(&items).Error
	return items, err
}

func (s *MySQLStore) GetTask(ctx context.Context, publicID string) (*model.EvolutionTask, error) {
	var item model.EvolutionTask
	if err := s.db.WithContext(ctx).Where("public_id = ?", publicID).First(&item).Error; err != nil {
		return nil, notFound(err)
	}
	return &item, nil
}

func (s *MySQLStore) GetTaskByVersion(ctx context.Context, versionID string) (*model.EvolutionTask, error) {
	var item model.EvolutionTask
	if err := s.db.WithContext(ctx).Where("version_public_id = ?", versionID).First(&item).Error; err != nil {
		return nil, notFound(err)
	}
	return &item, nil
}

func (s *MySQLStore) CreateTask(ctx context.Context, task *model.EvolutionTask) error {
	return s.db.WithContext(ctx).Create(task).Error
}

func (s *MySQLStore) UpdateTaskStatus(ctx context.Context, publicID, status string) error {
	result := s.db.WithContext(ctx).Model(&model.EvolutionTask{}).
		Where("public_id = ?", publicID).
		Update("status", status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
