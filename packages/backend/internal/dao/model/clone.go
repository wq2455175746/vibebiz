package model

type ProjectCloneSource struct {
	Base
	ProjectPublicID string `gorm:"column:project_public_id" json:"projectPublicId"`
	SourceURL       string `gorm:"column:source_url" json:"sourceUrl"`
	Status          string `gorm:"column:status" json:"status"`
}

func (ProjectCloneSource) TableName() string { return "project_clone_source" }

type ProjectCloneAsset struct {
	Base
	ProjectPublicID string `gorm:"column:project_public_id" json:"projectPublicId"`
	ResourceID      string `gorm:"column:resource_id" json:"resourceId"`
	Role            string `gorm:"column:role" json:"role"`
	SortOrder       int    `gorm:"column:sort_order" json:"sortOrder"`
}

func (ProjectCloneAsset) TableName() string { return "project_clone_asset" }
