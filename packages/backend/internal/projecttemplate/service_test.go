package projecttemplate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"vibebiz/internal/client"
	"vibebiz/internal/dao/model"
	appmodel "vibebiz/internal/model"
	"vibebiz/internal/store"
)

type fakeRepo struct {
	items    []model.ProjectTemplate
	topics   []model.ProjectTopic
	created  *model.ProjectTemplate
	archive  string
	fileTree []byte
}

func (f *fakeRepo) Create(_ context.Context, item *model.ProjectTemplate) error {
	item.ID = 1
	f.created = item
	return nil
}
func (f *fakeRepo) Get(_ context.Context, id int64) (*model.ProjectTemplate, error) {
	for index := range f.items {
		if f.items[index].ID == id {
			return &f.items[index], nil
		}
	}
	return nil, errors.New("not found")
}
func (f *fakeRepo) UpdateArchive(_ context.Context, _ int64, path string, tree []byte) error {
	f.archive = path
	f.fileTree = tree
	return nil
}
func (f *fakeRepo) List(context.Context) ([]model.ProjectTemplate, error)      { return f.items, nil }
func (f *fakeRepo) ListAdmin(context.Context) ([]model.ProjectTemplate, error) { return f.items, nil }
func (f *fakeRepo) Update(_ context.Context, _ int64, item *model.ProjectTemplate) error {
	f.created = item
	return nil
}
func (f *fakeRepo) Delete(context.Context, int64) error                     { return nil }
func (f *fakeRepo) AllTopics(context.Context) ([]model.ProjectTopic, error) { return f.topics, nil }
func (f *fakeRepo) CreateTopic(_ context.Context, item *model.ProjectTopic) error {
	f.topics = append(f.topics, *item)
	return nil
}
func (f *fakeRepo) UpdateTopic(_ context.Context, _ int64, item *model.ProjectTopic) error {
	f.topics = append(f.topics, *item)
	return nil
}
func (f *fakeRepo) Topics(context.Context) ([]model.ProjectTopic, error) { return f.topics, nil }
func (f *fakeRepo) Topic(_ context.Context, id int64) (*model.ProjectTopic, error) {
	for index := range f.topics {
		if f.topics[index].ID == id {
			return &f.topics[index], nil
		}
	}
	return nil, errors.New("not found")
}

type fakeSandbox struct {
	archive []byte
}

func (f *fakeSandbox) ExportArchive(context.Context, string) ([]byte, error) { return f.archive, nil }
func (f *fakeSandbox) ListFiles(context.Context, string) ([]client.FileEntry, error) {
	return []client.FileEntry{{Path: "index.html", Type: "file", Size: 10}}, nil
}

type fakeS3 struct{ key string }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func (f *fakeS3) Put(_ context.Context, key, _ string, _ int64, _ io.Reader) error {
	f.key = key
	return nil
}
func (f *fakeS3) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "https://s3/template", nil
}

func newFixture(t *testing.T) (*Service, *fakeRepo, *fakeSandbox, *fakeS3) {
	t.Helper()
	parentID := int64(100)
	childID := int64(101)
	repo := &fakeRepo{topics: []model.ProjectTopic{
		{Base: model.Base{ID: parentID}, TopicCode: "cat_web", TopicName: "网页"},
		{Base: model.Base{ID: childID}, ParentID: &parentID, TopicCode: "common", TopicName: "普通网页"},
	}}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), &appmodel.Project{
		InternalID: 10, ID: "p1", Name: "demo", WorkspaceID: 5, OwnerUserID: 7, State: appmodel.ProjectStateReady,
	})
	sandbox := &fakeSandbox{archive: []byte("archive")}
	s3 := &fakeS3{}
	service := NewService(repo, projects, sandbox, s3, time.Minute)
	return service, repo, sandbox, s3
}

func TestCreateTemplateValidatesScoreAndTopic(t *testing.T) {
	service, _, _, _ := newFixture(t)
	if _, err := service.Create(context.Background(), "p1", CreateInput{TemplateName: "x", TopicID: 101, RecommendScore: 101}, "ops"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	if _, err := service.Create(context.Background(), "p1", CreateInput{TemplateName: "x", TopicID: 100, RecommendScore: 80}, "ops"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("parent topic err = %v", err)
	}
}

func TestCreateTemplateStartsUpload(t *testing.T) {
	service, repo, _, _ := newFixture(t)
	view, err := service.Create(context.Background(), "p1", CreateInput{
		TemplateName: "Demo", Description: "desc", TopicID: 101, RecommendScore: 88,
	}, "ops")
	if err != nil {
		t.Fatal(err)
	}
	if view.ID != 1 || repo.created == nil || repo.created.RecommendScore != 88 || repo.created.Status != 0 {
		t.Fatalf("view = %+v created = %+v", view, repo.created)
	}
}

func TestUploadStoresArchiveAndTree(t *testing.T) {
	service, repo, _, s3 := newFixture(t)
	repo.items = []model.ProjectTemplate{{Base: model.Base{ID: 1}, Status: 0}}
	if err := service.Upload(context.Background(), 1, "p1"); err != nil {
		t.Fatal(err)
	}
	if s3.key != "templates/1.tar.gz" || repo.archive != s3.key || len(repo.fileTree) == 0 {
		t.Fatalf("key = %q archive = %q tree = %s", s3.key, repo.archive, repo.fileTree)
	}
}

func TestListFiltersByCategoryAndKeyword(t *testing.T) {
	parentID := int64(100)
	service, repo, _, _ := newFixture(t)
	repo.items = []model.ProjectTemplate{
		{Base: model.Base{ID: 1}, TemplateName: "网站", TopicID: 101, Status: 1, RecommendScore: 90},
		{Base: model.Base{ID: 2}, TemplateName: "游戏", TopicID: 101, Status: 1, RecommendScore: 80},
	}
	page, err := service.List(context.Background(), Query{Keyword: "网站", ParentTopicID: parentID, Page: 1, PageSize: 12})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].TemplateName != "网站" || page.Items[0].ParentTopicName != "网页" {
		t.Fatalf("page = %+v", page)
	}
}

func TestDownloadRequiresReadyTemplate(t *testing.T) {
	service, repo, _, _ := newFixture(t)
	repo.items = []model.ProjectTemplate{{Base: model.Base{ID: 1}, Status: 0}}
	if _, err := service.Download(context.Background(), 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
	repo.items[0].Status = 1
	repo.items[0].TemplateS3Path = "templates/1.tar.gz"
	service.HTTP.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte("archive")))}, nil
	})
	if data, err := service.Download(context.Background(), 1); err != nil || string(data) != "archive" {
		t.Fatalf("data = %q err = %v", data, err)
	}
}

func TestCreateTopicValidatesParentAndCode(t *testing.T) {
	service, _, _, _ := newFixture(t)
	input := TopicInput{Code: "game_action", Name: "动作游戏", Status: 1}
	parent := int64(100)
	input.ParentID = &parent
	topic, err := service.CreateTopic(context.Background(), input, "ops")
	if err != nil {
		t.Fatal(err)
	}
	if topic.Code != "game_action" || topic.ParentID == nil || *topic.ParentID != parent {
		t.Fatalf("topic = %+v", topic)
	}
	bad := TopicInput{Code: "Bad Code", Name: "x", Status: 1}
	if _, err := service.CreateTopic(context.Background(), bad, "ops"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestAdminListIncludesDisabledTemplates(t *testing.T) {
	service, repo, _, _ := newFixture(t)
	repo.items = []model.ProjectTemplate{{
		Base: model.Base{ID: 1}, TemplateName: "disabled", TopicID: 101, Status: 0,
	}}
	page, err := service.AdminList(context.Background(), Query{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || page.Items[0].Status != 0 {
		t.Fatalf("page = %+v", page)
	}
}

func TestUpdateTemplateValidatesScore(t *testing.T) {
	service, repo, _, _ := newFixture(t)
	repo.items = []model.ProjectTemplate{{Base: model.Base{ID: 1}, TemplateName: "demo", TopicID: 101, Status: 1}}
	_, err := service.UpdateTemplate(context.Background(), 1, UpdateInput{
		TemplateName: "demo", TopicID: 101, RecommendScore: 200, Status: 1,
	}, "ops")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildTopicsAttachesChildrenAcrossManyParents(t *testing.T) {
	parent1, parent2, parent3 := int64(1), int64(2), int64(3)
	items := []model.ProjectTopic{
		{Base: model.Base{ID: parent1}, TopicCode: "p1", TopicName: "P1"},
		{Base: model.Base{ID: parent2}, TopicCode: "p2", TopicName: "P2"},
		{Base: model.Base{ID: parent3}, TopicCode: "p3", TopicName: "P3"},
		{Base: model.Base{ID: 11}, ParentID: &parent1, TopicCode: "c1", TopicName: "C1"},
		{Base: model.Base{ID: 21}, ParentID: &parent2, TopicCode: "c2", TopicName: "C2"},
		{Base: model.Base{ID: 31}, ParentID: &parent3, TopicCode: "c3", TopicName: "C3"},
	}
	topics := buildTopics(items)
	if len(topics) != 3 {
		t.Fatalf("topics = %+v", topics)
	}
	for index, want := range []string{"C1", "C2", "C3"} {
		if len(topics[index].Children) != 1 || topics[index].Children[0].Name != want {
			t.Fatalf("parent %d children = %+v", index, topics[index].Children)
		}
	}
}
