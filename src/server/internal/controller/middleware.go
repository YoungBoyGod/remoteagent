package controller

import (
	"crypto/subtle"
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

// AdminAuth 管理员认证中间件，使用 timing-safe 比较防止时序攻击
func AdminAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("X-Register-Token")
		// 使用常量时间比较，防止通过响应时间差异推断 token 内容
		if subtle.ConstantTimeCompare([]byte(token), []byte(cfg.RegisterToken)) != 1 {
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
