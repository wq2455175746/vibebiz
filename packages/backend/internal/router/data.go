package router

import (
	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
)

type DataSandbox interface {
	handler.FileClient
	handler.ChatClient
	handler.ChatStopClient
	handler.ChatStatusClient
	handler.ChatLogsClient
	handler.SkillClient
	handler.PreviewClient
}

func RegisterData(e *echo.Echo, sandbox DataSandbox, obs *observability.Provider, authn *auth.Authenticator, metas auth.ProjectMetaStore, resources *handler.ResourceHandler, roles handler.ProjectRoleResolver, covers *handler.CoverHandler, storageSyncer handler.StorageSyncer, cloneHandler *handler.CloneHandler) {
	files := handler.NewFileHandler(sandbox, obs)
	chat := handler.NewChatHandler(sandbox, obs)
	chat.Roles = roles
	chat.Storage = storageSyncer
	stopChat := handler.NewChatStopHandler(sandbox, obs)
	chatStatus := handler.NewChatStatusHandler(sandbox, obs)
	chatLogs := handler.NewChatLogsHandler(sandbox, obs)
	skills := handler.NewSkillHandler(sandbox, obs)
	preview := handler.NewPreviewHandler(sandbox, obs)

	api := e.Group("/dp/api/v1")
	projects := api.Group("/projects/:id", authn.RequireUser, auth.RequireProjectAccess(metas))
	if resources != nil {
		api.POST("/resources/presign", resources.Presign, authn.RequireUser)
		api.POST("/resources/staging/:resourceId/upload", resources.UploadStaging, authn.RequireUser)
		api.DELETE("/resources/staging/:resourceId", resources.DeleteStaging, authn.RequireUser)
		projects.POST("/resources/adopt", resources.Adopt)
		projects.GET("/resources", resources.List)
		projects.DELETE("/resources/:resourceId", resources.Delete)
	}
	projects.GET("/files", files.List)
	projects.GET("/file", files.Read)
	projects.PUT("/file", files.Write)
	projects.POST("/chat", chat.Chat)
	projects.POST("/chat/stop", stopChat.Stop)
	projects.GET("/chat/status", chatStatus.Get)
	projects.GET("/logs", chatLogs.Get)
	projects.GET("/skills", skills.List)
	projects.GET("/preview/*", preview.Serve)
	if covers != nil {
		projects.POST("/cover", covers.Upload)
	}
	if cloneHandler != nil {
		projects.GET("/clone/assets", cloneHandler.ListAssets)
		projects.POST("/clone/assets", cloneHandler.AddAsset)
		projects.DELETE("/clone/assets/:resourceId", cloneHandler.DeleteAsset)
		projects.PUT("/clone/assets/:resourceId/cover", cloneHandler.SetCover)
		projects.POST("/clone/tests/run", cloneHandler.RunTests)
	}

	registerErrorHandler(e)
}
