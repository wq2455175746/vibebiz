package model

import "time"

type BusinessCard struct {
	Base
	PublicID                string     `gorm:"column:public_id" json:"publicId"`
	UserID                  int64      `gorm:"column:user_id" json:"userId"`
	Headline                string     `gorm:"column:headline" json:"headline"`
	Tags                    JSON       `gorm:"column:tags;type:json" json:"tags,omitempty"`
	FeaturedType            string     `gorm:"column:featured_type" json:"featuredType"`
	FeaturedProjectPublicID string     `gorm:"column:featured_project_public_id" json:"featuredProjectPublicId"`
	FeaturedS3Key           string     `gorm:"column:featured_s3_key" json:"featuredS3Key"`
	FeaturedFileName        string     `gorm:"column:featured_file_name" json:"featuredFileName"`
	FeaturedFileSize        int64      `gorm:"column:featured_file_size" json:"featuredFileSize"`
	Published               int8       `gorm:"column:published" json:"published"`
	PublishedAt             *time.Time `gorm:"column:published_at" json:"publishedAt,omitempty"`
	CommentCount            int        `gorm:"column:comment_count" json:"commentCount"`
	IsHidden                int8       `gorm:"column:is_hidden" json:"hidden"`
}

func (BusinessCard) TableName() string { return "business_card" }

type BusinessCardItem struct {
	Base
	CardID          int64  `gorm:"column:card_id" json:"cardId"`
	ProjectPublicID string `gorm:"column:project_public_id" json:"projectPublicId"`
	SortOrder       int    `gorm:"column:sort_order" json:"sortOrder"`
}

func (BusinessCardItem) TableName() string { return "business_card_item" }
