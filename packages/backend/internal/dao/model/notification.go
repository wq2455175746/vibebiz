package model

import "time"

type NotificationType string

const (
	NotificationVersionRelease NotificationType = "version_release"
	NotificationInbox          NotificationType = "inbox"
)

type NotificationStatus string

const (
	NotificationDraft       NotificationStatus = "draft"
	NotificationPublished   NotificationStatus = "published"
	NotificationUnpublished NotificationStatus = "unpublished"
)

type OpsNotification struct {
	Base
	Title           string             `gorm:"column:title" json:"title"`
	Content         string             `gorm:"column:content" json:"content"`
	Type            NotificationType   `gorm:"column:type" json:"type"`
	RecipientUserID int64              `gorm:"column:recipient_user_id" json:"recipientUserId"`
	Version         string             `gorm:"column:version" json:"version"`
	Link            string             `gorm:"column:link" json:"link"`
	Status          NotificationStatus `gorm:"column:status" json:"status"`
	ExpireAt        time.Time          `gorm:"column:expire_at" json:"expireAt"`
	PublishedAt     *time.Time         `gorm:"column:published_at" json:"publishedAt,omitempty"`
	CreatedBy       int64              `gorm:"column:created_by" json:"createdBy"`
	CreatedByName   string             `gorm:"column:created_by_name" json:"createdByName"`
}

func (OpsNotification) TableName() string { return "ops_notification" }
