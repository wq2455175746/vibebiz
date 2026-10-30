package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/dao"
	"vibebiz/internal/model"
	"vibebiz/internal/storagesync"
	"vibebiz/internal/store"
)

type fakeStorageReader struct {
	records []storagesync.Record
	err     error
}

func (f *fakeStorageReader) ListByWorkspace(context.Context, int64) ([]storagesync.Record, error) {
	return f.records, f.err
}

func TestStorageSummaryAggregatesCurrentWorkspace(t *testing.T) {
	repo, _, service := settingsFixture(t)
	repo.quota = &dao.UserQuotaRecord{StorageQuotaBytes: 10 << 30}

	projects := store.NewMemoryProjectStore()
	projects.Create(context.Background(), &model.Project{InternalID: 1, ID: "p1", Name: "one", WorkspaceID: 5})
	projects.Create(context.Background(), &model.Project{InternalID: 2, ID: "p2", Name: "two", WorkspaceID: 5})
	projects.Create(context.Background(), &model.Project{InternalID: 3, ID: "p3", Name: "other", WorkspaceID: 9})

	modified := time.UnixMilli(1793191707598).UTC()
	reader := &fakeStorageReader{records: []storagesync.Record{{
		ProjectID: 1, WorkspaceID: 5, UserID: 7,
		FileSizeBytes: 100, FileCount: 4, DirCount: 1, LastModifiedAt: &modified,
	}}}
	h := &StorageHandler{Projects: projects, Storage: reader, Account: service}

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
}

func TestStorageSummaryRequiresSession(t *testing.T) {
	_, _, service := settingsFixture(t)
	h := &StorageHandler{Projects: store.NewMemoryProjectStore(), Storage: &fakeStorageReader{}, Account: service}
	e := echo.New()
	e.Use(sessionMiddleware(nil))
	e.GET("/storage", h.Summary)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/storage", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}
