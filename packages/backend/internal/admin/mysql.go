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
	var running int64
	if err := r.db.WithContext(ctx).
		Model(&model.SandboxInstance{}).
		Where("status = ? AND health_status = ?", "online", "healthy").
		Count(&running).Error; err != nil {
		return Overview{}, fmt.Errorf("count sandboxes: %w", err)
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
		RunningSandboxes: int(running),
		TodayRequests:    int(today),
	}, nil
}

func (r *MySQLRepository) Users(ctx context.Context, query ListQuery) (Page[User], error) {
	query = NormalizeQuery(query)
	scope := func(db *gorm.DB) *gorm.DB {
		tx := db.Table("user AS u").
			Joins("LEFT JOIN user_quota AS q ON q.user_id = u.id AND q.deleted_at IS NULL").
			Where("u.deleted_at IS NULL")
		if query.Keyword != "" {
			like := "%" + query.Keyword + "%"
			tx = tx.Where("(u.username LIKE ? OR u.nickname LIKE ? OR u.email LIKE ?)", like, like, like)
		}
		return tx
	}
	var total int64
	if err := r.db.WithContext(ctx).Scopes(scope).Count(&total).Error; err != nil {
		return Page[User]{}, fmt.Errorf("count admin users: %w", err)
	}
	var rows []struct {
		ID        int64
		Username  string
		Nickname  string
		Email     string
		PlanCode  string
		Status    int8
		CreatedAt time.Time
	}
	err := r.db.WithContext(ctx).Scopes(scope).
		Select("u.id, u.username, u.nickname, u.email, COALESCE(q.plan_code, 'free') AS plan_code, u.status, u.created_at").
		Order("u.id ASC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Scan(&rows).Error
	if err != nil {
		return Page[User]{}, fmt.Errorf("list admin users: %w", err)
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
	return Page[User]{Items: out, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (r *MySQLRepository) Sandboxes(ctx context.Context, query ListQuery) (Page[Sandbox], error) {
	query = NormalizeQuery(query)
	scope := func(db *gorm.DB) *gorm.DB {
		tx := db.Model(&model.SandboxInstance{})
		if query.Keyword != "" {
			like := "%" + query.Keyword + "%"
			tx = tx.Where("(name LIKE ? OR host LIKE ? OR status LIKE ?)", like, like, like)
		}
		return tx
	}
	var total int64
	if err := r.db.WithContext(ctx).Scopes(scope).Count(&total).Error; err != nil {
		return Page[Sandbox]{}, fmt.Errorf("count admin sandboxes: %w", err)
	}
	var rows []model.SandboxInstance
	if err := r.db.WithContext(ctx).Scopes(scope).
		Order("name ASC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&rows).Error; err != nil {
		return Page[Sandbox]{}, fmt.Errorf("list admin sandboxes: %w", err)
	}
	out := make([]Sandbox, 0, len(rows))
	for i := range rows {
		row := rows[i]
		out = append(out, Sandbox{
			ID:              row.PublicID,
			Name:            row.Name,
			Host:            row.Host,
			Port:            row.Port,
			Status:          row.Status,
			HealthStatus:    row.HealthStatus,
			ActiveRequests:  row.ActiveRequests,
			MaxConcurrency:  row.MaxConcurrency,
			LastHeartbeatAt: row.LastHeartbeatAt,
			LastHealthAt:    row.LastHealthAt,
			HealthError:     row.HealthError,
		})
	}
	return Page[Sandbox]{Items: out, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (r *MySQLRepository) Quotas(ctx context.Context) ([]Quota, error) {
	var rows []struct {
		UserID              int64
		PlanCode            string
		LLMTokenQuotaMonth  int64
		LLMTokensUsedMonth  int64
		SandboxMinutesQuota int64
		SandboxMinutesUsed  int64
		StorageQuotaBytes   int64
		StorageUsedBytes    int64
	}
	err := r.db.WithContext(ctx).
		Table("user_quota").
		Select("user_id, plan_code, llm_token_quota_month, llm_tokens_used_month, sandbox_minutes_quota_month, sandbox_minutes_used_month, storage_quota_bytes, storage_used_bytes").
		Where("deleted_at IS NULL").
		Order("user_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list admin quotas: %w", err)
	}
	out := make([]Quota, 0, len(rows))
	for _, row := range rows {
		out = append(out, Quota{
			UserID:              strconv.FormatInt(row.UserID, 10),
			Plan:                row.PlanCode,
			LLMTokensQuota:      row.LLMTokenQuotaMonth,
			LLMTokensUsed:       row.LLMTokensUsedMonth,
			SandboxMinutesQuota: row.SandboxMinutesQuota,
			SandboxMinutesUsed:  row.SandboxMinutesUsed,
			StorageQuotaMB:      row.StorageQuotaBytes / (1024 * 1024),
			StorageUsedMB:       row.StorageUsedBytes / (1024 * 1024),
		})
	}
	return out, nil
}

func (r *MySQLRepository) UpdateUserStatus(ctx context.Context, id int64, status int8) error {
	result := r.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("update user status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return r.ensureUserExists(ctx, id)
	}
	return nil
}

func (r *MySQLRepository) ensureUserExists(ctx context.Context, id int64) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return fmt.Errorf("check user: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) UpdateQuota(ctx context.Context, userID int64, input QuotaUpdateInput) error {
	result := r.db.WithContext(ctx).Model(&model.UserQuota{}).
		Where("user_id = ?", userID).
		Updates(map[string]any{
			"llm_token_quota_month":       input.LLMTokensQuota,
			"sandbox_minutes_quota_month": input.SandboxMinutesQuota,
			"storage_quota_bytes":         input.StorageQuotaMB * 1024 * 1024,
		})
	if result.Error != nil {
		return fmt.Errorf("update user quota: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return r.ensureQuotaExists(ctx, userID)
	}
	return nil
}

func (r *MySQLRepository) ensureQuotaExists(ctx context.Context, userID int64) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.UserQuota{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return fmt.Errorf("check user quota: %w", err)
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *MySQLRepository) Audit(ctx context.Context, query ListQuery) (Page[AuditEvent], error) {
	query = NormalizeQuery(query)
	scope := func(db *gorm.DB) *gorm.DB {
		tx := db.Model(&model.OpsAuditLog{})
		if query.Keyword != "" {
			like := "%" + query.Keyword + "%"
			tx = tx.Where("(actor_name LIKE ? OR action LIKE ? OR resource_id LIKE ? OR request_path LIKE ?)", like, like, like, like)
		}
		return tx
	}
	var total int64
	if err := r.db.WithContext(ctx).Scopes(scope).Count(&total).Error; err != nil {
		return Page[AuditEvent]{}, fmt.Errorf("count admin audit: %w", err)
	}
	var rows []model.OpsAuditLog
	if err := r.db.WithContext(ctx).Scopes(scope).
		Order("created_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&rows).Error; err != nil {
		return Page[AuditEvent]{}, fmt.Errorf("list admin audit: %w", err)
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
	return Page[AuditEvent]{Items: out, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
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
