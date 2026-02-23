package controller

import (
	"time"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
)

// HealthHandler godoc
// @Summary 健康检查
// @Tags public
// @Produce json
// @Success 200 {object} api.Envelope{data=api.HealthResp}
// @Router /healthz [get]
func HealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		OK(c, api.HealthResp{
			Service:   "luoyi-server",
			Status:    "ok",
			Timestamp: time.Now().Unix(),
		})
	}
}
