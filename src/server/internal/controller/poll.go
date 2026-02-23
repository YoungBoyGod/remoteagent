package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/service"
)

// PollHandler godoc
// @Summary Agent 长轮询获取任务
// @Tags agent
// @Produce json
// @Param agent_id query string true "Agent ID"
// @Success 200 {object} model.Envelope
// @Failure 400 {object} model.Envelope
// @Security BearerAuth
// @Router /api/v1/agent/poll [get]
func PollHandler(svc *service.Service, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authAgentID := c.GetString("agent_id")

		agentID := c.Query("agent_id")
		if agentID == "" || agentID != authAgentID {
			Fail(c, http.StatusBadRequest, 400, "agent_id mismatch")
			return
		}

		data := svc.WaitPoll(agentID, cfg.PollTimeout)
		OK(c, data)
	}
}
