package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
	"vibebiz/internal/role"
)

type ChatClient interface {
	Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error)
}

type ProjectRoleResolver interface {
	Resolve(ctx context.Context, projectID string) (*role.Context, error)
}

type ChatHandler struct {
	Sandbox ChatClient
	Roles   ProjectRoleResolver
	Obs     *observability.Provider
}

func NewChatHandler(sandbox ChatClient, obs *observability.Provider) *ChatHandler {
	return &ChatHandler{Sandbox: sandbox, Obs: obs}
}

func (h *ChatHandler) Chat(c echo.Context) error {
	payload, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40006, "invalid request body")
	}
	if len(payload) == 0 {
		payload = []byte("{}")
	}

	projectID := c.Param("id")
	ctx := c.Request().Context()
	var span trace.Span
	if h.Obs != nil {
		ctx, span = h.Obs.Tracer.Start(ctx, "chat.stream",
			trace.WithAttributes(attribute.String("project.id", projectID)),
		)
		defer span.End()
		h.Obs.Log(ctx, slog.LevelInfo, "chat.start", "project_id", projectID)
	}

	payload = h.withAgentRoles(ctx, projectID, payload)

	stream, err := h.Sandbox.Chat(ctx, projectID, payload)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "chat.end", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "chat failed")
	}
	defer stream.Close()

	res := c.Response()
	res.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")
	res.Header().Set("X-Accel-Buffering", "no")
	res.WriteHeader(http.StatusOK)

	done := false
	reader := bufio.NewReader(stream)
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			if h.Obs != nil && strings.HasPrefix(line, "data:") {
				eventType := sseEventType(line)
				h.Obs.RecordSSEEvent(projectID, eventType)
				h.Obs.Log(ctx, slog.LevelInfo, "sse.event", "project_id", projectID, "type", eventType)
				if eventType == "done" {
					done = true
					span.SetAttributes(attribute.Bool("sse.done", true))
				}
			}
			if _, writeErr := res.Write([]byte(line)); writeErr != nil {
				return nil
			}
			res.Flush()
		}
		if readErr != nil {
			if h.Obs != nil {
				span.SetAttributes(attribute.Bool("sse.done", done))
				if errors.Is(readErr, io.EOF) {
					h.Obs.Log(ctx, slog.LevelInfo, "chat.end", "project_id", projectID, "status", "ok", "sse_done", done)
				} else {
					span.RecordError(readErr)
					span.SetStatus(codes.Error, readErr.Error())
					h.Obs.Log(ctx, slog.LevelError, "chat.end", "project_id", projectID, "status", "error", "sse_done", done, "error", readErr.Error())
				}
			}
			return nil
		}
	}
}

func roleIDsForLog(roles []agentrole.Role) []int64 {
	ids := make([]int64, 0, len(roles))
	for _, role := range roles {
		ids = append(ids, role.ID)
	}
	return ids
}

func (h *ChatHandler) withAgentRoles(ctx context.Context, projectID string, payload []byte) []byte {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(payload, &body); err != nil {
		return payload
	}
	if body == nil {
		body = make(map[string]json.RawMessage)
	}
	var resolved *role.Context
	var resolveErr error
	if h.Roles != nil {
		resolved, resolveErr = h.Roles.Resolve(ctx, projectID)
	}
	if resolveErr != nil && h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelWarn, "chat.roles", "project_id", projectID, "status", "fallback", "error", resolveErr.Error())
	}
	if resolveErr == nil && resolved != nil && len(resolved.Roles) > 0 {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelInfo, "chat.roles", "project_id", projectID,
				"agent_role_ids", roleIDsForLog(resolved.Roles), "team_mode", resolved.TeamMode)
		}
		rolesJSON, err := json.Marshal(resolved.Roles)
		if err != nil {
			return payload
		}
		teamModeJSON, err := json.Marshal(resolved.TeamMode)
		if err != nil {
			return payload
		}
		body["agentRoles"] = rolesJSON
		body["teamMode"] = teamModeJSON
	} else {
		delete(body, "agentRoles")
		delete(body, "teamMode")
	}
	out, err := json.Marshal(body)
	if err != nil {
		return payload
	}
	return out
}

func sseEventType(line string) string {
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "[DONE]" {
		return "done"
	}
	var event struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(payload), &event); err != nil || event.Type == "" {
		return "unknown"
	}
	return event.Type
}
