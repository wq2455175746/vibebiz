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

func RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, sandbox *client.Sandbox, obs *observability.Provider, authn *auth.Authenticator, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, storageHandler *handler.StorageHandler, metas auth.ProjectMetaStore, notifications *handler.NotificationHandler) {
	projectHandler := handler.NewProjectHandler(projects, sandbox, obs, metas)
	adminHandler := handler.NewAdminHandler(repo, projects)

	api := e.Group("/cp/api/v1")
	api.POST("/auth/login", authHandler.LoginUser)
	api.GET("/auth/me", authHandler.MeUser)
	api.POST("/auth/logout", authHandler.LogoutUser)
	api.PUT("/auth/workspace", authHandler.SwitchWorkspace, authn.RequireUser)

	meAPI := api.Group("/me", authn.RequireUser)
	meAPI.GET("/profile", settingsHandler.GetProfile)
	meAPI.PUT("/profile", settingsHandler.UpdateProfile)
	meAPI.POST("/avatar/presign", settingsHandler.PresignAvatar)
	meAPI.PUT("/avatar", settingsHandler.ConfirmAvatar)
	meAPI.POST("/avatar/upload", settingsHandler.UploadAvatar)

	workspaceAPI := api.Group("/workspaces", authn.RequireUser)
	workspaceAPI.GET("/current", settingsHandler.GetWorkspace)
	workspaceAPI.PUT("/current", settingsHandler.UpdateWorkspace)
	workspaceAPI.POST("/current/avatar/presign", settingsHandler.PresignWorkspaceAvatar)
	workspaceAPI.PUT("/current/avatar", settingsHandler.ConfirmWorkspaceAvatar)
	workspaceAPI.POST("/current/avatar/upload", settingsHandler.UploadWorkspaceAvatar)
	workspaceAPI.GET("/current/members", settingsHandler.Members)
	workspaceAPI.POST("/current/members", settingsHandler.InviteMember)
	workspaceAPI.GET("/current/folders", settingsHandler.Folders)
	workspaceAPI.POST("/current/folders", settingsHandler.CreateFolder)

	api.GET("/storage", storageHandler.Summary, authn.RequireUser)

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

	notificationsAPI := api.Group("/notifications", authn.RequireUser)
	notificationsAPI.GET("", notifications.WebList)
	notificationsAPI.GET("/ws", notifications.Socket)
	notificationsAPI.POST("/read", notifications.MarkRead)

	adminProtected.GET("/notifications", notifications.AdminList)
	adminProtected.POST("/notifications", notifications.AdminCreate)
	adminProtected.PUT("/notifications/:id", notifications.AdminUpdate)
	adminProtected.POST("/notifications/:id/publish", notifications.AdminPublish)
	adminProtected.POST("/notifications/:id/unpublish", notifications.AdminUnpublish)
	adminProtected.DELETE("/notifications/:id", notifications.AdminDelete)

	registerErrorHandler(e)
}
