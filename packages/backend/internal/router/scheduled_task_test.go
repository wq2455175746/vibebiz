package router

import (
	"net/http"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/handler"
)

func TestScheduledTaskRoutesRegistered(t *testing.T) {
	e := echo.New()
	authn := auth.NewAuthenticator(nil, auth.CookieManager{}, auth.CookieManager{})
	RegisterScheduledTasks(e, handler.NewScheduledTaskHandler(nil, nil), authn)
	routes := map[string]bool{}
	for _, route := range e.Routes() {
		routes[route.Method+" "+route.Path] = true
	}
	for _, want := range []string{
		http.MethodPost + " /cp/api/v1/scheduled-tasks/parse",
		http.MethodGet + " /cp/api/v1/scheduled-tasks",
		http.MethodGet + " /cp/api/v1/scheduled-tasks/:id",
		http.MethodPut + " /cp/api/v1/scheduled-tasks/:id",
		http.MethodDelete + " /cp/api/v1/scheduled-tasks/:id",
		http.MethodGet + " /cp/api/v1/admin/scheduled-tasks",
		http.MethodPost + " /cp/api/v1/admin/scheduled-tasks/:id/stop",
	} {
		if !routes[want] {
			t.Errorf("missing route %s", want)
		}
	}
}
