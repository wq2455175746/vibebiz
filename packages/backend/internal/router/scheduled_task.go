package router

import (
	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/handler"
)

func RegisterScheduledTasks(e *echo.Echo, scheduled *handler.ScheduledTaskHandler, authn *auth.Authenticator) {
	api := e.Group("/cp/api/v1")
	user := api.Group("/scheduled-tasks", authn.RequireUser)
	user.POST("/parse", scheduled.Parse)
	user.POST("", scheduled.Create)
	user.GET("", scheduled.List)
	user.GET("/:id", scheduled.Get)
	user.PUT("/:id", scheduled.Update)
	user.POST("/:id/pause", scheduled.Pause)
	user.POST("/:id/resume", scheduled.Resume)
	user.POST("/:id/stop", scheduled.Stop)
	user.DELETE("/:id", scheduled.Delete)

	admin := api.Group("/admin/scheduled-tasks", authn.RequireOps)
	admin.GET("", scheduled.AdminList)
	admin.POST("/:id/pause", scheduled.AdminPause)
	admin.POST("/:id/resume", scheduled.AdminResume)
	admin.POST("/:id/stop", scheduled.AdminStop)
}
