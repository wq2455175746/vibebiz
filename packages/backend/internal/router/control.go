package router

import (
	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
	"vibebiz/internal/store"
)

func RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, sandbox *client.Sandbox, obs *observability.Provider, authn *auth.Authenticator, authHandler *handler.AuthHandler, metas auth.ProjectMetaStore) {
	projectHandler := handler.NewProjectHandler(projects, sandbox, obs, metas)
	adminHandler := handler.NewAdminHandler(repo, projects)

	api := e.Group("/cp/api/v1")
	api.POST("/auth/login", authHandler.LoginUser)
	api.GET("/auth/me", authHandler.MeUser)
	api.POST("/auth/logout", authHandler.LogoutUser)

	projectsAPI := api.Group("/projects", authn.RequireUser)
	projectsAPI.GET("", projectHandler.List)
	projectsAPI.POST("", projectHandler.Create)
	projectsAPI.GET("/:id", projectHandler.Get)
	projectsAPI.DELETE("/:id", projectHandler.Delete)

	adminAPI := api.Group("/admin")
	adminAPI.POST("/auth/login", authHandler.LoginOps)
	adminAPI.GET("/auth/me", authHandler.MeOps)
	adminAPI.POST("/auth/logout", authHandler.LogoutOps)

	adminProtected := adminAPI.Group("", authn.RequireOps)
	adminProtected.GET("/overview", adminHandler.Overview)
	adminProtected.GET("/users", adminHandler.Users)
	adminProtected.GET("/projects", adminHandler.Projects)
	adminProtected.GET("/sandboxes", adminHandler.Sandboxes)
	adminProtected.GET("/quotas", adminHandler.Quotas)
	adminProtected.GET("/audit", adminHandler.Audit)

	registerErrorHandler(e)
}
