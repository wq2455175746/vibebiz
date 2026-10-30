package model

import "time"

const (
	ProjectStageLocked      = "locked"
	ProjectStageActive      = "active"
	ProjectStageDone        = "done"
	ProjectStagePlaceholder = "placeholder"
)

type ProjectStage struct {
	InternalID   int64      `json:"-"`
	ProjectID    string     `json:"projectId"`
	StageKey     string     `json:"stageKey"`
	Seq          int        `json:"seq"`
	Status       string     `json:"status"`
	ArtifactPath string     `json:"artifactPath"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}
