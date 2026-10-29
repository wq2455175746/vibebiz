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
	handler.SkillClient
	handler.PreviewClient
}

func RegisterData(e *echo.Echo, sandbox DataSandbox, obs *observability.Provider, authn *auth.Authenticator, metas auth.ProjectMetaStore, resources *handler.ResourceHandler, roles handler.ProjectRoleResolver) {
	files := handler.NewFileHandler(sandbox, obs)
	chat := handler.NewChatHandler(sandbox, obs)
	chat.Roles = roles
	stopChat := handler.NewChatStopHandler(sandbox, obs)
	chatStatus := handler.NewChatStatusHandler(sandbox, obs)
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
	projects.GET("/skills", skills.List)
	projects.GET("/preview/*", preview.Serve)

	registerErrorHandler(e)
}
