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

func TestCopyObject(t *testing.T) {
	var method, path, copySource, contentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		copySource = r.Header.Get("X-Amz-Copy-Source")
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"etag"</ETag></CopyObjectResult>`))
	}))
	defer srv.Close()

	p := newTestPresigner(t, srv.URL)
	if err := p.Copy(context.Background(), "staging/a.pdf", "resources/p/a.pdf", "application/pdf"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	if method != http.MethodPut || path != "/vibebiz/resources/p/a.pdf" {
		t.Fatalf("request = %s %s", method, path)
	}
	if copySource != "vibebiz/staging/a.pdf" || contentType != "application/pdf" {
		t.Fatalf("headers = %q/%q", copySource, contentType)
	}
}

func TestGetAndDeleteObject(t *testing.T) {
	var getPath, deletePath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			getPath = r.URL.Path
			w.Header().Set("Content-Length", "5")
			_, _ = w.Write([]byte("hello"))
		case http.MethodDelete:
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method: %s", r.Method)
		}
	}))
	defer srv.Close()

	p := newTestPresigner(t, srv.URL)
	body, size, err := p.Get(context.Background(), "resources/p/a.pdf")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	data, err := io.ReadAll(body)
	_ = body.Close()
	if err != nil || string(data) != "hello" || size != 5 {
		t.Fatalf("get = %q/%d/%v", data, size, err)
	}
	if err := p.Delete(context.Background(), "resources/p/a.pdf"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if getPath != "/vibebiz/resources/p/a.pdf" || deletePath != "/vibebiz/resources/p/a.pdf" {
		t.Fatalf("paths = %q/%q", getPath, deletePath)
	}
}

func TestRemoteDirPrefixAppliesToObjectKeys(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	p, err := NewPresigner(context.Background(), Config{
		Bucket:          "vibebiz",
		Region:          "us-east-1",
		Endpoint:        srv.URL,
		AccessKey:       "test-access",
		SecretKey:       "test-secret",
		RemoteDirPrefix: "/tmp/vibebiz/",
		ForcePathStyle:  true,
	})
	if err != nil {
		t.Fatalf("new presigner: %v", err)
	}
	if err := p.Put(context.Background(), "avatar/7/a.png", "image/png", 1, strings.NewReader("x")); err != nil {
		t.Fatalf("put: %v", err)
	}
	if path != "/vibebiz/tmp/vibebiz/avatar/7/a.png" {
		t.Fatalf("path = %q", path)
	}
}
