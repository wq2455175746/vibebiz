package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"vibebiz/internal/observability"
)

type fakeResolver struct {
	urls map[string]string
}

func (f fakeResolver) ResolveBaseURL(_ context.Context, projectID string) (string, error) {
	return f.urls[projectID], nil
}

func TestRouterUsesProjectResolver(t *testing.T) {
	hits := map[string]int{}
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits["a"]++
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer serverA.Close()
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits["b"]++
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer serverB.Close()

	router := NewRouter(fakeResolver{urls: map[string]string{"p1": serverA.URL, "p2": serverB.URL}}, nil)
	if _, err := router.ListFiles(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := router.ListFiles(context.Background(), "p2"); err != nil {
		t.Fatal(err)
	}
	if hits["a"] != 1 || hits["b"] != 1 {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestRouterReusesClientForSameBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer server.Close()

	created := 0
	router := NewRouter(fakeResolver{urls: map[string]string{"p1": server.URL, "p2": server.URL}}, nil)
	router.newClient = func(baseURL string, obs *observability.Provider) *Sandbox {
		created++
		return NewSandbox(baseURL, obs)
	}
	if _, err := router.ListFiles(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if _, err := router.ListFiles(context.Background(), "p2"); err != nil {
		t.Fatal(err)
	}
	if created != 1 {
		t.Fatalf("created = %d, want 1", created)
	}
}

func TestRouterPreviewURL(t *testing.T) {
	router := NewRouter(fakeResolver{urls: map[string]string{"p1": "http://sandbox-a:8000"}}, nil)
	got, err := router.PreviewURL(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://sandbox-a:8000/internal/projects/p1/preview/" {
		t.Fatalf("url = %s", got)
	}
}
