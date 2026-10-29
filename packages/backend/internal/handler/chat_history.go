package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

const (
	defaultChatHistoryLimit = 100
	maxChatHistoryLimit     = 500
)

type ChatHistoryClient interface {
	ReadChatHistory(ctx context.Context, id string, limit int) (client.ChatHistoryTail, error)
}

type ChatHistoryHandler struct {
	Sandbox  ChatHistoryClient
	Projects store.ProjectStore
}

func NewChatHistoryHandler(sandbox ChatHistoryClient, projects store.ProjectStore) *ChatHistoryHandler {
	return &ChatHistoryHandler{Sandbox: sandbox, Projects: projects}
}

type chatHistoryMessage struct {
	ID         string                 `json:"id"`
	Role       string                 `json:"role"`
	Text       string                 `json:"text"`
	Status     string                 `json:"status"`
	Tools      []chatHistoryTool      `json:"tools"`
	References []chatHistoryReference `json:"references"`
	CreatedAt  string                 `json:"createdAt"`
}

type chatHistoryReference struct {
	Kind         string `json:"kind"`
	ID           string `json:"id,omitempty"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	ContentType  string `json:"contentType,omitempty"`
	ResourceKind string `json:"resourceKind,omitempty"`
}

type chatHistoryTool struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type chatHistoryResponse struct {
	Messages  []chatHistoryMessage `json:"messages"`
	Truncated bool                 `json:"truncated"`
	Limit     int                  `json:"limit"`
}

type historyEntry struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	ParentID  *string         `json:"parentId"`
	Timestamp string          `json:"timestamp"`
	Message   *historyMessage `json:"message"`
}

type historyMessage struct {
	Role       string                 `json:"role"`
	Content    string                 `json:"content"`
	Status     string                 `json:"status"`
	Tools      []chatHistoryTool      `json:"tools"`
	References []chatHistoryReference `json:"references"`
}

func (h *ChatHistoryHandler) Get(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, err := h.Projects.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}

	limit, err := historyLimit(c.QueryParam("limit"))
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40007, "invalid limit")
	}
	tail, err := h.Sandbox.ReadChatHistory(c.Request().Context(), project.ID, limit)
	if err != nil {
		return failFromSandbox(c, err, "read chat history failed")
	}
	messages := parseHistory(tail.Content)
	return response.OK(c, chatHistoryResponse{
		Messages:  messages,
		Truncated: tail.Truncated,
		Limit:     limit,
	})
}

func historyLimit(raw string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return defaultChatHistoryLimit, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > maxChatHistoryLimit {
		return 0, errors.New("invalid limit")
	}
	return value, nil
}

func parseHistory(content string) []chatHistoryMessage {
	messages := make([]chatHistoryMessage, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry historyEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}
		if entry.Type != "message" || entry.Message == nil || entry.ID == "" {
			continue
		}
		if entry.Message.Role != "user" && entry.Message.Role != "assistant" {
			continue
		}
		tools := entry.Message.Tools
		if tools == nil {
			tools = []chatHistoryTool{}
		}
		references := entry.Message.References
		if references == nil {
			references = []chatHistoryReference{}
		}
		messages = append(messages, chatHistoryMessage{
			ID:         entry.ID,
			Role:       entry.Message.Role,
			Text:       entry.Message.Content,
			Status:     entry.Message.Status,
			Tools:      tools,
			References: references,
			CreatedAt:  entry.Timestamp,
		})
	}
	return messages
}
