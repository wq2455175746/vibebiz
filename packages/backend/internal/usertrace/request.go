package usertrace

import (
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

var sensitiveQueryKeys = map[string]struct{}{
	"password": {}, "passwd": {}, "token": {}, "access_token": {},
	"secret": {}, "api_key": {}, "key": {}, "authorization": {},
}

type RequestInfo struct {
	IP         string
	Method     string
	Path       string
	Query      string
	Route      string
	DeviceID   string
	DeviceType string
	OS         string
	Browser    string
	UserAgent  string
	Referer    string
	RequestID  string
	TraceID    string
}

func CaptureRequest(c echo.Context) RequestInfo {
	req := c.Request()
	info := RequestInfo{
		IP:        c.RealIP(),
		Method:    req.Method,
		Path:      truncate(req.URL.Path, 512),
		Query:     sanitizeQuery(req.URL.RawQuery),
		Route:     truncate(c.Path(), 255),
		DeviceID:  normalizeDeviceID(req.Header.Get("X-Device-Id")),
		UserAgent: truncate(req.UserAgent(), 512),
		Referer:   truncate(req.Referer(), 1024),
		RequestID: requestID(c),
		TraceID:   traceID(c),
	}
	if info.Route == "" {
		info.Route = info.Path
	}
	device := ParseUserAgent(info.UserAgent)
	info.DeviceType = device.DeviceType
	info.OS = device.OS
	info.Browser = device.Browser
	return info
}

func sanitizeQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return truncate(raw, 1024)
	}
	for key := range values {
		if _, ok := sensitiveQueryKeys[strings.ToLower(key)]; ok {
			values[key] = []string{"***"}
		}
	}
	return truncate(values.Encode(), 1024)
}

func normalizeDeviceID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 64 {
		return ""
	}
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return ""
	}
	return raw
}

func requestID(c echo.Context) string {
	if id := c.Response().Header().Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	return c.Request().Header.Get(echo.HeaderXRequestID)
}

func traceID(c echo.Context) string {
	spanContext := trace.SpanContextFromContext(c.Request().Context())
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
