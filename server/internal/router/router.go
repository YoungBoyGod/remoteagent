package router

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"luoyi2026/server/frontend"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/controller"
	"luoyi2026/server/internal/service"
	"luoyi2026/server/internal/storage"
)

// maxBodySize 请求体大小上限：1MB
const maxBodySize = 1 << 20

// BodySizeLimitMiddleware 限制请求体大小，超过 maxBodySize 返回 413
func BodySizeLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodySize)
		}
		c.Next()
	}
}

func Setup(cfg *config.Config, svc *service.Service, sto ...storage.Storage) *gin.Engine {
	var docSto storage.Storage
	if len(sto) > 0 {
		docSto = sto[0]
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	// 全局中间件：日志、恢复、请求体大小限制
	engine.Use(gin.Logger(), gin.Recovery(), BodySizeLimitMiddleware())

	// 公开路由
	engine.GET("/healthz", controller.HealthHandler())
	engine.GET("/metrics", controller.MetricsHandler(svc))

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
	v1.POST("/tasks/:task_id/preempt/ack", controller.BearerAuth(svc), controller.PreemptAckHandler(svc))
	v1.POST("/tasks/:task_id/preempt", controller.AdminAuth(cfg), controller.PreemptTaskHandler(svc))

	// Phase 2: 任务调度 API (AdminAuth)
	tasks := v1.Group("/tasks", controller.AdminAuth(cfg))
	tasks.POST("", controller.CreateTaskHandler(svc))
	tasks.POST("/batch", controller.BatchCreateTaskHandler(svc))
	tasks.GET("", controller.ListTasksHandler(svc))
	tasks.GET("/:task_id", controller.GetTaskHandler(svc))
	tasks.POST("/:task_id/cancel", controller.CancelTaskHandler(svc))
	tasks.PATCH("/:task_id/priority", controller.UpdateTaskPriorityHandler(svc))
	// complete/claim/heartbeat 用 BearerAuth（由 agent 调用）
	v1.POST("/tasks/:task_id/complete", controller.BearerAuth(svc), controller.CompleteTaskHandler(svc))
	v1.POST("/tasks/:task_id/claim", controller.BearerAuth(svc), controller.ClaimTaskHandler(svc))
	v1.POST("/tasks/:task_id/heartbeat", controller.BearerAuth(svc), controller.TaskHeartbeatHandler(svc))
	v1.POST("/agents/:agent_id/poll", controller.BearerAuth(svc), controller.PollTasksHandler(svc))

	// 主机管理路由组 (AdminAuth)
	hosts := v1.Group("/hosts", controller.AdminAuth(cfg))
	hosts.POST("", controller.CreateHostHandler(svc))
	hosts.GET("", controller.ListHostsHandler(svc))
	hosts.GET("/:host_id", controller.GetHostHandler(svc))
	hosts.PUT("/:host_id", controller.UpdateHostHandler(svc))
	hosts.DELETE("/:host_id", controller.DeleteHostHandler(svc))

	// 客户管理路由组 (AdminAuth)
	customers := v1.Group("/customers", controller.AdminAuth(cfg))
	customers.POST("", controller.CreateCustomerHandler(svc))
	customers.GET("", controller.ListCustomersHandler(svc))
	customers.GET("/:id", controller.GetCustomerHandler(svc))
	customers.PUT("/:id", controller.UpdateCustomerHandler(svc))
	customers.DELETE("/:id", controller.DeleteCustomerHandler(svc))
	customers.POST("/:id/hosts", controller.AssignHostHandler(svc))
	customers.DELETE("/:id/hosts/:host_id", controller.UnassignHostHandler(svc))
	customers.GET("/:id/hosts", controller.ListCustomerHostsHandler(svc))

	// 安全分发路由组 (AdminAuth)
	dist := v1.Group("/distributions", controller.AdminAuth(cfg))
	dist.POST("", controller.CreateDistributionHandler(svc))
	dist.GET("", controller.ListDistributionsHandler(svc))
	dist.GET("/:id", controller.GetDistributionHandler(svc))
	dist.PUT("/:id", controller.UpdateDistributionHandler(svc))
	dist.PATCH("/:id/status", controller.UpdateDistributionStatusHandler(svc))
	dist.POST("/callback", controller.DistributionCallbackHandler(svc))

	// 发布说明草稿路由组 (AdminAuth)
	rn := v1.Group("/release-notes", controller.AdminAuth(cfg))
	rn.POST("", controller.CreateReleaseNoteHandler(svc))
	rn.GET("", controller.ListReleaseNotesHandler(svc))
	rn.GET("/:id", controller.GetReleaseNoteHandler(svc))
	rn.PUT("/:id", controller.UpdateReleaseNoteHandler(svc))
	rn.DELETE("/:id", controller.DeleteReleaseNoteHandler(svc))

	// 操作日志路由组 (AdminAuth)
	v1.GET("/operation-logs", controller.AdminAuth(cfg), controller.ListOperationLogsHandler(svc))

	// 文档中心路由组 (AdminAuth)
	docs := v1.Group("/docs", controller.AdminAuth(cfg))
	docs.GET("", controller.ListDocsHandler(svc))
	docs.POST("", controller.CreateDocHandler(svc, docSto))
	docs.GET("/:slug", controller.GetDocHandler(svc, docSto))
	docs.PUT("/:slug", controller.UpdateDocHandler(svc, docSto))
	docs.DELETE("/:slug", controller.DeleteDocHandler(svc, docSto))
	// 分类
	docs.GET("/categories", controller.ListDocCategoriesHandler(svc))
	docs.POST("/categories", controller.CreateDocCategoryHandler(svc))
	docs.PUT("/categories/:id", controller.UpdateDocCategoryHandler(svc))
	docs.DELETE("/categories/:id", controller.DeleteDocCategoryHandler(svc))
	// 版本
	docs.GET("/:slug/versions", controller.ListDocVersionsHandler(svc))
	docs.GET("/:slug/versions/:version", controller.GetDocVersionHandler(svc, docSto))
	docs.POST("/:slug/versions", controller.CreateDocVersionHandler(svc, docSto))
	// 附件
	docs.POST("/:slug/attachments", controller.UploadAttachmentHandler(svc, docSto))
	docs.GET("/attachments/:id", controller.GetAttachmentHandler(svc, docSto))
	docs.DELETE("/attachments/:id", controller.DeleteAttachmentHandler(svc, docSto))
	// 反馈
	docs.POST("/:slug/feedback", controller.CreateDocFeedbackHandler(svc))
	docs.GET("/feedback", controller.ListDocFeedbackHandler(svc))
	docs.GET("/feedback/stats", controller.DocFeedbackStatsHandler(svc))
	docs.PUT("/feedback/:id", controller.UpdateDocFeedbackHandler(svc))
	// 版本 Diff
	docs.GET("/:slug/diff", controller.DiffDocVersionsHandler(svc, docSto))
	// 导出
	docs.GET("/:slug/export/html", controller.ExportDocHTMLHandler(svc, docSto))

	// debug 路由组 (AdminAuth)
	debug := v1.Group("/debug", controller.AdminAuth(cfg))
	debug.POST("/dispatch/task", controller.DebugDispatchTaskHandler(svc))
	debug.POST("/dispatch/control", controller.DebugDispatchControlHandler(svc))
	debug.GET("/state", controller.DebugStateHandler(svc))
	debug.GET("/task/:task_id", controller.DebugTaskResultHandler(svc))
	debug.GET("/agents", controller.DebugAgentsHandler(svc))
	debug.GET("/tasks", controller.DebugTasksHandler(svc))

	// swagger 文档
	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 内嵌前端静态文件（SPA fallback）
	if distFS := frontend.DistFS(); distFS != nil {
		fileServer := http.FileServer(http.FS(distFS))
		// 预读 index.html 并注入运行时配置
		indexHTML := buildIndexHTML(distFS, cfg)
		engine.NoRoute(func(c *gin.Context) {
			// 尝试提供静态文件
			f, err := fs.Stat(distFS, c.Request.URL.Path[1:]) // 去掉前导 /
			if err == nil && !f.IsDir() {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			// SPA fallback: 返回注入了运行时配置的 index.html
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
		})
	}

	return engine
}

// buildIndexHTML 读取 index.html 并注入运行时配置（register_token）
func buildIndexHTML(distFS fs.FS, cfg *config.Config) []byte {
	raw, err := fs.ReadFile(distFS, "index.html")
	if err != nil {
		return raw
	}
	script := `<script>window.__RUNTIME_CONFIG__={adminToken:"` + cfg.RegisterToken + `"}</script>`
	html := strings.Replace(string(raw), "</head>", script+"\n</head>", 1)
	return []byte(html)
}
