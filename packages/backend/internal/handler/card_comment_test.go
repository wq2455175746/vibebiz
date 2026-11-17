package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/businesscard"
	"vibebiz/internal/observability"
)

type fakeCardCommentService struct {
	listRoots   func(context.Context, string, int64, string, int) (*businesscard.CommentPage, error)
	listReplies func(context.Context, string, string, int64, string, int) (*businesscard.CommentPage, error)
	create      func(context.Context, string, int64, string, string) (*businesscard.Comment, error)
	update      func(context.Context, string, string, int64, string) (*businesscard.Comment, error)
	delete      func(context.Context, string, string, int64) (businesscard.CommentDeleteResult, error)
}

func (f *fakeCardCommentService) ListRoots(ctx context.Context, cardPublicID string, userID int64, cursor string, limit int) (*businesscard.CommentPage, error) {
	if f.listRoots == nil {
		return &businesscard.CommentPage{}, nil
	}
	return f.listRoots(ctx, cardPublicID, userID, cursor, limit)
}

func (f *fakeCardCommentService) ListReplies(ctx context.Context, cardPublicID, rootPublicID string, userID int64, cursor string, limit int) (*businesscard.CommentPage, error) {
	if f.listReplies == nil {
		return &businesscard.CommentPage{}, nil
	}
	return f.listReplies(ctx, cardPublicID, rootPublicID, userID, cursor, limit)
}

func (f *fakeCardCommentService) Create(ctx context.Context, cardPublicID string, userID int64, parentPublicID, content string) (*businesscard.Comment, error) {
	if f.create == nil {
		return &businesscard.Comment{}, nil
	}
	return f.create(ctx, cardPublicID, userID, parentPublicID, content)
}

func (f *fakeCardCommentService) Update(ctx context.Context, cardPublicID, commentPublicID string, userID int64, content string) (*businesscard.Comment, error) {
	if f.update == nil {
		return &businesscard.Comment{}, nil
	}
	return f.update(ctx, cardPublicID, commentPublicID, userID, content)
}

func (f *fakeCardCommentService) Delete(ctx context.Context, cardPublicID, commentPublicID string, userID int64) (businesscard.CommentDeleteResult, error) {
	if f.delete == nil {
		return businesscard.CommentDeleteResult{}, nil
	}
	return f.delete(ctx, cardPublicID, commentPublicID, userID)
}

func withCommentSession(userID int64) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			auth.SetSession(c, &auth.Session{UserID: userID, Realm: auth.RealmUser})
			return next(c)
		}
	}
}

func TestCardCommentCreateRequiresLogin(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{})
	e.POST("/cp/api/v1/public/cards/:id/comments", h.Create)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCardCommentListReturnsPage(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{
		listRoots: func(context.Context, string, int64, string, int) (*businesscard.CommentPage, error) {
			return &businesscard.CommentPage{Comments: []businesscard.Comment{{ID: "c-1"}}}, nil
		},
	})
	e.GET("/cp/api/v1/public/cards/:id/comments", h.ListRoots)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1/comments", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"c-1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardCommentCreateUsesSessionAndSignsAvatar(t *testing.T) {
	e := echo.New()
	var gotUserID int64
	var gotParent, gotContent string
	h := NewCardCommentHandler(&fakeCardCommentService{
		create: func(_ context.Context, _ string, userID int64, parentPublicID, content string) (*businesscard.Comment, error) {
			gotUserID = userID
			gotParent = parentPublicID
			gotContent = content
			return &businesscard.Comment{
				ID:     "c-new",
				Author: businesscard.CommentAuthor{UserID: userID, AvatarURL: "avatars/u7.png"},
			}, nil
		},
	})
	h.Covers = fakeCoverURLSigner{}
	e.POST("/cp/api/v1/public/cards/:id/comments", h.Create, withCommentSession(7))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi","parentId":"c-root"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotUserID != 7 || gotParent != "c-root" || gotContent != "hi" {
		t.Fatalf("userID=%d parent=%q content=%q", gotUserID, gotParent, gotContent)
	}
	if !strings.Contains(rec.Body.String(), `"avatarUrl":"https://s3.example/avatars/u7.png"`) {
		t.Fatalf("avatar not signed: %s", rec.Body.String())
	}
}

func TestCardCommentListSignsNestedRepliesAndPassesViewer(t *testing.T) {
	e := echo.New()
	var viewer int64
	h := NewCardCommentHandler(&fakeCardCommentService{
		listRoots: func(_ context.Context, _ string, userID int64, _ string, _ int) (*businesscard.CommentPage, error) {
			viewer = userID
			return &businesscard.CommentPage{Comments: []businesscard.Comment{{
				ID:     "c-root",
				Author: businesscard.CommentAuthor{AvatarURL: "https://cdn.example/u1.png"},
				Replies: []businesscard.Comment{{
					ID:     "c-reply",
					Author: businesscard.CommentAuthor{AvatarURL: "avatars/u2.png"},
				}},
			}}}, nil
		},
	})
	h.Covers = fakeCoverURLSigner{}
	e.GET("/cp/api/v1/public/cards/:id/comments", h.ListRoots, withCommentSession(9))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1/comments?cursor=abc&limit=5", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	if viewer != 9 {
		t.Fatalf("viewer=%d", viewer)
	}
	if !strings.Contains(body, `"avatarUrl":"https://cdn.example/u1.png"`) {
		t.Fatalf("absolute avatar was rewritten: %s", body)
	}
	if !strings.Contains(body, `"avatarUrl":"https://s3.example/avatars/u2.png"`) {
		t.Fatalf("nested reply avatar not signed: %s", body)
	}
}

func TestCardCommentListClearsRawAvatarKeysWithoutPresigner(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{
		listRoots: func(context.Context, string, int64, string, int) (*businesscard.CommentPage, error) {
			return &businesscard.CommentPage{Comments: []businesscard.Comment{{
				ID:     "c-root",
				Author: businesscard.CommentAuthor{AvatarURL: "avatars/u1.png"},
				Replies: []businesscard.Comment{{
					ID:     "c-reply",
					Author: businesscard.CommentAuthor{AvatarURL: "https://cdn.example/u2.png"},
				}},
			}}}, nil
		},
	})
	e.GET("/cp/api/v1/public/cards/:id/comments", h.ListRoots)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1/comments", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	if strings.Contains(body, "avatars/u1.png") {
		t.Fatalf("raw avatar key leaked: %s", body)
	}
	if !strings.Contains(body, `"avatarUrl":""`) {
		t.Fatalf("raw avatar key was not cleared: %s", body)
	}
	if !strings.Contains(body, `"avatarUrl":"https://cdn.example/u2.png"`) {
		t.Fatalf("absolute avatar was rewritten: %s", body)
	}
}

func TestCardCommentListRepliesPassesParams(t *testing.T) {
	e := echo.New()
	var gotCard, gotRoot string
	var gotUserID int64
	h := NewCardCommentHandler(&fakeCardCommentService{
		listReplies: func(_ context.Context, cardPublicID, rootPublicID string, userID int64, _ string, _ int) (*businesscard.CommentPage, error) {
			gotCard, gotRoot, gotUserID = cardPublicID, rootPublicID, userID
			return &businesscard.CommentPage{}, nil
		},
	})
	e.GET("/cp/api/v1/public/cards/:id/comments/:commentId/replies", h.ListReplies)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1/comments/c-root/replies", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if gotCard != "card-1" || gotRoot != "c-root" || gotUserID != 0 {
		t.Fatalf("card=%q root=%q userID=%d", gotCard, gotRoot, gotUserID)
	}
}

func TestCardCommentUpdateRequiresLogin(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{})
	e.PUT("/cp/api/v1/public/cards/:id/comments/:commentId", h.Update)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/cp/api/v1/public/cards/card-1/comments/c-1", strings.NewReader(`{"content":"edit"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCardCommentDeleteReturnsResult(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{
		delete: func(context.Context, string, string, int64) (businesscard.CommentDeleteResult, error) {
			return businesscard.CommentDeleteResult{Tombstoned: true, CommentCount: 4}, nil
		},
	})
	e.DELETE("/cp/api/v1/public/cards/:id/comments/:commentId", h.Delete, withCommentSession(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/public/cards/card-1/comments/c-1", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"tombstoned":true`) || !strings.Contains(rec.Body.String(), `"commentCount":4`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardCommentErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid", err: businesscard.ErrCommentInvalid, status: http.StatusBadRequest, code: `"code":40001`},
		{name: "forbidden", err: businesscard.ErrCommentForbidden, status: http.StatusForbidden, code: `"code":40301`},
		{name: "not found", err: businesscard.ErrCommentNotFound, status: http.StatusNotFound, code: `"code":40410`},
		{name: "rate limited", err: businesscard.ErrCommentRateLimit, status: http.StatusTooManyRequests, code: `"code":42901`},
		{name: "internal", err: errors.New("database unavailable"), status: http.StatusInternalServerError, code: `"code":50016`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			h := NewCardCommentHandler(&fakeCardCommentService{
				create: func(context.Context, string, int64, string, string) (*businesscard.Comment, error) {
					return nil, tc.err
				},
			})
			e.POST("/cp/api/v1/public/cards/:id/comments", h.Create, withCommentSession(7))
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi"}`))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			e.ServeHTTP(rec, req)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func newHandlerMetrics(t *testing.T) *observability.Provider {
	t.Helper()
	provider, err := observability.New(context.Background(), "vibebiz-test", "cp")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider
}

func metricsBody(t *testing.T, provider *observability.Provider) string {
	t.Helper()
	rec := httptest.NewRecorder()
	provider.MetricsHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

func TestCardCommentCreateRecordsMetrics(t *testing.T) {
	provider := newHandlerMetrics(t)
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{})
	h.Metrics = provider.Metrics
	e.POST("/cp/api/v1/public/cards/:id/comments", h.Create, withCommentSession(7))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if body := metricsBody(t, provider); !strings.Contains(body, `vibebiz_card_comments_total{action="create",status="ok"} 1`) {
		t.Fatalf("create ok metric missing: %s", body)
	}
}

func TestCardCommentRecordsErrorMetric(t *testing.T) {
	provider := newHandlerMetrics(t)
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{
		create: func(context.Context, string, int64, string, string) (*businesscard.Comment, error) {
			return nil, businesscard.ErrCommentRateLimit
		},
	})
	h.Metrics = provider.Metrics
	e.POST("/cp/api/v1/public/cards/:id/comments", h.Create, withCommentSession(7))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if body := metricsBody(t, provider); !strings.Contains(body, `vibebiz_card_comments_total{action="create",status="rate_limited"} 1`) {
		t.Fatalf("create rate_limited metric missing: %s", body)
	}
}

func TestCardCommentSerializesNullEditedAt(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{})
	e.POST("/cp/api/v1/public/cards/:id/comments", h.Create, withCommentSession(7))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"editedAt":null`) {
		t.Fatalf("editedAt key missing: %s", rec.Body.String())
	}
}

func TestCardCommentListDoesNotRecordMetrics(t *testing.T) {
	provider := newHandlerMetrics(t)
	e := echo.New()
	h := NewCardCommentHandler(&fakeCardCommentService{})
	h.Metrics = provider.Metrics
	e.GET("/cp/api/v1/public/cards/:id/comments", h.ListRoots)
	e.GET("/cp/api/v1/public/cards/:id/comments/:commentId/replies", h.ListReplies)
	for _, path := range []string{
		"/cp/api/v1/public/cards/card-1/comments",
		"/cp/api/v1/public/cards/card-1/comments/c-root/replies",
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
	body := metricsBody(t, provider)
	for _, action := range []string{"list_roots", "list_replies"} {
		if strings.Contains(body, `action="`+action+`"`) {
			t.Fatalf("list action %q recorded: %s", action, body)
		}
	}
}
