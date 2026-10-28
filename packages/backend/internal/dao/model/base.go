package model

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

type JSON = json.RawMessage

type Base struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	ExtraP    JSON           `gorm:"column:extra_p;type:json" json:"extraP,omitempty"`
}
