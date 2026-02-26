package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/config"
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
		// 分发任务，校验 agent 是否存在
		if _, err := svc.DispatchTask(req); err != nil {
			Fail(c, http.StatusNotFound, 404, err.Error())
			return
		}
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

// DebugTaskResultHandler godoc
// @Summary 调试：查询任务执行结果
// @Tags debug
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/debug/task/{task_id} [get]
func DebugTaskResultHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}
		result, err := svc.GetTaskResult(taskID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		if result == nil {
			Fail(c, http.StatusNotFound, 404, "task not found")
			return
		}
		OK(c, map[string]any{
			"task_id":     result.TaskID,
			"agent_id":    result.AgentID,
			"status":      result.Status,
			"exit_code":   result.ExitCode,
			"stdout":      result.Stdout,
			"stderr":      result.Stderr,
			"truncated":   result.Truncated,
			"started_at":  result.StartedAt,
			"finished_at": result.FinishedAt,
		})
	}
}

// DebugAgentsHandler godoc
// @Summary 调试：Agent 列表
// @Tags debug
// @Produce json
// @Param status query string false "online/offline 筛选"
// @Param search query string false "按 device_code 模糊搜索"
// @Success 200 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/debug/agents [get]
func DebugAgentsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := c.Query("status")
		search := c.Query("search")

		// 校验 status 参数
		if status != "" && status != "online" && status != "offline" {
			Fail(c, http.StatusBadRequest, 400, "status must be online or offline")
			return
		}

		items := svc.ListAgents(status, search)
		OK(c, items)
	}
}

// DebugTasksHandler godoc
// @Summary 调试：任务列表
// @Tags debug
// @Produce json
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20，最大100"
// @Param agent_id query string false "按 agent_id 筛选"
// @Param status query string false "pending/running/finished/failed"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/debug/tasks [get]
func DebugTasksHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		page := 1
		pageSize := 20

		if v := c.Query("page"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				page = n
			}
		}
		if v := c.Query("page_size"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				pageSize = n
			}
		}
		if pageSize > 100 {
			pageSize = 100
		}

		agentID := c.Query("agent_id")
		status := c.Query("status")

		// 校验 status 参数
		if status != "" {
			allowed := map[string]bool{
				"pending": true, "running": true,
				"finished": true, "failed": true,
			}
			if !allowed[status] {
				Fail(c, http.StatusBadRequest, 400, "status must be pending/running/finished/failed")
				return
			}
		}

		data, err := svc.ListTasks(agentID, status, page, pageSize)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, data)
	}
}

// DebugRefreshTokenHandler 为指定 agent 重新生成 JWT
func DebugRefreshTokenHandler(svc *service.Service, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("agent_id")
		if agentID == "" {
			Fail(c, http.StatusBadRequest, 400, "agent_id required")
			return
		}
		token, err := svc.RefreshAgentToken(agentID, cfg.JWTTTL)
		if err != nil {
			Fail(c, http.StatusNotFound, 404, err.Error())
			return
		}
		OK(c, map[string]any{"token": token})
	}
}

// DebugShutdownAgentHandler 向指定 agent 发送停止信号
func DebugShutdownAgentHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentID := c.Param("agent_id")
		if agentID == "" {
			Fail(c, http.StatusBadRequest, 400, "agent_id required")
			return
		}
		svc.DispatchControl(api.DebugControlDispatch{
			AgentID: agentID,
			Action:  "shutdown",
		})
		OK(c, nil)
	}
}
