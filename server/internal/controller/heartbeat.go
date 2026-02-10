package controller

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// HeartbeatHandler godoc
// @Summary Agent 心跳上报
// @Tags agent
// @Accept json
// @Produce json
// @Param body body api.HeartbeatRequest true "心跳请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security BearerAuth
// @Router /api/v1/agent/heartbeat [post]
func HeartbeatHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authAgentID := c.GetString("agent_id")

		var req api.HeartbeatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}
		if req.AgentID == "" || req.AgentID != authAgentID {
			Fail(c, http.StatusBadRequest, 400, "agent_id mismatch")
			return
		}

		if err := svc.Heartbeat(req); err != nil {
			Fail(c, http.StatusInternalServerError, 500, "internal error")
			return
		}

		OK(c, map[string]any{"server_time": time.Now().Unix()})
	}
}
