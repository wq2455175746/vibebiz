package clone

import (
	"context"
	"errors"
	"testing"
	"time"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/resource"
)

type fakeProjects struct {
	items  map[string]*model.Project
	covers map[string]string
}

func newFakeProjects() *fakeProjects {
	return &fakeProjects{items: map[string]*model.Project{}, covers: map[string]string{}}
}

func (f *fakeProjects) Create(_ context.Context, project *model.Project) error {
	if _, exists := f.items[project.ID]; exists {
		return errors.New("exists")
	}
	copied := *project
	f.items[project.ID] = &copied
	return nil
}

func (f *fakeProjects) Get(_ context.Context, id string) (*model.Project, error) {
	project, ok := f.items[id]
	if !ok {
		return nil, errTestNotFound
	}
	copied := *project
	return &copied, nil
}

func (f *fakeProjects) ListByWorkspaces(_ context.Context, workspaceIDs []int64) []*model.Project {
	out := []*model.Project{}
	for _, project := range f.items {
		for _, workspaceID := range workspaceIDs {
			if project.WorkspaceID == workspaceID {
				copied := *project
				out = append(out, &copied)
				break
			}
		}
	}
	return out
}

func (f *fakeProjects) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	delete(f.covers, id)
	return nil
}

func (f *fakeProjects) SetCover(_ context.Context, id, key string) error {
	if _, ok := f.items[id]; !ok {
		return errTestNotFound
	}
	f.covers[id] = key
	f.items[id].CoverS3Path = key
	return nil
}

var errTestNotFound = errors.New("test not found")

type fakeInitializer struct{ called int }

func (f *fakeInitializer) InitProject(context.Context, string) (int64, error) {
	f.called++
	return 9, nil
}

type fakeStages struct {
	createdMode string
	deleted     int
}

func (f *fakeStages) CreateInitialForMode(_ context.Context, _ string, mode string) error {
	f.createdMode = mode
	return nil
}

func (f *fakeStages) DeleteByProject(context.Context, string) error {
	f.deleted++
	return nil
}

type fakeHarness struct{ called int }

func (f *fakeHarness) Import(context.Context, string) error {
	f.called++
	return nil
}

type fakeMetas struct {
	set     map[string]auth.ProjectMeta
	deleted int
}

func (f *fakeMetas) Set(_ context.Context, projectID string, meta auth.ProjectMeta) error {
	if f.set == nil {
		f.set = map[string]auth.ProjectMeta{}
	}
	f.set[projectID] = meta
	return nil
}

func (f *fakeMetas) Delete(context.Context, string) error {
	f.deleted++
	return nil
}

type fakeRoles struct{ roles []agentrole.Role }

func (f *fakeRoles) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, nil
}

type fakeResources struct {
	items    map[string]resource.Item
	adoptErr error
	deleted  []string
}

func (f *fakeResources) Adopt(_ context.Context, _, _ int64, _ string, resourceID string) (resource.Item, error) {
	if f.adoptErr != nil {
		return resource.Item{}, f.adoptErr
	}
	item, ok := f.items[resourceID]
	if !ok {
		return resource.Item{}, errors.New("resource missing")
	}
	return item, nil
}

func (f *fakeResources) List(context.Context, string) (resource.ListResult, error) {
	items := make([]resource.Item, 0, len(f.items))
	for _, item := range f.items {
		items = append(items, item)
	}
	return resource.ListResult{Resources: items}, nil
}

func (f *fakeResources) Delete(_ context.Context, _, _ int64, _, resourceID string) error {
	f.deleted = append(f.deleted, resourceID)
	return nil
}

type fakeStore struct {
	sources map[string]Source
	assets  map[string][]Asset
}

func newFakeStore() *fakeStore {
	return &fakeStore{sources: map[string]Source{}, assets: map[string][]Asset{}}
}

func (f *fakeStore) CreateSource(_ context.Context, source Source) error {
	f.sources[source.ProjectID] = source
	return nil
}
func (f *fakeStore) GetSource(_ context.Context, projectID string) (Source, error) {
	source, ok := f.sources[projectID]
	if !ok {
		return Source{}, ErrNotFound
	}
	return source, nil
}
func (f *fakeStore) ListAssets(_ context.Context, projectID string) ([]Asset, error) {
	return append([]Asset(nil), f.assets[projectID]...), nil
}
func (f *fakeStore) CreateAsset(_ context.Context, asset Asset) error {
	f.assets[asset.ProjectID] = append(f.assets[asset.ProjectID], asset)
	return nil
}
func (f *fakeStore) DeleteAsset(_ context.Context, projectID, resourceID string) error {
	out := f.assets[projectID][:0]
	for _, asset := range f.assets[projectID] {
		if asset.ResourceID != resourceID {
			out = append(out, asset)
		}
	}
	f.assets[projectID] = out
	return nil
}
func (f *fakeStore) SetCover(_ context.Context, projectID, resourceID string) error {
	for index, asset := range f.assets[projectID] {
		switch {
		case asset.ResourceID == resourceID:
			f.assets[projectID][index].Role = RoleCover
		case asset.Role == RoleCover:
			f.assets[projectID][index].Role = RoleGallery
		}
	}
	return nil
}
func (f *fakeStore) DeleteByProject(_ context.Context, projectID string) error {
	delete(f.sources, projectID)
	delete(f.assets, projectID)
	return nil
}

type fakeCovers struct{}

func (fakeCovers) PresignGet(context.Context, string, time.Duration) (string, error) {
	return "https://cdn.example/cover", nil
}

func newTestService() (*Service, *fakeProjects, *fakeStages, *fakeHarness, *fakeMetas, *fakeRoles, *fakeResources, *fakeStore) {
	projects := newFakeProjects()
	stages := &fakeStages{}
	harness := &fakeHarness{}
	metas := &fakeMetas{}
	roles := &fakeRoles{roles: []agentrole.Role{
		{ID: 1, RoleCode: "team"}, {ID: 2, RoleCode: "emma"}, {ID: 3, RoleCode: "bob"},
		{ID: 4, RoleCode: "senior"}, {ID: 5, RoleCode: "qa"}, {ID: 6, RoleCode: "alex"},
	}}
	resources := &fakeResources{items: map[string]resource.Item{
		"cover": {ID: "cover", Name: "cover.png", DisplayName: "cover.png", StorageName: "cover.png", ContentType: "image/png", Size: 100, Kind: "image", S3Key: "resources/p/cover.png", Path: ".resources/cover.png"},
		"g1":    {ID: "g1", Name: "g1.png", DisplayName: "g1.png", StorageName: "g1.png", ContentType: "image/png", Size: 100, Kind: "image", S3Key: "resources/p/g1.png", Path: ".resources/g1.png"},
		"zip":   {ID: "zip", Name: "tests.zip", DisplayName: "tests.zip", StorageName: "tests.zip", ContentType: "application/zip", Size: 200, Kind: "document", S3Key: "resources/p/tests.zip", Path: ".resources/tests.zip"},
		"doc":   {ID: "doc", Name: "spec.pdf", DisplayName: "spec.pdf", StorageName: "spec.pdf", ContentType: "application/pdf", Size: 300, Kind: "document", S3Key: "resources/p/spec.pdf", Path: ".resources/spec.pdf"},
	}}
	store := newFakeStore()
	svc := NewService(projects, &fakeInitializer{}, stages, harness, metas, roles, resources, store)
	svc.Covers = fakeCovers{}
	svc.Now = func() time.Time { return time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC) }
	svc.NewID = func() string { return "clone-1" }
	return svc, projects, stages, harness, metas, roles, resources, store
}

func TestCreateCloneProject(t *testing.T) {
	svc, projects, stages, harness, metas, _, resources, store := newTestService()
	view, err := svc.Create(context.Background(), 3, 7, CreateInput{
		Name: "Acme", Description: "clone acme", SourceURL: "https://acme.example.com",
		CoverResourceID: "cover", GalleryResourceIDs: []string{"g1"},
		UnitTestResourceID: "zip", DocsResourceID: "doc",
	})
	if err != nil {
		t.Fatal(err)
	}
	project := projects.items["clone-1"]
	if project == nil || project.Mode != model.ProjectModeClone || project.Name != "Acme" || project.Prompt != "clone acme" {
		t.Fatalf("project = %+v", project)
	}
	if project.WorkspaceID != 3 || project.OwnerUserID != 7 || project.SandboxID == nil || *project.SandboxID != 9 {
		t.Fatalf("project ownership = %+v", project)
	}
	if len(project.AgentRoleIDs) != 6 || project.AgentRoleIDs[0] != 1 {
		t.Fatalf("roles = %v", project.AgentRoleIDs)
	}
	if stages.createdMode != model.ProjectModeClone || harness.called != 1 || metas.set["clone-1"].WorkspaceID != 3 {
		t.Fatalf("stages=%q harness=%d metas=%+v", stages.createdMode, harness.called, metas.set)
	}
	if projects.covers["clone-1"] != "resources/p/cover.png" {
		t.Fatalf("cover = %q", projects.covers["clone-1"])
	}
	if store.sources["clone-1"].SourceURL != "https://acme.example.com" {
		t.Fatalf("source = %+v", store.sources["clone-1"])
	}
	roles := map[string]string{}
	for _, asset := range store.assets["clone-1"] {
		roles[asset.ResourceID] = asset.Role
	}
	if roles["cover"] != RoleCover || roles["g1"] != RoleGallery || roles["zip"] != RoleUnitTest || roles["doc"] != RoleDocs {
		t.Fatalf("roles = %v", roles)
	}
	if len(view.Assets) != 4 || view.SourceURL != "https://acme.example.com" || view.CoverURL == "" {
		t.Fatalf("view = %+v", view)
	}
	if len(resources.deleted) != 0 {
		t.Fatalf("unexpected deletes = %v", resources.deleted)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	svc, _, _, _, _, _, _, _ := newTestService()
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: " ", Description: "x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("blank name err = %v", err)
	}
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x", SourceURL: "ftp://x"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("bad url err = %v", err)
	}
	gallery := make([]string, 9)
	for index := range gallery {
		gallery[index] = "g"
	}
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x", CoverResourceID: "cover", GalleryResourceIDs: gallery}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("too many images err = %v", err)
	}
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x", CoverResourceID: "cover", GalleryResourceIDs: []string{"cover"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate err = %v", err)
	}
}

func TestCreateRollsBackOnAdoptFailure(t *testing.T) {
	svc, projects, stages, _, metas, _, resources, store := newTestService()
	resources.adoptErr = errors.New("adopt failed")
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x", CoverResourceID: "cover"}); err == nil {
		t.Fatal("expected error")
	}
	if len(projects.items) != 0 || stages.deleted != 1 || metas.deleted != 1 {
		t.Fatalf("projects=%d stages.deleted=%d metas.deleted=%d", len(projects.items), stages.deleted, metas.deleted)
	}
	if len(store.assets["clone-1"]) != 0 || len(store.sources) != 0 {
		t.Fatalf("store = %+v", store)
	}
}

func TestListGetDelete(t *testing.T) {
	svc, projects, stages, _, metas, _, _, store := newTestService()
	_, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x"})
	if err != nil {
		t.Fatal(err)
	}
	other := &model.Project{ID: "p2", Name: "other", Mode: model.ProjectModeStandard, WorkspaceID: 3}
	_ = projects.Create(context.Background(), other)

	views, err := svc.List(context.Background(), []int64{3})
	if err != nil || len(views) != 1 || views[0].ID != "clone-1" {
		t.Fatalf("views = %+v err=%v", views, err)
	}
	if _, err := svc.Get(context.Background(), []int64{4}, "clone-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("wrong workspace err = %v", err)
	}
	if _, err := svc.Get(context.Background(), []int64{3}, "p2"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-clone err = %v", err)
	}
	if err := svc.Delete(context.Background(), []int64{3}, "clone-1"); err != nil {
		t.Fatal(err)
	}
	if len(projects.items) != 1 || stages.deleted != 1 || metas.deleted != 1 || len(store.sources) != 0 {
		t.Fatalf("projects=%d deleted=%d metas=%d sources=%d", len(projects.items), stages.deleted, metas.deleted, len(store.sources))
	}
}

func TestAssetsAttachLimitsAndCover(t *testing.T) {
	svc, projects, _, _, _, _, resources, store := newTestService()
	_, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x"})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 9; index++ {
		id := "img-" + string(rune('a'+index))
		resources.items[id] = resource.Item{ID: id, Name: id + ".png", Kind: "image", ContentType: "image/png", Size: 10, S3Key: "resources/p/" + id}
		if _, err := svc.AddAsset(context.Background(), 3, 7, "clone-1", id, RoleGallery); err != nil {
			t.Fatalf("add image %d: %v", index, err)
		}
	}
	resources.items["img-extra"] = resource.Item{ID: "img-extra", Kind: "image", ContentType: "image/png", S3Key: "resources/p/extra"}
	if _, err := svc.AddAsset(context.Background(), 3, 7, "clone-1", "img-extra", RoleGallery); !errors.Is(err, ErrConflict) {
		t.Fatalf("image limit err = %v", err)
	}
	resources.items["zip2"] = resource.Item{ID: "zip2", Kind: "document", ContentType: "application/zip", S3Key: "resources/p/zip2"}
	if _, err := svc.AddAsset(context.Background(), 3, 7, "clone-1", "zip", RoleUnitTest); err != nil {
		t.Fatalf("add zip: %v", err)
	}
	if _, err := svc.AddAsset(context.Background(), 3, 7, "clone-1", "zip2", RoleUnitTest); !errors.Is(err, ErrConflict) {
		t.Fatalf("unit test duplicate err = %v", err)
	}
	if _, err := svc.SetCover(context.Background(), 3, 7, "clone-1", "img-a"); err != nil {
		t.Fatal(err)
	}
	if projects.covers["clone-1"] == "" {
		t.Fatal("cover not set")
	}
	if store.assets["clone-1"][0].Role != RoleCover {
		t.Fatalf("assets = %+v", store.assets["clone-1"])
	}
	if err := svc.DeleteAsset(context.Background(), 3, 7, "clone-1", "img-a"); err != nil {
		t.Fatal(err)
	}
	if projects.covers["clone-1"] == "" {
		t.Fatal("cover should promote to another image")
	}
}

func TestCreateRollsBackAdoptedResourceWhenRoleInvalid(t *testing.T) {
	svc, projects, _, _, _, _, resources, store := newTestService()
	resources.items["not-image"] = resource.Item{ID: "not-image", Name: "a.pdf", Kind: "document", ContentType: "application/pdf", S3Key: "resources/p/a.pdf"}
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x", CoverResourceID: "not-image"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if len(projects.items) != 0 || len(store.assets["clone-1"]) != 0 {
		t.Fatalf("project or assets not rolled back")
	}
	if len(resources.deleted) != 1 || resources.deleted[0] != "not-image" {
		t.Fatalf("deleted = %v", resources.deleted)
	}
}

func TestAddAssetRollsBackInvalidResource(t *testing.T) {
	svc, _, _, _, _, _, resources, store := newTestService()
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x"}); err != nil {
		t.Fatal(err)
	}
	resources.items["not-zip"] = resource.Item{ID: "not-zip", Name: "tests.txt", Kind: "document", ContentType: "text/plain", S3Key: "resources/p/tests.txt"}
	if _, err := svc.AddAsset(context.Background(), 3, 7, "clone-1", "not-zip", RoleUnitTest); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if len(store.assets["clone-1"]) != 0 {
		t.Fatalf("assets = %+v", store.assets["clone-1"])
	}
	if len(resources.deleted) != 1 || resources.deleted[0] != "not-zip" {
		t.Fatalf("deleted = %v", resources.deleted)
	}
}

type fakeSandboxClone struct {
	setups []client.CloneSetup
	err    error
}

func (f *fakeSandboxClone) InitClone(_ context.Context, _ string, setup client.CloneSetup) error {
	if f.err != nil {
		return f.err
	}
	f.setups = append(f.setups, setup)
	return nil
}

func TestCreateSyncsSandbox(t *testing.T) {
	svc, _, _, _, _, _, _, _ := newTestService()
	sandbox := &fakeSandboxClone{}
	svc.SandboxClone = sandbox
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{
		Name: "Acme", Description: "clone acme", SourceURL: "https://acme.example.com",
		CoverResourceID: "cover", GalleryResourceIDs: []string{"g1"},
		UnitTestResourceID: "zip", DocsResourceID: "doc",
	}); err != nil {
		t.Fatal(err)
	}
	if len(sandbox.setups) != 1 {
		t.Fatalf("setups = %d", len(sandbox.setups))
	}
	setup := sandbox.setups[0]
	if setup.Name != "Acme" || setup.SourceURL != "https://acme.example.com" || len(setup.Assets) != 4 {
		t.Fatalf("setup = %+v", setup)
	}
	if setup.Assets[0].Role != RoleCover || setup.Assets[2].StorageName != "tests.zip" {
		t.Fatalf("assets = %+v", setup.Assets)
	}
}

func TestCreateRollsBackWhenSandboxCloneFails(t *testing.T) {
	svc, projects, stages, _, metas, _, resources, store := newTestService()
	svc.SandboxClone = &fakeSandboxClone{err: errors.New("sandbox sync failed")}
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x", CoverResourceID: "cover"}); err == nil {
		t.Fatal("expected error")
	}
	if len(projects.items) != 0 || stages.deleted != 1 || metas.deleted != 1 {
		t.Fatalf("projects=%d stages=%d metas=%d", len(projects.items), stages.deleted, metas.deleted)
	}
	if len(store.sources) != 0 || len(store.assets["clone-1"]) != 0 {
		t.Fatalf("store = %+v", store)
	}
	if len(resources.deleted) != 1 || resources.deleted[0] != "cover" {
		t.Fatalf("deleted = %v", resources.deleted)
	}
}

func TestAssetMutationsSyncSandbox(t *testing.T) {
	svc, _, _, _, _, _, resources, _ := newTestService()
	sandbox := &fakeSandboxClone{}
	svc.SandboxClone = sandbox
	if _, err := svc.Create(context.Background(), 3, 7, CreateInput{Name: "a", Description: "x"}); err != nil {
		t.Fatal(err)
	}
	resources.items["img"] = resource.Item{ID: "img", Name: "img.png", StorageName: "img.png", Kind: "image", ContentType: "image/png", S3Key: "resources/p/img.png"}
	if _, err := svc.AddAsset(context.Background(), 3, 7, "clone-1", "img", RoleGallery); err != nil {
		t.Fatal(err)
	}
	if len(sandbox.setups) != 2 || len(sandbox.setups[1].Assets) != 1 {
		t.Fatalf("setups = %+v", sandbox.setups)
	}
	if err := svc.DeleteAsset(context.Background(), 3, 7, "clone-1", "img"); err != nil {
		t.Fatal(err)
	}
	if len(sandbox.setups) != 3 || len(sandbox.setups[2].Assets) != 0 {
		t.Fatalf("setups after delete = %+v", sandbox.setups)
	}
}
