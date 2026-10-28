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

	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type ChatClient interface {
	Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error)
}

type ChatHandler struct {
	Sandbox ChatClient
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
