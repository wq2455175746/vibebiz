package model

import "time"

type ProjectLease struct {
	ProjectID  int64     `gorm:"column:project_id;primaryKey" json:"projectId"`
	SandboxID  int64     `gorm:"column:sandbox_id" json:"sandboxId"`
	LeaseUntil time.Time `gorm:"column:lease_until" json:"leaseUntil"`
	Epoch      int64     `gorm:"column:epoch" json:"epoch"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (ProjectLease) TableName() string { return "project_leases" }
