package model

import "time"

type SandboxInstance struct {
	Base
	PublicID        string     `gorm:"column:public_id" json:"publicId"`
	Name            string     `gorm:"column:name" json:"name"`
	Scheme          string     `gorm:"column:scheme" json:"scheme"`
	Host            string     `gorm:"column:host" json:"host"`
	Port            int        `gorm:"column:port" json:"port"`
	Status          string     `gorm:"column:status" json:"status"`
	HealthStatus    string     `gorm:"column:health_status" json:"healthStatus"`
	ActiveRequests  int        `gorm:"column:active_requests" json:"activeRequests"`
	MaxConcurrency  int        `gorm:"column:max_concurrency" json:"maxConcurrency"`
	LastHeartbeatAt *time.Time `gorm:"column:last_heartbeat_at" json:"lastHeartbeatAt,omitempty"`
	LastHealthAt    *time.Time `gorm:"column:last_health_at" json:"lastHealthAt,omitempty"`
	HealthError     string     `gorm:"column:health_error" json:"healthError"`
	LastSelectedAt  *time.Time `gorm:"column:last_selected_at" json:"lastSelectedAt,omitempty"`
}

func (SandboxInstance) TableName() string { return "sandbox_instance" }
