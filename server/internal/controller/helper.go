package controller

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/gin-gonic/gin"
	"luoyi2026/server/internal/api"
)

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, api.Envelope{
		Code:      0,
		Message:   "ok",
		RequestID: RequestID(),
		Data:      data,
	})
}

func Fail(c *gin.Context, httpCode int, appCode int, msg string) {
	c.JSON(httpCode, api.Envelope{
		Code:      appCode,
		Message:   msg,
		RequestID: RequestID(),
	})
}

func RequestID() string {
	return "req-" + RandHex(6)
}

func RandHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
