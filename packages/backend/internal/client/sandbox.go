package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"vibebiz/internal/observability"
	"vibebiz/internal/resource"
)

type APIError struct {
	Status  int
	Code    int
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("sandbox error (status=%d code=%d): %s", e.Status, e.Code, e.Message)
}

type FileEntry struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	Mtime int64  `json:"mtime"`
}

type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Version  string `json:"version"`
	Size     int64  `json:"size"`
	Mtime    int64  `json:"mtime"`
	IsBinary bool   `json:"isBinary"`
}

type WriteResult struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Size    int64  `json:"size"`
	Mtime   int64  `json:"mtime"`
}

type ProjectStats struct {
	FileSizeBytes  int64 `json:"fileSizeBytes"`
	FileCount      int64 `json:"fileCount"`
	DirCount       int64 `json:"dirCount"`
	LastModifiedAt int64 `json:"lastModifiedAt"`
}

type ChatHistoryTail struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Version   string `json:"version"`
}

type ChatStatus struct {
	Active bool   `json:"active"`
	TurnID string `json:"turnId"`
}

type SkillInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

type Sandbox struct {
	baseURL string
	http    *http.Client
	obs     *observability.Provider
}

func NewSandbox(baseURL string, obs *observability.Provider) *Sandbox {
	client := &http.Client{Timeout: 10 * time.Minute}
	if obs != nil {
		client.Transport = otelhttp.NewTransport(http.DefaultTransport)
	}
	return &Sandbox{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    client,
		obs:     obs,
	}
}

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

func (s *Sandbox) InitProject(ctx context.Context, id string) error {
	_, err := s.doJSON(ctx, "init", id, http.MethodPost, "/internal/projects/"+url.PathEscape(id)+"/init", map[string]string{})
	return err
}

func (s *Sandbox) ListFiles(ctx context.Context, id string) ([]FileEntry, error) {
	raw, err := s.doJSON(ctx, "list_files", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/files", nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Files []FileEntry `json:"files"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload.Files, nil
}

func (s *Sandbox) ReadFile(ctx context.Context, id, path string) (*FileContent, error) {
	q := url.Values{}
	q.Set("path", path)
	raw, err := s.doJSON(ctx, "read_file", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/file?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var content FileContent
	if err := json.Unmarshal(raw, &content); err != nil {
		return nil, err
	}
	return &content, nil
}

func (s *Sandbox) WriteFile(ctx context.Context, id, path, content, baseVersion string) (*WriteResult, error) {
	body := map[string]string{"path": path, "content": content, "baseVersion": baseVersion}
	raw, err := s.doJSON(ctx, "write_file", id, http.MethodPut, "/internal/projects/"+url.PathEscape(id)+"/file", body)
	if err != nil {
		return nil, err
	}
	var result WriteResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Sandbox) WriteResource(ctx context.Context, projectID string, meta resource.ResourceMeta, body io.Reader, size int64) (err error) {
	start := time.Now()
	ctx, span := s.startSpan(ctx, "write_resource", projectID)
	status := 0
	defer func() {
		s.finishSpan(ctx, span, "write_resource", projectID, status, start, err)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		s.baseURL+"/internal/projects/"+url.PathEscape(projectID)+"/resources/"+url.PathEscape(meta.ID), body)
	if err != nil {
		return err
	}
	req.ContentLength = size
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Resource-Id", meta.ID)
	req.Header.Set("X-Resource-Name", url.QueryEscape(meta.DisplayName))
	req.Header.Set("X-Resource-Display-Name", url.QueryEscape(meta.DisplayName))
	req.Header.Set("X-Resource-Storage-Name", meta.StorageName)
	req.Header.Set("X-Resource-Content-Type", meta.ContentType)
	req.Header.Set("X-Resource-S3-Key", meta.S3Key)
	req.Header.Set("X-Resource-Size", strconv.FormatInt(size, 10))
	req.Header.Set("X-Resource-Kind", meta.Kind)

	resp, err := s.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	status = resp.StatusCode
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		var env envelope
		_ = json.Unmarshal(raw, &env)
		message := env.Message
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		return &APIError{Status: resp.StatusCode, Code: env.Code, Message: message}
	}
	return nil
}

func (s *Sandbox) DeleteResource(ctx context.Context, projectID, resourceID string) error {
	_, err := s.doJSON(ctx, "delete_resource", projectID, http.MethodDelete,
		"/internal/projects/"+url.PathEscape(projectID)+"/resources/"+url.PathEscape(resourceID), nil)
	return err
}

func (s *Sandbox) ProjectStats(ctx context.Context, id string) (ProjectStats, error) {
	raw, err := s.doJSON(ctx, "project_stats", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/stats", nil)
	if err != nil {
		return ProjectStats{}, err
	}
	var stats ProjectStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return ProjectStats{}, err
	}
	return stats, nil
}

func (s *Sandbox) ReadChatHistory(ctx context.Context, id string, limit int) (ChatHistoryTail, error) {
	q := url.Values{}
	q.Set("limit", strconv.Itoa(limit))
	raw, err := s.doJSON(ctx, "read_chat_history", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/chat/history?"+q.Encode(), nil)
	if err != nil {
		return ChatHistoryTail{}, err
	}
	var tail ChatHistoryTail
	if err := json.Unmarshal(raw, &tail); err != nil {
		return ChatHistoryTail{}, err
	}
	return tail, nil
}

func (s *Sandbox) StopChat(ctx context.Context, id string) error {
	_, err := s.doJSON(ctx, "stop_chat", id, http.MethodPost, "/internal/projects/"+url.PathEscape(id)+"/chat/stop", map[string]string{})
	return err
}

func (s *Sandbox) ChatStatus(ctx context.Context, id string) (ChatStatus, error) {
	raw, err := s.doJSON(ctx, "chat_status", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/chat/status", nil)
	if err != nil {
		return ChatStatus{}, err
	}
	var status ChatStatus
	if err := json.Unmarshal(raw, &status); err != nil {
		return ChatStatus{}, err
	}
	return status, nil
}

func (s *Sandbox) ListSkills(ctx context.Context, id string) ([]SkillInfo, error) {
	raw, err := s.doJSON(ctx, "list_skills", id, http.MethodGet, "/internal/projects/"+url.PathEscape(id)+"/skills", nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Skills []SkillInfo `json:"skills"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload.Skills, nil
}

func (s *Sandbox) Chat(ctx context.Context, id string, payload []byte) (out io.ReadCloser, err error) {
	start := time.Now()
	ctx, span := s.startSpan(ctx, "chat", id)
	status := 0
	defer func() {
		s.finishSpan(ctx, span, "chat", id, status, start, err)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/internal/projects/"+url.PathEscape(id)+"/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	status = resp.StatusCode
	if resp.StatusCode >= http.StatusBadRequest {
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		err = &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}
		return nil, err
	}
	return resp.Body, nil
}

func (s *Sandbox) PreviewURL(id string) string {
	return s.baseURL + "/internal/projects/" + url.PathEscape(id) + "/preview/"
}

func (s *Sandbox) doJSON(ctx context.Context, operation, projectID, method, path string, body any) (result json.RawMessage, err error) {
	start := time.Now()
	ctx, span := s.startSpan(ctx, operation, projectID)
	status := 0
	defer func() {
		s.finishSpan(ctx, span, operation, projectID, status, start, err)
	}()

	var reader io.Reader
	if body != nil {
		raw, marshalErr := json.Marshal(body)
		if marshalErr != nil {
			err = marshalErr
			return nil, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	status = resp.StatusCode

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var env envelope
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &env)
	}
	if resp.StatusCode >= http.StatusBadRequest || env.Code != 0 {
		message := env.Message
		if message == "" {
			message = strings.TrimSpace(string(raw))
		}
		err = &APIError{Status: resp.StatusCode, Code: env.Code, Message: message}
		return nil, err
	}
	return env.Result, nil
}

func (s *Sandbox) startSpan(ctx context.Context, operation, projectID string) (context.Context, trace.Span) {
	if s.obs == nil {
		return ctx, nil
	}
	ctx, span := s.obs.Tracer.Start(ctx, "sandbox."+operation,
		trace.WithAttributes(
			attribute.String("project.id", projectID),
			attribute.String("sandbox.operation", operation),
		),
	)
	s.obs.Log(ctx, slog.LevelInfo, "sandbox.call", "project_id", projectID, "operation", operation, "phase", "start")
	return ctx, span
}

func (s *Sandbox) finishSpan(ctx context.Context, span trace.Span, operation, projectID string, status int, start time.Time, err error) {
	if s.obs == nil {
		return
	}
	if span != nil {
		span.SetAttributes(attribute.Int("http.status_code", status))
		span.End()
	}

	statusLabel := strconv.Itoa(status)
	if status == 0 && err != nil {
		statusLabel = "error"
	}
	s.obs.RecordSandboxRequest(operation, statusLabel, projectID)

	attrs := []any{
		"project_id", projectID,
		"operation", operation,
		"status", statusLabel,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		s.obs.Log(ctx, slog.LevelError, "sandbox.call", append(attrs, "error", err.Error())...)
		return
	}
	s.obs.Log(ctx, slog.LevelInfo, "sandbox.call", attrs...)
}
