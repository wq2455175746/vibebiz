package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
	"vibebiz/internal/sandboxregistry"
)

const maxSkillUploadBytes = 20 * 1024 * 1024

type StoredSkillClient interface {
	ListStoredSkills(ctx context.Context, scope string, userID int64) ([]client.SkillInfo, error)
	UploadSkill(ctx context.Context, scope string, userID int64, filename string, source io.Reader, size int64, overwrite bool) (client.SkillInfo, error)
	DeleteSkill(ctx context.Context, scope string, userID int64, name string) error
}

type SkillManager struct {
	Client            StoredSkillClient
	Users             admin.Repository
	Obs               *observability.Provider
	UserWritesEnabled bool
}

func NewSkillManager(client StoredSkillClient, users admin.Repository, obs *observability.Provider) *SkillManager {
	return &SkillManager{Client: client, Users: users, Obs: obs, UserWritesEnabled: false}
}

func (h *SkillManager) ListMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	items, err := h.Client.ListStoredSkills(c.Request().Context(), "all", session.UserID)
	if err != nil {
		return skillFailure(c, err, "list skills failed")
	}
	return response.OK(c, map[string]any{"skills": items})
}

func (h *SkillManager) UploadMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if !h.UserWritesEnabled {
		return response.Fail(c, http.StatusForbidden, 40301, "user skill upload is disabled")
	}
	return h.upload(c, "user", session.UserID)
}

func (h *SkillManager) DeleteMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if !h.UserWritesEnabled {
		return response.Fail(c, http.StatusForbidden, 40301, "user skill upload is disabled")
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid skill name")
	}
	if err := h.Client.DeleteSkill(c.Request().Context(), "user", session.UserID, name); err != nil {
		return skillFailure(c, err, "delete skill failed")
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *SkillManager) AdminList(c echo.Context) error {
	scope, userID, err := h.adminScope(c)
	if err != nil {
		return err
	}
	items, listErr := h.Client.ListStoredSkills(c.Request().Context(), scope, userID)
	if listErr != nil {
		return skillFailure(c, listErr, "list skills failed")
	}
	keyword := strings.ToLower(strings.TrimSpace(c.QueryParam("keyword")))
	if keyword != "" {
		filtered := make([]client.SkillInfo, 0, len(items))
		for _, item := range items {
			if strings.Contains(strings.ToLower(item.Name), keyword) || strings.Contains(strings.ToLower(item.Description), keyword) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	return response.OK(c, map[string]any{"skills": items})
}

func (h *SkillManager) AdminUpload(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxSkillUploadBytes+1024*1024)
	scope, userID, err := h.adminScope(c)
	if err != nil {
		return err
	}
	return h.upload(c, scope, userID)
}

func (h *SkillManager) AdminDelete(c echo.Context) error {
	scope, userID, err := h.adminScope(c)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid skill name")
	}
	if err := h.Client.DeleteSkill(c.Request().Context(), scope, userID, name); err != nil {
		return skillFailure(c, err, "delete skill failed")
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *SkillManager) upload(c echo.Context, scope string, userID int64) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxSkillUploadBytes+1024*1024)
	file, err := c.FormFile("file")
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "zip file is required")
	}
	if file.Size <= 0 || file.Size > maxSkillUploadBytes {
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, "zip file is too large")
	}
	source, err := file.Open()
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "open zip failed")
	}
	defer source.Close()
	overwrite, _ := strconv.ParseBool(c.FormValue("overwrite"))
	item, err := h.Client.UploadSkill(c.Request().Context(), scope, userID, file.Filename, source, file.Size, overwrite)
	if err != nil {
		return skillFailure(c, err, "upload skill failed")
	}
	if h.Obs != nil {
		h.Obs.Log(c.Request().Context(), slog.LevelInfo, "skills.upload",
			"scope", scope, "user_id", userID, "skill_name", item.Name, "status", "ok")
	}
	return response.OK(c, item)
}

func (h *SkillManager) adminScope(c echo.Context) (string, int64, error) {
	scope := strings.TrimSpace(c.QueryParam("scope"))
	if scope == "" {
		scope = strings.TrimSpace(c.FormValue("scope"))
	}
	if scope != "public" && scope != "user" {
		return "", 0, response.Fail(c, http.StatusBadRequest, 40001, "invalid scope")
	}
	if scope == "public" {
		return scope, 0, nil
	}
	raw := strings.TrimSpace(c.QueryParam("userId"))
	if raw == "" {
		raw = strings.TrimSpace(c.FormValue("userId"))
	}
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || userID <= 0 {
		return "", 0, response.Fail(c, http.StatusBadRequest, 40001, "invalid user id")
	}
	if h.Users != nil {
		if _, err := h.Users.User(c.Request().Context(), userID); err != nil {
			if errors.Is(err, admin.ErrNotFound) {
				return "", 0, response.Fail(c, http.StatusNotFound, 40401, "user not found")
			}
			return "", 0, response.Fail(c, http.StatusInternalServerError, 50001, "user lookup failed")
		}
	}
	return scope, userID, nil
}

func skillFailure(c echo.Context, err error, fallback string) error {
	if errors.Is(err, sandboxregistry.ErrNoHealthySandbox) {
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "sandbox unavailable")
	}
	return failFromSandbox(c, err, fallback)
}
