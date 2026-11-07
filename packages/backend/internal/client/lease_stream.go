package client

import (
	"context"
	"io"
	"sync"
	"time"
)

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
	err := r.closeFn()
	<-r.done
	return err
}

func (r *Router) wrapLeaseStream(ctx context.Context, projectID string, epoch int64, stream io.ReadCloser) io.ReadCloser {
	if r.renewer == nil || epoch <= 0 || r.renewInterval <= 0 {
		return stream
	}
	renewCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	var closeOnce sync.Once
	var closeErr error
	closeStream := func() error {
		closeOnce.Do(func() {
			cancel()
			closeErr = stream.Close()
		})
		return closeErr
	}

	go func() {
		defer close(done)
		ticker := time.NewTicker(r.renewInterval)
		defer ticker.Stop()
		for {
			select {
			case <-renewCtx.Done():
				return
			case <-ticker.C:
				if err := r.renewer.Renew(renewCtx, projectID, epoch); err != nil {
					_ = closeStream()
					return
				}
			}
		}
	}()
	return &renewingReadCloser{ReadCloser: stream, cancel: cancel, done: done, closeFn: closeStream}
}
