package handler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
	"vibebiz/internal/resource"
)

type uploadRepo struct {
	item *model.ProjectResource
}

func (r *uploadRepo) Create(context.Context, *model.ProjectResource) error { return nil }

func (r *uploadRepo) GetByResourceID(_ context.Context, id string) (*model.ProjectResource, error) {
	if r.item == nil || r.item.ResourceID != id {
		return nil, dao.ErrNotFound
	}
	copied := *r.item
	return &copied, nil
}

func (r *uploadRepo) ListByProject(context.Context, string) ([]model.ProjectResource, error) {
	return nil, nil
}

func (r *uploadRepo) ListReadyByProject(context.Context, string) ([]model.ProjectResource, error) {
	return nil, nil
}

func (r *uploadRepo) SumReadyByProject(context.Context, string) (int64, error) { return 0, nil }

func (r *uploadRepo) Update(context.Context, *model.ProjectResource) error { return nil }

func (r *uploadRepo) SoftDelete(context.Context, string) error { return nil }

type uploadS3 struct {
	body []byte
}

func (s *uploadS3) PresignPut(context.Context, string, string, int64, time.Duration) (string, map[string]string, error) {
	return "", nil, nil
}

func (s *uploadS3) Head(context.Context, string) (int64, string, error) { return 0, "", nil }

func (s *uploadS3) Copy(context.Context, string, string, string) error { return nil }

func (s *uploadS3) Get(context.Context, string) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader(nil)), 0, nil
}

func (s *uploadS3) Put(_ context.Context, _ string, _ string, _ int64, body io.Reader) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.body = data
	return nil
}

func (s *uploadS3) Delete(context.Context, string) error { return nil }

func TestResourceUploadStagingStreamsBody(t *testing.T) {
	payload := []byte("zip-bytes")
	repo := &uploadRepo{item: &model.ProjectResource{
		ResourceID:   "r1",
		WorkspaceID:  3,
		UserID:       7,
		OriginalName: "game.zip",
		ContentType:  "application/zip",
		SizeBytes:    int64(len(payload)),
		Purpose:      resource.PurposeChannelBuild,
		S3StagingKey: "staging/key.zip",
		Status:       "staging",
	}}
	s3 := &uploadS3{}
	svc := resource.NewService(repo, s3, nil)
	h := NewResourceHandler(svc, nil)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/dp/api/v1/resources/staging/:resourceId/upload", h.UploadStaging, sessionForTest(3))

	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/resources/staging/r1/upload", bytes.NewReader(payload))
	req.Header.Set(echo.HeaderContentType, "application/zip")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Equal(s3.body, payload) {
		t.Fatalf("uploaded body = %q, want %q", s3.body, payload)
	}
}
