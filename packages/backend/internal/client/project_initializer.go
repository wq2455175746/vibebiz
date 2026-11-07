package client

import (
	"context"

	"vibebiz/internal/observability"
)

type HTTPInitializer struct {
	obs *observability.Provider
}

func NewHTTPInitializer(obs *observability.Provider) *HTTPInitializer {
	return &HTTPInitializer{obs: obs}
}

func (h *HTTPInitializer) InitProject(ctx context.Context, baseURL, projectID string) error {
	return NewSandbox(baseURL, h.obs).InitProject(ctx, projectID)
}
