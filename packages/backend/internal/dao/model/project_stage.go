package model

import "time"

type ProjectStage struct {
	ID           int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ProjectID    string     `gorm:"column:project_id" json:"projectId"`
	StageKey     string     `gorm:"column:stage_key" json:"stageKey"`
	Seq          int        `gorm:"column:seq" json:"seq"`
	Status       string     `gorm:"column:status" json:"status"`
	ArtifactPath string     `gorm:"column:artifact_path" json:"artifactPath"`
	StartedAt    *time.Time `gorm:"column:started_at" json:"startedAt,omitempty"`
	CompletedAt  *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (ProjectStage) TableName() string { return "project_stage" }
