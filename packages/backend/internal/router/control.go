package router

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
	"vibebiz/internal/projectstage"
	"vibebiz/internal/store"
)

func RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, provisioner handler.ProjectInitializer, obs *observability.Provider, authn *auth.Authenticator, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, storageHandler *handler.StorageHandler, metas auth.ProjectMetaStore, notifications *handler.NotificationHandler, registry *handler.SandboxRegistryHandler, models *handler.ModelHandler, roles *handler.RoleHandler, coverSigner handler.CoverURLSigner, coverExpiry time.Duration, adminSandbox handler.AdminSandbox, templates *handler.TemplateHandler, harness handler.HarnessImporter, stageStore projectstage.Store, stageSandbox handler.StageSandbox, notificationChannels *handler.NotificationChannelHandler, fdeResources *handler.FdeResourceHandler, publishApp *handler.PublishHandler) {
	var roleStore handler.AgentRoleStore
	if roles != nil {
		roleStore = roles.Service
	}
	projectHandler := handler.NewProjectHandler(projects, provisioner, obs, metas, roleStore)
	projectHandler.Covers = coverSigner
	projectHandler.CoverExpiry = coverExpiry
	projectHandler.Harness = harness
	projectHandler.Stages = stageStore
	var stageHandler *handler.StageHandler
	if stageStore != nil {
		stageHandler = handler.NewStageHandler(projects, stageStore, stageSandbox)
	}
	adminHandler := handler.NewAdminHandler(repo, projects)
	adminHandler.Sandbox = adminSandbox
	adminHandler.Covers = coverSigner
	adminHandler.CoverExpiry = coverExpiry
	adminHandler.Metas = metas
	if notifications != nil {
		adminHandler.Notifications = notifications.Service
	}

	api := e.Group("/cp/api/v1")
	if registry != nil {
		api.POST("/internal/sandboxes/heartbeat", registry.Heartbeat)
	}
	if templates != nil {
		api.GET("/templates", templates.List, authn.RequireUser)
		api.GET("/templates/topics", templates.Topics, authn.RequireUser)
		api.POST("/templates/:id/use", templates.Use, authn.RequireUser)
	}
	api.POST("/auth/login", authHandler.LoginUser)
	api.GET("/auth/me", authHandler.MeUser)
	api.POST("/auth/logout", authHandler.LogoutUser)
	api.PUT("/auth/workspace", authHandler.SwitchWorkspace, authn.RequireUser)

	meAPI := api.Group("/me", authn.RequireUser)
	meAPI.GET("/profile", settingsHandler.GetProfile)
	if publishApp != nil {
		meAPI.GET("/published-projects", publishApp.ListPublished)
	}
	meAPI.PUT("/profile", settingsHandler.UpdateProfile)
	meAPI.POST("/avatar/presign", settingsHandler.PresignAvatar)
	meAPI.PUT("/avatar", settingsHandler.ConfirmAvatar)
	meAPI.POST("/avatar/upload", settingsHandler.UploadAvatar)
	if notificationChannels != nil {
		meAPI.GET("/notification-channels", notificationChannels.List)
		meAPI.POST("/notification-channels", notificationChannels.Create)
		meAPI.PUT("/notification-channels/:id", notificationChannels.Update)
		meAPI.DELETE("/notification-channels/:id", notificationChannels.Delete)
		meAPI.POST("/notification-channels/:id/test", notificationChannels.Test)
	}

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

	if roles != nil {
		api.GET("/agent-roles", roles.PublicList, authn.RequireUser)
	}
	if fdeResources != nil {
		api.GET("/fde-resources", fdeResources.UserList, authn.RequireUser)
	}
	if publishApp != nil {
		e.GET("/p/:slug", publishApp.Preview)
		e.GET("/p/:slug/*", publishApp.Preview)
	}

	projectsAPI := api.Group("/projects", authn.RequireUser)
	projectsAPI.GET("", projectHandler.List)
	projectsAPI.POST("", projectHandler.Create)
	projectsAPI.GET("/:id", projectHandler.Get)
	projectsAPI.DELETE("/:id", projectHandler.Delete)
	projectsAPI.PUT("/:id/agent-roles", projectHandler.UpdateAgentRoles)
	projectsAPI.PUT("/:id/entry", projectHandler.UpdatePreviewPath)
	if publishApp != nil {
		projectsAPI.POST("/:id/publish", publishApp.Publish)
		projectsAPI.DELETE("/:id/publish", publishApp.DeletePublished)
	}
	if stageHandler != nil {
		projectsAPI.GET("/:id/stages", stageHandler.List)
		projectsAPI.POST("/:id/stages/:stageKey/start", stageHandler.Start)
		projectsAPI.POST("/:id/stages/:stageKey/complete", stageHandler.Complete)
	}

	adminAPI := api.Group("/admin")
	adminAPI.POST("/auth/login", authHandler.LoginOps)
	adminAPI.GET("/auth/me", authHandler.MeOps)
	adminAPI.POST("/auth/logout", authHandler.LogoutOps)

	adminProtected := adminAPI.Group("", authn.RequireOps)
	var auditLogger *slog.Logger
	if obs != nil {
		auditLogger = obs.Logger
	}
	adminProtected.Use(admin.AuditMiddleware(repo, auditLogger))
	adminProtected.GET("/overview", adminHandler.Overview)
	adminProtected.GET("/users", adminHandler.Users)
	adminProtected.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
	adminProtected.GET("/projects", adminHandler.Projects)
	adminProtected.GET("/projects/:id/files", adminHandler.ProjectFiles)
	adminProtected.GET("/projects/:id/live/*", adminHandler.ProjectLive)
	adminProtected.PUT("/projects/:id/review", adminHandler.ReviewProject)
	adminProtected.GET("/sandboxes", adminHandler.Sandboxes)
	adminProtected.GET("/quotas", adminHandler.Quotas)
	adminProtected.PUT("/quotas/:userId", adminHandler.UpdateQuota)
	adminProtected.GET("/audit", adminHandler.Audit)
	if models != nil {
		adminProtected.GET("/models", models.List)
		adminProtected.POST("/models", models.Create)
	}
	if templates != nil {
		adminProtected.GET("/topics", templates.Topics)
		adminProtected.POST("/topics", templates.CreateTopic)
		adminProtected.PUT("/topics/:id", templates.UpdateTopic)
		adminProtected.POST("/projects/:id/templates", templates.Create)
		adminProtected.GET("/templates", templates.AdminList)
		adminProtected.PUT("/templates/:id", templates.AdminUpdate)
		adminProtected.DELETE("/templates/:id", templates.AdminDelete)
		adminProtected.GET("/templates/:id/files", templates.AdminFiles)
	}
	if fdeResources != nil {
		adminProtected.GET("/fde-resources", fdeResources.AdminList)
		adminProtected.POST("/fde-resources", fdeResources.Create)
		adminProtected.PUT("/fde-resources/:id", fdeResources.Update)
	}
	if roles != nil {
		adminProtected.GET("/roles", roles.List)
		adminProtected.POST("/roles", roles.Create)
		adminProtected.PUT("/roles/:id", roles.Update)
		adminProtected.POST("/roles/avatar/presign", roles.PresignAvatar)
		adminProtected.POST("/roles/avatar/upload", roles.UploadAvatar)
	}

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

func RegisterChatHistory(e *echo.Echo, history *handler.ChatHistoryHandler, authn *auth.Authenticator) {
	api := e.Group("/cp/api/v1/projects", authn.RequireUser)
	api.GET("/:id/chat/history", history.Get)
}
