package model

import "time"

type ChatConversation struct {
	Base
	WorkspaceID          int64      `gorm:"column:workspace_id" json:"workspaceId"`
	ProjectID            int64      `gorm:"column:project_id" json:"projectId"`
	UserID               int64      `gorm:"column:user_id" json:"userId"`
	Title                string     `gorm:"column:title" json:"title"`
	ModelID              int64      `gorm:"column:model_id" json:"modelId"`
	AgentRoleID          int64      `gorm:"column:agent_role_id" json:"agentRoleId"`
	SystemPrompt         string     `gorm:"column:system_prompt;type:mediumtext" json:"systemPrompt"`
	Source               string     `gorm:"column:source" json:"source"`
	ExtChatID            *string    `gorm:"column:ext_chat_id" json:"extChatId,omitempty"`
	Status               string     `gorm:"column:status" json:"status"`
	MessageCount         int64      `gorm:"column:message_count" json:"messageCount"`
	TotalTokens          int64      `gorm:"column:total_tokens" json:"totalTokens"`
	LastMessageAt        *time.Time `gorm:"column:last_message_at" json:"lastMessageAt,omitempty"`
	StartAt              *time.Time `gorm:"column:start_at" json:"startAt,omitempty"`
	EndAt                *time.Time `gorm:"column:end_at" json:"endAt,omitempty"`
	ParentConversationID *int64     `gorm:"column:parent_conversation_id" json:"parentConversationId,omitempty"`
}

func (ChatConversation) TableName() string { return "chat_conversation" }

type ChatMessage struct {
	Base
	ConversationID  int64   `gorm:"column:conversation_id" json:"conversationId"`
	Role            string  `gorm:"column:role" json:"role"`
	Content         string  `gorm:"column:content;type:mediumtext" json:"content"`
	Attachments     JSON    `gorm:"column:attachments;type:json" json:"attachments,omitempty"`
	Commands        JSON    `gorm:"column:commands;type:json" json:"commands,omitempty"`
	Status          string  `gorm:"column:status" json:"status"`
	ErrorMessage    string  `gorm:"column:error_message;type:text" json:"errorMessage"`
	TokenUsage      *int64  `gorm:"column:token_usage" json:"tokenUsage,omitempty"`
	ResponseTimeMS  *int64  `gorm:"column:response_time_ms" json:"responseTimeMs,omitempty"`
	ParentMessageID *int64  `gorm:"column:parent_message_id" json:"parentMessageId,omitempty"`
	Version         int     `gorm:"column:version" json:"version"`
	ModelID         *int64  `gorm:"column:model_id" json:"modelId,omitempty"`
	CommitSHA       *string `gorm:"column:commit_sha" json:"commitSha,omitempty"`
	TraceID         *string `gorm:"column:trace_id" json:"traceId,omitempty"`
}

func (ChatMessage) TableName() string { return "chat_message" }

type ChatMemory struct {
	Base
	Scope            string     `gorm:"column:scope" json:"scope"`
	UserID           int64      `gorm:"column:user_id" json:"userId"`
	WorkspaceID      *int64     `gorm:"column:workspace_id" json:"workspaceId,omitempty"`
	ProjectID        *int64     `gorm:"column:project_id" json:"projectId,omitempty"`
	Name             string     `gorm:"column:name" json:"name"`
	ConversationIDs  JSON       `gorm:"column:conversation_ids;type:json" json:"conversationIds,omitempty"`
	Mem              string     `gorm:"column:mem;type:mediumtext" json:"mem"`
	MemType          string     `gorm:"column:mem_type" json:"memType"`
	TimeRangeStartAt *time.Time `gorm:"column:time_range_start_at" json:"timeRangeStartAt,omitempty"`
	TimeRangeEndAt   *time.Time `gorm:"column:time_range_end_at" json:"timeRangeEndAt,omitempty"`
	SourceMsgCount   int64      `gorm:"column:source_msg_count" json:"sourceMsgCount"`
	Version          int        `gorm:"column:version" json:"version"`
	IsActive         int8       `gorm:"column:is_active" json:"isActive"`
	ExpireAt         *time.Time `gorm:"column:expire_at" json:"expireAt,omitempty"`
	ConfidenceScore  *int8      `gorm:"column:confidence_score" json:"confidenceScore,omitempty"`
	CreatedBy        string     `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy        string     `gorm:"column:updated_by" json:"updatedBy"`
}

func (ChatMemory) TableName() string { return "chat_memory" }
