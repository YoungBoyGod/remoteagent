package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
	"luoyi2026/server/internal/store"
)

// ClaimTaskHandler POST /v1/tasks/:task_id/claim
func ClaimTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.TaskClaimRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		resp, err := svc.ClaimTask(taskID, req)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrTaskNotFound):
				Fail(c, http.StatusNotFound, 404, "task not found")
			case errors.Is(err, store.ErrTaskStateConflict):
				Fail(c, http.StatusConflict, 409, "task already claimed or not pending")
			default:
				Fail(c, http.StatusInternalServerError, 500, err.Error())
			}
			return
		}
		OK(c, resp)
	}
}

// TaskHeartbeatHandler POST /v1/tasks/:task_id/heartbeat
func TaskHeartbeatHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.TaskHeartbeatRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		resp, err := svc.HeartbeatTask(taskID, req)
		if err != nil {
			switch {
			case errors.Is(err, store.ErrTaskNotFound):
				Fail(c, http.StatusNotFound, 404, "task not found")
			case errors.Is(err, store.ErrTaskAgentMismatch):
				Fail(c, http.StatusForbidden, 403, "agent mismatch")
			case errors.Is(err, store.ErrTaskStateConflict):
				Fail(c, http.StatusConflict, 409, "task not in leasable state")
			default:
				Fail(c, http.StatusInternalServerError, 500, err.Error())
			}
			return
		}
		OK(c, resp)
	}
}

// PollTasksHandler POST /v1/agents/:agent_id/poll
func PollTasksHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.TaskPollRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		// 从 URL 参数覆盖 agent_id
		if agentID := c.Param("agent_id"); agentID != "" {
			req.AgentID = agentID
		}

		if req.MaxConcurrent <= 0 {
			req.MaxConcurrent = 4
		}

		resp, err := svc.PollTasks(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}
