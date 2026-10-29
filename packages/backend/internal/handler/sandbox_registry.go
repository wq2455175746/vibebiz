package handler

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/observability"
	"vibebiz/internal/response"
	"vibebiz/internal/sandboxregistry"
)

const sandboxTokenHeader = "X-Sandbox-Token"

type SandboxRegistryHandler struct {
	Repo  sandboxregistry.Repository
	Token string
	Obs   *observability.Provider
	Now   func() time.Time
}

func NewSandboxRegistryHandler(repo sandboxregistry.Repository, token string, obs *observability.Provider) *SandboxRegistryHandler {
	return &SandboxRegistryHandler{Repo: repo, Token: token, Obs: obs, Now: time.Now}
}

type sandboxHeartbeatRequest struct {
	Name           string `json:"name"`
	Scheme         string `json:"scheme"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	ActiveRequests int    `json:"activeRequests"`
	MaxConcurrency int    `json:"maxConcurrency"`
}

func (h *SandboxRegistryHandler) Heartbeat(c echo.Context) error {
	if strings.TrimSpace(h.Token) == "" {
		return response.Fail(c, http.StatusServiceUnavailable, 50303, "sandbox registry disabled")
	}
	token := c.Request().Header.Get(sandboxTokenHeader)
	if subtle.ConstantTimeCompare([]byte(token), []byte(h.Token)) != 1 {
		return response.Fail(c, http.StatusUnauthorized, 40102, "invalid sandbox token")
	}

	var req sandboxHeartbeatRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40007, "invalid sandbox heartbeat")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Scheme = strings.TrimSpace(req.Scheme)
	req.Host = strings.TrimSpace(req.Host)
	if req.Scheme == "" {
		req.Scheme = "http"
	}
	if req.Name == "" || len(req.Name) > 128 || req.Host == "" || req.Port < 1 || req.Port > 65535 ||
		req.MaxConcurrency < 1 || req.ActiveRequests < 0 {
		return response.Fail(c, http.StatusBadRequest, 40007, "invalid sandbox heartbeat")
	}

	now := h.Now().UTC()
	instance, err := h.Repo.UpsertHeartbeat(c.Request().Context(), sandboxregistry.Heartbeat{
		Name:           req.Name,
		Scheme:         req.Scheme,
		Host:           req.Host,
		Port:           req.Port,
		ActiveRequests: req.ActiveRequests,
		MaxConcurrency: req.MaxConcurrency,
	}, now)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(c.Request().Context(), slog.LevelError, "sandbox.heartbeat", "sandbox_name", req.Name, "status", "error", "error", err.Error())
		}
		return response.Fail(c, http.StatusInternalServerError, 50001, "sandbox heartbeat failed")
	}
	if h.Obs != nil {
		h.Obs.Log(c.Request().Context(), slog.LevelInfo, "sandbox.heartbeat", "sandbox_name", req.Name, "active_requests", req.ActiveRequests, "status", "ok")
	}
	return response.OK(c, map[string]string{"instanceId": instance.PublicID})
}
