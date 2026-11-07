package client

import (
	"context"
	"io"
	"sync"
	"time"

	"vibebiz/internal/observability"
	"vibebiz/internal/resource"
	"vibebiz/internal/sandboxregistry"
)

type ProjectResolver interface {
	ResolveBaseURL(ctx context.Context, projectID string) (string, error)
}

type Route = sandboxregistry.Route

type RouteResolver interface {
	ResolveRoute(ctx context.Context, projectID string) (Route, error)
}

type RouteRenewer interface {
	Renew(ctx context.Context, projectID string, epoch int64) error
}

var (
	_ RouteResolver = (*sandboxregistry.Resolver)(nil)
	_ RouteRenewer  = (*sandboxregistry.Resolver)(nil)
)

type epochKey struct{}

func WithProjectEpoch(ctx context.Context, epoch int64) context.Context {
	return context.WithValue(ctx, epochKey{}, epoch)
}

func projectEpoch(ctx context.Context) int64 {
	value, _ := ctx.Value(epochKey{}).(int64)
	return value
}

type Router struct {
	resolver      ProjectResolver
	obs           *observability.Provider
	clients       sync.Map
	newClient     func(baseURL string, obs *observability.Provider) *Sandbox
	renewer       RouteRenewer
	renewInterval time.Duration
}

func NewRouter(resolver ProjectResolver, obs *observability.Provider) *Router {
	return &Router{resolver: resolver, obs: obs, newClient: NewSandbox}
}

func (r *Router) SetLeaseRenewer(renewer RouteRenewer, interval time.Duration) {
	r.renewer = renewer
	r.renewInterval = interval
}

func (r *Router) sandboxFor(baseURL string) (*Sandbox, error) {
	if value, ok := r.clients.Load(baseURL); ok {
		return value.(*Sandbox), nil
	}
	created := r.newClient(baseURL, r.obs)
	actual, _ := r.clients.LoadOrStore(baseURL, created)
	return actual.(*Sandbox), nil
}

func (r *Router) route(ctx context.Context, projectID string) (context.Context, *Sandbox, Route, error) {
	var route Route
	if resolver, ok := r.resolver.(RouteResolver); ok {
		got, err := resolver.ResolveRoute(ctx, projectID)
		if err != nil {
			return ctx, nil, Route{}, err
		}
		route = got
	} else {
		baseURL, err := r.resolver.ResolveBaseURL(ctx, projectID)
		if err != nil {
			return ctx, nil, Route{}, err
		}
		route = Route{BaseURL: baseURL}
	}
	sandbox, err := r.sandboxFor(route.BaseURL)
	if err != nil {
		return ctx, nil, Route{}, err
	}
	return WithProjectEpoch(ctx, route.Epoch), sandbox, route, nil
}

func (r *Router) ListFiles(ctx context.Context, id string) ([]FileEntry, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ListFiles(ctx, id)
}

func (r *Router) ReadFile(ctx context.Context, id, path string) (*FileContent, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ReadFile(ctx, id, path)
}

func (r *Router) WriteFile(ctx context.Context, id, path, content, baseVersion string) (*WriteResult, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.WriteFile(ctx, id, path, content, baseVersion)
}

func (r *Router) WriteResource(ctx context.Context, projectID string, meta resource.ResourceMeta, body io.Reader, size int64) error {
	ctx, sandbox, _, err := r.route(ctx, projectID)
	if err != nil {
		return err
	}
	return sandbox.WriteResource(ctx, projectID, meta, body, size)
}

func (r *Router) InitClone(ctx context.Context, projectID string, setup CloneSetup) error {
	ctx, sandbox, _, err := r.route(ctx, projectID)
	if err != nil {
		return err
	}
	return sandbox.InitClone(ctx, projectID, setup)
}

func (r *Router) RunCloneTests(ctx context.Context, projectID string) (CloneTestResult, error) {
	ctx, sandbox, _, err := r.route(ctx, projectID)
	if err != nil {
		return CloneTestResult{}, err
	}
	return sandbox.RunCloneTests(ctx, projectID)
}

func (r *Router) DeleteResource(ctx context.Context, projectID, resourceID string) error {
	ctx, sandbox, _, err := r.route(ctx, projectID)
	if err != nil {
		return err
	}
	return sandbox.DeleteResource(ctx, projectID, resourceID)
}

func (r *Router) ProjectStats(ctx context.Context, id string) (ProjectStats, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return ProjectStats{}, err
	}
	return sandbox.ProjectStats(ctx, id)
}

func (r *Router) ExportArchive(ctx context.Context, id string) ([]byte, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ExportArchive(ctx, id)
}

func (r *Router) ImportArchive(ctx context.Context, id string, data []byte) error {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return err
	}
	return sandbox.ImportArchive(ctx, id, data)
}

func (r *Router) ReadChatHistory(ctx context.Context, id string, limit int) (ChatHistoryTail, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return ChatHistoryTail{}, err
	}
	return sandbox.ReadChatHistory(ctx, id, limit)
}

func (r *Router) StopChat(ctx context.Context, id string) error {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return err
	}
	return sandbox.StopChat(ctx, id)
}

func (r *Router) ChatLogs(ctx context.Context, id string, limit int) ([]ChatLog, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ChatLogs(ctx, id, limit)
}

func (r *Router) ChatStatus(ctx context.Context, id string) (ChatStatus, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return ChatStatus{}, err
	}
	return sandbox.ChatStatus(ctx, id)
}

func (r *Router) ListSkills(ctx context.Context, id string, userID int64) ([]SkillInfo, error) {
	ctx, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ListSkills(ctx, id, userID)
}

func (r *Router) Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error) {
	ctx, sandbox, route, err := r.route(ctx, id)
	if err != nil {
		return nil, err
	}
	renewal := r.startLeaseRenewal(ctx, id, route.Epoch)
	requestCtx := ctx
	if renewal != nil {
		requestCtx = renewal.ctx
	}
	stream, err := sandbox.Chat(requestCtx, id, payload)
	if err != nil {
		if renewal != nil {
			renewal.stop()
		}
		return nil, err
	}
	if renewal != nil {
		return renewal.wrap(stream), nil
	}
	return stream, nil
}

func (r *Router) PreviewURL(ctx context.Context, id string) (string, error) {
	_, sandbox, _, err := r.route(ctx, id)
	if err != nil {
		return "", err
	}
	return sandbox.PreviewURL(id), nil
}
