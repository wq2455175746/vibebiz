package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/account"
	"vibebiz/internal/auth"
	"vibebiz/internal/response"
	"vibebiz/internal/storagesync"
	"vibebiz/internal/store"
)

type StorageReader interface {
	ListByWorkspace(ctx context.Context, workspaceID int64) ([]storagesync.Record, error)
}

type StorageHandler struct {
	Projects store.ProjectStore
	Storage  StorageReader
	Account  *account.Service
}

type StorageProject struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	SizeBytes      int64     `json:"sizeBytes"`
	Ratio          float64   `json:"ratio"`
	FileCount      int64     `json:"fileCount"`
	DirCount       int64     `json:"dirCount"`
	LastModifiedAt time.Time `json:"lastModifiedAt"`
	Available      bool      `json:"available"`
}

type StorageSummary struct {
	UsedBytes    int64            `json:"usedBytes"`
	QuotaBytes   int64            `json:"quotaBytes"`
	ProjectCount int              `json:"projectCount"`
	Projects     []StorageProject `json:"projects"`
}

func (h *StorageHandler) Summary(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	ctx := c.Request().Context()
	workspace, err := h.Account.CurrentWorkspace(ctx, session)
	if err != nil {
		return settingsError(c, err)
	}
	quota, err := h.Account.Quota(ctx, session.UserID)
	if err != nil {
		return settingsError(c, err)
	}
	projects := h.Projects.ListByWorkspaces(ctx, []int64{workspace.ID})
	records, err := h.Storage.ListByWorkspace(ctx, workspace.ID)
	if err != nil {
		return settingsError(c, err)
	}
	byProject := make(map[int64]storagesync.Record, len(records))
	for _, record := range records {
		byProject[record.ProjectID] = record
	}
	summary := StorageSummary{
		QuotaBytes:   quota.StorageQuotaBytes,
		ProjectCount: len(projects),
		Projects:     make([]StorageProject, 0, len(projects)),
	}
	for _, project := range projects {
		item := StorageProject{ID: project.ID, Name: project.Name}
		if record, ok := byProject[project.InternalID]; ok {
			item.Available = true
			item.SizeBytes = record.FileSizeBytes
			item.FileCount = record.FileCount
			item.DirCount = record.DirCount
			if record.LastModifiedAt != nil {
				item.LastModifiedAt = *record.LastModifiedAt
			}
			if quota.StorageQuotaBytes > 0 {
				item.Ratio = float64(record.FileSizeBytes) / float64(quota.StorageQuotaBytes)
			}
		}
		summary.Projects = append(summary.Projects, item)
		summary.UsedBytes += item.SizeBytes
	}
	return response.OK(c, summary)
}
