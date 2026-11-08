package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/evolution"
	"vibebiz/internal/response"
)

type EvolutionHandler struct {
	Service *evolution.Service
}

func NewEvolutionHandler(service *evolution.Service) *EvolutionHandler {
	return &EvolutionHandler{Service: service}
}

type evolutionContentView struct {
	Content string `json:"content"`
}

type evolutionTaskView struct {
	VersionID string `json:"versionId"`
	Status    string `json:"status"`
}

func (h *EvolutionHandler) List(c echo.Context) error {
	if _, ok := evolutionViewer(c); !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	versions, origin, err := h.Service.ListVersions(c.Request().Context())
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{
		"projectId":     evolution.ProjectID,
		"originVersion": origin,
		"versions":      versions,
	})
}

func (h *EvolutionHandler) Conversation(c echo.Context) error {
	if _, ok := evolutionViewer(c); !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	return h.conversation(c)
}

func (h *EvolutionHandler) EnsureDraft(c echo.Context) error {
	viewer, ok := evolutionViewer(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	version, err := h.Service.EnsureDraft(c.Request().Context(), viewer)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{"version": version})
}

func (h *EvolutionHandler) SubmitProposal(c echo.Context) error {
	viewer, ok := evolutionViewer(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req evolutionContentView
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	proposal, version, err := h.Service.SubmitProposal(c.Request().Context(), viewer, req.Content)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.Created(c, map[string]any{"proposal": proposal, "version": version})
}

func (h *EvolutionHandler) SubmitChat(c echo.Context) error {
	viewer, ok := evolutionViewer(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req evolutionContentView
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	messages, err := h.Service.SubmitChat(c.Request().Context(), viewer, req.Content)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{"messages": messages})
}

func (h *EvolutionHandler) ConfirmVersion(c echo.Context) error {
	if _, ok := evolutionViewer(c); !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	version, err := h.Service.ConfirmVersion(c.Request().Context(), c.Param("id"))
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{"version": version})
}

func (h *EvolutionHandler) AdminSnapshot(c echo.Context) error {
	snapshot, err := h.Service.Snapshot(c.Request().Context())
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{
		"projectId":     evolution.ProjectID,
		"originVersion": evolution.OriginVersion,
		"versions":      snapshot.Versions,
		"proposals":     snapshot.Proposals,
		"tasks":         snapshot.Tasks,
	})
}

func (h *EvolutionHandler) AdminConversation(c echo.Context) error {
	return h.conversation(c)
}

func (h *EvolutionHandler) AdminReply(c echo.Context) error {
	var req evolutionContentView
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	message, err := h.Service.AdminReply(c.Request().Context(), c.Param("id"), req.Content)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.Created(c, map[string]any{"message": message})
}

func (h *EvolutionHandler) AdminSubmitTask(c echo.Context) error {
	var req evolutionTaskView
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.VersionID) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "versionId is required")
	}
	task, err := h.Service.SubmitTask(c.Request().Context(), req.VersionID)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.Created(c, map[string]any{"task": task})
}

func (h *EvolutionHandler) AdminUpdateTask(c echo.Context) error {
	var req evolutionTaskView
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Status) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "status is required")
	}
	task, err := h.Service.UpdateTaskStatus(c.Request().Context(), c.Param("id"), req.Status)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{"task": task})
}

func (h *EvolutionHandler) conversation(c echo.Context) error {
	userID := strings.TrimSpace(c.Param("userId"))
	if userID == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "userId is required")
	}
	messages, proposals, err := h.Service.Conversation(c.Request().Context(), userID)
	if err != nil {
		return evolutionError(c, err)
	}
	return response.OK(c, map[string]any{"messages": messages, "proposals": proposals})
}

func evolutionViewer(c echo.Context) (evolution.Viewer, bool) {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return evolution.Viewer{}, false
	}
	id := strconv.FormatInt(session.UserID, 10)
	name := strings.TrimSpace(session.Username)
	if name == "" {
		name = id
	}
	return evolution.Viewer{
		ID:          id,
		InternalID:  session.UserID,
		Name:        name,
		WorkspaceID: session.CurrentWorkspaceID,
	}, true
}

func evolutionError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, evolution.ErrUnknownUser):
		return response.Fail(c, http.StatusBadRequest, 40001, "userId is required")
	case errors.Is(err, evolution.ErrNoWorkspace):
		return response.Fail(c, http.StatusForbidden, 40305, "no workspace available")
	case errors.Is(err, evolution.ErrEmptyContent):
		return response.Fail(c, http.StatusBadRequest, 40001, "content is required")
	case errors.Is(err, evolution.ErrContentTooLong):
		return response.Fail(c, http.StatusBadRequest, 40002, "content is too long")
	case errors.Is(err, evolution.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "evolution record not found")
	case errors.Is(err, evolution.ErrOfficialReadonly):
		return response.Fail(c, http.StatusBadRequest, 40003, "official version is readonly")
	case errors.Is(err, evolution.ErrTaskExists):
		return response.Fail(c, http.StatusConflict, 40901, "task already exists")
	case errors.Is(err, evolution.ErrInvalidStatus):
		return response.Fail(c, http.StatusBadRequest, 40004, "invalid task status")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50015, "evolution operation failed")
	}
}
