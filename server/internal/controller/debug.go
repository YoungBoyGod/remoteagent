package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// DebugDispatchTaskHandler godoc
// @Summary 调试：下发任务
// @Tags debug
// @Accept json
// @Produce json
// @Param body body api.DebugTaskDispatch true "任务下发请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/debug/dispatch/task [post]
func DebugDispatchTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DebugTaskDispatch
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}
		if req.AgentID == "" || req.TaskID == "" || req.Command == "" {
			Fail(c, http.StatusBadRequest, 400, "agent_id/task_id/command required")
			return
		}
		svc.DispatchTask(req)
		OK(c, nil)
	}
}

// DebugDispatchControlHandler godoc
// @Summary 调试：下发控制指令
// @Tags debug
// @Accept json
// @Produce json
// @Param body body api.DebugControlDispatch true "控制指令请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/debug/dispatch/control [post]
func DebugDispatchControlHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DebugControlDispatch
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json")
			return
		}
		if req.AgentID == "" || req.Action == "" {
			Fail(c, http.StatusBadRequest, 400, "agent_id/action required")
			return
		}
		allowed := map[string]bool{
			"refresh_token": true,
			"shutdown":      true,
			"reload_config": true,
			"cancel_task":   true,
			"cancel":        true,
		}
		if !allowed[req.Action] {
			Fail(c, http.StatusBadRequest, 400, "invalid action")
			return
		}
		svc.DispatchControl(req)
		OK(c, nil)
	}
}

// DebugStateHandler godoc
// @Summary 调试：查看内存状态统计
// @Tags debug
// @Produce json
// @Success 200 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/debug/state [get]
func DebugStateHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		agents, tasks := svc.Stats()
		OK(c, map[string]any{
			"agents": agents,
			"tasks":  tasks,
		})
	}
}
