package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/response"
	"vibebiz/internal/scheduledtask"
)

type ScheduledTaskHandler struct {
	Service *scheduledtask.Service
	Parser  scheduledtask.DraftParser
}

func NewScheduledTaskHandler(service *scheduledtask.Service, parser scheduledtask.DraftParser) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{Service: service, Parser: parser}
}

type scheduledTaskInput struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ScheduleType   string `json:"scheduleType"`
	Time           string `json:"time"`
	Weekdays       []int  `json:"weekdays"`
	CronExpression string `json:"cronExpression"`
	Timezone       string `json:"timezone"`
	Source         string `json:"source"`
}

type parseTaskRequest struct {
	Prompt string `json:"prompt"`
}

func (h *ScheduledTaskHandler) Parse(c echo.Context) error {
	if _, ok := auth.SessionFrom(c); !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req parseTaskRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	if h.Parser == nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "task parser unavailable")
	}
	draft, err := h.Parser.Parse(c.Request().Context(), req.Prompt)
	if err != nil {
		if errors.Is(err, scheduledtask.ErrInvalid) {
			return response.Fail(c, http.StatusBadRequest, 40001, "prompt is required")
		}
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return response.Fail(c, http.StatusBadGateway, 50201, apiErr.Message)
		}
		return response.Fail(c, http.StatusBadGateway, 50201, "task parse failed")
	}
	return response.OK(c, draft)
}

func (h *ScheduledTaskHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if session.CurrentWorkspaceID <= 0 || !containsInt64(session.WorkspaceIDs, session.CurrentWorkspaceID) {
		return response.Fail(c, http.StatusForbidden, 40302, "no workspace available")
	}
	var req scheduledTaskInput
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	task, err := h.Service.Create(c.Request().Context(), scheduledtask.CreateInput{
		Name: req.Name, Description: req.Description, ScheduleType: req.ScheduleType,
		Time: req.Time, Weekdays: req.Weekdays, CronExpression: req.CronExpression,
		Timezone: req.Timezone, Source: req.Source,
	}, session.UserID, session.CurrentWorkspaceID)
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.Created(c, task)
}

func (h *ScheduledTaskHandler) List(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	tasks, err := h.Service.List(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, map[string]any{"items": tasks})
}

func (h *ScheduledTaskHandler) Get(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := scheduledTaskID(c)
	if err != nil {
		return err
	}
	task, err := h.Service.Get(c.Request().Context(), id, session.UserID)
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, task)
}

func (h *ScheduledTaskHandler) Update(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := scheduledTaskID(c)
	if err != nil {
		return err
	}
	var req scheduledTaskInput
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	task, err := h.Service.Update(c.Request().Context(), id, session.UserID, scheduledtask.UpdateInput{
		Name: req.Name, Description: req.Description, ScheduleType: req.ScheduleType,
		Time: req.Time, Weekdays: req.Weekdays, CronExpression: req.CronExpression,
		Timezone: req.Timezone,
	})
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, task)
}

func (h *ScheduledTaskHandler) Pause(c echo.Context) error {
	return h.userAction(c, func(id, userID int64) (scheduledtask.Task, error) {
		return h.Service.Pause(c.Request().Context(), id, userID)
	})
}

func (h *ScheduledTaskHandler) Resume(c echo.Context) error {
	return h.userAction(c, func(id, userID int64) (scheduledtask.Task, error) {
		return h.Service.Resume(c.Request().Context(), id, userID)
	})
}

func (h *ScheduledTaskHandler) Stop(c echo.Context) error {
	return h.userAction(c, func(id, userID int64) (scheduledtask.Task, error) {
		return h.Service.Stop(c.Request().Context(), id, userID)
	})
}

func (h *ScheduledTaskHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := scheduledTaskID(c)
	if err != nil {
		return err
	}
	deleteProject := c.QueryParam("deleteProject") == "true"
	if err := h.Service.Delete(c.Request().Context(), id, session.UserID, deleteProject); err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *ScheduledTaskHandler) AdminList(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	var enabled *bool
	switch c.QueryParam("enabled") {
	case "1":
		value := true
		enabled = &value
	case "0":
		value := false
		enabled = &value
	}
	result, err := h.Service.AdminList(c.Request().Context(), scheduledtask.ListFilter{
		Keyword: c.QueryParam("keyword"), Enabled: enabled, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, result)
}

func (h *ScheduledTaskHandler) AdminPause(c echo.Context) error {
	return h.adminAction(c, func(id int64) (scheduledtask.Task, error) {
		return h.Service.Pause(c.Request().Context(), id, 0)
	})
}

func (h *ScheduledTaskHandler) AdminResume(c echo.Context) error {
	return h.adminAction(c, func(id int64) (scheduledtask.Task, error) {
		return h.Service.Resume(c.Request().Context(), id, 0)
	})
}

func (h *ScheduledTaskHandler) AdminStop(c echo.Context) error {
	return h.adminAction(c, func(id int64) (scheduledtask.Task, error) {
		return h.Service.Stop(c.Request().Context(), id, 0)
	})
}

func (h *ScheduledTaskHandler) userAction(c echo.Context, action func(id, userID int64) (scheduledtask.Task, error)) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := scheduledTaskID(c)
	if err != nil {
		return err
	}
	task, err := action(id, session.UserID)
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, task)
}

func (h *ScheduledTaskHandler) adminAction(c echo.Context, action func(id int64) (scheduledtask.Task, error)) error {
	id, err := scheduledTaskID(c)
	if err != nil {
		return err
	}
	task, err := action(id)
	if err != nil {
		return scheduledTaskError(c, err)
	}
	return response.OK(c, task)
}

func scheduledTaskID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, response.Fail(c, http.StatusBadRequest, 40001, "invalid task id")
	}
	return id, nil
}

func scheduledTaskError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, scheduledtask.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "scheduled task not found")
	case errors.Is(err, scheduledtask.ErrForbidden):
		return response.Fail(c, http.StatusNotFound, 40401, "scheduled task not found")
	case errors.Is(err, scheduledtask.ErrInvalid), errors.Is(err, scheduledtask.ErrBusy):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid scheduled task")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "scheduled task operation failed")
	}
}
