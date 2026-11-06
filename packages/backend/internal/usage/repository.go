package usage

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type Repository interface {
	Quota(ctx context.Context, userID int64) (Quota, error)
	Usage(ctx context.Context, userID int64, date time.Time) (Usage, error)
	Increment(ctx context.Context, userID int64, date time.Time, delta Usage) error
	Series(ctx context.Context, filter Filter) ([]DailyRow, error)
}

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Quota(ctx context.Context, userID int64) (Quota, error) {
	var row model.UserQuota
	err := r.db.WithContext(ctx).
		Select("project_daily_quota, llm_token_daily_quota, upload_daily_count_quota, upload_daily_bytes_quota").
		Where("user_id = ?", userID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Quota{}, nil
	}
	if err != nil {
		return Quota{}, err
	}
	return Quota{
		ProjectDailyQuota:     row.ProjectDailyQuota,
		LLMTokenDailyQuota:    row.LLMTokenDailyQuota,
		UploadDailyCountQuota: row.UploadDailyCountQuota,
		UploadDailyBytesQuota: row.UploadDailyBytesQuota,
	}, nil
}

func (r *MySQLRepository) Usage(ctx context.Context, userID int64, date time.Time) (Usage, error) {
	var row model.UserUsageDaily
	err := r.db.WithContext(ctx).
		Select("projects_created, input_tokens, output_tokens, model_requests, upload_count, upload_bytes").
		Where("user_id = ? AND usage_date = ?", userID, date.Format("2006-01-02")).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Usage{}, nil
	}
	if err != nil {
		return Usage{}, err
	}
	return Usage{
		ProjectsCreated: row.ProjectsCreated,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		ModelRequests:   row.ModelRequests,
		UploadCount:     row.UploadCount,
		UploadBytes:     row.UploadBytes,
	}, nil
}

type usageIncrementRow struct {
	UserID          int64  `gorm:"column:user_id"`
	UsageDate       string `gorm:"column:usage_date;type:date"`
	ProjectsCreated int64  `gorm:"column:projects_created"`
	InputTokens     int64  `gorm:"column:input_tokens"`
	OutputTokens    int64  `gorm:"column:output_tokens"`
	ModelRequests   int64  `gorm:"column:model_requests"`
	UploadCount     int64  `gorm:"column:upload_count"`
	UploadBytes     int64  `gorm:"column:upload_bytes"`
}

func newUsageIncrementRow(userID int64, date time.Time, delta Usage) *usageIncrementRow {
	return &usageIncrementRow{
		UserID:          userID,
		UsageDate:       date.Format("2006-01-02"),
		ProjectsCreated: delta.ProjectsCreated,
		InputTokens:     delta.InputTokens,
		OutputTokens:    delta.OutputTokens,
		ModelRequests:   delta.ModelRequests,
		UploadCount:     delta.UploadCount,
		UploadBytes:     delta.UploadBytes,
	}
}

func (r *MySQLRepository) Increment(ctx context.Context, userID int64, date time.Time, delta Usage) error {
	row := newUsageIncrementRow(userID, date, delta)
	return r.db.WithContext(ctx).Table("user_usage_daily").Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "usage_date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"projects_created": gorm.Expr("projects_created + ?", delta.ProjectsCreated),
			"input_tokens":     gorm.Expr("input_tokens + ?", delta.InputTokens),
			"output_tokens":    gorm.Expr("output_tokens + ?", delta.OutputTokens),
			"model_requests":   gorm.Expr("model_requests + ?", delta.ModelRequests),
			"upload_count":     gorm.Expr("upload_count + ?", delta.UploadCount),
			"upload_bytes":     gorm.Expr("upload_bytes + ?", delta.UploadBytes),
		}),
	}).Create(row).Error
}

func (r *MySQLRepository) Series(ctx context.Context, filter Filter) ([]DailyRow, error) {
	query := r.db.WithContext(ctx).Model(&model.UserUsageDaily{}).
		Select("user_id, DATE_FORMAT(usage_date, '%Y-%m-%d') AS date, projects_created, input_tokens, output_tokens, model_requests, upload_count, upload_bytes").
		Where("usage_date >= ? AND usage_date <= ?", filter.From.Format("2006-01-02"), filter.To.Format("2006-01-02"))
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	var rows []DailyRow
	err := query.Group("user_id, usage_date").
		Order("usage_date ASC, user_id ASC").
		Scan(&rows).Error
	return rows, err
}
