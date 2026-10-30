package model

import "time"

const (
	ProjectStateReady = "ready"
	ProjectStateError = "error"
)

type Project struct {
	InternalID         int64     `json:"-"`
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Prompt             string    `json:"prompt"`
	State              string    `json:"state"`
	WorkspaceID        int64     `json:"workspaceId"`
	OwnerUserID        int64     `json:"ownerUserId"`
	SandboxID          *int64    `json:"sandboxId,omitempty"`
	DefaultAgentRoleID *int64    `json:"defaultAgentRoleId,omitempty"`
	AgentRoleIDs       []int64   `json:"agentRoleIds"`
	CoverS3Path        string    `json:"coverS3Path,omitempty"`
	CoverURL           string    `json:"coverUrl,omitempty"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}
