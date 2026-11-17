package model

import "time"

type BusinessCardComment struct {
	Base
	PublicID   string     `gorm:"column:public_id" json:"publicId"`
	CardID     int64      `gorm:"column:card_id" json:"cardId"`
	UserID     int64      `gorm:"column:user_id" json:"userId"`
	ParentID   int64      `gorm:"column:parent_id" json:"parentId"`
	RootID     int64      `gorm:"column:root_id" json:"rootId"`
	Depth      uint8      `gorm:"column:depth" json:"depth"`
	Content    string     `gorm:"column:content" json:"content"`
	Status     int8       `gorm:"column:status" json:"status"`
	EditedAt   *time.Time `gorm:"column:edited_at" json:"editedAt,omitempty"`
	ReplyCount int        `gorm:"column:reply_count" json:"replyCount"`
}

func (BusinessCardComment) TableName() string { return "business_card_comment" }
