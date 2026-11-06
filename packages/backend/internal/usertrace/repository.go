package usertrace

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type Sink interface {
	Write(ctx context.Context, events []Event) error
}

type Query struct {
	Keyword   string
	EventType string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

type Log struct {
	ID           string    `json:"id"`
	EventType    string    `json:"eventType"`
	UserID       int64     `json:"userId"`
	Username     string    `json:"username"`
	WorkspaceID  int64     `json:"workspaceId"`
	ProjectID    string    `json:"projectId"`
	IP           string    `json:"ip"`
	Region       string    `json:"region"`
	DeviceID     string    `json:"deviceId"`
	DeviceType   string    `json:"deviceType"`
	OS           string    `json:"os"`
	Browser      string    `json:"browser"`
	UserAgent    string    `json:"userAgent"`
	Method       string    `json:"method"`
	Path         string    `json:"path"`
	Query        string    `json:"query"`
	Route        string    `json:"route"`
	StatusCode   int       `json:"statusCode"`
	DurationMs   int       `json:"durationMs"`
	LoginResult  string    `json:"loginResult"`
	ErrorMessage string    `json:"errorMessage"`
	EventAt      time.Time `json:"eventAt"`
}

type Page struct {
	Items    []Log
	Total    int64
	Page     int
	PageSize int
}

type Queryer interface {
	List(ctx context.Context, query Query) (Page, error)
}

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Write(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]model.UserAccessLog, 0, len(events))
	for _, event := range events {
		var userID *int64
		if event.UserID > 0 {
			id := event.UserID
			userID = &id
		}
		rows = append(rows, model.UserAccessLog{
			EventID:        event.EventID,
			EventType:      event.EventType,
			UserID:         userID,
			Username:       event.Username,
			WorkspaceID:    event.WorkspaceID,
			ProjectID:      event.ProjectID,
			IP:             event.IP,
			RegionCountry:  event.RegionCountry,
			RegionProvince: event.RegionProvince,
			RegionCity:     event.RegionCity,
			DeviceID:       event.DeviceID,
			DeviceType:     event.DeviceType,
			OS:             event.OS,
			Browser:        event.Browser,
			UserAgent:      event.UserAgent,
			Referer:        event.Referer,
			RequestMethod:  event.RequestMethod,
			RequestPath:    event.RequestPath,
			RequestQuery:   event.RequestQuery,
			Route:          event.Route,
			StatusCode:     event.StatusCode,
			DurationMs:     event.DurationMs,
			RequestID:      event.RequestID,
			TraceID:        event.TraceID,
			LoginResult:    event.LoginResult,
			ErrorMessage:   event.ErrorMessage,
			EventAt:        event.EventAt,
		})
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).
		Create(&rows).Error
	if err != nil {
		return fmt.Errorf("write user access log: %w", err)
	}
	return nil
}

func (r *MySQLRepository) List(ctx context.Context, query Query) (Page, error) {
	query = normalizeQuery(query)
	scope := func(db *gorm.DB) *gorm.DB {
		tx := db.Model(&model.UserAccessLog{})
		if query.Keyword != "" {
			like := "%" + query.Keyword + "%"
			tx = tx.Where("(username LIKE ? OR ip LIKE ? OR request_path LIKE ? OR device_id LIKE ? OR project_id LIKE ?)", like, like, like, like, like)
		}
		if query.EventType != "" {
			tx = tx.Where("event_type = ?", query.EventType)
		}
		if query.From != nil {
			tx = tx.Where("event_at >= ?", *query.From)
		}
		if query.To != nil {
			tx = tx.Where("event_at < ?", *query.To)
		}
		return tx
	}

	var total int64
	if err := r.db.WithContext(ctx).Scopes(scope).Count(&total).Error; err != nil {
		return Page{}, fmt.Errorf("count user access log: %w", err)
	}

	var rows []model.UserAccessLog
	err := r.db.WithContext(ctx).Scopes(scope).
		Order("event_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&rows).Error
	if err != nil {
		return Page{}, fmt.Errorf("list user access log: %w", err)
	}

	items := make([]Log, 0, len(rows))
	for _, row := range rows {
		var userID int64
		if row.UserID != nil {
			userID = *row.UserID
		}
		region := strings.TrimSpace(strings.Join([]string{row.RegionCountry, row.RegionProvince, row.RegionCity}, " "))
		items = append(items, Log{
			ID:           strconv.FormatInt(row.ID, 10),
			EventType:    row.EventType,
			UserID:       userID,
			Username:     row.Username,
			WorkspaceID:  row.WorkspaceID,
			ProjectID:    row.ProjectID,
			IP:           row.IP,
			Region:       region,
			DeviceID:     row.DeviceID,
			DeviceType:   row.DeviceType,
			OS:           row.OS,
			Browser:      row.Browser,
			UserAgent:    row.UserAgent,
			Method:       row.RequestMethod,
			Path:         row.RequestPath,
			Query:        row.RequestQuery,
			Route:        row.Route,
			StatusCode:   row.StatusCode,
			DurationMs:   row.DurationMs,
			LoginResult:  row.LoginResult,
			ErrorMessage: row.ErrorMessage,
			EventAt:      row.EventAt,
		})
	}
	return Page{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

func normalizeQuery(query Query) Query {
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = defaultPageSize
	}
	if query.PageSize > maxPageSize {
		query.PageSize = maxPageSize
	}
	return query
}
