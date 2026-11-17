package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/businesscard"
)

type fakeCardLikeService struct {
	set        func(context.Context, string, int64, bool) (businesscard.LikeState, error)
	lastCardID string
	lastUserID int64
	lastLiked  bool
	calls      int
}

func (f *fakeCardLikeService) Set(ctx context.Context, cardPublicID string, userID int64, liked bool) (businesscard.LikeState, error) {
	f.calls++
	f.lastCardID, f.lastUserID, f.lastLiked = cardPublicID, userID, liked
	if f.set == nil {
		return businesscard.LikeState{Liked: liked, Count: 1, Found: true}, nil
	}
	return f.set(ctx, cardPublicID, userID, liked)
}

type fakeLikeEnricher struct {
	state      businesscard.LikeState
	err        error
	calls      int
	lastUserID int64
	lastIDs    []int64
}

func (f *fakeLikeEnricher) States(_ context.Context, cards []businesscard.Card, userID int64) error {
	f.calls++
	f.lastUserID = userID
	f.lastIDs = f.lastIDs[:0]
	for i := range cards {
		f.lastIDs = append(f.lastIDs, cards[i].InternalID)
		if f.err == nil {
			cards[i].LikeCount = f.state.Count
			cards[i].Liked = f.state.Liked
		}
	}
	return f.err
}

func TestCardLikeSetRequiresLogin(t *testing.T) {
	e := echo.New()
	h := NewCardLikeHandler(&fakeCardLikeService{})
	e.PUT("/cp/api/v1/public/cards/:id/like", h.Set)
	e.DELETE("/cp/api/v1/public/cards/:id/like", h.Unset)
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(method, "/cp/api/v1/public/cards/card-1/like", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d", method, rec.Code)
		}
	}
}

func TestCardLikeSetReturnsState(t *testing.T) {
	e := echo.New()
	fake := &fakeCardLikeService{}
	h := NewCardLikeHandler(fake)
	e.PUT("/cp/api/v1/public/cards/:id/like", h.Set, withCommentSession(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/cp/api/v1/public/cards/card-1/like", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"liked":true`) || !strings.Contains(body, `"likeCount":1`) {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	if fake.lastCardID != "card-1" || fake.lastUserID != 7 || !fake.lastLiked {
		t.Fatalf("call card=%q user=%d liked=%v", fake.lastCardID, fake.lastUserID, fake.lastLiked)
	}
}

func TestCardLikeUnsetReturnsState(t *testing.T) {
	e := echo.New()
	fake := &fakeCardLikeService{
		set: func(_ context.Context, _ string, _ int64, liked bool) (businesscard.LikeState, error) {
			return businesscard.LikeState{Liked: liked, Count: 0, Found: true}, nil
		},
	}
	h := NewCardLikeHandler(fake)
	e.DELETE("/cp/api/v1/public/cards/:id/like", h.Unset, withCommentSession(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/public/cards/card-1/like", nil))
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, `"liked":false`) || !strings.Contains(body, `"likeCount":0`) {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	if fake.lastLiked {
		t.Fatalf("delete should pass liked=false")
	}
}

func TestCardLikeNotFound(t *testing.T) {
	e := echo.New()
	h := NewCardLikeHandler(&fakeCardLikeService{
		set: func(context.Context, string, int64, bool) (businesscard.LikeState, error) {
			return businesscard.LikeState{}, businesscard.ErrNotFound
		},
	})
	e.PUT("/cp/api/v1/public/cards/:id/like", h.Set, withCommentSession(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/cp/api/v1/public/cards/card-1/like", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":40410`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardLikeStoreErrorReturns503(t *testing.T) {
	e := echo.New()
	h := NewCardLikeHandler(&fakeCardLikeService{
		set: func(context.Context, string, int64, bool) (businesscard.LikeState, error) {
			return businesscard.LikeState{}, errors.New("redis unavailable")
		},
	})
	e.PUT("/cp/api/v1/public/cards/:id/like", h.Set, withCommentSession(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/cp/api/v1/public/cards/card-1/like", nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"code":50304`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublicCardIncludesLikeState(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.likes = &fakeLikeEnricher{state: businesscard.LikeState{Liked: true, Count: 3}}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1", nil))
	if !strings.Contains(rec.Body.String(), `"likeCount":3`) || !strings.Contains(rec.Body.String(), `"liked":true`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestPublicCardLikesUseViewerSession(t *testing.T) {
	fake := &fakeCardService{
		getPublic: func(context.Context, string) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
	}
	enricher := &fakeLikeEnricher{state: businesscard.LikeState{Liked: true, Count: 2}}
	h := NewCardHandler(fake, "")
	h.Likes = enricher
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/public/cards/:id", h.PublicCard, sessionForTest(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if enricher.lastUserID != 7 {
		t.Fatalf("userID = %d, want 7", enricher.lastUserID)
	}
	if !strings.Contains(rec.Body.String(), `"likeCount":2`) || !strings.Contains(rec.Body.String(), `"liked":true`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestCardMineAndDiscoverIncludeLikeState(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.likes = &fakeLikeEnricher{state: businesscard.LikeState{Liked: true, Count: 4}}
	fake.discover = func(context.Context, int, int) (*businesscard.DiscoverResult, error) {
		return &businesscard.DiscoverResult{Cards: []businesscard.Card{*fullCardFixture("")}}, nil
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/me/card", nil))
	if !strings.Contains(rec.Body.String(), `"likeCount":4`) {
		t.Fatalf("mine body = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/cards/discover", nil))
	if !strings.Contains(rec.Body.String(), `"likeCount":4`) || !strings.Contains(rec.Body.String(), `"liked":true`) {
		t.Fatalf("discover body = %s", rec.Body.String())
	}
}

func TestCardLikeRecordsMetrics(t *testing.T) {
	provider := newHandlerMetrics(t)
	e := echo.New()
	h := NewCardLikeHandler(&fakeCardLikeService{})
	h.Metrics = provider.Metrics
	e.PUT("/cp/api/v1/public/cards/:id/like", h.Set, withCommentSession(7))
	e.DELETE("/cp/api/v1/public/cards/:id/like", h.Unset, withCommentSession(7))
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(method, "/cp/api/v1/public/cards/card-1/like", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status=%d body=%s", method, rec.Code, rec.Body.String())
		}
	}
	body := metricsBody(t, provider)
	for _, want := range []string{
		`vibebiz_card_likes_total{action="like",status="ok"} 1`,
		`vibebiz_card_likes_total{action="unlike",status="ok"} 1`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metric %q missing: %s", want, body)
		}
	}
}

func TestCardLikeRecordsNotFoundMetric(t *testing.T) {
	provider := newHandlerMetrics(t)
	e := echo.New()
	h := NewCardLikeHandler(&fakeCardLikeService{
		set: func(context.Context, string, int64, bool) (businesscard.LikeState, error) {
			return businesscard.LikeState{}, businesscard.ErrNotFound
		},
	})
	h.Metrics = provider.Metrics
	e.PUT("/cp/api/v1/public/cards/:id/like", h.Set, withCommentSession(7))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/cp/api/v1/public/cards/card-1/like", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if body := metricsBody(t, provider); !strings.Contains(body, `vibebiz_card_likes_total{action="like",status="not_found"} 1`) {
		t.Fatalf("like not_found metric missing: %s", body)
	}
}
