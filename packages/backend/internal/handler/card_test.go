package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/businesscard"
)

type fakeCardService struct {
	getMine            func(context.Context, int64, []int64) (*businesscard.Card, error)
	updateProfile      func(context.Context, int64, []int64, string, []string) (*businesscard.Card, error)
	addProject         func(context.Context, int64, []int64, string) (*businesscard.Card, error)
	removeProject      func(context.Context, int64, []int64, string) (*businesscard.Card, error)
	setFeaturedProject func(context.Context, int64, []int64, string) (*businesscard.Card, error)
	uploadFeatured     func(context.Context, int64, []int64, string, string, int64, io.Reader) (*businesscard.Card, error)
	clearFeatured      func(context.Context, int64, []int64) (*businesscard.Card, error)
	publish            func(context.Context, int64, []int64) (*businesscard.Card, error)
	discover           func(context.Context, int, int) (*businesscard.DiscoverResult, error)
	getPublic          func(context.Context, string) (*businesscard.Card, error)
	publicProject      func(context.Context, string) (*businesscard.PublicProject, error)
	adminList          func(context.Context, string, int, int) (*businesscard.AdminListResult, error)
	adminSetHidden     func(context.Context, string, bool) (*businesscard.AdminRow, error)
	adminDelete        func(context.Context, string) error
}

func (f *fakeCardService) GetMine(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error) {
	return f.getMine(ctx, userID, workspaceIDs)
}

func (f *fakeCardService) UpdateProfile(ctx context.Context, userID int64, workspaceIDs []int64, headline string, tags []string) (*businesscard.Card, error) {
	return f.updateProfile(ctx, userID, workspaceIDs, headline, tags)
}

func (f *fakeCardService) AddProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error) {
	return f.addProject(ctx, userID, workspaceIDs, projectID)
}

func (f *fakeCardService) RemoveProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error) {
	return f.removeProject(ctx, userID, workspaceIDs, projectID)
}

func (f *fakeCardService) SetFeaturedProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error) {
	return f.setFeaturedProject(ctx, userID, workspaceIDs, projectID)
}

func (f *fakeCardService) UploadFeatured(ctx context.Context, userID int64, workspaceIDs []int64, contentType, filename string, size int64, body io.Reader) (*businesscard.Card, error) {
	return f.uploadFeatured(ctx, userID, workspaceIDs, contentType, filename, size, body)
}

func (f *fakeCardService) ClearFeatured(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error) {
	return f.clearFeatured(ctx, userID, workspaceIDs)
}

func (f *fakeCardService) Publish(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error) {
	return f.publish(ctx, userID, workspaceIDs)
}

func (f *fakeCardService) Discover(ctx context.Context, page, pageSize int) (*businesscard.DiscoverResult, error) {
	return f.discover(ctx, page, pageSize)
}

func (f *fakeCardService) GetPublic(ctx context.Context, publicID string) (*businesscard.Card, error) {
	return f.getPublic(ctx, publicID)
}

func (f *fakeCardService) PublicProject(ctx context.Context, projectID string) (*businesscard.PublicProject, error) {
	return f.publicProject(ctx, projectID)
}

func (f *fakeCardService) AdminList(ctx context.Context, keyword string, page, pageSize int) (*businesscard.AdminListResult, error) {
	return f.adminList(ctx, keyword, page, pageSize)
}

func (f *fakeCardService) AdminSetHidden(ctx context.Context, publicID string, hidden bool) (*businesscard.AdminRow, error) {
	return f.adminSetHidden(ctx, publicID, hidden)
}

func (f *fakeCardService) AdminDelete(ctx context.Context, publicID string) error {
	return f.adminDelete(ctx, publicID)
}

func fullCardFixture(headline string) *businesscard.Card {
	return &businesscard.Card{
		ID:          "card-1",
		OwnerUserID: 7,
		Headline:    headline,
		Tags:        []string{},
		Items:       []businesscard.CardItem{},
		Published:   true,
	}
}

func newCardEcho(t *testing.T) (*echo.Echo, *fakeCardService) {
	t.Helper()
	fake := &fakeCardService{
		getMine: func(context.Context, int64, []int64) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		updateProfile: func(_ context.Context, _ int64, _ []int64, headline string, _ []string) (*businesscard.Card, error) {
			return fullCardFixture(headline), nil
		},
		addProject: func(context.Context, int64, []int64, string) (*businesscard.Card, error) {
			return nil, businesscard.ErrInvalidProject
		},
		removeProject: func(context.Context, int64, []int64, string) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		setFeaturedProject: func(context.Context, int64, []int64, string) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		uploadFeatured: func(context.Context, int64, []int64, string, string, int64, io.Reader) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		clearFeatured: func(context.Context, int64, []int64) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		publish: func(context.Context, int64, []int64) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		discover: func(context.Context, int, int) (*businesscard.DiscoverResult, error) {
			return &businesscard.DiscoverResult{Cards: []businesscard.Card{}, Total: 0, Page: 1, PageSize: 12}, nil
		},
		getPublic: func(context.Context, string) (*businesscard.Card, error) {
			return fullCardFixture(""), nil
		},
		publicProject: func(_ context.Context, _ string) (*businesscard.PublicProject, error) {
			return &businesscard.PublicProject{
				Card: businesscard.PublicCardSummary{ID: "card-1"},
				Item: businesscard.CardItem{ProjectID: "p1", Name: "Demo", ShareURL: "/p/p1/"},
			}, nil
		},
		adminList: func(context.Context, string, int, int) (*businesscard.AdminListResult, error) {
			return &businesscard.AdminListResult{Rows: []businesscard.AdminRow{}, Total: 0, Page: 1, PageSize: 20}, nil
		},
		adminSetHidden: func(_ context.Context, id string, hidden bool) (*businesscard.AdminRow, error) {
			return &businesscard.AdminRow{ID: id, Hidden: hidden}, nil
		},
		adminDelete: func(context.Context, string) error { return nil },
	}

	h := NewCardHandler(fake, "")
	session := sessionForTest(7)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/me/card", h.GetMine, session)
	routeWithSession(e, http.MethodPut, "/cp/api/v1/me/card", h.UpdateMine, session)
	routeWithSession(e, http.MethodPost, "/cp/api/v1/me/card/items", h.AddItem, session)
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/me/card/items/:projectId", h.RemoveItem, session)
	routeWithSession(e, http.MethodPut, "/cp/api/v1/me/card/featured", h.SetFeatured, session)
	routeWithSession(e, http.MethodPost, "/cp/api/v1/me/card/featured/upload", h.UploadFeatured, session)
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/me/card/featured", h.ClearFeatured, session)
	routeWithSession(e, http.MethodPost, "/cp/api/v1/me/card/publish", h.Publish, session)
	routeWithSession(e, http.MethodGet, "/cp/api/v1/cards/discover", h.Discover, session)
	e.GET("/cp/api/v1/public/cards/:id", h.PublicCard)
	e.GET("/cp/api/v1/public/card-projects/:projectId", h.PublicProject)
	e.GET("/cp/api/v1/admin/cards", h.AdminList)
	e.PUT("/cp/api/v1/admin/cards/:id/hidden", h.AdminHide)
	e.DELETE("/cp/api/v1/admin/cards/:id", h.AdminDelete)
	return e, fake
}

func TestCardMineGetAndUpdate(t *testing.T) {
	e, _ := newCardEcho(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/me/card", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"card-1"`) {
		t.Fatalf("get status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/cp/api/v1/me/card", strings.NewReader(`{"headline":"前端工程师","tags":["Vue","Vue"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"headline":"前端工程师"`) {
		t.Fatalf("put status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardAddProjectValidation(t *testing.T) {
	e, _ := newCardEcho(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/me/card/items", strings.NewReader(`{"projectId":"missing"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":40016`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardUploadTooLarge(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.uploadFeatured = func(context.Context, int64, []int64, string, string, int64, io.Reader) (*businesscard.Card, error) {
		return nil, businesscard.ErrFileTooLarge
	}
	body := strings.NewReader("%PDF-12345")
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/me/card/featured/upload", body)
	req.Header.Set(echo.HeaderContentType, "application/pdf")
	req.Header.Set("X-File-Name", "a.pdf")
	req.ContentLength = 11
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardPublicAndAdminRoutes(t *testing.T) {
	e, _ := newCardEcho(t)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("public status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/cards", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total"`) {
		t.Fatalf("admin status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardPublicEndpointsReturn500OnInfraError(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.getPublic = func(context.Context, string) (*businesscard.Card, error) {
		return nil, errors.New("database unavailable")
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":50016`) {
		t.Fatalf("public card status=%d body=%s", rec.Code, rec.Body.String())
	}

	fake.publicProject = func(context.Context, string) (*businesscard.PublicProject, error) {
		return nil, errors.New("database unavailable")
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/card-projects/p1", nil))
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"code":50016`) {
		t.Fatalf("public project status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardRoutesNormalizeShareURL(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.discover = func(context.Context, int, int) (*businesscard.DiscoverResult, error) {
		return &businesscard.DiscoverResult{
			Cards: []businesscard.Card{{
				ID:    "card-1",
				Items: []businesscard.CardItem{{ProjectID: "p1", Name: "Demo", ShareURL: "/p/p1/"}},
			}},
			Total: 1, Page: 1, PageSize: 12,
		}, nil
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/cards/discover", nil)
	req.Host = "cards.example.com"
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"shareUrl":"http://cards.example.com/p/p1/"`) {
		t.Fatalf("discover status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/card-projects/p1", nil)
	req.Host = "cards.example.com"
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"shareUrl":"http://cards.example.com/p/p1/"`) {
		t.Fatalf("public project status=%d body=%s", rec.Code, rec.Body.String())
	}
}
