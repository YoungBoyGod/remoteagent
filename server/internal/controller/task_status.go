package controller

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// TaskStatusHandler godoc
// @Summary 上报任务状态
// @Tags agent
// @Accept json
// @Produce json
// @Param body body api.TaskStatusRequest true "任务状态请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security BearerAuth
// @Router /api/v1/agent/task/status [post]
func TaskStatusHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authAgentID := c.GetString("agent_id")

		var req api.TaskStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}

		if req.EventID == "" || req.AgentID == "" || req.TaskID == "" || req.Status == "" {
			Fail(c, http.StatusBadRequest, 400, "event_id/agent_id/task_id/status required")
			return
		}
		if req.AgentID != authAgentID {
			Fail(c, http.StatusBadRequest, 400, "agent_id mismatch")
			return
		}
		if !service.IsTaskStatus(req.Status) {
			Fail(c, http.StatusBadRequest, 400, "invalid status")
			return
		}

		if err := svc.ProcessTaskStatus(req); err != nil {
			log.Printf("task status process failed: %v", err)
			Fail(c, http.StatusInternalServerError, 500, "internal error")
			return
		}

		OK(c, nil)
	}
}
