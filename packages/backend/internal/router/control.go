package router

import (
	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/client"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
	"vibebiz/internal/store"
)

func RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, sandbox *client.Sandbox, obs *observability.Provider) {
	projectHandler := handler.NewProjectHandler(projects, sandbox, obs)
	adminHandler := handler.NewAdminHandler(repo, projects)

	api := e.Group("/cp/api/v1")
	api.GET("/projects", projectHandler.List)
	api.POST("/projects", projectHandler.Create)
	api.GET("/projects/:id", projectHandler.Get)

	adminAPI := api.Group("/admin")
	adminAPI.GET("/overview", adminHandler.Overview)
	adminAPI.GET("/users", adminHandler.Users)
	adminAPI.GET("/projects", adminHandler.Projects)
	adminAPI.GET("/sandboxes", adminHandler.Sandboxes)
	adminAPI.GET("/quotas", adminHandler.Quotas)
	adminAPI.GET("/audit", adminHandler.Audit)

	registerErrorHandler(e)
}
