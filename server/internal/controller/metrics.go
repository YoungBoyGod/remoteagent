package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"luoyi2026/server/internal/service"
)

// MetricsHandler 聚合所有 agent 上报的 Prometheus 指标
func MetricsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Data(http.StatusOK,
			"text/plain; version=0.0.4; charset=utf-8",
			[]byte(svc.RenderAllMetrics()))
	}
}
