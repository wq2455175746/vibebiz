package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/llmmodel"
)

func withModelSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("auth.session", &auth.Session{Username: "admin"})
		return next(c)
	}
}

type fakeModelService struct {
	models  []llmmodel.Model
	created llmmodel.Model
	input   llmmodel.CreateInput
	by      string
	err     error
}

func (f *fakeModelService) List(context.Context) ([]llmmodel.Model, error) {
	return f.models, f.err
}

func (f *fakeModelService) Create(_ context.Context, input llmmodel.CreateInput, by string) (llmmodel.Model, error) {
	f.input = input
	f.by = by
	return f.created, f.err
}

func TestModelList(t *testing.T) {
	h := NewModelHandler(&fakeModelService{models: []llmmodel.Model{{ID: 1, ModelName: "DeepSeek", ModelCode: "deepseek-chat", ModelType: "LLM"}}})
	e := echo.New()
	e.GET("/cp/api/v1/admin/models", withModelSession(h.List))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/models", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Models []llmmodel.Model `json:"models"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Models) != 1 {
		t.Fatalf("models = %+v", env.Result.Models)
	}
}

func TestModelCreateMapsErrors(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{llmmodel.ErrInvalid, http.StatusBadRequest},
		{llmmodel.ErrConflict, http.StatusConflict},
		{llmmodel.ErrKeyMissing, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		h := NewModelHandler(&fakeModelService{err: tc.err})
		e := echo.New()
		e.POST("/cp/api/v1/admin/models", withModelSession(h.Create))
		body := `{"modelName":"x","modelCode":"x","apiUrl":"https://x","apiKey":"k","modelType":"LLM"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/models", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Fatalf("err %v status = %d, want %d", tc.err, rec.Code, tc.code)
		}
	}
}

func TestModelCreatePassesInput(t *testing.T) {
	service := &fakeModelService{created: llmmodel.Model{ID: 2, ModelCode: "embed"}}
	h := NewModelHandler(service)
	e := echo.New()
	e.POST("/cp/api/v1/admin/models", withModelSession(h.Create))
	body := `{"modelName":"Embed","modelCode":"embed","apiUrl":"https://x","apiKey":"k","providerCode":"Other","modelType":"EMBEDDING","isDefault":true,"status":1,"extraParams":{"dimension":1024}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/models", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.input.ModelType != "EMBEDDING" || !service.input.IsDefault {
		t.Fatalf("input = %+v", service.input)
	}
	if service.by != "admin" {
		t.Fatalf("createdBy = %q", service.by)
	}
	if !strings.Contains(string(service.input.ExtraParams), "1024") {
		t.Fatalf("extra = %s", service.input.ExtraParams)
	}
}

func TestModelCreateWithoutSessionIsUnauthorized(t *testing.T) {
	h := NewModelHandler(&fakeModelService{})
	e := echo.New()
	e.POST("/cp/api/v1/admin/models", h.Create)
	body := `{"modelName":"x","modelCode":"x","apiUrl":"https://x","apiKey":"k","modelType":"LLM"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/models", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
