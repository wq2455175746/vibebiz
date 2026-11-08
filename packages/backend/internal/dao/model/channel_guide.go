package model

import "time"

type ChannelGuide struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Channel      string    `gorm:"column:channel" json:"channel"`
	Title        string    `gorm:"column:title" json:"title"`
	Summary      string    `gorm:"column:summary" json:"summary"`
	Markdown     string    `gorm:"column:markdown" json:"markdown"`
	Steps        JSON      `gorm:"column:steps;type:json" json:"steps"`
	Requirements JSON      `gorm:"column:requirements;type:json" json:"requirements"`
	Enabled      bool      `gorm:"column:enabled" json:"enabled"`
	Builtin      bool      `gorm:"column:builtin" json:"builtin"`
	SortOrder    int       `gorm:"column:sort_order" json:"sortOrder"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (ChannelGuide) TableName() string { return "channel_guide" }
