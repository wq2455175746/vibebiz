package model

import "time"

type ProjectTopic struct {
	Base
	TopicCode         string `gorm:"column:topic_code" json:"topicCode"`
	TopicName         string `gorm:"column:topic_name" json:"topicName"`
	Description       string `gorm:"column:description" json:"description"`
	PromptTemplate    string `gorm:"column:prompt_template;type:mediumtext" json:"promptTemplate"`
	Icon              string `gorm:"column:icon" json:"icon"`
	DefaultTemplateID *int64 `gorm:"column:default_template_id" json:"defaultTemplateId,omitempty"`
	IsDefault         int8   `gorm:"column:is_default" json:"isDefault"`
	SortOrder         int    `gorm:"column:sort_order" json:"sortOrder"`
	Status            int8   `gorm:"column:status" json:"status"`
	CreatedBy         string `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy         string `gorm:"column:updated_by" json:"updatedBy"`
}

func (ProjectTopic) TableName() string { return "project_topic" }

type ProjectTemplate struct {
	Base
	Scope          string `gorm:"column:scope" json:"scope"`
	WorkspaceID    int64  `gorm:"column:workspace_id" json:"workspaceId"`
	TemplateCode   string `gorm:"column:template_code" json:"templateCode"`
	TemplateName   string `gorm:"column:template_name" json:"templateName"`
	Description    string `gorm:"column:description" json:"description"`
	TopicID        int64  `gorm:"column:topic_id" json:"topicId"`
	TemplateS3Path string `gorm:"column:template_s3_path" json:"templateS3Path"`
	CoverURL       string `gorm:"column:cover_url" json:"coverUrl"`
	TemplateLabel  JSON   `gorm:"column:template_label;type:json" json:"templateLabel,omitempty"`
	FileTree       JSON   `gorm:"column:file_tree;type:json" json:"fileTree,omitempty"`
	TechStack      JSON   `gorm:"column:tech_stack;type:json" json:"techStack,omitempty"`
	Version        int    `gorm:"column:version" json:"version"`
	IsBuiltin      int8   `gorm:"column:is_builtin" json:"isBuiltin"`
	Status         int8   `gorm:"column:status" json:"status"`
	SortOrder      int    `gorm:"column:sort_order" json:"sortOrder"`
	CreatedBy      string `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy      string `gorm:"column:updated_by" json:"updatedBy"`
}

func (ProjectTemplate) TableName() string { return "project_template" }

type Project struct {
	Base
	PublicID           string     `gorm:"column:public_id" json:"publicId"`
	WorkspaceID        int64      `gorm:"column:workspace_id" json:"workspaceId"`
	UserID             int64      `gorm:"column:user_id" json:"userId"`
	Name               string     `gorm:"column:name" json:"name"`
	Prompt             string     `gorm:"column:prompt" json:"prompt"`
	Description        string     `gorm:"column:description" json:"description"`
	TopicID            int64      `gorm:"column:topic_id" json:"topicId"`
	TemplateID         *int64     `gorm:"column:template_id" json:"templateId,omitempty"`
	S3Path             string     `gorm:"column:s3_path" json:"s3Path"`
	LocalPath          string     `gorm:"column:local_path" json:"localPath"`
	SandboxID          *int64     `gorm:"column:sandbox_id" json:"sandboxId,omitempty"`
	ProjectStructure   JSON       `gorm:"column:project_structure;type:json" json:"projectStructure,omitempty"`
	IsPublic           int8       `gorm:"column:is_public" json:"isPublic"`
	Status             string     `gorm:"column:status" json:"status"`
	DefaultAgentRoleID *int64     `gorm:"column:default_agent_role_id" json:"defaultAgentRoleId,omitempty"`
	LastActiveAt       *time.Time `gorm:"column:last_active_at" json:"lastActiveAt,omitempty"`
	ArchivedAt         *time.Time `gorm:"column:archived_at" json:"archivedAt,omitempty"`
	CreatedBy          string     `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy          string     `gorm:"column:updated_by" json:"updatedBy"`
}

func (Project) TableName() string { return "project" }

type ProjectStorage struct {
	Base
	WorkspaceID    int64      `gorm:"column:workspace_id" json:"workspaceId"`
	ProjectID      int64      `gorm:"column:project_id" json:"projectId"`
	UserID         int64      `gorm:"column:user_id" json:"userId"`
	StoragePath    string     `gorm:"column:storage_path" json:"storagePath"`
	FileSizeBytes  int64      `gorm:"column:file_size_bytes" json:"fileSizeBytes"`
	FileCount      int64      `gorm:"column:file_count" json:"fileCount"`
	DirCount       int64      `gorm:"column:dir_count" json:"dirCount"`
	FileTree       JSON       `gorm:"column:file_tree;type:json" json:"fileTree,omitempty"`
	LastModifiedAt *time.Time `gorm:"column:last_modified_at" json:"lastModifiedAt,omitempty"`
	LastScannedAt  *time.Time `gorm:"column:last_scanned_at" json:"lastScannedAt,omitempty"`
	Status         int8       `gorm:"column:status" json:"status"`
}

func (ProjectStorage) TableName() string { return "project_storage" }
