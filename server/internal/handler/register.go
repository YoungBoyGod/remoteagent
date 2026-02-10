package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/state"
	"luoyi2026/server/internal/store"
)

func Register(st *state.ServerState, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}

		var req model.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.AgentID == "" || req.DeviceCode == "" {
			writeBadRequest(w, "agent_id and device_code required")
			return
		}

		token := randHex(24)
		expiresAt := time.Now().Add(cfg.JWTTTL)
		heartbeatInterval := 30
		pollTimeout := int(cfg.PollTimeout / time.Second)

		st.Mu.Lock()
		st.Tokens[token] = model.TokenRecord{AgentID: req.AgentID, ExpiresAt: expiresAt}
		record, ok := st.Agents[req.AgentID]
		if !ok {
			record = &model.AgentRecord{AgentID: req.AgentID, RunningTasks: make(map[string]struct{})}
			st.Agents[req.AgentID] = record
		}
		record.DeviceCode = req.DeviceCode
		record.Token = token
		record.TokenExpiresAt = expiresAt
		record.HeartbeatInterval = heartbeatInterval
		record.PollTimeoutSeconds = pollTimeout
		st.Mu.Unlock()

		if err := store.UpsertAgent(st.DB, req, heartbeatInterval, pollTimeout); err != nil {
			log.Printf("register persist failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, model.Envelope{
			Code:      0,
			Message:   "ok",
			RequestID: requestID(),
			Data: map[string]any{
				"token":              token,
				"heartbeat_interval": heartbeatInterval,
				"poll_timeout":       pollTimeout,
				"server_time":        time.Now().Unix(),
			},
		})
	}
}
