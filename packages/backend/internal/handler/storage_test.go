package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/dao"
	"vibebiz/internal/model"
	"vibebiz/internal/store"
)

type fakeStatsClient struct {
	stats map[string]client.ProjectStats
	errs  map[string]error
}

func (f *fakeStatsClient) ProjectStats(_ context.Context, id string) (client.ProjectStats, error) {
	if err := f.errs[id]; err != nil {
		return client.ProjectStats{}, err
	}
	return f.stats[id], nil
}

func TestStorageSummaryAggregatesCurrentWorkspace(t *testing.T) {
	repo, _, service := settingsFixture(t)
	repo.quota = &dao.UserQuotaRecord{StorageQuotaBytes: 10 << 30}

	projects := store.NewMemoryProjectStore()
	projects.Create(context.Background(), &model.Project{ID: "p1", Name: "one", WorkspaceID: 5})
	projects.Create(context.Background(), &model.Project{ID: "p2", Name: "two", WorkspaceID: 5})
	projects.Create(context.Background(), &model.Project{ID: "p3", Name: "other", WorkspaceID: 9})

	stats := &fakeStatsClient{
		stats: map[string]client.ProjectStats{
			"p1": {FileSizeBytes: 100, FileCount: 4, DirCount: 1, LastModifiedAt: 1793191707598},
		},
		errs: map[string]error{"p2": errors.New("scan failed")},
	}
	h := &StorageHandler{Projects: projects, Sandbox: stats, Account: service}

	e := echo.New()
	e.Use(sessionMiddleware(sessionForTest(5)))
	e.GET("/storage", h.Summary)

	req := httptest.NewRequest(http.MethodGet, "/storage", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var summary StorageSummary
	if err := json.Unmarshal(decodeAuthEnvelope(t, rec).Result, &summary); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if summary.QuotaBytes != 10<<30 || summary.ProjectCount != 2 || summary.UsedBytes != 100 {
		t.Fatalf("summary = %+v", summary)
	}
	if len(summary.Projects) != 2 {
		t.Fatalf("projects = %+v", summary.Projects)
	}
	var available, unavailable bool
	for _, project := range summary.Projects {
		switch project.ID {
		case "p1":
			available = project.Available && project.SizeBytes == 100 && project.FileCount == 4 && project.DirCount == 1 && project.Ratio > 0
			if project.LastModifiedAt.UnixMilli() != 1793191707598 {
				t.Fatalf("last modified = %v", project.LastModifiedAt)
			}
		case "p2":
			unavailable = !project.Available && project.SizeBytes == 0 && project.Ratio == 0
		}
	}
	if !available || !unavailable {
		t.Fatalf("availability = %+v", summary.Projects)
	}
	if repo.updatedUsed != 100 {
		t.Fatalf("storage used synced = %d, want 100", repo.updatedUsed)
	}
}

func TestStorageSummaryRequiresSession(t *testing.T) {
	repo, _, service := settingsFixture(t)
	h := &StorageHandler{Projects: store.NewMemoryProjectStore(), Sandbox: &fakeStatsClient{}, Account: service}
	e := echo.New()
	e.Use(sessionMiddleware(nil))
	e.GET("/storage", h.Summary)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/storage", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if repo.updatedUsed != 0 {
		t.Fatalf("storage synced without session: %d", repo.updatedUsed)
	}
}
