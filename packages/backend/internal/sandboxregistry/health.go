package sandboxregistry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/hibiken/asynq"

	"vibebiz/internal/observability"
)

const TaskTypeHealthCheck = "sandbox:healthcheck"

type HealthChecker struct {
	repo     Repository
	client   *http.Client
	ttl      time.Duration
	obs      *observability.Provider
	now      func() time.Time
	parallel int
}

func NewHealthChecker(repo Repository, ttl, timeout time.Duration, obs *observability.Provider) *HealthChecker {
	return &HealthChecker{
		repo:     repo,
		client:   &http.Client{Timeout: timeout},
		ttl:      ttl,
		obs:      obs,
		now:      time.Now,
		parallel: 4,
	}
}

func (h *HealthChecker) Run(ctx context.Context) error {
	now := h.now().UTC()
	if err := h.repo.MarkStaleOffline(ctx, now, h.ttl); err != nil {
		return err
	}
	instances, err := h.repo.ListPendingHealth(ctx, now, h.ttl)
	if err != nil {
		return err
	}

	semaphore := make(chan struct{}, h.parallel)
	var wg sync.WaitGroup
	for _, instance := range instances {
		instance := instance
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				return
			}

			start := time.Now()
			healthy, message := h.probe(ctx, instance)
			if err := h.repo.UpdateHealth(ctx, instance.ID, healthy, h.now().UTC(), message); err != nil && h.obs != nil {
				h.obs.Log(ctx, slog.LevelError, "sandbox.healthcheck", "instance_id", instance.ID, "status", "error", "error", err.Error())
				return
			}
			if h.obs == nil {
				return
			}
			attrs := []any{
				"instance_id", instance.ID,
				"sandbox_name", instance.Name,
				"healthy", healthy,
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if message != "" {
				attrs = append(attrs, "error", message)
			}
			h.obs.Log(ctx, slog.LevelInfo, "sandbox.healthcheck", attrs...)
		}()
	}
	wg.Wait()
	return nil
}

func (h *HealthChecker) HandleTask(ctx context.Context, _ *asynq.Task) error {
	return h.Run(ctx)
}

func (h *HealthChecker) probe(ctx context.Context, instance Instance) (bool, string) {
	target := url.URL{
		Scheme: instance.Scheme,
		Host:   net.JoinHostPort(instance.Host, strconv.Itoa(instance.Port)),
		Path:   "/health",
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return false, err.Error()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false, err.Error()
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Sprintf("health status %d", resp.StatusCode)
	}
	var envelope struct {
		Code   int `json:"code"`
		Result struct {
			Status string `json:"status"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return false, "invalid health response"
	}
	if envelope.Code != 0 || envelope.Result.Status != "ok" {
		return false, "sandbox not ready"
	}
	return true, ""
}
