package sandboxregistry

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type fakeRepository struct {
	mu         sync.Mutex
	instances  []Instance
	binding    Binding
	bindingSeq []Binding
	selectSeq  []Instance
	selectErr  error
	selectArgs [][]int64
	updateErr  error
	updates    []bindingUpdate

	lease      Lease
	acquireErr error
	acquires   []leaseAcquire
	renewErr   error
	renews     []int64
}

type bindingUpdate struct {
	projectID int64
	oldID     *int64
	newID     int64
}

type leaseAcquire struct {
	projectID int64
	oldID     *int64
	newID     int64
	now       time.Time
	ttl       time.Duration
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
	if len(f.bindingSeq) > 0 {
		binding := f.bindingSeq[0]
		f.bindingSeq = f.bindingSeq[1:]
		return binding, nil
	}
	return f.binding, nil
}
func (f *fakeRepository) AcquireProjectLease(_ context.Context, projectID int64, oldID *int64, newID int64, now time.Time, ttl time.Duration) (Lease, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acquires = append(f.acquires, leaseAcquire{projectID: projectID, oldID: oldID, newID: newID, now: now, ttl: ttl})
	if f.acquireErr != nil {
		err := f.acquireErr
		f.acquireErr = nil
		return Lease{}, err
	}
	now = now.UTC()
	epoch := f.binding.Epoch
	if f.binding.SandboxID != nil && f.binding.LeaseUntil.After(now) {
		if *f.binding.SandboxID != newID {
			return Lease{}, ErrBindingConflict
		}
	} else if epoch == 0 {
		epoch = 1
	} else {
		epoch++
	}
	f.lease = Lease{ProjectInternalID: projectID, SandboxID: newID, LeaseUntil: now.Add(ttl), Epoch: epoch}
	f.binding.SandboxID = &newID
	f.binding.LeaseUntil = f.lease.LeaseUntil
	f.binding.Epoch = f.lease.Epoch
	return f.lease, nil
}
func (f *fakeRepository) RenewProjectLease(_ context.Context, _ int64, _ int64, epoch int64, _ time.Time, _ time.Duration) error {
	f.renews = append(f.renews, epoch)
	if f.renewErr != nil {
		err := f.renewErr
		f.renewErr = nil
		return err
	}
	return nil
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
		LeaseUntil:        now.Add(20 * time.Second),
		Epoch:             2,
		Instance: &Instance{
			ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
			Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat,
		},
	}}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.BaseURL != "http://sandbox-a:8000" || route.Epoch != 2 {
		t.Fatalf("route = %+v", route)
	}
	if len(repo.acquires) != 0 || len(repo.renews) != 0 {
		t.Fatalf("acquires = %+v, renews = %+v", repo.acquires, repo.renews)
	}
	baseURL, err := resolver.ResolveBaseURL(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if baseURL != "http://sandbox-a:8000" {
		t.Fatalf("baseURL = %s", baseURL)
	}
}

func TestResolverReturnsLeaseEpoch(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)
	bound := int64(1)
	repo := &fakeRepository{binding: Binding{
		ProjectInternalID: 42,
		SandboxID:         &bound,
		LeaseUntil:        now.Add(20 * time.Second),
		Epoch:             3,
		Instance: &Instance{ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
			Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat},
	}}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })
	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.BaseURL != "http://sandbox-a:8000" || route.Epoch != 3 {
		t.Fatalf("route = %+v", route)
	}
}

func TestResolverRenewsLeaseNearExpiry(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)
	bound := int64(1)
	repo := &fakeRepository{binding: Binding{
		ProjectInternalID: 42,
		SandboxID:         &bound,
		LeaseUntil:        now.Add(5 * time.Second),
		Epoch:             3,
		Instance: &Instance{ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
			Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat},
	}}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })
	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.Epoch != 3 {
		t.Fatalf("route = %+v", route)
	}
	if len(repo.renews) != 1 || repo.renews[0] != 3 {
		t.Fatalf("renews = %+v", repo.renews)
	}
}

func TestResolverRebindsExpiredLease(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	oldID := int64(1)
	oldHeartbeat := now.Add(-time.Minute)
	repo := &fakeRepository{
		binding: Binding{
			ProjectInternalID: 42,
			SandboxID:         &oldID,
			LeaseUntil:        now.Add(-time.Second),
			Epoch:             4,
			Instance: &Instance{
				ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000,
				Status: StatusOffline, HealthStatus: HealthUnhealthy, LastHeartbeatAt: &oldHeartbeat,
			},
		},
		selectSeq: []Instance{{ID: 2, Scheme: "http", Host: "sandbox-b", Port: 8000}},
	}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.BaseURL != "http://sandbox-b:8000" || route.Epoch != 5 {
		t.Fatalf("route = %+v", route)
	}
	if len(repo.acquires) != 1 || repo.acquires[0].oldID == nil || *repo.acquires[0].oldID != 1 || repo.acquires[0].newID != 2 {
		t.Fatalf("acquires = %+v", repo.acquires)
	}
	if len(repo.updates) != 0 {
		t.Fatalf("updates = %+v", repo.updates)
	}
}

func TestResolverRetriesAcquireConflict(t *testing.T) {
	now := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Second)
	oldID := int64(1)
	newID := int64(2)
	repo := &fakeRepository{
		bindingSeq: []Binding{
			{
				ProjectInternalID: 42,
				SandboxID:         &oldID,
				LeaseUntil:        now.Add(-time.Second),
				Epoch:             2,
				Instance:          &Instance{ID: 1, Status: StatusOffline, HealthStatus: HealthUnhealthy, LastHeartbeatAt: &heartbeat},
			},
			{
				ProjectInternalID: 42,
				SandboxID:         &newID,
				LeaseUntil:        now.Add(20 * time.Second),
				Epoch:             3,
				Instance: &Instance{
					ID: newID, Scheme: "http", Host: "sandbox-b", Port: 8000,
					Status: StatusOnline, HealthStatus: HealthHealthy, LastHeartbeatAt: &heartbeat,
				},
			},
		},
		selectSeq:  []Instance{{ID: newID, Scheme: "http", Host: "sandbox-b", Port: 8000}},
		acquireErr: ErrBindingConflict,
	}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.BaseURL != "http://sandbox-b:8000" || route.Epoch != 3 {
		t.Fatalf("route = %+v", route)
	}
	if len(repo.acquires) != 1 {
		t.Fatalf("acquires = %+v", repo.acquires)
	}
}

func TestResolverConflictsOnLiveLease(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	heartbeat := now.Add(-time.Minute)
	bound := int64(1)
	repo := &fakeRepository{
		binding: Binding{
			ProjectInternalID: 42,
			SandboxID:         &bound,
			LeaseUntil:        now.Add(20 * time.Second),
			Epoch:             3,
			Instance:          &Instance{ID: 1, Status: StatusOffline, HealthStatus: HealthUnhealthy, LastHeartbeatAt: &heartbeat},
		},
		instances: []Instance{{ID: 2, Scheme: "http", Host: "sandbox-b", Port: 8000}},
	}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	if _, err := resolver.ResolveRoute(context.Background(), "p1"); !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("err = %v, want ErrBindingConflict", err)
	}
	if len(repo.acquires) != 2 {
		t.Fatalf("acquires = %+v", repo.acquires)
	}
}

func TestResolverAcquireSameOwnerRenewsLease(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	bound := int64(1)
	repo := &fakeRepository{
		binding: Binding{
			ProjectInternalID: 42,
			SandboxID:         &bound,
			LeaseUntil:        now.Add(20 * time.Second),
			Epoch:             3,
		},
		instances: []Instance{{ID: 1, Scheme: "http", Host: "sandbox-a", Port: 8000}},
	}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	route, err := resolver.ResolveRoute(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if route.BaseURL != "http://sandbox-a:8000" || route.Epoch != 3 {
		t.Fatalf("route = %+v", route)
	}
	if len(repo.acquires) != 1 {
		t.Fatalf("acquires = %+v", repo.acquires)
	}
}

func TestAcquireProjectLeaseFirstAcquisitionSingleWinner(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = repo.AcquireProjectLease(context.Background(), 42, nil, int64(i+1), now, 30*time.Second)
		}(i)
	}
	wg.Wait()

	successes, conflicts := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrBindingConflict):
			conflicts++
		default:
			t.Fatalf("unexpected err: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes = %d, conflicts = %d, errs = %v", successes, conflicts, errs)
	}
}

func TestResolverRenew(t *testing.T) {
	now := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	bound := int64(1)
	repo := &fakeRepository{binding: Binding{ProjectInternalID: 42, SandboxID: &bound, Epoch: 3}}
	resolver := NewResolver(repo, NewSelector(repo, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })

	if err := resolver.Renew(context.Background(), "p1", 3); err != nil {
		t.Fatal(err)
	}
	if len(repo.renews) != 1 || repo.renews[0] != 3 {
		t.Fatalf("renews = %+v", repo.renews)
	}

	empty := &fakeRepository{binding: Binding{ProjectInternalID: 42}}
	emptyResolver := NewResolver(empty, NewSelector(empty, 15*time.Second, func() time.Time { return now }), 15*time.Second, func() time.Time { return now })
	if err := emptyResolver.Renew(context.Background(), "p1", 3); !errors.Is(err, ErrBindingConflict) {
		t.Fatalf("err = %v, want ErrBindingConflict", err)
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
