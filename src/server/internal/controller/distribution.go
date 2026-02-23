package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// CreateDistributionHandler POST /v1/distribute — 创建分发任务
func CreateDistributionHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DistributionCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		resp, err := svc.CreateDistribution(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}

// ListDistributionsHandler GET /v1/distributions — 分发记录列表
func ListDistributionsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.DistributionListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		resp, err := svc.ListDistributions(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}

// GetDistributionHandler GET /v1/distributions/:id — 查询单条分发详情
func GetDistributionHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}

		item, err := svc.GetDistribution(id)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		if item == nil {
			Fail(c, http.StatusNotFound, 404, "distribution not found")
			return
		}
		OK(c, item)
	}
}

// UpdateDistributionHandler PUT /v1/distributions/:id — 更新分发记录
func UpdateDistributionHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}

		var req api.DistributionUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.UpdateDistribution(id, req); err != nil {
			if err.Error() == "distribution not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// UpdateDistributionStatusHandler PATCH /v1/distributions/:id/status — 更新分发状态
func UpdateDistributionStatusHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid id")
			return
		}

		var req api.DistributionStatusRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.UpdateDistributionStatus(id, req); err != nil {
			msg := err.Error()
			if msg == "distribution not found" {
				Fail(c, http.StatusNotFound, 404, msg)
				return
			}
			// 状态转换错误返回 409
			if len(msg) > 7 && msg[:7] == "invalid" {
				Fail(c, http.StatusConflict, 409, msg)
				return
			}
			Fail(c, http.StatusInternalServerError, 500, msg)
			return
		}
		OK(c, nil)
	}
}

// DistributionCallbackHandler POST /v1/distributions/callback — Agent 完成回调
func DistributionCallbackHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			DistTaskID string `json:"dist_task_id" binding:"required"`
			Stdout     string `json:"stdout" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid request: "+err.Error())
			return
		}

		if err := svc.HandleDistributionCallback(req.DistTaskID, req.Stdout); err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}
