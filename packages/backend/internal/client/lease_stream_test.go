package client

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vibebiz/internal/observability"
)

type renewCall struct {
	projectID string
	epoch     int64
}

type fakeRenewer struct {
	called chan struct{}
	once   sync.Once

	mu    sync.Mutex
	calls []renewCall
	err   error
}

func newFakeRenewer(err error) *fakeRenewer {
	return &fakeRenewer{called: make(chan struct{}), err: err}
}

func (f *fakeRenewer) Renew(_ context.Context, projectID string, epoch int64) error {
	f.mu.Lock()
	f.calls = append(f.calls, renewCall{projectID: projectID, epoch: epoch})
	f.mu.Unlock()
	f.once.Do(func() { close(f.called) })
	return f.err
}

func (f *fakeRenewer) snapshot() []renewCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]renewCall(nil), f.calls...)
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

type countingReadCloser struct {
	closes atomic.Int64
}

func (c *countingReadCloser) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (c *countingReadCloser) Close() error {
	c.closes.Add(1)
	return nil
}

type contextWaitRenewer struct {
	entered  chan struct{}
	canceled chan struct{}
	once     sync.Once
}

func (c *contextWaitRenewer) Renew(ctx context.Context, _ string, _ int64) error {
	c.once.Do(func() { close(c.entered) })
	<-ctx.Done()
	close(c.canceled)
	return ctx.Err()
}

type captureHandler struct {
	records chan slog.Record
}

func (c *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (c *captureHandler) Handle(_ context.Context, record slog.Record) error {
	select {
	case c.records <- record:
	default:
	}
	return nil
}

func (c *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return c }

func (c *captureHandler) WithGroup(string) slog.Handler { return c }

func recordAttrs(record slog.Record) map[string]string {
	attrs := map[string]string{}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.String()
		return true
	})
	return attrs
}

func waitClosed(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal(message)
	}
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

	renewer := newFakeRenewer(nil)
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 5*time.Millisecond)

	stream, err := router.Chat(context.Background(), "p1", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	waitClosed(t, renewer.called, "expected lease renewal")

	calls := renewer.snapshot()
	last := calls[len(calls)-1]
	if last.projectID != "p1" || last.epoch != 9 {
		t.Fatalf("renew call = %+v", last)
	}

	if err := stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := stream.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestRouterChatRenewsBeforeResponseHeaders(t *testing.T) {
	requestSeen := make(chan struct{})
	releaseHeader := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestSeen)
		<-releaseHeader
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()

	renewer := newFakeRenewer(nil)
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 5*time.Millisecond)

	type result struct {
		stream io.ReadCloser
		err    error
	}
	resultCh := make(chan result, 1)
	go func() {
		stream, err := router.Chat(context.Background(), "p1", []byte(`{}`))
		resultCh <- result{stream: stream, err: err}
	}()

	waitClosed(t, requestSeen, "chat request was not sent")
	waitClosed(t, renewer.called, "renewal did not start before response headers")
	close(releaseHeader)

	got := <-resultCh
	if got.err != nil {
		t.Fatal(got.err)
	}
	if err := got.stream.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	last := renewer.snapshot()[0]
	if last.projectID != "p1" || last.epoch != 9 {
		t.Fatalf("renew call = %+v", last)
	}
}

func TestRenewFailureDuringHeaderWaitCancelsChat(t *testing.T) {
	requestSeen := make(chan struct{})
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestSeen)
		select {
		case <-r.Context().Done():
		case <-release:
		}
	}))
	defer func() {
		close(release)
		server.Close()
	}()

	renewer := newFakeRenewer(errors.New("lease lost"))
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 5*time.Millisecond)

	resultCh := make(chan error, 1)
	go func() {
		stream, err := router.Chat(context.Background(), "p1", []byte(`{}`))
		if stream != nil {
			_ = stream.Close()
		}
		resultCh <- err
	}()

	waitClosed(t, requestSeen, "chat request was not sent")
	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("expected Chat to fail after lease renewal failure")
		}
	case <-time.After(time.Second):
		t.Fatal("Chat did not return after lease renewal failure")
	}

	last := renewer.snapshot()[0]
	if last.projectID != "p1" || last.epoch != 9 {
		t.Fatalf("renew call = %+v", last)
	}
}

func TestRouterChatFailureStopsRenewer(t *testing.T) {
	requestSeen := make(chan struct{})
	releaseResponse := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestSeen)
		<-releaseResponse
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	renewer := &contextWaitRenewer{
		entered:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	router := NewRouter(fakeRouteResolver{url: server.URL, epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, 5*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan error, 1)
	go func() {
		_, err := router.Chat(ctx, "p1", []byte(`{}`))
		resultCh <- err
	}()

	waitClosed(t, requestSeen, "chat request was not sent")
	waitClosed(t, renewer.entered, "renewal did not start")
	close(releaseResponse)

	if err := <-resultCh; err == nil {
		t.Fatal("expected Chat to fail")
	}
	waitClosed(t, renewer.canceled, "renewer was not canceled after Chat failure")
}

func TestRenewFailureLogsAndStoresError(t *testing.T) {
	records := make(chan slog.Record, 4)
	obs := &observability.Provider{Logger: slog.New(&captureHandler{records: records})}
	renewErr := errors.New("lease lost")
	renewer := newFakeRenewer(renewErr)
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9, sandboxID: 7}, obs)
	router.SetLeaseRenewer(renewer, time.Millisecond)

	renewal := router.startLeaseRenewal(context.Background(), "p1", 9, 7)
	stream := newBlockingReadCloser()
	wrapped := renewal.wrap(stream)

	waitClosed(t, stream.closed, "stream was not closed after renew failure")
	_ = wrapped.Close()

	select {
	case record := <-records:
		if record.Level != slog.LevelError || record.Message != "sandbox.lease.renew_failed" {
			t.Fatalf("record = %+v", record)
		}
		attrs := recordAttrs(record)
		if attrs["project_id"] != "p1" || attrs["epoch"] != "9" || attrs["sandbox_id"] != "7" {
			t.Fatalf("attrs = %+v", attrs)
		}
	case <-time.After(time.Second):
		t.Fatal("renewal failure was not logged")
	}

	renewal.mu.Lock()
	stored := renewal.err
	renewal.mu.Unlock()
	if !errors.Is(stored, renewErr) {
		t.Fatalf("stored error = %v", stored)
	}
}

func TestLeaseStreamDoubleClose(t *testing.T) {
	renewer := newFakeRenewer(nil)
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, time.Hour)

	renewal := router.startLeaseRenewal(context.Background(), "p1", 9, 7)
	stream := &countingReadCloser{}
	wrapped := renewal.wrap(stream)

	if err := wrapped.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := wrapped.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
	if stream.closes.Load() != 1 {
		t.Fatalf("stream closed %d times", stream.closes.Load())
	}
}

func TestLeaseRenewalStopCancelsAndWaits(t *testing.T) {
	renewer := newFakeRenewer(nil)
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9}, nil)
	router.SetLeaseRenewer(renewer, time.Hour)

	renewal := router.startLeaseRenewal(context.Background(), "p1", 9, 7)
	done := make(chan struct{})
	go func() {
		renewal.stop()
		close(done)
	}()
	waitClosed(t, done, "stop did not wait for the renewal goroutine")

	select {
	case <-renewal.ctx.Done():
	default:
		t.Fatal("renewal context was not canceled")
	}
	select {
	case <-renewer.called:
		t.Fatal("unexpected renewal after stop")
	default:
	}
}

func TestStartLeaseRenewalSkipsWithoutRenewal(t *testing.T) {
	router := NewRouter(fakeRouteResolver{url: "http://sandbox-a:8000", epoch: 9}, nil)
	if got := router.startLeaseRenewal(context.Background(), "p1", 9, 7); got != nil {
		t.Fatal("expected nil renewal without renewer")
	}

	router.SetLeaseRenewer(newFakeRenewer(nil), 0)
	if got := router.startLeaseRenewal(context.Background(), "p1", 9, 7); got != nil {
		t.Fatal("expected nil renewal with zero interval")
	}

	router.SetLeaseRenewer(newFakeRenewer(nil), time.Second)
	if got := router.startLeaseRenewal(context.Background(), "p1", 0, 0); got != nil {
		t.Fatal("expected nil renewal with zero epoch")
	}
}
