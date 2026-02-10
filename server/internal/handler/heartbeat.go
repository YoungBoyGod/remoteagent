package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/state"
	"luoyi2026/server/internal/store"
)

func Heartbeat(st *state.ServerState) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthFailed(w)
			return
		}
		agentID, ok := st.Auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}

		var req model.HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.AgentID == "" || req.AgentID != agentID {
			writeBadRequest(w, "agent_id mismatch")
			return
		}

		st.Mu.Lock()
		record := st.Agents[agentID]
		record.LastHeartbeatAt = time.Now()
		record.RunningTasks = toTaskSet(req.RunningTasks)
		st.Mu.Unlock()

		if err := store.UpdateHeartbeat(st.DB, req.AgentID, req.Timestamp); err != nil {
			log.Printf("heartbeat persist failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, model.Envelope{
			Code: 0, Message: "ok", RequestID: requestID(),
			Data: map[string]any{"server_time": time.Now().Unix()},
		})
	}
}
