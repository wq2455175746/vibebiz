package model

import "time"

type FdeResource struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name" json:"name"`
	Contact   string    `gorm:"column:contact" json:"contact"`
	Skills    JSON      `gorm:"column:skills;type:json" json:"skills"`
	Region    string    `gorm:"column:region" json:"region"`
	Note      string    `gorm:"column:note" json:"note"`
	Enabled   bool      `gorm:"column:enabled" json:"enabled"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time `gorm:"column:updated_at" json:"updatedAt"`
}

func (FdeResource) TableName() string { return "fde_resource" }
