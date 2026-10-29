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
	handler.PreviewClient
}

func RegisterData(e *echo.Echo, sandbox DataSandbox, obs *observability.Provider, authn *auth.Authenticator, metas auth.ProjectMetaStore) {
	files := handler.NewFileHandler(sandbox, obs)
	chat := handler.NewChatHandler(sandbox, obs)
	stopChat := handler.NewChatStopHandler(sandbox, obs)
	preview := handler.NewPreviewHandler(sandbox, obs)

	api := e.Group("/dp/api/v1")
	projects := api.Group("/projects/:id", authn.RequireUser, auth.RequireProjectAccess(metas))
	projects.GET("/files", files.List)
	projects.GET("/file", files.Read)
	projects.PUT("/file", files.Write)
	projects.POST("/chat", chat.Chat)
	projects.POST("/chat/stop", stopChat.Stop)
	projects.GET("/preview/*", preview.Serve)

	registerErrorHandler(e)
}
