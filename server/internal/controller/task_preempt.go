package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
	"luoyi2026/server/internal/store"
)

// PreemptTaskHandler godoc
// @Summary 请求抢占运行中的可抢占任务
// @Tags task
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param body body api.PreemptRequest true "抢占请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Failure 409 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/tasks/{task_id}/preempt [post]
func PreemptTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.PreemptRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}
		if req.Reason == "" {
			Fail(c, http.StatusBadRequest, 400, "reason required")
			return
		}
		if req.GracePeriodSeconds <= 0 {
			Fail(c, http.StatusBadRequest, 400, "grace_period_seconds must be > 0")
			return
		}

		data, err := svc.RequestTaskPreempt(taskID, req)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrTaskNotFound):
				Fail(c, http.StatusNotFound, 404, "task not found")
			case errors.Is(err, store.ErrTaskStateConflict), errors.Is(err, store.ErrTaskAgentMismatch):
				Fail(c, http.StatusConflict, 409, "task state conflict")
			default:
				Fail(c, http.StatusInternalServerError, 500, "internal error")
			}
			return
		}

		OK(c, data)
	}
}

// PreemptAckHandler godoc
// @Summary Agent 确认收到抢占并开始终止
// @Tags agent
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param body body api.PreemptAckRequest true "抢占确认请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Failure 409 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security BearerAuth
// @Router /api/v1/tasks/{task_id}/preempt/ack [post]
func PreemptAckHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		authAgentID := c.GetString("agent_id")
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.PreemptAckRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}
		if req.EventID == "" || req.AgentID == "" || req.TaskID == "" || req.PreemptState == "" {
			Fail(c, http.StatusBadRequest, 400, "event_id/agent_id/task_id/preempt_state required")
			return
		}
		if req.AgentID != authAgentID {
			Fail(c, http.StatusBadRequest, 400, "agent_id mismatch")
			return
		}
		if req.TaskID != taskID {
			Fail(c, http.StatusBadRequest, 400, "task_id mismatch")
			return
		}
		if req.PreemptState != "acknowledged" && req.PreemptState != "terminating" {
			Fail(c, http.StatusBadRequest, 400, "invalid preempt_state")
			return
		}

		if err := svc.AckTaskPreempt(req); err != nil {
			switch {
			case errors.Is(err, store.ErrTaskNotFound):
				Fail(c, http.StatusNotFound, 404, "task not found")
			case errors.Is(err, store.ErrTaskStateConflict), errors.Is(err, store.ErrTaskAgentMismatch):
				Fail(c, http.StatusConflict, 409, "task state conflict")
			default:
				Fail(c, http.StatusInternalServerError, 500, "internal error")
			}
			return
		}

		OK(c, nil)
	}
}

