package model

import "time"

type NotificationChannel struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID         int64      `gorm:"column:user_id" json:"userId"`
	Name           string     `gorm:"column:name" json:"name"`
	Type           string     `gorm:"column:type" json:"type"`
	ConfigCipher   string     `gorm:"column:config_cipher" json:"-"`
	Enabled        bool       `gorm:"column:enabled" json:"enabled"`
	LastTestAt     *time.Time `gorm:"column:last_test_at" json:"lastTestAt,omitempty"`
	LastTestStatus string     `gorm:"column:last_test_status" json:"lastTestStatus"`
	LastTestError  string     `gorm:"column:last_test_error" json:"lastTestError"`
	CreatedAt      time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      time.Time  `gorm:"column:updated_at" json:"updatedAt"`
}

func (NotificationChannel) TableName() string { return "notification_channel" }
