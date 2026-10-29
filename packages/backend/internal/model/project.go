package model

import "time"

const (
	ProjectStateReady = "ready"
	ProjectStateError = "error"
)

type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Prompt      string    `json:"prompt"`
	State       string    `json:"state"`
	WorkspaceID int64     `json:"workspaceId"`
	OwnerUserID int64     `json:"ownerUserId"`
	SandboxID   *int64    `json:"sandboxId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
