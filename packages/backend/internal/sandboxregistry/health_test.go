package sandboxregistry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

type healthUpdate struct {
	id      int64
	healthy bool
	err     string
}

type fakeHealthRepository struct {
	Repository
	pending     []Instance
	updates     []healthUpdate
	staleCalls  int
	pendingErr  error
	staleErr    error
	updateError error
}

func (f *fakeHealthRepository) MarkStaleOffline(context.Context, time.Time, time.Duration) error {
	f.staleCalls++
	return f.staleErr
}

func (f *fakeHealthRepository) ListPendingHealth(context.Context, time.Time, time.Duration) ([]Instance, error) {
	return f.pending, f.pendingErr
}

func (f *fakeHealthRepository) UpdateHealth(_ context.Context, id int64, healthy bool, _ time.Time, errMessage string) error {
	f.updates = append(f.updates, healthUpdate{id: id, healthy: healthy, err: errMessage})
	return f.updateError
}

func instanceForURL(t *testing.T, serverURL string) Instance {
	t.Helper()
	parsed, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	port := 0
	if _, err := fmt.Sscanf(parsed.Port(), "%d", &port); err != nil {
		t.Fatal(err)
	}
	return Instance{ID: 1, Scheme: parsed.Scheme, Host: parsed.Hostname(), Port: port, Name: "sandbox-a"}
}

func TestHealthCheckerMarksHealthy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"status":"ok"}}`))
	}))
	defer server.Close()
	repo := &fakeHealthRepository{pending: []Instance{instanceForURL(t, server.URL)}}
	checker := NewHealthChecker(repo, 15*time.Second, time.Second, nil)

	if err := checker.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repo.staleCalls != 1 || len(repo.updates) != 1 || !repo.updates[0].healthy || repo.updates[0].err != "" {
		t.Fatalf("stale=%d updates=%+v", repo.staleCalls, repo.updates)
	}
}

func TestHealthCheckerMarksUnhealthyOn500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	repo := &fakeHealthRepository{pending: []Instance{instanceForURL(t, server.URL)}}
	checker := NewHealthChecker(repo, 15*time.Second, time.Second, nil)

	if err := checker.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 1 || repo.updates[0].healthy || repo.updates[0].err == "" {
		t.Fatalf("updates = %+v", repo.updates)
	}
}

func TestHealthCheckerMarksUnhealthyOnBadEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"code":500,"message":"bad"}`))
	}))
	defer server.Close()
	repo := &fakeHealthRepository{pending: []Instance{instanceForURL(t, server.URL)}}
	checker := NewHealthChecker(repo, 15*time.Second, time.Second, nil)

	if err := checker.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.updates) != 1 || repo.updates[0].healthy {
		t.Fatalf("updates = %+v", repo.updates)
	}
}

func TestHealthCheckerHandleTask(t *testing.T) {
	repo := &fakeHealthRepository{}
	checker := NewHealthChecker(repo, 15*time.Second, time.Second, nil)
	if err := checker.HandleTask(context.Background(), asynq.NewTask(TaskTypeHealthCheck, nil)); err != nil {
		t.Fatal(err)
	}
	if repo.staleCalls != 1 {
		t.Fatalf("stale calls = %d", repo.staleCalls)
	}
}
