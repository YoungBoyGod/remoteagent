package router

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"golang.org/x/time/rate"

	"luoyi2026/server/internal/logging"

	"luoyi2026/server/frontend"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/controller"
	"luoyi2026/server/internal/search"
	"luoyi2026/server/internal/service"
	"luoyi2026/server/internal/storage"
)

// Deps 路由依赖注入
type Deps struct {
	Search *search.Client
}

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

// CORSMiddleware 添加 CORS 响应头，限制允许的 Origin
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Register-Token")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ipLimiter 按 IP 维护速率限制器
type ipLimiter struct {
	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

var registerLimiter = &ipLimiter{limiters: make(map[string]*rate.Limiter)}

func (l *ipLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	if lim, ok := l.limiters[ip]; ok {
		return lim
	}
	// 每分钟 10 次，burst 20
	lim := rate.NewLimiter(rate.Every(6*time.Second), 20)
	l.limiters[ip] = lim
	return lim
}

// RegisterRateLimitMiddleware 对注册接口按 IP 限速
func RegisterRateLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if !registerLimiter.get(ip).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

// InternalOnlyMiddleware 仅允许内网 IP 访问
func InternalOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := net.ParseIP(strings.TrimSpace(c.ClientIP()))
		if ip == nil || !isInternalIP(ip) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
			return
		}
		c.Next()
	}
}

func isInternalIP(ip net.IP) bool {
	private := []string{"127.0.0.0/8", "10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "::1/128"}
	for _, cidr := range private {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func Setup(cfg *config.Config, svc *service.Service, sto ...storage.Storage) *gin.Engine {
	return SetupWithDeps(cfg, svc, nil, sto...)
}

func SetupWithDeps(cfg *config.Config, svc *service.Service, deps *Deps, sto ...storage.Storage) *gin.Engine {
	var docSto storage.Storage
	if len(sto) > 0 {
		docSto = sto[0]
	}
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = logging.Writer
	engine := gin.New()
	// 全局中间件：日志、恢复、请求体大小限制、CORS
	engine.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		src := "[R]"
		if ip := net.ParseIP(p.ClientIP); ip != nil && ip.IsLoopback() {
			src = "[L]"
		}
		return fmt.Sprintf("[GIN] %s | %s %3d | %13v | %15s | %-7s %s\n",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"),
			src, p.StatusCode, p.Latency, p.ClientIP, p.Method, p.Path)
	}), gin.Recovery(), BodySizeLimitMiddleware(), CORSMiddleware())

	// 公开路由
	engine.GET("/healthz", controller.HealthHandler())
	engine.GET("/metrics", InternalOnlyMiddleware(), controller.MetricsHandler(svc))

	// agent 路由组
	v1 := engine.Group("/api/v1")

	// register 用 AdminAuth + 速率限制
	v1.POST("/agent/register",
		RegisterRateLimitMiddleware(),
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

	// Dashboard 聚合接口 (AdminAuth)
	v1.GET("/dashboard/summary", controller.AdminAuth(cfg), controller.DashboardSummaryHandler(svc))

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
	dist.GET("/s3-objects", controller.ListDistributionS3ObjectsHandler(svc))
	dist.GET("/:id", controller.GetDistributionHandler(svc))
	dist.PUT("/:id", controller.UpdateDistributionHandler(svc))
	dist.PATCH("/:id/status", controller.UpdateDistributionStatusHandler(svc))

	// 分发回调由 Agent 调用，使用 BearerAuth
	v1.POST("/distributions/callback", controller.BearerAuth(svc), controller.DistributionCallbackHandler(svc))

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
	// 搜索
	if deps != nil && deps.Search != nil {
		docs.GET("/search", controller.SearchDocsHandler(deps.Search))
		docs.GET("/search/suggest", controller.SuggestDocsHandler(deps.Search))
	}

	// debug 路由组 (AdminAuth)
	debug := v1.Group("/debug", controller.AdminAuth(cfg))
	debug.POST("/dispatch/task", controller.DebugDispatchTaskHandler(svc))
	debug.POST("/dispatch/control", controller.DebugDispatchControlHandler(svc))
	debug.GET("/state", controller.DebugStateHandler(svc))
	debug.GET("/task/:task_id", controller.DebugTaskResultHandler(svc))
	debug.GET("/agents", controller.DebugAgentsHandler(svc))
	debug.GET("/tasks", controller.DebugTasksHandler(svc))
	debug.POST("/agents/:agent_id/refresh-token", controller.DebugRefreshTokenHandler(svc, cfg))
	debug.POST("/agents/:agent_id/shutdown", controller.DebugShutdownAgentHandler(svc))

	// swagger 文档（可通过 SERVER_ENABLE_SWAGGER=false 禁用）
	if cfg.EnableSwagger {
		engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// 内嵌前端静态文件（SPA fallback），开发模式下 distFS 为 nil 自动跳过
	if distFS := frontend.DistFS(); distFS != nil {
		fileServer := http.FileServer(http.FS(distFS))
		indexHTML := buildIndexHTML(distFS, cfg)
		engine.NoRoute(func(c *gin.Context) {
			f, err := fs.Stat(distFS, c.Request.URL.Path[1:])
			if err == nil && !f.IsDir() {
				fileServer.ServeHTTP(c.Writer, c.Request)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
		})
	}

	return engine
}

func buildIndexHTML(distFS fs.FS, _ *config.Config) []byte {
	raw, _ := fs.ReadFile(distFS, "index.html")
	return raw
}
