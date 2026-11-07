package model

import "time"

const (
	ChannelPackageStatusBuilding = "building"
	ChannelPackageStatusReady    = "ready"
	ChannelPackageStatusFailed   = "failed"
)

type ProjectChannelPackage struct {
	Base
	PublicID         string    `gorm:"column:public_id" json:"publicId"`
	ProjectPublicID  string    `gorm:"column:project_public_id" json:"projectPublicId"`
	WorkspaceID      int64     `gorm:"column:workspace_id" json:"workspaceId"`
	SourceType       string    `gorm:"column:source_type" json:"sourceType"`
	SourceResourceID *string   `gorm:"column:source_resource_id" json:"sourceResourceId,omitempty"`
	Channel          string    `gorm:"column:channel" json:"channel"`
	Status           string    `gorm:"column:status" json:"status"`
	Version          string    `gorm:"column:version" json:"version"`
	FilePath         string    `gorm:"column:file_path" json:"filePath"`
	FileSizeBytes    int64     `gorm:"column:file_size_bytes" json:"fileSizeBytes"`
	Manifest         JSON      `gorm:"column:manifest;type:json" json:"manifest,omitempty"`
	ErrorMessage     string    `gorm:"column:error_message" json:"errorMessage"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (ProjectChannelPackage) TableName() string { return "project_channel_package" }
