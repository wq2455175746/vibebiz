package client

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vibebiz/internal/observability"
	"vibebiz/internal/resource"
)

type fakeResolver struct {
	urls map[string]string
}

func (f fakeResolver) ResolveBaseURL(_ context.Context, projectID string) (string, error) {
	return f.urls[projectID], nil
}

type fakeRouteResolver struct {
	url   string
	epoch int64
}

func (f fakeRouteResolver) ResolveBaseURL(context.Context, string) (string, error) {
	return f.url, nil
}

func (f fakeRouteResolver) ResolveRoute(context.Context, string) (Route, error) {
	return Route{BaseURL: f.url, Epoch: f.epoch}, nil
}

func TestRouterSendsProjectEpoch(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Project-Epoch")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer server.Close()
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	if _, err := router.ListFiles(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if got != "9" {
		t.Fatalf("epoch header = %q", got)
	}
}

func TestRouterSendsProjectEpochOnWritesAndChat(t *testing.T) {
	seen := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen[r.URL.Path] = r.Header.Get("X-Project-Epoch")
		if strings.HasSuffix(r.URL.Path, "/chat") {
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{}}`))
	}))
	defer server.Close()
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)

	if err := router.WriteResource(context.Background(), "p1", resource.ResourceMeta{ID: "r1"}, strings.NewReader("x"), 1); err != nil {
		t.Fatal(err)
	}
	if err := router.ImportArchive(context.Background(), "p1", []byte("x")); err != nil {
		t.Fatal(err)
	}
	stream, err := router.Chat(context.Background(), "p1", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, stream)
	_ = stream.Close()

	for path, epoch := range seen {
		if epoch != "9" {
			t.Fatalf("epoch header for %s = %q", path, epoch)
		}
	}
	if len(seen) != 3 {
		t.Fatalf("paths = %+v", seen)
	}
}

func TestRouterOmitsEpochWhenFallbackResolverUsed(t *testing.T) {
	var got string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Project-Epoch")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
	}))
	defer server.Close()
	router := NewRouter(fakeResolver{urls: map[string]string{"p1": server.URL}}, nil)
	if _, err := router.ListFiles(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("epoch header = %q, want empty", got)
	}
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
