package controller

import (
	"strings"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/service"
)

func BearerAuth(svc *service.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			Fail(c, 401, 401, "unauthorized")
			c.Abort()
			return
		}
		agentID, valid := svc.Auth(token)
		if !valid {
			Fail(c, 401, 401, "unauthorized")
			c.Abort()
			return
		}
		c.Set("agent_id", agentID)
		c.Next()
	}
}

func AdminAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("X-Register-Token") != cfg.RegisterToken {
			Fail(c, 401, 401, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

func bearerToken(header string) (string, bool) {
	if header == "" {
		return "", false
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}
