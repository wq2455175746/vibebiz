package model

type WorkspaceFolder struct {
	Base
	WorkspaceID int64  `gorm:"column:workspace_id" json:"workspaceId"`
	ParentID    int64  `gorm:"column:parent_id" json:"parentId"`
	Name        string `gorm:"column:name" json:"name"`
	CreatedBy   int64  `gorm:"column:created_by" json:"createdBy"`
}

func (WorkspaceFolder) TableName() string { return "workspace_folder" }
