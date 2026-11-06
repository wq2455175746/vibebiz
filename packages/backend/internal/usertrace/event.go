package usertrace

import "time"

const (
	EventLogin   = "login"
	EventRequest = "request"
)

type Event struct {
	EventID        string    `json:"eventId"`
	EventType      string    `json:"eventType"`
	UserID         int64     `json:"userId"`
	Username       string    `json:"username"`
	WorkspaceID    int64     `json:"workspaceId"`
	ProjectID      string    `json:"projectId"`
	IP             string    `json:"ip"`
	RegionCountry  string    `json:"regionCountry"`
	RegionProvince string    `json:"regionProvince"`
	RegionCity     string    `json:"regionCity"`
	DeviceID       string    `json:"deviceId"`
	DeviceType     string    `json:"deviceType"`
	OS             string    `json:"os"`
	Browser        string    `json:"browser"`
	UserAgent      string    `json:"userAgent"`
	Referer        string    `json:"referer"`
	RequestMethod  string    `json:"requestMethod"`
	RequestPath    string    `json:"requestPath"`
	RequestQuery   string    `json:"requestQuery"`
	Route          string    `json:"route"`
	StatusCode     int       `json:"statusCode"`
	DurationMs     int       `json:"durationMs"`
	RequestID      string    `json:"requestId"`
	TraceID        string    `json:"traceId"`
	LoginResult    string    `json:"loginResult"`
	ErrorMessage   string    `json:"errorMessage"`
	EventAt        time.Time `json:"eventAt"`
}

type Appender interface {
	Append(event Event)
}
