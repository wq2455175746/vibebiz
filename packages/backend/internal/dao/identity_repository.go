package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type IdentityRepository struct {
	db *gorm.DB
}

func NewIdentityRepository(db *gorm.DB) *IdentityRepository {
	return &IdentityRepository{db: db}
}

func (r *IdentityRepository) FindUserByIdentifier(ctx context.Context, identifier string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("(username = ? OR email = ?)", identifier, identifier).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user: %w", err)
	}
	return &user, nil
}

func (r *IdentityRepository) FindOpsByIdentifier(ctx context.Context, identifier string) (*model.OpsUser, error) {
	var user model.OpsUser
	err := r.db.WithContext(ctx).
		Where("(username = ? OR email = ?)", identifier, identifier).
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find ops user: %w", err)
	}
	return &user, nil
}

func (r *IdentityRepository) WorkspaceIDs(ctx context.Context, userID int64) ([]int64, error) {
	ids := make([]int64, 0)
	err := r.db.WithContext(ctx).
		Table("workspace_user").
		Select("workspace_user.workspace_id").
		Joins("JOIN workspace ON workspace.id = workspace_user.workspace_id").
		Where("workspace_user.user_id = ? AND workspace_user.status = 1 AND workspace_user.deleted_at IS NULL", userID).
		Where("workspace.status = 1 AND workspace.deleted_at IS NULL").
		Order("workspace_user.workspace_id ASC").
		Pluck("workspace_user.workspace_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("workspace ids: %w", err)
	}
	return ids, nil
}

func (r *IdentityRepository) Workspaces(ctx context.Context, userID int64) ([]WorkspaceMembership, error) {
	memberships := make([]WorkspaceMembership, 0)
	err := r.db.WithContext(ctx).
		Table("workspace_user").
		Select("workspace.id, workspace.name, workspace.owner_user_id, workspace_user.role").
		Joins("JOIN workspace ON workspace.id = workspace_user.workspace_id").
		Where("workspace_user.user_id = ? AND workspace_user.status = 1 AND workspace_user.deleted_at IS NULL", userID).
		Where("workspace.status = 1 AND workspace.deleted_at IS NULL").
		Order("workspace.id ASC").
		Scan(&memberships).Error
	if err != nil {
		return nil, fmt.Errorf("workspaces: %w", err)
	}
	return memberships, nil
}

func (r *IdentityRepository) TouchUserLogin(ctx context.Context, id int64, ip string, at time.Time) error {
	return r.touchLogin(ctx, &model.User{}, id, ip, at)
}

func (r *IdentityRepository) TouchOpsLogin(ctx context.Context, id int64, ip string, at time.Time) error {
	return r.touchLogin(ctx, &model.OpsUser{}, id, ip, at)
}

func (r *IdentityRepository) touchLogin(ctx context.Context, entity any, id int64, ip string, at time.Time) error {
	err := r.db.WithContext(ctx).
		Model(entity).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_login_time": at,
			"last_login_ip":   ip,
		}).Error
	if err != nil {
		return fmt.Errorf("touch login: %w", err)
	}
	return nil
}
