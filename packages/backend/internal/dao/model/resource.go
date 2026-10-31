package model

import (
	"time"

	"gorm.io/gorm"
)

type ExternalResource struct {
	Base
	WorkspaceID      int64      `gorm:"column:workspace_id" json:"workspaceId"`
	UserID           int64      `gorm:"column:user_id" json:"userId"`
	Name             string     `gorm:"column:name" json:"name"`
	ResourceType     string     `gorm:"column:resource_type" json:"resourceType"`
	ResourceURL      string     `gorm:"column:resource_url" json:"resourceUrl"`
	AuthType         string     `gorm:"column:auth_type" json:"authType"`
	CredentialCipher string     `gorm:"column:credential_cipher;type:text" json:"-"`
	ExtraConfigs     JSON       `gorm:"column:extra_configs;type:json" json:"extraConfigs,omitempty"`
	SyncStatus       string     `gorm:"column:sync_status" json:"syncStatus"`
	LastSyncAt       *time.Time `gorm:"column:last_sync_at" json:"lastSyncAt,omitempty"`
	LastError        string     `gorm:"column:last_error" json:"lastError"`
	Status           int8       `gorm:"column:status" json:"status"`
	CreatedBy        string     `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy        string     `gorm:"column:updated_by" json:"updatedBy"`
}

func (ExternalResource) TableName() string { return "external_resource" }

type ProjectExternalResource struct {
	Base
	WorkspaceID int64  `gorm:"column:workspace_id" json:"workspaceId"`
	ProjectID   int64  `gorm:"column:project_id" json:"projectId"`
	ResourceID  int64  `gorm:"column:resource_id" json:"resourceId"`
	CreatedBy   string `gorm:"column:created_by" json:"createdBy"`
}

func (ProjectExternalResource) TableName() string { return "project_external_resource" }

type ProjectResource struct {
	ID              int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ResourceID      string         `gorm:"column:resource_id" json:"resourceId"`
	ProjectPublicID *string        `gorm:"column:project_public_id" json:"projectPublicId,omitempty"`
	WorkspaceID     int64          `gorm:"column:workspace_id" json:"workspaceId"`
	UserID          int64          `gorm:"column:user_id" json:"userId"`
	OriginalName    string         `gorm:"column:original_name" json:"originalName"`
	DisplayName     string         `gorm:"column:display_name" json:"displayName"`
	StorageName     string         `gorm:"column:storage_name" json:"storageName"`
	ContentType     string         `gorm:"column:content_type" json:"contentType"`
	SizeBytes       int64          `gorm:"column:size_bytes" json:"sizeBytes"`
	Kind            string         `gorm:"column:kind" json:"kind"`
	Purpose         string         `gorm:"column:upload_purpose" json:"purpose"`
	S3StagingKey    string         `gorm:"column:s3_staging_key" json:"s3StagingKey"`
	S3Key           string         `gorm:"column:s3_key" json:"s3Key"`
	Status          string         `gorm:"column:status" json:"status"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (ProjectResource) TableName() string { return "project_resource" }
