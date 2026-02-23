package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
	"luoyi2026/server/internal/service"
)

// ListOperationLogsHandler GET /api/v1/operation-logs
func ListOperationLogsHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.OperationLogListRequest
		if err := c.ShouldBindQuery(&req); err != nil {
			Fail(c, http.StatusBadRequest, 400, "invalid query params: "+err.Error())
			return
		}
		data, err := svc.ListOperationLogs(req)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, data)
	}
}
