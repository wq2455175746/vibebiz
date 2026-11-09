package businesscard

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type fakeRepo struct {
	card             *CardRecord
	getOrCreateErr   error
	items            []ItemRecord
	addItemErr       error
	removeItemErr    error
	setFeaturedErr   error
	clearFeaturedErr error
	publishErr       error
}

func (f *fakeRepo) GetOrCreate(context.Context, int64) (*CardRecord, error) {
	if f.getOrCreateErr != nil {
		return nil, f.getOrCreateErr
	}
	if f.card == nil {
		f.card = &CardRecord{ID: 1, PublicID: "card-1", UserID: 7, OwnerName: "alice", Tags: []string{}}
	}
	f.card.Items = append([]ItemRecord(nil), f.items...)
	return f.card, nil
}
func (f *fakeRepo) GetByUser(context.Context, int64) (*CardRecord, error) {
	return f.GetOrCreate(context.Background(), 0)
}
func (f *fakeRepo) GetByPublicID(context.Context, string) (*CardRecord, error) {
	if f.card == nil {
		return nil, ErrNotFound
	}
	f.card.Items = append([]ItemRecord(nil), f.items...)
	return f.card, nil
}
func (f *fakeRepo) UpdateProfile(_ context.Context, _ int64, headline string, tags []string) error {
	f.card.Headline = headline
	f.card.Tags = tags
	return nil
}
func (f *fakeRepo) ListItems(context.Context, int64) ([]ItemRecord, error) { return f.items, nil }
func (f *fakeRepo) AddItem(_ context.Context, _ int64, projectID string, sortOrder int) error {
	if f.addItemErr != nil {
		return f.addItemErr
	}
	f.items = append(f.items, ItemRecord{ProjectID: projectID, SortOrder: sortOrder})
	return nil
}
func (f *fakeRepo) RemoveItem(_ context.Context, _ int64, projectID string) error {
	if f.removeItemErr != nil {
		return f.removeItemErr
	}
	out := f.items[:0]
	for _, item := range f.items {
		if item.ProjectID != projectID {
			out = append(out, item)
		}
	}
	f.items = out
	return nil
}
func (f *fakeRepo) SetFeaturedProject(_ context.Context, _ int64, projectID string) error {
	if f.setFeaturedErr != nil {
		return f.setFeaturedErr
	}
	f.card.FeaturedType = "project"
	f.card.FeaturedProjectID = projectID
	return nil
}
func (f *fakeRepo) SetFeaturedFile(_ context.Context, _ int64, key, name, fileType string, size int64) error {
	f.card.FeaturedType = fileType
	f.card.FeaturedS3Key = key
	f.card.FeaturedFileName = name
	f.card.FeaturedFileSize = size
	return nil
}
func (f *fakeRepo) ClearFeatured(context.Context, int64) error {
	if f.clearFeaturedErr != nil {
		return f.clearFeaturedErr
	}
	f.card.FeaturedType = ""
	f.card.FeaturedProjectID = ""
	f.card.FeaturedS3Key = ""
	return nil
}
func (f *fakeRepo) Publish(_ context.Context, _ int64, at time.Time) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	f.card.Published = true
	f.card.PublishedAt = &at
	return nil
}
func (f *fakeRepo) SetHidden(_ context.Context, _ int64, hidden bool) error {
	f.card.Hidden = hidden
	return nil
}
func (f *fakeRepo) Delete(context.Context, int64) (string, error) {
	return f.card.FeaturedS3Key, nil
}
func (f *fakeRepo) Discover(context.Context, int, int) ([]*CardRecord, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) AdminList(context.Context, string, int, int) ([]*CardRecord, int64, error) {
	return nil, 0, nil
}
func (f *fakeRepo) FindByProject(context.Context, string) (*CardRecord, error) {
	return f.card, nil
}
func (f *fakeRepo) OwnerProfile(context.Context, int64) (*OwnerProfile, error) {
	return &OwnerProfile{UserID: 7, Username: "alice"}, nil
}

type fakeProjects struct{ items map[string]*model.Project }

func (f fakeProjects) Get(_ context.Context, id string) (*model.Project, error) {
	project, ok := f.items[id]
	if !ok {
		return nil, errors.New("project not found")
	}
	return project, nil
}

type fakePublisher struct{ items []publish.Published }

func (f fakePublisher) List(context.Context) ([]publish.Published, error) { return f.items, nil }

type fakeS3 struct {
	putKey  string
	deleted []string
	presign string
}

func (f *fakeS3) Put(_ context.Context, key, _ string, _ int64, _ io.Reader) error {
	f.putKey = key
	return nil
}
func (f *fakeS3) PresignGet(context.Context, string, time.Duration) (string, error) {
	return f.presign, nil
}
func (f *fakeS3) Delete(_ context.Context, key string) error {
	f.deleted = append(f.deleted, key)
	return nil
}

func newTestService(repo Repository) (*Service, fakeProjects, fakePublisher, *fakeS3) {
	projects := fakeProjects{items: map[string]*model.Project{
		"p1": {ID: "p1", Name: "作品一", WorkspaceID: 10, OwnerUserID: 7, PreviewPath: "index.html", CoverURL: "https://cdn/p1.png"},
	}}
	published := fakePublisher{items: []publish.Published{{ProjectID: "p1", Title: "作品一", Description: "介绍"}}}
	s3 := &fakeS3{presign: "https://s3/card.pdf"}
	svc := NewService(repo, projects, published, s3, "https://play.example.com", 15*time.Minute)
	svc.Now = func() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC) }
	svc.NewID = func() string { return "fixed-id" }
	return svc, projects, published, s3
}

func TestGetMineResolvesProjectItems(t *testing.T) {
	repo := &fakeRepo{items: []ItemRecord{{ProjectID: "p1", SortOrder: 0}}}
	svc, _, _, _ := newTestService(repo)
	card, err := svc.GetMine(context.Background(), 7, []int64{10})
	if err != nil {
		t.Fatal(err)
	}
	if card.ID != "card-1" || card.OwnerName != "alice" {
		t.Fatalf("card = %+v", card)
	}
	if len(card.Items) != 1 || card.Items[0].ShareURL != "https://play.example.com/p/p1/" || card.Items[0].Type != "html" {
		t.Fatalf("items = %+v", card.Items)
	}
}

func TestAddProjectRejectsUnpublished(t *testing.T) {
	repo := &fakeRepo{}
	svc, projects, published, _ := newTestService(repo)
	projects.items["p2"] = &model.Project{ID: "p2", WorkspaceID: 10}
	_ = published
	_, err := svc.AddProject(context.Background(), 7, []int64{10}, "p2")
	if !errors.Is(err, ErrInvalidProject) {
		t.Fatalf("err = %v", err)
	}
}

func TestAddProjectRejectsSixthItem(t *testing.T) {
	repo := &fakeRepo{items: []ItemRecord{{ProjectID: "p1"}}}
	repo.items = append(repo.items, ItemRecord{ProjectID: "p1"}, ItemRecord{ProjectID: "p1"}, ItemRecord{ProjectID: "p1"}, ItemRecord{ProjectID: "p1"})
	svc, _, _, _ := newTestService(repo)
	_, err := svc.AddProject(context.Background(), 7, []int64{10}, "p1")
	if !errors.Is(err, ErrItemLimit) {
		t.Fatalf("err = %v", err)
	}
}

func TestPublishRequiresVisibleProject(t *testing.T) {
	repo := &fakeRepo{items: []ItemRecord{{ProjectID: "p2"}}}
	svc, _, _, _ := newTestService(repo)
	_, err := svc.Publish(context.Background(), 7, []int64{10})
	if !errors.Is(err, ErrNoItems) {
		t.Fatalf("err = %v", err)
	}
}

var _ = bytes.NewReader
var _ = strings.TrimSpace

func TestUploadFeaturedRejectsMismatchedMagic(t *testing.T) {
	repo := &fakeRepo{}
	svc, _, _, _ := newTestService(repo)
	body := strings.NewReader("<html>not a pdf</html>")
	_, err := svc.UploadFeatured(context.Background(), 7, []int64{10}, "application/pdf", "x.pdf", int64(body.Len()), body)
	if !errors.Is(err, ErrInvalidFile) {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadFeaturedStoresS3KeyAndDeletesOld(t *testing.T) {
	repo := &fakeRepo{card: &CardRecord{ID: 1, PublicID: "card-1", UserID: 7, FeaturedS3Key: "cards/7/old.pdf"}}
	svc, _, _, s3 := newTestService(repo)
	data := []byte("%PDF-1.4\n")
	_, err := svc.UploadFeatured(context.Background(), 7, []int64{10}, "application/pdf", "resume.pdf", int64(len(data)), bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if s3.putKey != "cards/7/fixed-id.pdf" {
		t.Fatalf("key = %q", s3.putKey)
	}
	if len(s3.deleted) != 1 || s3.deleted[0] != "cards/7/old.pdf" {
		t.Fatalf("deleted = %+v", s3.deleted)
	}
}

func TestUploadFeaturedRejectsTooLarge(t *testing.T) {
	repo := &fakeRepo{}
	svc, _, _, _ := newTestService(repo)
	svc.MaxFeaturedBytes = 4
	_, err := svc.UploadFeatured(context.Background(), 7, []int64{10}, "application/pdf", "x.pdf", 5, strings.NewReader("%PDF-"))
	if !errors.Is(err, ErrFileTooLarge) {
		t.Fatalf("err = %v", err)
	}
}
