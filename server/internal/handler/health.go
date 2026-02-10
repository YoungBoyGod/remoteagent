package handler

import (
	"net/http"
	"time"

	"luoyi2026/server/internal/model"
)

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, model.Envelope{
			Code:      0,
			Message:   "ok",
			RequestID: requestID(),
			Data: model.HealthResp{
				Service:   "luoyi-server",
				Status:    "ok",
				Timestamp: time.Now().Unix(),
			},
		})
	}
}
