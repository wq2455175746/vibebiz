package admin

import "time"

type Overview struct {
	Projects         int `json:"projects"`
	Users            int `json:"users"`
	RunningSandboxes int `json:"runningSandboxes"`
	TodayRequests    int `json:"todayRequests"`
}

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Plan      string    `json:"plan"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

type Sandbox struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Node      string    `json:"node"`
	State     string    `json:"state"`
	StartedAt time.Time `json:"startedAt"`
}

type Quota struct {
	UserID         string `json:"userId"`
	Plan           string `json:"plan"`
	LLMTokens      int    `json:"llmTokens"`
	SandboxMinutes int    `json:"sandboxMinutes"`
	StorageMB      int    `json:"storageMb"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"createdAt"`
}

type Repository interface {
	Overview(projectCount int) Overview
	Users() []User
	Sandboxes() []Sandbox
	Quotas() []Quota
	Audit() []AuditEvent
}
