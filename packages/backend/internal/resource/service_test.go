package resource

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
)

type fakeS3 struct {
	contentType string
	size        int64
	copied      string
	deleted     []string
}

func (f *fakeS3) PresignPut(_ context.Context, key, contentType string, _ int64, _ time.Duration) (string, map[string]string, error) {
	f.contentType = contentType
	return "https://s3.example/put", map[string]string{"Content-Type": contentType}, nil
}

func (f *fakeS3) Head(_ context.Context, _ string) (int64, string, error) {
	return f.size, f.contentType, nil
}

func (f *fakeS3) Copy(_ context.Context, _ string, dst string, _ string) error {
	f.copied = dst
	return nil
}

func (f *fakeS3) Get(_ context.Context, _ string) (io.ReadCloser, int64, error) {
	return io.NopCloser(bytes.NewReader([]byte("hello"))), f.size, nil
}

func (f *fakeS3) Put(_ context.Context, _ string, _ string, _ int64, _ io.Reader) error {
	return nil
}

func (f *fakeS3) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

type fakeRepo struct {
	items map[string]*model.ProjectResource
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: map[string]*model.ProjectResource{}} }

func (f *fakeRepo) Create(_ context.Context, item *model.ProjectResource) error {
	copied := *item
	f.items[item.ResourceID] = &copied
	return nil
}

func (f *fakeRepo) GetByResourceID(_ context.Context, id string) (*model.ProjectResource, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, dao.ErrNotFound
	}
	copied := *item
	return &copied, nil
}

func (f *fakeRepo) ListByProject(_ context.Context, projectID string) ([]model.ProjectResource, error) {
	var out []model.ProjectResource
	for _, item := range f.items {
		if item.ProjectPublicID != nil && *item.ProjectPublicID == projectID && item.Status != "deleted" {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeRepo) ListReadyByProject(_ context.Context, projectID string) ([]model.ProjectResource, error) {
	var out []model.ProjectResource
	for _, item := range f.items {
		if item.ProjectPublicID != nil && *item.ProjectPublicID == projectID && item.Status == "ready" {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeRepo) SumReadyByProject(ctx context.Context, projectID string) (int64, error) {
	items, _ := f.ListReadyByProject(ctx, projectID)
	var total int64
	for _, item := range items {
		total += item.SizeBytes
	}
	return total, nil
}

func (f *fakeRepo) Update(_ context.Context, item *model.ProjectResource) error {
	copied := *item
	f.items[item.ResourceID] = &copied
	return nil
}

func (f *fakeRepo) SoftDelete(_ context.Context, id string) error {
	if item, ok := f.items[id]; ok {
		item.Status = "deleted"
		return nil
	}
	return dao.ErrNotFound
}

type fakeSandbox struct {
	written bool
	meta    ResourceMeta
	deleted string
	body    string
}

func (f *fakeSandbox) WriteResource(_ context.Context, _ string, meta ResourceMeta, body io.Reader, _ int64) error {
	data, _ := io.ReadAll(body)
	f.written = true
	f.meta = meta
	f.body = string(data)
	return nil
}

func (f *fakeSandbox) DeleteResource(_ context.Context, _, id string) error {
	f.deleted = id
	return nil
}

func newTestService() (*Service, *fakeRepo, *fakeS3, *fakeSandbox) {
	repo := newFakeRepo()
	s3 := &fakeS3{contentType: "text/plain", size: 22}
	sandbox := &fakeSandbox{}
	return NewService(repo, s3, sandbox), repo, s3, sandbox
}

func TestPresignUsesOpaqueStorageName(t *testing.T) {
	svc, repo, _, _ := newTestService()
	result, err := svc.Presign(context.Background(), 1, 2, PresignInput{Filename: "知识.txt", ContentType: "text/plain", Size: 22})
	if err != nil {
		t.Fatal(err)
	}
	item := repo.items[result.ResourceID]
	if item == nil || item.OriginalName != "知识.txt" || item.StorageName != result.ResourceID+".txt" {
		t.Fatalf("item = %+v", item)
	}
	if !strings.Contains(result.Key, result.ResourceID+".txt") {
		t.Fatalf("key = %q", result.Key)
	}
}

func TestAdoptCopiesAndWritesResource(t *testing.T) {
	svc, repo, s3, sandbox := newTestService()
	presign, err := svc.Presign(context.Background(), 1, 2, PresignInput{Filename: "需求.txt", ContentType: "text/plain", Size: 22})
	if err != nil {
		t.Fatal(err)
	}
	item, err := svc.Adopt(context.Background(), 1, 2, "p1", presign.ResourceID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Name != "需求.txt" || item.Path != ".resources/"+presign.ResourceID+".txt" {
		t.Fatalf("item = %+v", item)
	}
	if s3.copied != "resources/p1/"+presign.ResourceID+"/"+presign.ResourceID+".txt" {
		t.Fatalf("copied = %q", s3.copied)
	}
	if !sandbox.written || sandbox.meta.S3Key != s3.copied || sandbox.body != "hello" {
		t.Fatalf("sandbox = %+v", sandbox)
	}
	if repo.items[presign.ResourceID].Status != "ready" {
		t.Fatalf("status = %s", repo.items[presign.ResourceID].Status)
	}
}

func TestAdoptIsIdempotent(t *testing.T) {
	svc, repo, _, _ := newTestService()
	presign, _ := svc.Presign(context.Background(), 1, 2, PresignInput{Filename: "a.txt", ContentType: "text/plain", Size: 22})
	item := repo.items[presign.ResourceID]
	project := "p1"
	item.ProjectPublicID = &project
	item.DisplayName = "a.txt"
	item.S3Key = "resources/p1/a.txt"
	item.Status = "ready"

	got, err := svc.Adopt(context.Background(), 1, 2, "p1", presign.ResourceID)
	if err != nil || got.ID != presign.ResourceID {
		t.Fatalf("got = %+v err = %v", got, err)
	}
}

func TestPresignRejectsUnsupportedFile(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.Presign(context.Background(), 1, 2, PresignInput{Filename: "run.exe", ContentType: "application/octet-stream", Size: 22})
	if err != ErrInvalidFile {
		t.Fatalf("err = %v", err)
	}
}
