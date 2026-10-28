package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type WorkspaceMembership struct {
	ID          int64
	Name        string
	OwnerUserID int64
	Role        string
}

type UserProfile struct {
	ID           int64
	Username     string
	Email        string
	AvatarS3Path string
}

type WorkspaceDetail struct {
	ID           int64
	OwnerUserID  int64
	Name         string
	Description  string
	AvatarURL    string
	AvatarS3Path string
	MemberCount  int64
}

type WorkspaceMember struct {
	UserID       int64
	Username     string
	Email        string
	AvatarS3Path string
	Role         string
	Status       int8
	JoinedAt     *time.Time
}

type UserQuotaRecord struct {
	StorageQuotaBytes int64
	StorageUsedBytes  int64
}

type WorkspaceFolderRecord struct {
	ID          int64
	WorkspaceID int64
	ParentID    int64
	Name        string
	CreatedAt   time.Time
}

type SettingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) UserProfile(ctx context.Context, userID int64) (*UserProfile, error) {
	var profile UserProfile
	err := r.db.WithContext(ctx).
		Table("user").
		Select("id, username, email, avatar_s3_path").
		Where("id = ?", userID).
		Take(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user profile: %w", err)
	}
	return &profile, nil
}

func (r *SettingsRepository) FindUserByEmail(ctx context.Context, email string) (*UserProfile, error) {
	var profile UserProfile
	err := r.db.WithContext(ctx).
		Table("user").
		Select("id, username, email, avatar_s3_path").
		Where("email = ?", email).
		Take(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}
	return &profile, nil
}

func (r *SettingsRepository) UsernameExists(ctx context.Context, username string, excludeUserID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("username = ? AND id <> ?", username, excludeUserID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("count username: %w", err)
	}
	return count > 0, nil
}

func (r *SettingsRepository) UpdateUsername(ctx context.Context, userID int64, username string) error {
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("username", username).Error
	if err != nil {
		return fmt.Errorf("update username: %w", err)
	}
	return nil
}

func (r *SettingsRepository) UpdateUserAvatar(ctx context.Context, userID int64, key string) error {
	err := r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("avatar_s3_path", key).Error
	if err != nil {
		return fmt.Errorf("update user avatar: %w", err)
	}
	return nil
}

func (r *SettingsRepository) Workspaces(ctx context.Context, userID int64) ([]WorkspaceMembership, error) {
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
		return nil, fmt.Errorf("workspace memberships: %w", err)
	}
	return memberships, nil
}

func (r *SettingsRepository) Workspace(ctx context.Context, workspaceID int64) (*WorkspaceDetail, error) {
	var detail WorkspaceDetail
	err := r.db.WithContext(ctx).
		Table("workspace").
		Select("id, owner_user_id, name, description, avatar_url, avatar_s3_path").
		Where("id = ?", workspaceID).
		Take(&detail).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find workspace: %w", err)
	}
	if err := r.db.WithContext(ctx).
		Model(&model.WorkspaceUser{}).
		Where("workspace_id = ? AND status = 1", workspaceID).
		Count(&detail.MemberCount).Error; err != nil {
		return nil, fmt.Errorf("count workspace members: %w", err)
	}
	return &detail, nil
}

func (r *SettingsRepository) UpdateWorkspace(ctx context.Context, workspaceID int64, name, description string) error {
	err := r.db.WithContext(ctx).
		Model(&model.Workspace{}).
		Where("id = ?", workspaceID).
		Updates(map[string]any{"name": name, "description": description}).Error
	if err != nil {
		return fmt.Errorf("update workspace: %w", err)
	}
	return nil
}

func (r *SettingsRepository) UpdateWorkspaceAvatar(ctx context.Context, workspaceID int64, key string) error {
	err := r.db.WithContext(ctx).
		Model(&model.Workspace{}).
		Where("id = ?", workspaceID).
		Update("avatar_s3_path", key).Error
	if err != nil {
		return fmt.Errorf("update workspace avatar: %w", err)
	}
	return nil
}

func (r *SettingsRepository) Members(ctx context.Context, workspaceID int64) ([]WorkspaceMember, error) {
	members := make([]WorkspaceMember, 0)
	err := r.db.WithContext(ctx).
		Table("workspace_user").
		Select("user.id AS user_id, user.username, user.email, user.avatar_s3_path, workspace_user.role, workspace_user.status, workspace_user.joined_at").
		Joins("JOIN user ON user.id = workspace_user.user_id").
		Where("workspace_user.workspace_id = ? AND workspace_user.deleted_at IS NULL", workspaceID).
		Where("user.deleted_at IS NULL").
		Order("workspace_user.role = 'owner' DESC, workspace_user.joined_at ASC").
		Scan(&members).Error
	if err != nil {
		return nil, fmt.Errorf("workspace members: %w", err)
	}
	return members, nil
}

func (r *SettingsRepository) IsWorkspaceMember(ctx context.Context, workspaceID, userID int64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WorkspaceUser{}).
		Where("workspace_id = ? AND user_id = ? AND status = 1", workspaceID, userID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("check workspace member: %w", err)
	}
	return count > 0, nil
}

func (r *SettingsRepository) AddWorkspaceMember(ctx context.Context, workspaceID, userID int64, role string, invitedBy int64, joinedAt time.Time) error {
	member := &model.WorkspaceUser{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        role,
		Status:      1,
		InvitedBy:   &invitedBy,
		JoinedAt:    &joinedAt,
	}
	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		return fmt.Errorf("add workspace member: %w", err)
	}
	return nil
}

func (r *SettingsRepository) Folders(ctx context.Context, workspaceID int64) ([]WorkspaceFolderRecord, error) {
	folders := make([]WorkspaceFolderRecord, 0)
	err := r.db.WithContext(ctx).
		Table("workspace_folder").
		Select("id, workspace_id, parent_id, name, created_at").
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Order("parent_id ASC, name ASC").
		Scan(&folders).Error
	if err != nil {
		return nil, fmt.Errorf("workspace folders: %w", err)
	}
	return folders, nil
}

func (r *SettingsRepository) Folder(ctx context.Context, workspaceID, folderID int64) (*WorkspaceFolderRecord, error) {
	var folder WorkspaceFolderRecord
	err := r.db.WithContext(ctx).
		Table("workspace_folder").
		Select("id, workspace_id, parent_id, name, created_at").
		Where("id = ? AND workspace_id = ? AND deleted_at IS NULL", folderID, workspaceID).
		Take(&folder).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find workspace folder: %w", err)
	}
	return &folder, nil
}

func (r *SettingsRepository) FolderNameExists(ctx context.Context, workspaceID, parentID int64, name string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&model.WorkspaceFolder{}).
		Where("workspace_id = ? AND parent_id = ? AND name = ?", workspaceID, parentID, name).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("count workspace folder: %w", err)
	}
	return count > 0, nil
}

func (r *SettingsRepository) CreateFolder(ctx context.Context, workspaceID, parentID int64, name string, createdBy int64) (*WorkspaceFolderRecord, error) {
	folder := &model.WorkspaceFolder{
		WorkspaceID: workspaceID,
		ParentID:    parentID,
		Name:        name,
		CreatedBy:   createdBy,
	}
	if err := r.db.WithContext(ctx).Create(folder).Error; err != nil {
		return nil, fmt.Errorf("create workspace folder: %w", err)
	}
	return &WorkspaceFolderRecord{
		ID:          folder.ID,
		WorkspaceID: folder.WorkspaceID,
		ParentID:    folder.ParentID,
		Name:        folder.Name,
		CreatedAt:   folder.CreatedAt,
	}, nil
}

func (r *SettingsRepository) UserQuota(ctx context.Context, userID int64) (*UserQuotaRecord, error) {
	var quota UserQuotaRecord
	err := r.db.WithContext(ctx).
		Table("user_quota").
		Select("storage_quota_bytes, storage_used_bytes").
		Where("user_id = ?", userID).
		Take(&quota).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("find user quota: %w", err)
	}
	return &quota, nil
}

func (r *SettingsRepository) UpdateStorageUsed(ctx context.Context, userID int64, used int64) error {
	err := r.db.WithContext(ctx).
		Model(&model.UserQuota{}).
		Where("user_id = ?", userID).
		Update("storage_used_bytes", used).Error
	if err != nil {
		return fmt.Errorf("update storage used: %w", err)
	}
	return nil
}
