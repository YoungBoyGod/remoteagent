package router

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/controller"
	"luoyi2026/server/internal/service"
)

func Setup(cfg *config.Config, svc *service.Service) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Logger(), gin.Recovery())

	// 公开路由
	engine.GET("/healthz", controller.HealthHandler())

	// agent 路由组
	v1 := engine.Group("/api/v1")

	// register 用 AdminAuth
	v1.POST("/agent/register",
		controller.AdminAuth(cfg),
		controller.RegisterHandler(svc, cfg),
	)

	// agent 认证路由组
	agent := v1.Group("/agent", controller.BearerAuth(svc))
	agent.POST("/heartbeat", controller.HeartbeatHandler(svc))
	agent.GET("/poll", controller.PollHandler(svc, cfg))
	agent.POST("/task/status", controller.TaskStatusHandler(svc))
	agent.POST("/task/report", controller.TaskReportHandler(svc))

	// debug 路由组 (AdminAuth)
	debug := v1.Group("/debug", controller.AdminAuth(cfg))
	debug.POST("/dispatch/task", controller.DebugDispatchTaskHandler(svc))
	debug.POST("/dispatch/control", controller.DebugDispatchControlHandler(svc))
	debug.GET("/state", controller.DebugStateHandler(svc))

	// swagger 文档
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return engine
}
