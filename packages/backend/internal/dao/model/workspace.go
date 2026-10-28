package model

import "time"

type Workspace struct {
	Base
	OwnerUserID   int64  `gorm:"column:owner_user_id" json:"ownerUserId"`
	Name          string `gorm:"column:name" json:"name"`
	Description   string `gorm:"column:description" json:"description"`
	WorkspaceType string `gorm:"column:workspace_type" json:"workspaceType"`
	Visibility    string `gorm:"column:visibility" json:"visibility"`
	S3Prefix      string `gorm:"column:s3_prefix" json:"s3Prefix"`
	LocalPath     string `gorm:"column:local_path" json:"localPath"`
	Status        int8   `gorm:"column:status" json:"status"`
	CreatedBy     string `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy     string `gorm:"column:updated_by" json:"updatedBy"`
}

func (Workspace) TableName() string { return "workspace" }

type WorkspaceUser struct {
	Base
	WorkspaceID  int64      `gorm:"column:workspace_id" json:"workspaceId"`
	UserID       int64      `gorm:"column:user_id" json:"userId"`
	Role         string     `gorm:"column:role" json:"role"`
	Status       int8       `gorm:"column:status" json:"status"`
	InvitedBy    *int64     `gorm:"column:invited_by" json:"invitedBy,omitempty"`
	JoinedAt     *time.Time `gorm:"column:joined_at" json:"joinedAt,omitempty"`
	LastAccessAt *time.Time `gorm:"column:last_access_at" json:"lastAccessAt,omitempty"`
}

func (WorkspaceUser) TableName() string { return "workspace_user" }

type UserQuota struct {
	Base
	UserID                   int64      `gorm:"column:user_id" json:"userId"`
	PlanCode                 string     `gorm:"column:plan_code" json:"planCode"`
	StorageQuotaBytes        int64      `gorm:"column:storage_quota_bytes" json:"storageQuotaBytes"`
	StorageUsedBytes         int64      `gorm:"column:storage_used_bytes" json:"storageUsedBytes"`
	ProjectQuota             int64      `gorm:"column:project_quota" json:"projectQuota"`
	ProjectUsed              int64      `gorm:"column:project_used" json:"projectUsed"`
	WorkspaceQuota           int64      `gorm:"column:workspace_quota" json:"workspaceQuota"`
	WorkspaceUsed            int64      `gorm:"column:workspace_used" json:"workspaceUsed"`
	LLMTokenQuotaMonth       int64      `gorm:"column:llm_token_quota_month" json:"llmTokenQuotaMonth"`
	LLMTokensUsedMonth       int64      `gorm:"column:llm_tokens_used_month" json:"llmTokensUsedMonth"`
	SandboxMinutesQuotaMonth int64      `gorm:"column:sandbox_minutes_quota_month" json:"sandboxMinutesQuotaMonth"`
	SandboxMinutesUsedMonth  int64      `gorm:"column:sandbox_minutes_used_month" json:"sandboxMinutesUsedMonth"`
	ConcurrentAgentRuns      int64      `gorm:"column:concurrent_agent_runs" json:"concurrentAgentRuns"`
	ResetAt                  *time.Time `gorm:"column:reset_at" json:"resetAt,omitempty"`
	Status                   int8       `gorm:"column:status" json:"status"`
}

func (UserQuota) TableName() string { return "user_quota" }
