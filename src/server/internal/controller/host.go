package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// CreateHostHandler godoc
// @Summary 创建主机
// @Tags hosts
// @Accept json
// @Produce json
// @Param body body api.HostCreateRequest true "主机创建请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Router /api/v1/hosts [post]
func CreateHostHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.HostCreateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		hostID, err := svc.CreateHost(req)
		if err != nil {
			Fail(c, http.StatusBadRequest, 400, err.Error())
			return
		}
		OK(c, map[string]any{"host_id": hostID})
	}
}

// UpdateHostHandler godoc
// @Summary 更新主机
// @Tags hosts
// @Accept json
// @Produce json
// @Param host_id path string true "主机ID"
// @Param body body api.HostUpdateRequest true "主机更新请求"
// @Success 200 {object} api.Envelope
// @Failure 400 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/hosts/{host_id} [put]
func UpdateHostHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostID := c.Param("host_id")
		if hostID == "" {
			Fail(c, http.StatusBadRequest, 400, "host_id required")
			return
		}
		var req api.HostUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid json: "+err.Error())
			return
		}
		if err := svc.UpdateHost(hostID, req); err != nil {
			if err.Error() == "host not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// DeleteHostHandler godoc
// @Summary 删除主机
// @Tags hosts
// @Produce json
// @Param host_id path string true "主机ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/hosts/{host_id} [delete]
func DeleteHostHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostID := c.Param("host_id")
		if hostID == "" {
			Fail(c, http.StatusBadRequest, 400, "host_id required")
			return
		}
		if err := svc.DeleteHost(hostID); err != nil {
			if err.Error() == "host not found" {
				Fail(c, http.StatusNotFound, 404, err.Error())
				return
			}
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, nil)
	}
}

// GetHostHandler godoc
// @Summary 查询单个主机详情
// @Tags hosts
// @Produce json
// @Param host_id path string true "主机ID"
// @Success 200 {object} api.Envelope
// @Failure 404 {object} api.Envelope
// @Router /api/v1/hosts/{host_id} [get]
func GetHostHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		hostID := c.Param("host_id")
		if hostID == "" {
			Fail(c, http.StatusBadRequest, 400, "host_id required")
			return
		}
		item, err := svc.GetHost(hostID)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		if item == nil {
			Fail(c, http.StatusNotFound, 404, "host not found")
			return
		}
		OK(c, item)
	}
}

// ListHostsHandler godoc
// @Summary 主机列表（分页）
// @Tags hosts
// @Produce json
// @Param page query int false "页码，默认1"
// @Param page_size query int false "每页条数，默认20，最大100"
// @Param status query string false "按状态筛选"
// @Param search query string false "按名称/IP/主机名模糊搜索"
// @Success 200 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Router /api/v1/hosts [get]
func ListHostsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.HostListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		data, err := svc.ListHosts(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, data)
	}
}
