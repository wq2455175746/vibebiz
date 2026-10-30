package admin

import (
	"context"
	"strings"
	"time"
)

const (
	DefaultPageSize = 10
	MaxPageSize     = 100
)

type ListQuery struct {
	Keyword  string
	Page     int
	PageSize int
}

func NormalizeQuery(query ListQuery) ListQuery {
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = DefaultPageSize
	}
	if query.PageSize > MaxPageSize {
		query.PageSize = MaxPageSize
	}
	return query
}

type Page[T any] struct {
	Items    []T
	Total    int64
	Page     int
	PageSize int
}

func Paginate[T any](items []T, query ListQuery) Page[T] {
	query = NormalizeQuery(query)
	total := int64(len(items))
	start := (query.Page - 1) * query.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + query.PageSize
	if end > len(items) {
		end = len(items)
	}
	pageItems := make([]T, end-start)
	copy(pageItems, items[start:end])
	return Page[T]{Items: pageItems, Total: total, Page: query.Page, PageSize: query.PageSize}
}

func MatchesKeyword(keyword string, fields ...string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return true
	}
	for _, field := range fields {
		if strings.Contains(strings.ToLower(field), keyword) {
			return true
		}
	}
	return false
}

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
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	Status          string     `json:"status"`
	HealthStatus    string     `json:"healthStatus"`
	ActiveRequests  int        `json:"activeRequests"`
	MaxConcurrency  int        `json:"maxConcurrency"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt,omitempty"`
	LastHealthAt    *time.Time `json:"lastHealthAt,omitempty"`
	HealthError     string     `json:"healthError"`
}

type Quota struct {
	UserID         string `json:"userId"`
	Plan           string `json:"plan"`
	LLMTokens      int64  `json:"llmTokens"`
	SandboxMinutes int64  `json:"sandboxMinutes"`
	StorageMB      int64  `json:"storageMb"`
}

type AuditEvent struct {
	ID        string    `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	CreatedAt time.Time `json:"createdAt"`
}

type RequestCounter interface {
	Today(ctx context.Context) (int64, error)
}

type AuditRecord struct {
	OpsUserID     int64
	ActorName     string
	Action        string
	ResourceType  string
	ResourceID    string
	RequestMethod string
	RequestPath   string
	RequestID     string
	TraceID       string
	IP            string
	UserAgent     string
	ResultStatus  string
	ErrorMessage  string
}

type AuditRecorder interface {
	RecordAudit(ctx context.Context, event AuditRecord) error
}

type Repository interface {
	AuditRecorder
	Overview(ctx context.Context, projectCount int) (Overview, error)
	Users(ctx context.Context, query ListQuery) (Page[User], error)
	Sandboxes(ctx context.Context, query ListQuery) (Page[Sandbox], error)
	Quotas(ctx context.Context) ([]Quota, error)
	Audit(ctx context.Context, query ListQuery) (Page[AuditEvent], error)
	RecordAudit(ctx context.Context, event AuditRecord) error
}
