package router

import (
	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/handler"
	"vibebiz/internal/observability"
)

func RegisterData(e *echo.Echo, sandbox *client.Sandbox, obs *observability.Provider) {
	files := handler.NewFileHandler(sandbox, obs)
	chat := handler.NewChatHandler(sandbox, obs)
	preview := handler.NewPreviewHandler(sandbox, obs)

	api := e.Group("/dp/api/v1")
	api.GET("/projects/:id/files", files.List)
	api.GET("/projects/:id/file", files.Read)
	api.PUT("/projects/:id/file", files.Write)
	api.POST("/projects/:id/chat", chat.Chat)
	api.GET("/projects/:id/preview/*", preview.Serve)

	registerErrorHandler(e)
}
