package client

import (
	"context"
	"io"
	"sync"

	"vibebiz/internal/observability"
)

type ProjectResolver interface {
	ResolveBaseURL(ctx context.Context, projectID string) (string, error)
}

type Router struct {
	resolver  ProjectResolver
	obs       *observability.Provider
	clients   sync.Map
	newClient func(baseURL string, obs *observability.Provider) *Sandbox
}

func NewRouter(resolver ProjectResolver, obs *observability.Provider) *Router {
	return &Router{resolver: resolver, obs: obs, newClient: NewSandbox}
}

func (r *Router) client(ctx context.Context, projectID string) (*Sandbox, error) {
	baseURL, err := r.resolver.ResolveBaseURL(ctx, projectID)
	if err != nil {
		return nil, err
	}
	if value, ok := r.clients.Load(baseURL); ok {
		return value.(*Sandbox), nil
	}
	created := r.newClient(baseURL, r.obs)
	actual, _ := r.clients.LoadOrStore(baseURL, created)
	return actual.(*Sandbox), nil
}

func (r *Router) ListFiles(ctx context.Context, id string) ([]FileEntry, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ListFiles(ctx, id)
}

func (r *Router) ReadFile(ctx context.Context, id, path string) (*FileContent, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.ReadFile(ctx, id, path)
}

func (r *Router) WriteFile(ctx context.Context, id, path, content, baseVersion string) (*WriteResult, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.WriteFile(ctx, id, path, content, baseVersion)
}

func (r *Router) ProjectStats(ctx context.Context, id string) (ProjectStats, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return ProjectStats{}, err
	}
	return sandbox.ProjectStats(ctx, id)
}

func (r *Router) ReadChatHistory(ctx context.Context, id string, limit int) (ChatHistoryTail, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return ChatHistoryTail{}, err
	}
	return sandbox.ReadChatHistory(ctx, id, limit)
}

func (r *Router) StopChat(ctx context.Context, id string) error {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return err
	}
	return sandbox.StopChat(ctx, id)
}

func (r *Router) Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return nil, err
	}
	return sandbox.Chat(ctx, id, payload)
}

func (r *Router) PreviewURL(ctx context.Context, id string) (string, error) {
	sandbox, err := r.client(ctx, id)
	if err != nil {
		return "", err
	}
	return sandbox.PreviewURL(id), nil
}
