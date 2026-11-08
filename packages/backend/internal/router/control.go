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
	"vibebiz/internal/usage"
)

func RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, provisioner handler.ProjectInitializer, obs *observability.Provider, authn *auth.Authenticator, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, storageHandler *handler.StorageHandler, metas auth.ProjectMetaStore, notifications *handler.NotificationHandler, registry *handler.SandboxRegistryHandler, models *handler.ModelHandler, roles *handler.RoleHandler, coverSigner handler.CoverURLSigner, coverExpiry time.Duration, adminSandbox handler.AdminSandbox, templates *handler.TemplateHandler, harness handler.HarnessImporter, stageStore projectstage.Store, stageSandbox handler.StageSandbox, notificationChannels *handler.NotificationChannelHandler, fdeResources *handler.FdeResourceHandler, publishApp *handler.PublishHandler, channelPackages *handler.ChannelPackageHandler, channelGuides *handler.ChannelGuideHandler, cloneHandler *handler.CloneHandler, usageService *usage.Service, accessLogs *handler.AccessLogHandler, skillManager *handler.SkillManager, leaderboardHandler *handler.LeaderboardHandler, evolutionHandler *handler.EvolutionHandler) {
	var roleStore handler.AgentRoleStore
	if roles != nil {
		roleStore = roles.Service
	}
	projectHandler := handler.NewProjectHandler(projects, provisioner, obs, metas, roleStore)
	projectHandler.Covers = coverSigner
	projectHandler.CoverExpiry = coverExpiry
	projectHandler.Harness = harness
	projectHandler.Stages = stageStore
	projectHandler.Usage = usageService
	var stageHandler *handler.StageHandler
	if stageStore != nil {
		stageHandler = handler.NewStageHandler(projects, stageStore, stageSandbox)
	}
	adminHandler := handler.NewAdminHandler(repo, projects)
	if authn != nil && authn.Service != nil {
		if revoker, ok := authn.Service.Sessions.(auth.SessionRevoker); ok {
			adminHandler.Sessions = revoker
		}
	}
	adminHandler.Sandbox = adminSandbox
	adminHandler.Covers = coverSigner
	adminHandler.CoverExpiry = coverExpiry
	adminHandler.Metas = metas
	adminHandler.Usage = usageService
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
	api.GET("/auth/me", authHandler.MeUser, authHandler.AttachUserSession)
	api.POST("/auth/logout", authHandler.LogoutUser, authHandler.AttachUserSession)
	api.PUT("/auth/workspace", authHandler.SwitchWorkspace, authn.RequireUser)

	meAPI := api.Group("/me", authn.RequireUser)
	meAPI.GET("/profile", settingsHandler.GetProfile)
	if skillManager != nil {
		meAPI.GET("/skills", skillManager.ListMine)
		meAPI.POST("/skills", skillManager.UploadMine)
		meAPI.DELETE("/skills/:name", skillManager.DeleteMine)
	}
	if publishApp != nil {
		meAPI.GET("/published-projects", publishApp.ListPublished)
		meAPI.GET("/published-projects/stats", publishApp.PublishedStats)
	}
	if channelGuides != nil {
		api.GET("/channel-guides", channelGuides.PublicList, authn.RequireUser)
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
		e.POST("/p/:slug/track/click", publishApp.TrackClick)
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
	if channelPackages != nil {
		projectsAPI.POST("/:id/channel-packages", channelPackages.Generate)
		projectsAPI.GET("/:id/channel-packages", channelPackages.List)
		projectsAPI.GET("/:id/channel-packages/:packageId/download", channelPackages.Download)
		projectsAPI.DELETE("/:id/channel-packages/:packageId", channelPackages.Delete)
	}
	if stageHandler != nil {
		projectsAPI.GET("/:id/stages", stageHandler.List)
		projectsAPI.POST("/:id/stages/:stageKey/start", stageHandler.Start)
		projectsAPI.POST("/:id/stages/:stageKey/complete", stageHandler.Complete)
	}
	if cloneHandler != nil {
		clonesAPI := api.Group("/clones", authn.RequireUser)
		clonesAPI.POST("", cloneHandler.Create)
		clonesAPI.GET("", cloneHandler.List)
		clonesAPI.GET("/:id", cloneHandler.Get)
		clonesAPI.DELETE("/:id", cloneHandler.Delete)
	}

	if leaderboardHandler != nil {
		api.GET("/leaderboard", leaderboardHandler.ListPublic, authn.RequireUser)
	}

	if evolutionHandler != nil {
		evolveAPI := api.Group("/evolve", authn.RequireUser)
		evolveAPI.GET("", evolutionHandler.List)
		evolveAPI.GET("/conversations/:userId", evolutionHandler.Conversation)
		evolveAPI.POST("/draft", evolutionHandler.EnsureDraft)
		evolveAPI.POST("/proposals", evolutionHandler.SubmitProposal)
		evolveAPI.POST("/chat", evolutionHandler.SubmitChat)
		evolveAPI.POST("/versions/:id/confirm", evolutionHandler.ConfirmVersion)
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
	adminProtected.PUT("/auth/password", authHandler.ChangeOpsPassword)
	adminProtected.GET("/overview", adminHandler.Overview)
	if skillManager != nil {
		adminProtected.GET("/skills", skillManager.AdminList)
		adminProtected.POST("/skills", skillManager.AdminUpload)
		adminProtected.DELETE("/skills/:name", skillManager.AdminDelete)
	}
	if channelGuides != nil {
		adminProtected.GET("/channel-guides", channelGuides.List)
		adminProtected.GET("/channel-guides/:channel", channelGuides.Get)
		adminProtected.POST("/channel-guides", channelGuides.Create)
		adminProtected.PUT("/channel-guides/:channel", channelGuides.Update)
		adminProtected.DELETE("/channel-guides/:channel", channelGuides.Delete)
		adminProtected.POST("/channel-guides/:channel/markdown", channelGuides.UploadMarkdown)
	}
	adminProtected.GET("/users", adminHandler.Users)
	adminProtected.GET("/users/:id", adminHandler.UserDetail)
	adminProtected.PUT("/users/:id/status", adminHandler.UpdateUserStatus)
	adminProtected.PUT("/users/:id/password", adminHandler.UpdateUserPassword)
	adminProtected.GET("/projects", adminHandler.Projects)
	adminProtected.GET("/projects/:id/files", adminHandler.ProjectFiles)
	adminProtected.GET("/projects/:id/live/*", adminHandler.ProjectLive)
	adminProtected.PUT("/projects/:id/review", adminHandler.ReviewProject)
	adminProtected.PUT("/projects/:id/hidden", adminHandler.UpdateProjectHidden)
	adminProtected.GET("/sandboxes", adminHandler.Sandboxes)
	adminProtected.GET("/quotas", adminHandler.Quotas)
	adminProtected.PUT("/quotas/:userId", adminHandler.UpdateQuota)
	adminProtected.GET("/usage/daily", adminHandler.UsageDaily)
	adminProtected.GET("/audit", adminHandler.Audit)
	if accessLogs != nil {
		adminProtected.GET("/access-logs", accessLogs.List)
		adminProtected.GET("/access-logs/ip-stats", accessLogs.IPStats)
	}
	if leaderboardHandler != nil {
		adminProtected.GET("/leaderboard", leaderboardHandler.ListAdmin)
		adminProtected.PUT("/leaderboard/:projectId/hidden", leaderboardHandler.Hide)
		adminProtected.DELETE("/leaderboard/:projectId", leaderboardHandler.Delete)
	}
	if evolutionHandler != nil {
		adminProtected.GET("/evolve", evolutionHandler.AdminSnapshot)
		adminProtected.GET("/evolve/conversations/:userId", evolutionHandler.AdminConversation)
		adminProtected.POST("/evolve/versions/:id/reply", evolutionHandler.AdminReply)
		adminProtected.POST("/evolve/tasks", evolutionHandler.AdminSubmitTask)
		adminProtected.PATCH("/evolve/tasks/:id", evolutionHandler.AdminUpdateTask)
	}
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
