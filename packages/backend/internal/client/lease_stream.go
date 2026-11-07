package client

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"
)

type streamCloser struct {
	once   sync.Once
	mu     sync.Mutex
	stream io.Closer
	closed bool
	err    error
}

func (c *streamCloser) set(stream io.Closer) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		_ = stream.Close()
		return
	}
	c.stream = stream
	c.mu.Unlock()
}

func (c *streamCloser) close() error {
	c.once.Do(func() {
		c.mu.Lock()
		c.closed = true
		stream := c.stream
		c.mu.Unlock()
		if stream != nil {
			c.err = stream.Close()
		}
	})
	return c.err
}

type leaseRenewal struct {
	router    *Router
	projectID string
	epoch     int64
	sandboxID int64
	ctx       context.Context
	cancel    context.CancelFunc
	done      chan struct{}

	mu     sync.Mutex
	err    error
	closer streamCloser
}

func (r *Router) startLeaseRenewal(ctx context.Context, projectID string, epoch, sandboxID int64) *leaseRenewal {
	if r.renewer == nil || epoch <= 0 || r.renewInterval <= 0 {
		return nil
	}
	renewCtx, cancel := context.WithCancel(ctx)
	renewal := &leaseRenewal{
		router:    r,
		projectID: projectID,
		epoch:     epoch,
		sandboxID: sandboxID,
		ctx:       renewCtx,
		cancel:    cancel,
		done:      make(chan struct{}),
	}
	go func() {
		defer close(renewal.done)
		ticker := time.NewTicker(r.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := r.renewer.Renew(renewCtx, projectID, epoch); err != nil {
					if renewCtx.Err() != nil {
						return
					}
					renewal.fail(err)
					return
				}
			}
		}
	}()
	return renewal
}

func (l *leaseRenewal) fail(err error) {
	l.mu.Lock()
	if l.err == nil {
		l.err = err
	}
	l.mu.Unlock()
	l.router.logLeaseRenewError(l.ctx, l.projectID, l.sandboxID, l.epoch, err)
	l.cancel()
	_ = l.closer.close()
}

func (l *leaseRenewal) stop() {
	l.cancel()
	<-l.done
}

func (l *leaseRenewal) wrap(stream io.ReadCloser) io.ReadCloser {
	l.closer.set(stream)
	return &renewingReadCloser{
		ReadCloser: stream,
		cancel:     l.cancel,
		done:       l.done,
		closeFn:    l.closer.close,
	}
}

func (r *Router) logLeaseRenewError(ctx context.Context, projectID string, sandboxID, epoch int64, err error) {
	if r.obs == nil || r.obs.Logger == nil {
		return
	}
	r.obs.Logger.Log(ctx, slog.LevelError, "sandbox.lease.renew_failed",
		"project_id", projectID,
		"sandbox_id", sandboxID,
		"epoch", epoch,
		"error", err.Error(),
	)
}

type renewingReadCloser struct {
	io.ReadCloser
	cancel  context.CancelFunc
	done    chan struct{}
	closeFn func() error
}

func (r *renewingReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil {
		r.cancel()
	}
	return n, err
}

func (r *renewingReadCloser) Close() error {
	r.cancel()
	err := r.closeFn()
	<-r.done
	return err
}
