package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/model"
	"vibebiz/internal/observability"
	"vibebiz/internal/projecttemplate"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type ArchiveImporter interface {
	ImportArchive(ctx context.Context, id string, data []byte) error
}

type TemplateHandler struct {
	Service     *projecttemplate.Service
	Projects    store.ProjectStore
	Initializer ProjectInitializer
	Archives    ArchiveImporter
	Metas       auth.ProjectMetaStore
	Obs         *observability.Provider
	Now         func() time.Time
}

func NewTemplateHandler(service *projecttemplate.Service, projects store.ProjectStore, initializer ProjectInitializer, archives ArchiveImporter) *TemplateHandler {
	return &TemplateHandler{Service: service, Projects: projects, Initializer: initializer, Archives: archives, Now: time.Now}
}

func (h *TemplateHandler) List(c echo.Context) error {
	parentID, _ := strconv.ParseInt(c.QueryParam("parentTopicId"), 10, 64)
	topicID, _ := strconv.ParseInt(c.QueryParam("topicId"), 10, 64)
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	result, err := h.Service.List(c.Request().Context(), projecttemplate.Query{
		Keyword: c.QueryParam("keyword"), ParentTopicID: parentID, TopicID: topicID, Page: page, PageSize: pageSize,
	})
	if err != nil {
		return templateError(c, err)
	}
	return response.OK(c, result)
}

func (h *TemplateHandler) Topics(c echo.Context) error {
	var topics []projecttemplate.Topic
	var err error
	if c.QueryParam("all") == "1" {
		topics, err = h.Service.AdminTopics(c.Request().Context())
	} else {
		topics, err = h.Service.Topics(c.Request().Context())
	}
	if err != nil {
		return templateError(c, err)
	}
	return response.OK(c, map[string]any{"topics": topics})
}

type topicRequest struct {
	ParentID       *int64 `json:"parentId"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PromptTemplate string `json:"promptTemplate"`
	SortOrder      int    `json:"sortOrder"`
	Status         int8   `json:"status"`
}

func (h *TemplateHandler) CreateTopic(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req topicRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	topic, err := h.Service.CreateTopic(c.Request().Context(), projecttemplate.TopicInput{
		ParentID: req.ParentID, Code: req.Code, Name: req.Name, Description: req.Description,
		PromptTemplate: req.PromptTemplate, SortOrder: req.SortOrder, Status: req.Status,
	}, session.Username)
	if err != nil {
		return templateError(c, err)
	}
	return response.Created(c, topic)
}

func (h *TemplateHandler) UpdateTopic(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid topic id")
	}
	var req topicRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	topic, err := h.Service.UpdateTopic(c.Request().Context(), id, projecttemplate.TopicInput{
		ParentID: req.ParentID, Code: req.Code, Name: req.Name, Description: req.Description,
		PromptTemplate: req.PromptTemplate, SortOrder: req.SortOrder, Status: req.Status,
	}, session.Username)
	if err != nil {
		return templateError(c, err)
	}
	return response.OK(c, topic)
}

func (h *TemplateHandler) AdminList(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	result, err := h.Service.AdminList(c.Request().Context(), projecttemplate.Query{
		Keyword: c.QueryParam("keyword"), Page: page, PageSize: pageSize,
	})
	if err != nil {
		return templateError(c, err)
	}
	return response.OK(c, result)
}

type updateTemplateRequest struct {
	TemplateName   string `json:"templateName"`
	Description    string `json:"description"`
	TopicID        int64  `json:"topicId"`
	RecommendScore int    `json:"recommendScore"`
	Status         int8   `json:"status"`
}

func (h *TemplateHandler) AdminUpdate(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid template id")
	}
	var req updateTemplateRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	result, err := h.Service.UpdateTemplate(c.Request().Context(), id, projecttemplate.UpdateInput{
		TemplateName: req.TemplateName, Description: req.Description, TopicID: req.TopicID,
		RecommendScore: req.RecommendScore, Status: req.Status,
	}, session.Username)
	if err != nil {
		return templateError(c, err)
	}
	return response.OK(c, result)
}

func (h *TemplateHandler) AdminDelete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid template id")
	}
	if err := h.Service.DeleteTemplate(c.Request().Context(), id); err != nil {
		return templateError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *TemplateHandler) AdminFiles(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid template id")
	}
	raw, err := h.Service.FileTree(c.Request().Context(), id)
	if err != nil {
		return templateError(c, err)
	}
	var files any
	if err := json.Unmarshal(raw, &files); err != nil {
		files = []any{}
	}
	return response.OK(c, map[string]any{"files": files})
}

type createTemplateRequest struct {
	TemplateName   string `json:"templateName"`
	Description    string `json:"description"`
	TopicID        int64  `json:"topicId"`
	RecommendScore int    `json:"recommendScore"`
}

func (h *TemplateHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req createTemplateRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	result, err := h.Service.Create(c.Request().Context(), c.Param("id"), projecttemplate.CreateInput{
		TemplateName: req.TemplateName, Description: req.Description,
		TopicID: req.TopicID, RecommendScore: req.RecommendScore,
	}, session.Username)
	if err != nil {
		return templateError(c, err)
	}
	return response.Created(c, result)
}

type useTemplateRequest struct {
	Name string `json:"name"`
}

func (h *TemplateHandler) Use(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	templateID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || templateID <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid template id")
	}
	var req useTemplateRequest
	_ = c.Bind(&req)
	template, err := h.Service.Template(c.Request().Context(), templateID)
	if err != nil {
		return templateError(c, err)
	}
	archive, err := h.Service.Download(c.Request().Context(), templateID)
	if err != nil {
		return templateError(c, err)
	}
	workspaceID := session.CurrentWorkspaceID
	if workspaceID == 0 || !containsInt64(session.WorkspaceIDs, workspaceID) {
		return response.Fail(c, http.StatusForbidden, 40302, "no workspace available")
	}
	id := uuid.NewString()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = template.TemplateName
	}
	sandboxID, err := h.Initializer.InitProject(c.Request().Context(), id)
	if err != nil {
		return response.Fail(c, http.StatusBadGateway, 50201, "workspace init failed")
	}
	now := h.now()
	templateIDValue := template.ID
	project := &model.Project{
		ID: id, Name: name, Prompt: template.Description,
		State: model.ProjectStateReady, ReviewStatus: model.ReviewStatusPending,
		WorkspaceID: workspaceID, TopicID: template.TopicID, TemplateID: &templateIDValue,
		OwnerUserID: session.UserID, SandboxID: &sandboxID, AgentRoleIDs: []int64{},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := h.Projects.Create(c.Request().Context(), project); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "create project failed")
	}
	if h.Metas != nil {
		if err := h.Metas.Set(c.Request().Context(), id, auth.ProjectMeta{
			WorkspaceID: workspaceID, OwnerUserID: session.UserID,
		}); err != nil {
			_ = h.Projects.Delete(c.Request().Context(), id)
			return response.Fail(c, http.StatusInternalServerError, 50003, "project metadata write failed")
		}
	}
	if err := h.Archives.ImportArchive(c.Request().Context(), id, archive); err != nil {
		_ = h.Projects.Delete(c.Request().Context(), id)
		if h.Metas != nil {
			_ = h.Metas.Delete(c.Request().Context(), id)
		}
		return response.Fail(c, http.StatusBadGateway, 50202, "import template failed")
	}
	return response.Created(c, project)
}

func (h *TemplateHandler) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func templateError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, projecttemplate.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "template not found")
	case errors.Is(err, projecttemplate.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "topic code already exists")
	case errors.Is(err, projecttemplate.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid template")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "template operation failed")
	}
}
