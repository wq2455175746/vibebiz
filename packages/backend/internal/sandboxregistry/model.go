package sandboxregistry

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"
)

const (
	StatusOnline  = "online"
	StatusOffline = "offline"

	HealthUnknown   = "unknown"
	HealthHealthy   = "healthy"
	HealthUnhealthy = "unhealthy"
)

var (
	ErrNoHealthySandbox = errors.New("no healthy sandbox")
	ErrProjectNotFound  = errors.New("project not found")
	ErrBindingConflict  = errors.New("project binding changed")
)

type Instance struct {
	ID              int64
	PublicID        string
	Name            string
	Scheme          string
	Host            string
	Port            int
	Status          string
	HealthStatus    string
	ActiveRequests  int
	MaxConcurrency  int
	LastHeartbeatAt *time.Time
	LastHealthAt    *time.Time
	HealthError     string
}

func (i Instance) BaseURL() string {
	return fmt.Sprintf("%s://%s", i.Scheme, net.JoinHostPort(i.Host, strconv.Itoa(i.Port)))
}

func (i Instance) Usable(now time.Time, ttl time.Duration) bool {
	return i.Status == StatusOnline &&
		i.HealthStatus == HealthHealthy &&
		i.LastHeartbeatAt != nil &&
		now.Sub(*i.LastHeartbeatAt) <= ttl
}

type Heartbeat struct {
	Name           string
	Scheme         string
	Host           string
	Port           int
	ActiveRequests int
	MaxConcurrency int
}

type Binding struct {
	ProjectInternalID int64
	ProjectPublicID   string
	SandboxID         *int64
	Instance          *Instance
}

type Repository interface {
	UpsertHeartbeat(ctx context.Context, heartbeat Heartbeat, now time.Time) (Instance, error)
	List(ctx context.Context) ([]Instance, error)
	Get(ctx context.Context, id int64) (Instance, error)
	Select(ctx context.Context, now time.Time, ttl time.Duration, exclude []int64) (Instance, error)
	MarkStaleOffline(ctx context.Context, now time.Time, ttl time.Duration) error
	ListPendingHealth(ctx context.Context, now time.Time, ttl time.Duration) ([]Instance, error)
	UpdateHealth(ctx context.Context, id int64, healthy bool, at time.Time, errMessage string) error
	GetProjectBinding(ctx context.Context, publicProjectID string) (Binding, error)
	UpdateProjectBinding(ctx context.Context, projectInternalID int64, oldSandboxID *int64, newSandboxID int64) error
}
