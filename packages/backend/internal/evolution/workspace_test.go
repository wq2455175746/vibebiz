package evolution

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/model"
)

type fakeProjects struct {
	items   map[string]*model.Project
	created int
	deleted []string
}

func (f *fakeProjects) Get(_ context.Context, id string) (*model.Project, error) {
	if project, ok := f.items[id]; ok {
		clone := *project
		return &clone, nil
	}
	return nil, errors.New("not found")
}

func (f *fakeProjects) Create(_ context.Context, project *model.Project) error {
	if f.items == nil {
		f.items = map[string]*model.Project{}
	}
	if _, exists := f.items[project.ID]; exists {
		return errors.New("exists")
	}
	clone := *project
	f.items[project.ID] = &clone
	f.created++
	return nil
}

func (f *fakeProjects) Delete(_ context.Context, id string) error {
	delete(f.items, id)
	f.deleted = append(f.deleted, id)
	return nil
}

type fakeInitializer struct {
	sandboxID int64
	calls     int
	err       error
}

func (f *fakeInitializer) InitProject(_ context.Context, _ string) (int64, error) {
	f.calls++
	return f.sandboxID, f.err
}

type fakeFiles struct {
	files map[string]string
	err   error
}

func (f *fakeFiles) WriteFile(_ context.Context, _ string, path, content, _ string) (*client.WriteResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.files == nil {
		f.files = map[string]string{}
	}
	f.files[path] = content
	return &client.WriteResult{Path: path}, nil
}

type fakeMetas struct {
	values  map[string]auth.ProjectMeta
	deleted []string
}

func (f *fakeMetas) Set(_ context.Context, projectID string, meta auth.ProjectMeta) error {
	if f.values == nil {
		f.values = map[string]auth.ProjectMeta{}
	}
	f.values[projectID] = meta
	return nil
}

func (f *fakeMetas) Delete(_ context.Context, projectID string) error {
	delete(f.values, projectID)
	f.deleted = append(f.deleted, projectID)
	return nil
}

func newTestProvisioner(projects *fakeProjects, init *fakeInitializer, files *fakeFiles, metas *fakeMetas) *WorkspaceProvisioner {
	return &WorkspaceProvisioner{
		Projects:    projects,
		Initializer: init,
		Files:       files,
		Metas:       metas,
		Now:         func() time.Time { return time.Unix(1, 0).UTC() },
	}
}

func TestWorkspaceProvisionerCreatesHiddenProjectAndBase(t *testing.T) {
	projects := &fakeProjects{}
	init := &fakeInitializer{sandboxID: 7}
	files := &fakeFiles{}
	metas := &fakeMetas{}
	provisioner := newTestProvisioner(projects, init, files, metas)

	projectID, err := provisioner.Ensure(context.Background(), Viewer{
		ID:          "42",
		InternalID:  42,
		WorkspaceID: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if projectID != "evolve-42" {
		t.Fatalf("projectID = %q, want evolve-42", projectID)
	}
	project := projects.items[projectID]
	if project == nil {
		t.Fatal("project was not created")
	}
	if !project.IsHidden {
		t.Fatal("evolution project should be hidden")
	}
	if project.WorkspaceID != 9 || project.OwnerUserID != 42 {
		t.Fatalf("project scope = %d/%d", project.WorkspaceID, project.OwnerUserID)
	}
	if project.SandboxID == nil || *project.SandboxID != 7 {
		t.Fatalf("sandbox id = %v", project.SandboxID)
	}
	if init.calls != 1 {
		t.Fatalf("init calls = %d, want 1", init.calls)
	}
	if meta := metas.values[projectID]; meta.WorkspaceID != 9 || meta.OwnerUserID != 42 {
		t.Fatalf("meta = %+v", meta)
	}
	if !strings.Contains(files.files["index.html"], "<!doctype html>") {
		t.Fatal("base index.html was not written")
	}
}

func TestWorkspaceProvisionerIsIdempotent(t *testing.T) {
	projects := &fakeProjects{}
	init := &fakeInitializer{sandboxID: 7}
	files := &fakeFiles{}
	metas := &fakeMetas{}
	provisioner := newTestProvisioner(projects, init, files, metas)
	viewer := Viewer{ID: "42", InternalID: 42, WorkspaceID: 9}

	if _, err := provisioner.Ensure(context.Background(), viewer); err != nil {
		t.Fatal(err)
	}
	if _, err := provisioner.Ensure(context.Background(), viewer); err != nil {
		t.Fatal(err)
	}
	if init.calls != 1 {
		t.Fatalf("init calls = %d, want 1", init.calls)
	}
	if projects.created != 1 {
		t.Fatalf("created = %d, want 1", projects.created)
	}
}

func TestWorkspaceProvisionerRequiresWorkspace(t *testing.T) {
	provisioner := newTestProvisioner(&fakeProjects{}, &fakeInitializer{}, &fakeFiles{}, &fakeMetas{})
	_, err := provisioner.Ensure(context.Background(), Viewer{ID: "42"})
	if !errors.Is(err, ErrNoWorkspace) {
		t.Fatalf("err = %v, want ErrNoWorkspace", err)
	}
}

func TestWorkspaceProvisionerCleansUpOnBaseFailure(t *testing.T) {
	projects := &fakeProjects{}
	init := &fakeInitializer{sandboxID: 7}
	files := &fakeFiles{err: errors.New("write failed")}
	metas := &fakeMetas{}
	provisioner := newTestProvisioner(projects, init, files, metas)

	_, err := provisioner.Ensure(context.Background(), Viewer{ID: "42", InternalID: 42, WorkspaceID: 9})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(projects.deleted) != 1 || len(metas.deleted) != 1 {
		t.Fatalf("cleanup = projects %v metas %v", projects.deleted, metas.deleted)
	}
}
