package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type fakeCoverStore struct {
	id  string
	key string
	err error
}

func (f *fakeCoverStore) SetCover(_ context.Context, id, key string) error {
	f.id = id
	f.key = key
	return f.err
}

type fakeCoverS3 struct {
	key         string
	contentType string
	size        int64
	body        string
}

func (f *fakeCoverS3) Put(_ context.Context, key, contentType string, size int64, body io.Reader) error {
	raw, _ := io.ReadAll(body)
	f.key = key
	f.contentType = contentType
	f.size = size
	f.body = string(raw)
	return nil
}

func (f *fakeCoverS3) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example/" + key, nil
}

func TestCoverUploadStoresS3Path(t *testing.T) {
	store := &fakeCoverStore{}
	s3 := &fakeCoverS3{}
	h := NewCoverHandler(store, s3, 15*time.Minute)
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/cover", h.Upload)
	png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 32)...)

	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/cover", bytes.NewReader(png))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if s3.key != "projects/p1/cover.png" || s3.contentType != "image/png" || s3.size != int64(len(png)) {
		t.Fatalf("s3 = %+v", s3)
	}
	if store.id != "p1" || store.key != "projects/p1/cover.png" {
		t.Fatalf("store = %+v", store)
	}
	var env struct {
		Result struct {
			CoverURL string `json:"coverUrl"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.CoverURL != "https://s3.example/projects/p1/cover.png" {
		t.Fatalf("cover url = %q", env.Result.CoverURL)
	}
}

func TestCoverUploadRejectsInvalidImage(t *testing.T) {
	h := NewCoverHandler(&fakeCoverStore{}, &fakeCoverS3{}, time.Minute)
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/cover", h.Upload)

	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/cover", bytes.NewReader([]byte("not png")))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}
