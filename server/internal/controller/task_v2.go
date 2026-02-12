package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
	"luoyi2026/server/internal/store"
)

// CreateTaskHandler godoc
// @Summary 创建任务
// @Tags task-v2
// @Accept json
// @Produce json
// @Param body body api.TaskCreateRequest true "任务创建请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/tasks [post]
func CreateTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.TaskCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		resp, err := svc.CreateTask(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}

// ListTasksHandler godoc
// @Summary 任务列表查询
// @Tags task-v2
// @Produce json
// @Param status query string false "任务状态筛选"
// @Param exec_mode query string false "执行模式筛选 shared/exclusive"
// @Param agent_id query string false "按 agent_id 筛选"
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20，最大100"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/tasks [get]
func ListTasksHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query api.TaskListRequest
		query.Status = c.Query("status")
		query.ExecMode = c.Query("exec_mode")
		query.AgentID = c.Query("agent_id")
		query.Page = 1
		query.PageSize = 20

		if v := c.Query("page"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				query.Page = n
			}
		}
		if v := c.Query("page_size"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				query.PageSize = n
			}
		}

		// 校验 status
		if query.Status != "" {
			allowed := map[string]bool{
				"pending": true, "leased": true, "running": true,
				"success": true, "failed": true, "timeout": true,
				"canceled": true, "canceling": true,
			}
			if !allowed[query.Status] {
				Fail(c, http.StatusBadRequest, 400, "invalid status")
				return
			}
		}

		// 校验 exec_mode
		if query.ExecMode != "" && query.ExecMode != "shared" && query.ExecMode != "exclusive" {
			Fail(c, http.StatusBadRequest, 400, "exec_mode must be shared or exclusive")
			return
		}

		resp, err := svc.ListTasksV2(query)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}

// GetTaskHandler godoc
// @Summary 查询单个任务详情
// @Tags task-v2
// @Produce json
// @Param task_id path string true "任务ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/tasks/{task_id} [get]
func GetTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		detail, err := svc.GetTaskDetail(taskID)
		if err != nil {
			if errors.Is(err, store.ErrTaskNotFound) {
				Fail(c, http.StatusNotFound, 404, "task not found")
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, detail)
	}
}

// CancelTaskHandler godoc
// @Summary 取消任务
// @Tags task-v2
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param body body api.TaskCancelRequest false "取消原因"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Failure 409 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/tasks/{task_id}/cancel [post]
func CancelTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.TaskCancelRequest
		// body 可选，忽略绑定错误
		_ = c.ShouldBindJSON(&req)

		if err := svc.CancelTask(taskID, req); err != nil {
			switch {
			case errors.Is(err, store.ErrTaskNotFound):
				Fail(c, http.StatusNotFound, 404, "task not found")
			case errors.Is(err, store.ErrTaskStateConflict):
				Fail(c, http.StatusConflict, 409, err.Error())
			default:
				Fail(c, http.StatusInternalServerError, 500, err.Error())
			}
			return
		}
		OK(c, nil)
	}
}

// UpdateTaskPriorityHandler godoc
// @Summary 调整任务优先级
// @Tags task-v2
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param body body api.TaskPriorityRequest true "优先级"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Failure 409 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/tasks/{task_id}/priority [patch]
func UpdateTaskPriorityHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.TaskPriorityRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.UpdateTaskPriority(taskID, req); err != nil {
			switch {
			case errors.Is(err, store.ErrTaskNotFound):
				Fail(c, http.StatusNotFound, 404, "task not found")
			case errors.Is(err, store.ErrTaskStateConflict):
				Fail(c, http.StatusConflict, 409, err.Error())
			default:
				Fail(c, http.StatusInternalServerError, 500, err.Error())
			}
			return
		}
		OK(c, nil)
	}
}

// CompleteTaskHandler godoc
// @Summary 任务完成上报
// @Tags task-v2
// @Accept json
// @Produce json
// @Param task_id path string true "任务ID"
// @Param body body api.TaskCompleteRequest true "完成结果"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 409 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security BearerAuth
// @Router /api/v1/tasks/{task_id}/complete [post]
func CompleteTaskHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("task_id")
		if taskID == "" {
			Fail(c, http.StatusBadRequest, 400, "task_id required")
			return
		}

		var req api.TaskCompleteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.CompleteTask(taskID, req); err != nil {
			if errors.Is(err, store.ErrTaskStateConflict) {
				Fail(c, http.StatusConflict, 409, "task state conflict")
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}
