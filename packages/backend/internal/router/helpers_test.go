package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
	"vibebiz/internal/handler"
	"vibebiz/internal/llmmodel"
	"vibebiz/internal/notification"
	"vibebiz/internal/observability"
	"vibebiz/internal/server"
	"vibebiz/internal/store"
)

type routerNotificationRepo struct{}

func (routerNotificationRepo) List(context.Context, dao.NotificationFilter, int, int) ([]model.OpsNotification, int64, error) {
	return nil, 0, nil
}

func (routerNotificationRepo) Get(context.Context, int64) (*model.OpsNotification, error) {
	return nil, dao.ErrNotFound
}

func (routerNotificationRepo) Create(context.Context, *model.OpsNotification) error {
	return nil
}

func (routerNotificationRepo) Update(context.Context, *model.OpsNotification) error {
	return nil
}

func (routerNotificationRepo) UpdateStatus(context.Context, int64, string, *time.Time) error {
	return nil
}

func (routerNotificationRepo) Delete(context.Context, int64) error {
	return nil
}

func (routerNotificationRepo) Published(context.Context, time.Time) ([]model.OpsNotification, error) {
	return nil, nil
}

type routerNotificationCache struct{}

func (routerNotificationCache) Put(context.Context, model.OpsNotification) error {
	return nil
}

func (routerNotificationCache) Remove(context.Context, int64) error {
	return nil
}

func (routerNotificationCache) List(context.Context) ([]notification.Published, error) {
	return []notification.Published{}, nil
}

func (routerNotificationCache) Rebuild(context.Context, []model.OpsNotification) error {
	return nil
}

func (routerNotificationCache) LastRead(context.Context, int64) (time.Time, error) {
	return time.Time{}, nil
}

func (routerNotificationCache) MarkRead(context.Context, int64, time.Time) error {
	return nil
}

func newNotificationHandler(t *testing.T) *handler.NotificationHandler {
	t.Helper()
	hub := notification.NewHub()
	t.Cleanup(hub.Close)
	service := notification.NewService(routerNotificationRepo{}, routerNotificationCache{}, hub)
	return handler.NewNotificationHandler(service, hub, nil)
}

func newProvider(t *testing.T, plane string) *observability.Provider {
	t.Helper()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	p, err := observability.New(context.Background(), "vibebiz-"+plane, plane)
	if err != nil {
		t.Fatalf("observability.New: %v", err)
	}
	t.Cleanup(func() {
		_ = p.Shutdown(context.Background())
	})
	return p
}

type routerSessionStore struct {
	items map[string]*auth.Session
}

func newRouterSessionStore() *routerSessionStore {
	return &routerSessionStore{items: make(map[string]*auth.Session)}
}

func routerSessionKey(realm auth.Realm, id string) string {
	return string(realm) + ":" + id
}

func (s *routerSessionStore) Create(_ context.Context, session *auth.Session, _ time.Duration) error {
	copied := *session
	s.items[routerSessionKey(session.Realm, session.ID)] = &copied
	return nil
}

func (s *routerSessionStore) Get(_ context.Context, realm auth.Realm, id string) (*auth.Session, error) {
	session, ok := s.items[routerSessionKey(realm, id)]
	if !ok {
		return nil, auth.ErrSessionNotFound
	}
	copied := *session
	return &copied, nil
}

func (s *routerSessionStore) Update(_ context.Context, session *auth.Session) error {
	key := routerSessionKey(session.Realm, session.ID)
	if _, ok := s.items[key]; !ok {
		return auth.ErrSessionNotFound
	}
	copied := *session
	s.items[key] = &copied
	return nil
}

func (s *routerSessionStore) Delete(_ context.Context, realm auth.Realm, id string) error {
	delete(s.items, routerSessionKey(realm, id))
	return nil
}

type routerMetaStore struct {
	items map[string]auth.ProjectMeta
}

func newRouterMetaStore() *routerMetaStore {
	return &routerMetaStore{items: make(map[string]auth.ProjectMeta)}
}

func (s *routerMetaStore) Set(_ context.Context, projectID string, meta auth.ProjectMeta) error {
	s.items[projectID] = meta
	return nil
}

func (s *routerMetaStore) Get(_ context.Context, projectID string) (auth.ProjectMeta, error) {
	meta, ok := s.items[projectID]
	if !ok {
		return auth.ProjectMeta{}, auth.ErrProjectMetaNotFound
	}
	return meta, nil
}

func (s *routerMetaStore) Delete(_ context.Context, projectID string) error {
	delete(s.items, projectID)
	return nil
}

type routerAuth struct {
	authn      *auth.Authenticator
	handler    *handler.AuthHandler
	sessions   *routerSessionStore
	metas      *routerMetaStore
	userCookie *http.Cookie
	opsCookie  *http.Cookie
}

func cookieForSession(t *testing.T, manager auth.CookieManager, sid string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	if err := manager.Set(c, auth.CookieIdentity{SID: sid, Username: "u", Email: "u@example.com"}); err != nil {
		t.Fatalf("set cookie: %v", err)
	}
	return rec.Result().Cookies()[0]
}

func newRouterAuth(t *testing.T) *routerAuth {
	t.Helper()
	sessions := newRouterSessionStore()
	sessions.items[routerSessionKey(auth.RealmUser, "user-sid")] = &auth.Session{
		ID: "user-sid", Realm: auth.RealmUser, UserID: 7, Username: "wanghg11", Email: "wang@example.com", WorkspaceIDs: []int64{1, 2},
	}
	sessions.items[routerSessionKey(auth.RealmOps, "ops-sid")] = &auth.Session{
		ID: "ops-sid", Realm: auth.RealmOps, UserID: 9, Username: "admin", Email: "admin@example.com", Role: "super_admin",
	}
	service := &auth.Service{Sessions: sessions, UserTTL: time.Hour, OpsTTL: time.Hour, Now: time.Now}
	userCookie := auth.CookieManager{Name: "vibebiz_user", TTL: time.Hour, Now: time.Now}
	opsCookie := auth.CookieManager{Name: "vibebiz_ops", TTL: time.Hour, Now: time.Now}
	return &routerAuth{
		authn:      &auth.Authenticator{Service: service, UserCookie: userCookie, OpsCookie: opsCookie},
		handler:    &handler.AuthHandler{Service: service, UserCookie: userCookie, OpsCookie: opsCookie},
		sessions:   sessions,
		metas:      newRouterMetaStore(),
		userCookie: cookieForSession(t, userCookie, "user-sid"),
		opsCookie:  cookieForSession(t, opsCookie, "ops-sid"),
	}
}

type staticProvisioner struct {
	id  int64
	err error
}

func (s staticProvisioner) InitProject(context.Context, string) (int64, error) {
	return s.id, s.err
}

type singleResolver string

func (s singleResolver) ResolveBaseURL(context.Context, string) (string, error) {
	return string(s), nil
}

func newCP(t *testing.T, projects store.ProjectStore) (*echo.Echo, *observability.Provider, *routerAuth) {
	t.Helper()
	obs := newProvider(t, "cp")
	authState := newRouterAuth(t)
	e := server.New(config.Config{Plane: "cp", ServiceName: "vibebiz-cp"}, obs)
	RegisterControl(e, admin.NewMemoryRepository(), projects, staticProvisioner{id: 1}, obs, authState.authn, authState.handler, &handler.SettingsHandler{}, &handler.StorageHandler{}, authState.metas, newNotificationHandler(t), nil, newModelHandler(t), newRoleHandler(t))
	return e, obs, authState
}

func newDP(t *testing.T, sandboxURL string) (*echo.Echo, *observability.Provider, *routerAuth) {
	t.Helper()
	obs := newProvider(t, "dp")
	authState := newRouterAuth(t)
	authState.metas.items["p1"] = auth.ProjectMeta{WorkspaceID: 1, OwnerUserID: 7}
	e := server.New(config.Config{Plane: "dp", ServiceName: "vibebiz-dp"}, obs)
	RegisterData(e, client.NewRouter(singleResolver(sandboxURL), obs), obs, authState.authn, authState.metas, nil, nil)
	return e, obs, authState
}

func newSandboxServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/files"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
		case strings.HasSuffix(r.URL.Path, "/file"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"path":"index.html","content":"x","version":"v1","size":1,"mtime":1,"isBinary":false}}`))
		case strings.HasSuffix(r.URL.Path, "/skills"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"skills":[{"name":"demo","description":"demo skill","source":"project"}]}}`))
		default:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serve(e *echo.Echo, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func serveWithAuth(e *echo.Echo, method, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func serveWithAuthBody(e *echo.Echo, method, target string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

type mapResolver map[string]string

func (m mapResolver) ResolveBaseURL(_ context.Context, projectID string) (string, error) {
	return m[projectID], nil
}

func newModelHandler(t *testing.T) *handler.ModelHandler {
	t.Helper()
	return handler.NewModelHandler(llmmodel.NewService(llmmodel.NewMemoryRepository(), nil))
}

func newRoleHandler(t *testing.T) *handler.RoleHandler {
	t.Helper()
	return handler.NewRoleHandler(agentrole.NewService(agentrole.NewMemoryRepository(), nil, 0))
}
