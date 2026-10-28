package s3

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func newTestPresigner(t *testing.T, endpoint string) *Presigner {
	t.Helper()
	p, err := NewPresigner(context.Background(), Config{
		Bucket:         "vibebiz",
		Region:         "us-east-1",
		Endpoint:       endpoint,
		AccessKey:      "test-access",
		SecretKey:      "test-secret",
		ForcePathStyle: true,
	})
	if err != nil {
		t.Fatalf("new presigner: %v", err)
	}
	return p
}

func TestPresignPutAndGetUseServerKey(t *testing.T) {
	p := newTestPresigner(t, "http://127.0.0.1:9000")
	key := "avatar/7/abc.png"

	putURL, headers, err := p.PresignPut(context.Background(), key, "image/png", 128, 15*time.Minute)
	if err != nil {
		t.Fatalf("presign put: %v", err)
	}
	if !strings.Contains(putURL, key) || !strings.Contains(putURL, "X-Amz-Signature=") {
		t.Fatalf("put url = %s", putURL)
	}
	if headers["Content-Type"] != "image/png" {
		t.Fatalf("headers = %+v", headers)
	}

	getURL, err := p.PresignGet(context.Background(), key, 15*time.Minute)
	if err != nil {
		t.Fatalf("presign get: %v", err)
	}
	if !strings.Contains(getURL, key) || !strings.Contains(getURL, "X-Amz-Signature=") {
		t.Fatalf("get url = %s", getURL)
	}
}

func TestPutObjectUploadsBody(t *testing.T) {
	var method, path, contentType, body string
	var size int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		size = r.ContentLength
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		body = string(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newTestPresigner(t, srv.URL)
	if err := p.Put(context.Background(), "avatar/7/upload.png", "image/png", 5, strings.NewReader("hello")); err != nil {
		t.Fatalf("put: %v", err)
	}
	if method != http.MethodPut || path != "/vibebiz/avatar/7/upload.png" {
		t.Fatalf("request = %s %s", method, path)
	}
	if contentType != "image/png" || size != 5 || body != "hello" {
		t.Fatalf("content = %q/%d/%q", contentType, size, body)
	}
}

func TestPutObjectError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newTestPresigner(t, srv.URL)
	if err := p.Put(context.Background(), "avatar/7/upload.png", "image/png", 1, strings.NewReader("x")); err == nil {
		t.Fatal("put error = nil")
	}
}

func TestHeadObject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead || r.URL.Path != "/vibebiz/avatar/7/abc.png" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Length", "256")
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p := newTestPresigner(t, srv.URL)
	size, contentType, err := p.Head(context.Background(), "avatar/7/abc.png")
	if err != nil {
		t.Fatalf("head: %v", err)
	}
	if size != 256 || contentType != "image/png" {
		t.Fatalf("head = %d/%q", size, contentType)
	}
}

func TestHeadObjectMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := newTestPresigner(t, srv.URL)
	if _, _, err := p.Head(context.Background(), "avatar/7/missing.png"); err == nil {
		t.Fatal("head missing err = nil")
	}
}
