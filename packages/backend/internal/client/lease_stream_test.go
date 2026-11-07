package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeRenewer struct {
	calls atomic.Int64
	err   error
}

func (f *fakeRenewer) Renew(context.Context, string, int64) error {
	f.calls.Add(1)
	return f.err
}

type blockingReadCloser struct {
	closed chan struct{}
	once   sync.Once
}

func newBlockingReadCloser() *blockingReadCloser {
	return &blockingReadCloser{closed: make(chan struct{})}
}

func (b *blockingReadCloser) Read([]byte) (int, error) {
	<-b.closed
	return 0, io.EOF
}

func (b *blockingReadCloser) Close() error {
	b.once.Do(func() { close(b.closed) })
	return nil
}

func TestRouterChatRenewsLease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	renewer := &fakeRenewer{}
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 10*time.Millisecond)

	stream, err := router.Chat(context.Background(), "p1", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(35 * time.Millisecond)
	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if renewer.calls.Load() == 0 {
		t.Fatal("expected lease renewal")
	}
	after := renewer.calls.Load()
	time.Sleep(25 * time.Millisecond)
	if renewer.calls.Load() != after {
		t.Fatal("renewal continued after Close")
	}
}

func TestRenewFailureClosesStream(t *testing.T) {
	renewer := &fakeRenewer{err: errors.New("lease lost")}
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 10*time.Millisecond)

	stream := newBlockingReadCloser()
	wrapped := router.wrapLeaseStream(context.Background(), "p1", 9, stream)

	select {
	case <-stream.closed:
	case <-time.After(time.Second):
		t.Fatal("stream was not closed after renew failure")
	}
	if err := wrapped.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if renewer.calls.Load() == 0 {
		t.Fatal("expected renewal attempt")
	}
}

func TestLeaseStreamCloseUnblocksOnCanceledContext(t *testing.T) {
	renewer := &fakeRenewer{}
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	wrapped := router.wrapLeaseStream(ctx, "p1", 9, io.NopCloser(strings.NewReader("")))
	cancel()

	done := make(chan struct{})
	go func() {
		_ = wrapped.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close blocked after context cancellation")
	}
}

func TestWrapLeaseStreamSkipsWithoutRenewal(t *testing.T) {
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9}, nil)
	stream := io.NopCloser(strings.NewReader(""))
	if got := router.wrapLeaseStream(context.Background(), "p1", 9, stream); got != stream {
		t.Fatal("expected original stream without renewer")
	}

	router.SetLeaseRenewer(&fakeRenewer{}, 0)
	if got := router.wrapLeaseStream(context.Background(), "p1", 9, stream); got != stream {
		t.Fatal("expected original stream with zero interval")
	}

	router.SetLeaseRenewer(&fakeRenewer{}, time.Second)
	if got := router.wrapLeaseStream(context.Background(), "p1", 0, stream); got != stream {
		t.Fatal("expected original stream with zero epoch")
	}
}
