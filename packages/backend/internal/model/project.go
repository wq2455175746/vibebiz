package model

import "time"

const (
	ProjectModeStandard   = "standard"
	ProjectModeEnterprise = "enterprise"
	ProjectModeClone      = "clone"
)

const (
	ProjectStateReady    = "ready"
	ProjectStateError    = "error"
	ProjectStateDisabled = "disabled"
)

const (
	ReviewStatusPending  = "pending"
	ReviewStatusApproved = "approved"
	ReviewStatusRejected = "rejected"
)

type Project struct {
	InternalID         int64      `json:"-"`
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Prompt             string     `json:"prompt"`
	Mode               string     `json:"mode"`
	PreviewPath        string     `json:"previewPath"`
	State              string     `json:"state"`
	ReviewStatus       string     `json:"reviewStatus"`
	ReviewNote         string     `json:"reviewNote"`
	ReviewedBy         string     `json:"reviewedBy"`
	ReviewedAt         *time.Time `json:"reviewedAt,omitempty"`
	WorkspaceID        int64      `json:"workspaceId"`
	TopicID            int64      `json:"topicId"`
	TemplateID         *int64     `json:"templateId,omitempty"`
	OwnerUserID        int64      `json:"ownerUserId"`
	SandboxID          *int64     `json:"sandboxId,omitempty"`
	DefaultAgentRoleID *int64     `json:"defaultAgentRoleId,omitempty"`
	AgentRoleIDs       []int64    `json:"agentRoleIds"`
	CoverS3Path        string     `json:"coverS3Path,omitempty"`
	CoverURL           string     `json:"coverUrl,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}
