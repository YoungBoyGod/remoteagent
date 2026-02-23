package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/service"
)

// DashboardSummaryHandler godoc
// @Summary Dashboard 聚合信息
// @Tags dashboard
// @Produce json
// @Param recent_limit query int false "最近任务条数，默认10，最大100"
// @Success 200 {object} api.Envelope
// @Failure 500 {object} api.Envelope
// @Security AdminAuth
// @Router /api/v1/dashboard/summary [get]
func DashboardSummaryHandler(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		recentLimit := 10
		if v := c.Query("recent_limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				recentLimit = n
			}
		}

		resp, err := svc.DashboardSummary(recentLimit)
		if err != nil {
			Fail(c, http.StatusInternalServerError, 500, err.Error())
			return
		}
		OK(c, resp)
	}
}
