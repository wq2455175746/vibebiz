package model

import (
	"time"

	"gorm.io/gorm"
)

type EvolutionVersion struct {
	ID            int64          `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PublicID      string         `gorm:"column:public_id" json:"id"`
	UserID        string         `gorm:"column:user_id" json:"userId"`
	UserName      string         `gorm:"column:user_name" json:"userName"`
	Label         string         `gorm:"column:label" json:"label"`
	Status        string         `gorm:"column:status" json:"status"`
	Scenario      string         `gorm:"column:scenario" json:"scenario"`
	Summary       string         `gorm:"column:summary" json:"summary"`
	OriginVersion string         `gorm:"column:origin_version" json:"originVersion"`
	ProposalIDs   JSON           `gorm:"column:proposal_ids;type:json" json:"proposalIds"`
	CreatedAt     time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt     gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (EvolutionVersion) TableName() string { return "evolution_version" }

type EvolutionProposal struct {
	ID              int64          `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PublicID        string         `gorm:"column:public_id" json:"id"`
	UserID          string         `gorm:"column:user_id" json:"userId"`
	UserName        string         `gorm:"column:user_name" json:"userName"`
	Kind            string         `gorm:"column:kind" json:"kind"`
	Content         string         `gorm:"column:content" json:"content"`
	Status          string         `gorm:"column:status" json:"status"`
	AgentReply      string         `gorm:"column:agent_reply" json:"agentReply"`
	VersionPublicID string         `gorm:"column:version_public_id" json:"versionId"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (EvolutionProposal) TableName() string { return "evolution_proposal" }

type EvolutionMessage struct {
	ID        int64          `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PublicID  string         `gorm:"column:public_id" json:"id"`
	UserID    string         `gorm:"column:user_id" json:"userId"`
	UserName  string         `gorm:"column:user_name" json:"userName"`
	Role      string         `gorm:"column:role" json:"role"`
	Content   string         `gorm:"column:content" json:"content"`
	CreatedAt time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (EvolutionMessage) TableName() string { return "evolution_message" }

type EvolutionTask struct {
	ID              int64          `gorm:"column:id;primaryKey;autoIncrement" json:"-"`
	PublicID        string         `gorm:"column:public_id" json:"id"`
	VersionPublicID string         `gorm:"column:version_public_id" json:"versionId"`
	UserID          string         `gorm:"column:user_id" json:"userId"`
	UserName        string         `gorm:"column:user_name" json:"userName"`
	VersionLabel    string         `gorm:"column:version_label" json:"versionLabel"`
	OriginVersion   string         `gorm:"column:origin_version" json:"originVersion"`
	ChatSummary     string         `gorm:"column:chat_summary" json:"chatSummary"`
	Status          string         `gorm:"column:status" json:"status"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (EvolutionTask) TableName() string { return "evolution_task" }
