package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// RegisterHandler godoc
// @Summary Agent 注册
// @Tags agent
// @Accept json
// @Produce json
// @Param body body api.RegisterRequest true "注册请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/agent/register [post]
func RegisterHandler(svc *service.Service, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}
		if req.AgentID == "" || req.DeviceCode == "" {
			Fail(c, http.StatusBadRequest, 400, "agent_id and device_code required")
			return
		}

		data, err := svc.Register(req, cfg.JWTTTL, cfg.PollTimeout)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, "internal error")
			return
		}

		OK(c, data)
	}
}
