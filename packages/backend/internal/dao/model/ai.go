package model

import "time"

type LLMModel struct {
	Base
	Scope         string `gorm:"column:scope" json:"scope"`
	UserID        int64  `gorm:"column:user_id" json:"userId"`
	ModelName     string `gorm:"column:model_name" json:"modelName"`
	ModelCode     string `gorm:"column:model_code" json:"modelCode"`
	APIURL        string `gorm:"column:api_url" json:"apiUrl"`
	APIKeyCipher  string `gorm:"column:api_key_cipher" json:"-"`
	ProviderCode  string `gorm:"column:provider_code" json:"providerCode"`
	ModelType     string `gorm:"column:model_type" json:"modelType"`
	MaxTokens     int    `gorm:"column:max_tokens" json:"maxTokens"`
	ContextWindow *int   `gorm:"column:context_window" json:"contextWindow,omitempty"`
	IsSmallModel  int8   `gorm:"column:is_small_model" json:"isSmallModel"`
	IsDefault     int8   `gorm:"column:is_default" json:"isDefault"`
	Established   int8   `gorm:"column:established" json:"established"`
	Status        int8   `gorm:"column:status" json:"status"`
	ExtraParams   JSON   `gorm:"column:extra_params;type:json" json:"extraParams,omitempty"`
	CreatedBy     string `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy     string `gorm:"column:updated_by" json:"updatedBy"`
}

func (LLMModel) TableName() string { return "llm_model" }

type AgentRole struct {
	Base
	RoleCode     string `gorm:"column:role_code" json:"roleCode"`
	RoleNameEN   string `gorm:"column:role_name_en" json:"roleNameEn"`
	RoleNameZH   string `gorm:"column:role_name_zh" json:"roleNameZh"`
	Description  string `gorm:"column:description" json:"description"`
	SystemPrompt string `gorm:"column:system_prompt;type:mediumtext" json:"systemPrompt"`
	AvatarURL    string `gorm:"column:avatar_url" json:"avatarUrl"`
	IsBuiltin    int8   `gorm:"column:is_builtin" json:"isBuiltin"`
	Status       int8   `gorm:"column:status" json:"status"`
	SortOrder    int    `gorm:"column:sort_order" json:"sortOrder"`
	Version      int    `gorm:"column:version" json:"version"`
	CreatedBy    string `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy    string `gorm:"column:updated_by" json:"updatedBy"`
}

func (AgentRole) TableName() string { return "agent_role" }

type ScheduledTask struct {
	Base
	UserID      int64      `gorm:"column:user_id" json:"userId"`
	WorkspaceID *int64     `gorm:"column:workspace_id" json:"workspaceId,omitempty"`
	ProjectID   *int64     `gorm:"column:project_id" json:"projectId,omitempty"`
	Name        string     `gorm:"column:name" json:"name"`
	TaskType    string     `gorm:"column:task_type" json:"taskType"`
	CronExpr    string     `gorm:"column:cron_expr" json:"cronExpr"`
	Timezone    string     `gorm:"column:timezone" json:"timezone"`
	Prompt      string     `gorm:"column:prompt;type:mediumtext" json:"prompt"`
	Payload     JSON       `gorm:"column:payload;type:json" json:"payload,omitempty"`
	AgentRoleID *int64     `gorm:"column:agent_role_id" json:"agentRoleId,omitempty"`
	ModelID     *int64     `gorm:"column:model_id" json:"modelId,omitempty"`
	Enabled     int8       `gorm:"column:enabled" json:"enabled"`
	NextRunAt   *time.Time `gorm:"column:next_run_at" json:"nextRunAt,omitempty"`
	LastRunAt   *time.Time `gorm:"column:last_run_at" json:"lastRunAt,omitempty"`
	LastStatus  string     `gorm:"column:last_status" json:"lastStatus"`
	LastError   string     `gorm:"column:last_error" json:"lastError"`
	RunCount    int64      `gorm:"column:run_count" json:"runCount"`
	MaxRuns     int64      `gorm:"column:max_runs" json:"maxRuns"`
	StartAt     *time.Time `gorm:"column:start_at" json:"startAt,omitempty"`
	EndAt       *time.Time `gorm:"column:end_at" json:"endAt,omitempty"`
	CreatedBy   string     `gorm:"column:created_by" json:"createdBy"`
	UpdatedBy   string     `gorm:"column:updated_by" json:"updatedBy"`
}

func (ScheduledTask) TableName() string { return "scheduled_task" }
