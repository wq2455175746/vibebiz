package sandboxregistry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeRepository struct {
	instances  []Instance
	binding    Binding
	selectSeq  []Instance
	selectErr  error
	selectArgs [][]int64
	updateErr  error
	updates    []bindingUpdate
}

type bindingUpdate struct {
	projectID int64
	oldID     *int64
	newID     int64
}

func (f *fakeRepository) UpsertHeartbeat(context.Context, Heartbeat, time.Time) (Instance, error) {
	return Instance{}, nil
}
func (f *fakeRepository) List(context.Context) ([]Instance, error) { return f.instances, nil }
func (f *fakeRepository) Get(context.Context, int64) (Instance, error) {
	return Instance{}, nil
}
func (f *fakeRepository) Select(_ context.Context, _ time.Time, _ time.Duration, exclude []int64) (Instance, error) {
	f.selectArgs = append(f.selectArgs, append([]int64(nil), exclude...))
	if f.selectErr != nil {
		return Instance{}, f.selectErr
	}
	if len(f.selectSeq) > 0 {
		instance := f.selectSeq[0]
		f.selectSeq = f.selectSeq[1:]
		return instance, nil
	}
	if len(f.instances) > 0 {
		return f.instances[0], nil
	}
	return Instance{}, ErrNoHealthySandbox
}
func (f *fakeRepository) MarkStaleOffline(context.Context, time.Time, time.Duration) error {
	return nil
}
func (f *fakeRepository) ListPendingHealth(context.Context, time.Time, time.Duration) ([]Instance, error) {
	return f.instances, nil
}
func (f *fakeRepository) UpdateHealth(context.Context, int64, bool, time.Time, string) error {
	return nil
}
func (f *fakeRepository) GetProjectBinding(context.Context, string) (Binding, error) {
	return f.binding, nil
}
func (f *fakeRepository) UpdateProjectBinding(_ context.Context, projectID int64, oldID *int64, newID int64) error {
	if f.updateErr != nil {
		err := f.updateErr
		f.updateErr = nil
		return err
	}
	f.updates = append(f.updates, bindingUpdate{projectID: projectID, oldID: oldID, newID: newID})
	f.binding.SandboxID = &newID
	return nil
}

type fakeInitializer struct {
	failures map[string]bool
	calls    []string
}

func (f *fakeInitializer) InitProject(_ context.Context, baseURL, _ string) error {
	f.calls = append(f.calls, baseURL)
	if f.failures[baseURL] {
		return errors.New("init failed")
	}
	return nil
}

func TestProvisionerSkipsFailedNode(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{selectSeq: []Instance{
		{ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000},
		{ID: 2, Scheme: "http", Host: "sandbox-b", Port: 8000},
	}}
	init := &fakeInitializer{failures: map[string]bool{"http://sandbox-a:8000": true}}
	provisioner := NewProvisioner(NewSelector(repo, 15*time.Second, func() time.Time { return now }), init, 3)

	id, err := provisioner.InitProject(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if id != 2 {
		t.Fatalf("id = %d, want 2", id)
	}
	if len(init.calls) != 2 || init.calls[0] != "http://sandbox-a:8000" || init.calls[1] != "http://sandbox-b:8000" {
		t.Fatalf("calls = %v", init.calls)
	}
	if len(repo.selectArgs) != 2 || len(repo.selectArgs[1]) != 1 || repo.selectArgs[1][0] != 1 {
		t.Fatalf("select exclusions = %+v", repo.selectArgs)
	}
}

func TestProvisionerReturnsLastInitError(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{selectSeq: []Instance{
		{ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000},
	}}
	init := &fakeInitializer{failures: map[string]bool{"http://sandbox-a:8000": true}}
	provisioner := NewProvisioner(NewSelector(repo, 15*time.Second, func() time.Time { return now }), init, 1)

	if _, err := provisioner.InitProject(context.Background(), "p1"); err == nil {
		t.Fatal("expected init error")
	}
}

func TestResolverUsesHealthyBinding(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)
	bound := int64(1)
	repo := &fakeRepository{binding: Binding{
		ProjectInternalID: 42,
		SandboxID:         &bound,
		Instance: &Instance{
			ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
			Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat,
		},
	}}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	baseURL, err := resolver.ResolveBaseURL(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != "http://sandbox-a:8000" || len(repo.updates) != 0 {
		t.Fatalf("baseURL = %s, updates = %+v", baseURL, repo.updates)
	}
}

func TestResolverRebindsOfflineBinding(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	oldID := int64(1)
	oldHeartbeat := now.Add(-time.Minute)
	repo := &fakeRepository{
		binding: Binding{
			ProjectInternalID: 42,
			SandboxID:         &oldID,
			Instance: &Instance{
				ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
				Status: StatusOffline, HealthStatus: HealthUnhealthy, LastHeartbeatAt: &oldHeartbeat,
			},
		},
		selectSeq: []Instance{{ID: 2, Scheme: "http", Host: "sandbox-b", Port: 8000}},
	}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	baseURL, err := resolver.ResolveBaseURL(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != "http://sandbox-b:8000" {
		t.Fatalf("baseURL = %s", baseURL)
	}
	if len(repo.updates) != 1 || repo.updates[0].oldID == nil || *repo.updates[0].oldID != 1 || repo.updates[0].newID != 2 {
		t.Fatalf("updates = %+v", repo.updates)
	}
}

func TestResolverRetriesCASConflict(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)
	oldID := int64(1)
	newID := int64(2)
	repo := &fakeRepository{
		binding: Binding{
			ProjectInternalID: 42,
			SandboxID:         &oldID,
			Instance:          &Instance{ID: 1, Status: StatusOffline, HealthStatus: HealthUnhealthy, LastHeartbeatAt: &heartbeat},
		},
		selectSeq: []Instance{{ID: newID, Scheme: "http", Host: "sandbox-b", Port: 8000}},
		updateErr: ErrBindingConflict,
	}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	// After the conflict, the fake binding is already node 2 and healthy.
	repo.binding.SandboxID = &newID
	repo.binding.Instance = &Instance{
		ID: newID, Scheme: "http", Host: "sandbox-b", Port: 8000,
		Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat,
	}

	baseURL, err := resolver.ResolveBaseURL(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != "http://sandbox-b:8000" {
		t.Fatalf("baseURL = %s", baseURL)
	}
}

func TestResolverNoHealthyNode(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{selectErr: ErrNoHealthySandbox}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	if _, err := resolver.ResolveBaseURL(context.Background(), "p1"); !errors.Is(err, ErrNoHealthySandbox) {
		t.Fatalf("err = %v, want ErrNoHealthySandbox", err)
	}
}
