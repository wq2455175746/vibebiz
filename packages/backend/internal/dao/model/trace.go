package model

import "time"

type UserAccessLog struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	EventID        string    `gorm:"column:event_id" json:"eventId"`
	EventType      string    `gorm:"column:event_type" json:"eventType"`
	UserID         *int64    `gorm:"column:user_id" json:"userId"`
	Username       string    `gorm:"column:username" json:"username"`
	WorkspaceID    int64     `gorm:"column:workspace_id" json:"workspaceId"`
	ProjectID      string    `gorm:"column:project_id" json:"projectId"`
	IP             string    `gorm:"column:ip" json:"ip"`
	RegionCountry  string    `gorm:"column:region_country" json:"regionCountry"`
	RegionProvince string    `gorm:"column:region_province" json:"regionProvince"`
	RegionCity     string    `gorm:"column:region_city" json:"regionCity"`
	DeviceID       string    `gorm:"column:device_id" json:"deviceId"`
	DeviceType     string    `gorm:"column:device_type" json:"deviceType"`
	OS             string    `gorm:"column:os" json:"os"`
	Browser        string    `gorm:"column:browser" json:"browser"`
	UserAgent      string    `gorm:"column:user_agent" json:"userAgent"`
	Referer        string    `gorm:"column:referer" json:"referer"`
	RequestMethod  string    `gorm:"column:request_method" json:"requestMethod"`
	RequestPath    string    `gorm:"column:request_path" json:"requestPath"`
	RequestQuery   string    `gorm:"column:request_query" json:"requestQuery"`
	Route          string    `gorm:"column:route" json:"route"`
	StatusCode     int       `gorm:"column:status_code" json:"statusCode"`
	DurationMs     int       `gorm:"column:duration_ms" json:"durationMs"`
	RequestID      string    `gorm:"column:request_id" json:"requestId"`
	TraceID        string    `gorm:"column:trace_id" json:"traceId"`
	LoginResult    string    `gorm:"column:login_result" json:"loginResult"`
	ErrorMessage   string    `gorm:"column:error_message" json:"errorMessage"`
	EventAt        time.Time `gorm:"column:event_at" json:"eventAt"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (UserAccessLog) TableName() string { return "user_access_log" }
