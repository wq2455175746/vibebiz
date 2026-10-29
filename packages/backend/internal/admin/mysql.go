package admin

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type MySQLRepository struct {
	db       *gorm.DB
	requests RequestCounter
}

func NewMySQLRepository(db *gorm.DB, requests RequestCounter) *MySQLRepository {
	return &MySQLRepository{db: db, requests: requests}
}

func (r *MySQLRepository) Overview(ctx context.Context, projectCount int) (Overview, error) {
	var users int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Count(&users).Error; err != nil {
		return Overview{}, fmt.Errorf("count users: %w", err)
	}
	today := int64(0)
	if r.requests != nil {
		if value, err := r.requests.Today(ctx); err == nil {
			today = value
		}
	}
	return Overview{
		Projects:         projectCount,
		Users:            int(users),
		RunningSandboxes: 0,
		TodayRequests:    int(today),
	}, nil
}

func (r *MySQLRepository) Users(ctx context.Context) ([]User, error) {
	var rows []struct {
		ID        int64
		Username  string
		Nickname  string
		Email     string
		PlanCode  string
		Status    int8
		CreatedAt time.Time
	}
	err := r.db.WithContext(ctx).
		Table("user AS u").
		Select("u.id, u.username, u.nickname, u.email, COALESCE(q.plan_code, 'free') AS plan_code, u.status, u.created_at").
		Joins("LEFT JOIN user_quota AS q ON q.user_id = u.id AND q.deleted_at IS NULL").
		Where("u.deleted_at IS NULL").
		Order("u.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}
	out := make([]User, 0, len(rows))
	for _, row := range rows {
		name := row.Nickname
		if name == "" {
			name = row.Username
		}
		status := "disabled"
		if row.Status == 1 {
			status = "active"
		}
		out = append(out, User{
			ID:        strconv.FormatInt(row.ID, 10),
			Name:      name,
			Email:     row.Email,
			Plan:      row.PlanCode,
			Status:    status,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (r *MySQLRepository) Sandboxes(_ context.Context) ([]Sandbox, error) {
	return []Sandbox{}, nil
}

func (r *MySQLRepository) Quotas(ctx context.Context) ([]Quota, error) {
	var rows []struct {
		UserID             int64
		PlanCode           string
		LLMTokensUsedMonth int64
		SandboxMinutesUsed int64
		StorageUsedBytes   int64
	}
	err := r.db.WithContext(ctx).
		Table("user_quota").
		Select("user_id, plan_code, llm_tokens_used_month, sandbox_minutes_used_month, storage_used_bytes").
		Where("deleted_at IS NULL").
		Order("user_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list admin quotas: %w", err)
	}
	out := make([]Quota, 0, len(rows))
	for _, row := range rows {
		out = append(out, Quota{
			UserID:         strconv.FormatInt(row.UserID, 10),
			Plan:           row.PlanCode,
			LLMTokens:      row.LLMTokensUsedMonth,
			SandboxMinutes: row.SandboxMinutesUsed,
			StorageMB:      row.StorageUsedBytes / (1024 * 1024),
		})
	}
	return out, nil
}

func (r *MySQLRepository) Audit(ctx context.Context) ([]AuditEvent, error) {
	var rows []model.OpsAuditLog
	if err := r.db.WithContext(ctx).Order("created_at DESC, id DESC").Limit(200).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list admin audit: %w", err)
	}
	out := make([]AuditEvent, 0, len(rows))
	for _, row := range rows {
		target := row.ResourceID
		if target == "" {
			target = row.RequestPath
		}
		out = append(out, AuditEvent{
			ID:        strconv.FormatInt(row.ID, 10),
			Actor:     row.ActorName,
			Action:    row.Action,
			Target:    target,
			CreatedAt: row.CreatedAt,
		})
	}
	return out, nil
}

func (r *MySQLRepository) RecordAudit(ctx context.Context, event AuditRecord) error {
	var opsUserID *int64
	if event.OpsUserID > 0 {
		opsUserID = &event.OpsUserID
	}
	row := &model.OpsAuditLog{
		OpsUserID:     opsUserID,
		ActorName:     event.ActorName,
		Action:        event.Action,
		ResourceType:  event.ResourceType,
		ResourceID:    event.ResourceID,
		RequestMethod: event.RequestMethod,
		RequestPath:   event.RequestPath,
		RequestID:     event.RequestID,
		TraceID:       event.TraceID,
		IP:            event.IP,
		UserAgent:     event.UserAgent,
		ResultStatus:  event.ResultStatus,
		ErrorMessage:  event.ErrorMessage,
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("record admin audit: %w", err)
	}
	return nil
}
