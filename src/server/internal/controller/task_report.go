package controller

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// maxOutputSize stdout/stderr 最大长度：64KB
const maxOutputSize = 65536

// TaskReportHandler godoc
// @Summary 上报任务执行结果
// @Tags agent
// @Accept json
// @Produce json
// @Param body body api.TaskReportRequest true "任务报告请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security BearerAuth
// @Router /api/v1/agent/task/report [post]
func TaskReportHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authAgentID := c.GetString("agent_id")

		var req api.TaskReportRequest
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
		if req.Status != "success" && req.Status != "failed" && req.Status != "canceled" {
			Fail(c, http.StatusBadRequest, 400, "invalid status")
			return
		}

		// stdout/stderr 超过 64KB 时截断，并标记 truncated
		if len(req.Result.Stdout) > maxOutputSize {
			req.Result.Stdout = req.Result.Stdout[:maxOutputSize]
			req.Result.Truncated = true
		}
		if len(req.Result.Stderr) > maxOutputSize {
			req.Result.Stderr = req.Result.Stderr[:maxOutputSize]
			req.Result.Truncated = true
		}

		if err := svc.ProcessTaskReport(req); err != nil {
			log.Printf("task report process failed: %v", err)
			Fail(c, http.StatusInternalServerError, 500, "internal error")
			return
		}

		OK(c, nil)
	}
}
