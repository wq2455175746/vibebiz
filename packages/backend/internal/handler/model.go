package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/llmmodel"
	"vibebiz/internal/response"
)

type ModelService interface {
	List(ctx context.Context) ([]llmmodel.Model, error)
	Create(ctx context.Context, input llmmodel.CreateInput, createdBy string) (llmmodel.Model, error)
}

type ModelHandler struct {
	Service ModelService
}

func NewModelHandler(service ModelService) *ModelHandler {
	return &ModelHandler{Service: service}
}

type modelRequest struct {
	ModelName    string          `json:"modelName"`
	ModelCode    string          `json:"modelCode"`
	APIURL       string          `json:"apiUrl"`
	APIKey       string          `json:"apiKey"`
	ProviderCode string          `json:"providerCode"`
	ModelType    string          `json:"modelType"`
	MaxTokens    int             `json:"maxTokens"`
	IsDefault    bool            `json:"isDefault"`
	Status       *int8           `json:"status"`
	ExtraParams  json.RawMessage `json:"extraParams"`
}

func (h *ModelHandler) List(c echo.Context) error {
	models, err := h.Service.List(c.Request().Context())
	if err != nil {
		return modelError(c, err)
	}
	return response.OK(c, map[string]any{"models": models})
}

func (h *ModelHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req modelRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	model, err := h.Service.Create(c.Request().Context(), llmmodel.CreateInput{
		ModelName: req.ModelName, ModelCode: req.ModelCode, APIURL: req.APIURL,
		APIKey: req.APIKey, ProviderCode: req.ProviderCode, ModelType: req.ModelType,
		MaxTokens: req.MaxTokens, IsDefault: req.IsDefault, Status: status,
		ExtraParams: req.ExtraParams,
	}, session.Username)
	if err != nil {
		return modelError(c, err)
	}
	return response.OK(c, model)
}

func modelError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, llmmodel.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid model")
	case errors.Is(err, llmmodel.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "model code already exists")
	case errors.Is(err, llmmodel.ErrKeyMissing):
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "model secret key not configured")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "model operation failed")
	}
}
