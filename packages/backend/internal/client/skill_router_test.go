package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSkillRouterListStoredSkills(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/skills" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Query().Get("scope") != "public" || r.URL.Query().Get("userId") != "7" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"skills":[{"name":"demo","source":"public"}]}}`))
	}))
	defer srv.Close()

	router := NewSkillRouter(BaseURLSelectorFunc(func(context.Context) (string, error) {
		return srv.URL, nil
	}), nil)
	skills, err := router.ListStoredSkills(context.Background(), "public", 7)
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) != 1 || skills[0].Name != "demo" {
		t.Fatalf("skills = %+v", skills)
	}
}

func TestSkillRouterUploadsZip(t *testing.T) {
	var received string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/skills/upload" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatal(err)
		}
		received = r.FormValue("scope") + "/" + r.FormValue("userId") + "/" + r.FormValue("overwrite")
		file, _, err := r.FormFile("file")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		body, _ := io.ReadAll(file)
		if string(body) != "zipdata" {
			t.Fatalf("body = %q", body)
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"name":"demo","source":"user"}}`))
	}))
	defer srv.Close()

	router := NewSkillRouter(BaseURLSelectorFunc(func(context.Context) (string, error) {
		return srv.URL, nil
	}), nil)
	item, err := router.UploadSkill(context.Background(), "user", 7, "demo.zip", bytes.NewBufferString("zipdata"), int64(len("zipdata")), true)
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "demo" || received != "user/7/true" {
		t.Fatalf("item = %+v received = %q", item, received)
	}
}

func TestSkillRouterDeleteSkill(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || !strings.HasSuffix(r.URL.Path, "/internal/skills/public/demo") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"deleted":true}}`))
	}))
	defer srv.Close()

	router := NewSkillRouter(BaseURLSelectorFunc(func(context.Context) (string, error) {
		return srv.URL, nil
	}), nil)
	if err := router.DeleteSkill(context.Background(), "public", 0, "demo"); err != nil {
		t.Fatal(err)
	}
}
