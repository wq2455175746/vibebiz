package router

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDataServesSkills(t *testing.T) {
	srv := newSandboxServer(t)
	e, _, authState := newDP(t, srv.URL)

	rec := serveWithAuth(e, http.MethodGet, "/dp/api/v1/projects/p1/skills", authState.userCookie)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Skills []struct {
				Name string `json:"name"`
			} `json:"skills"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Skills) != 1 || env.Result.Skills[0].Name != "demo" {
		t.Fatalf("result = %+v", env.Result.Skills)
	}
}
