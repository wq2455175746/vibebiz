package usertrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"vibebiz/internal/dao/model"
)

type stringField struct {
	name  string
	value string
	limit int
}

func oversizedEvent() Event {
	repeat := func(char string, limit int) string {
		return strings.Repeat(char, limit+10)
	}
	return Event{
		EventID:        strings.Repeat("e", maxEventID+10),
		EventType:      repeat("界", maxEventType),
		Username:       repeat("界", maxUsername),
		ProjectID:      repeat("界", maxProjectID),
		IP:             repeat("界", maxIP),
		RegionCountry:  repeat("界", maxRegion),
		RegionProvince: repeat("界", maxRegion),
		RegionCity:     repeat("界", maxRegion),
		DeviceID:       strings.Repeat("a", maxDeviceID+10),
		DeviceType:     repeat("界", maxDeviceType),
		OS:             repeat("界", maxOS),
		Browser:        repeat("界", maxBrowser),
		UserAgent:      repeat("界", maxUserAgent),
		Referer:        repeat("界", maxReferer),
		RequestMethod:  repeat("界", maxRequestMethod),
		RequestPath:    repeat("界", maxRequestPath),
		RequestQuery:   repeat("界", maxRequestQuery),
		Route:          repeat("界", maxRoute),
		RequestID:      repeat("界", maxRequestID),
		TraceID:        repeat("界", maxTraceID),
		LoginResult:    repeat("界", maxLoginResult),
		ErrorMessage:   repeat("界", maxErrorMessage),
		EventAt:        time.Date(2026, 11, 6, 10, 0, 0, 0, time.Local),
	}
}

func eventStringFields(event Event) []stringField {
	return []stringField{
		{"event_id", event.EventID, maxEventID},
		{"event_type", event.EventType, maxEventType},
		{"username", event.Username, maxUsername},
		{"project_id", event.ProjectID, maxProjectID},
		{"ip", event.IP, maxIP},
		{"region_country", event.RegionCountry, maxRegion},
		{"region_province", event.RegionProvince, maxRegion},
		{"region_city", event.RegionCity, maxRegion},
		{"device_id", event.DeviceID, maxDeviceID},
		{"device_type", event.DeviceType, maxDeviceType},
		{"os", event.OS, maxOS},
		{"browser", event.Browser, maxBrowser},
		{"user_agent", event.UserAgent, maxUserAgent},
		{"referer", event.Referer, maxReferer},
		{"request_method", event.RequestMethod, maxRequestMethod},
		{"request_path", event.RequestPath, maxRequestPath},
		{"request_query", event.RequestQuery, maxRequestQuery},
		{"route", event.Route, maxRoute},
		{"request_id", event.RequestID, maxRequestID},
		{"trace_id", event.TraceID, maxTraceID},
		{"login_result", event.LoginResult, maxLoginResult},
		{"error_message", event.ErrorMessage, maxErrorMessage},
	}
}

func modelStringFields(row model.UserAccessLog) []stringField {
	return []stringField{
		{"event_id", row.EventID, maxEventID},
		{"event_type", row.EventType, maxEventType},
		{"username", row.Username, maxUsername},
		{"project_id", row.ProjectID, maxProjectID},
		{"ip", row.IP, maxIP},
		{"region_country", row.RegionCountry, maxRegion},
		{"region_province", row.RegionProvince, maxRegion},
		{"region_city", row.RegionCity, maxRegion},
		{"device_id", row.DeviceID, maxDeviceID},
		{"device_type", row.DeviceType, maxDeviceType},
		{"os", row.OS, maxOS},
		{"browser", row.Browser, maxBrowser},
		{"user_agent", row.UserAgent, maxUserAgent},
		{"referer", row.Referer, maxReferer},
		{"request_method", row.RequestMethod, maxRequestMethod},
		{"request_path", row.RequestPath, maxRequestPath},
		{"request_query", row.RequestQuery, maxRequestQuery},
		{"route", row.Route, maxRoute},
		{"request_id", row.RequestID, maxRequestID},
		{"trace_id", row.TraceID, maxTraceID},
		{"login_result", row.LoginResult, maxLoginResult},
		{"error_message", row.ErrorMessage, maxErrorMessage},
	}
}

func assertFieldLimits(t *testing.T, label string, fields []stringField) {
	t.Helper()
	for _, field := range fields {
		if !utf8.ValidString(field.value) {
			t.Fatalf("%s %s is not valid UTF-8", label, field.name)
		}
		if got := utf8.RuneCountInString(field.value); got != field.limit {
			t.Fatalf("%s %s rune count = %d, want %d", label, field.name, got, field.limit)
		}
	}
}

func TestToModelTruncatesEveryStringField(t *testing.T) {
	row := toModel(oversizedEvent())
	assertFieldLimits(t, "toModel", modelStringFields(row))
}

func TestRecorderAppendTruncatesEveryStringField(t *testing.T) {
	event := oversizedEvent()
	dir := t.TempDir()
	recorder := NewRecorder(dir, "cp", nil)
	recorder.newID = func() string { return "unused" }

	recorder.Append(event)

	path := filepath.Join(dir, "cp", event.EventAt.Format("2006-01-02"), "anonymous.log")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trace log: %v", err)
	}
	var stored Event
	if err := json.Unmarshal(body, &stored); err != nil {
		t.Fatalf("decode trace log: %v", err)
	}
	assertFieldLimits(t, "Append", eventStringFields(stored))
}
