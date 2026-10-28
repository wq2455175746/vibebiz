package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/account"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type ProjectStatsClient interface {
	ProjectStats(ctx context.Context, id string) (client.ProjectStats, error)
}

type StorageHandler struct {
	Projects store.ProjectStore
	Sandbox  ProjectStatsClient
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
	projects := h.Projects.ListByWorkspaces([]int64{workspace.ID})
	summary := StorageSummary{
		QuotaBytes:   quota.StorageQuotaBytes,
		ProjectCount: len(projects),
		Projects:     make([]StorageProject, 0, len(projects)),
	}
	for _, project := range projects {
		summary.Projects = append(summary.Projects, h.projectStorage(ctx, project, quota.StorageQuotaBytes))
		summary.UsedBytes += summary.Projects[len(summary.Projects)-1].SizeBytes
	}
	if err := h.Account.SyncStorageUsed(ctx, session.UserID, summary.UsedBytes); err != nil {
		return settingsError(c, err)
	}
	return response.OK(c, summary)
}

func (h *StorageHandler) projectStorage(ctx context.Context, project *model.Project, quotaBytes int64) StorageProject {
	item := StorageProject{ID: project.ID, Name: project.Name}
	stats, err := h.Sandbox.ProjectStats(ctx, project.ID)
	if err != nil {
		return item
	}
	item.Available = true
	item.SizeBytes = stats.FileSizeBytes
	item.FileCount = stats.FileCount
	item.DirCount = stats.DirCount
	item.LastModifiedAt = time.UnixMilli(stats.LastModifiedAt).UTC()
	if stats.LastModifiedAt <= 0 {
		item.LastModifiedAt = time.Time{}
	}
	if quotaBytes > 0 {
		item.Ratio = float64(stats.FileSizeBytes) / float64(quotaBytes)
	}
	return item
}
