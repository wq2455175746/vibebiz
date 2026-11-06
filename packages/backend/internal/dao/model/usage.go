package model

import "time"

type UserUsageDaily struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID          int64     `gorm:"column:user_id" json:"userId"`
	UsageDate       time.Time `gorm:"column:usage_date;type:date" json:"usageDate"`
	ProjectsCreated int64     `gorm:"column:projects_created" json:"projectsCreated"`
	InputTokens     int64     `gorm:"column:input_tokens" json:"inputTokens"`
	OutputTokens    int64     `gorm:"column:output_tokens" json:"outputTokens"`
	ModelRequests   int64     `gorm:"column:model_requests" json:"modelRequests"`
	UploadCount     int64     `gorm:"column:upload_count" json:"uploadCount"`
	UploadBytes     int64     `gorm:"column:upload_bytes" json:"uploadBytes"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (UserUsageDaily) TableName() string { return "user_usage_daily" }
