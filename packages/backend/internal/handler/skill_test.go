package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
)

type fakeSkillClient struct {
	skills []client.SkillInfo
	err    error
}

func (f fakeSkillClient) ListSkills(context.Context, string) ([]client.SkillInfo, error) {
	return f.skills, f.err
}

func TestSkillList(t *testing.T) {
	h := NewSkillHandler(fakeSkillClient{skills: []client.SkillInfo{
		{Name: "demo", Description: "demo skill", Source: "project"},
	}}, nil)
	e := echo.New()
	e.GET("/dp/api/v1/projects/:id/skills", h.List)

	req := httptest.NewRequest(http.MethodGet, "/dp/api/v1/projects/p1/skills", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Skills []client.SkillInfo `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Skills) != 1 || env.Result.Skills[0].Name != "demo" {
		t.Fatalf("result = %+v", env.Result.Skills)
	}
}
