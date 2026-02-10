package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/state"
)

func DebugDispatchTask(st *state.ServerState, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}
		var req model.DebugTaskDispatch
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.AgentID == "" || req.TaskID == "" || req.Command == "" {
			writeBadRequest(w, "agent_id/task_id/command required")
			return
		}
		if req.Timeout <= 0 {
			req.Timeout = 30
		}
		st.Enqueue(req.AgentID, map[string]any{
			"type":        "task",
			"delivery_id": "dly-" + randHex(8),
			"server_time": time.Now().Unix(),
			"data": map[string]any{
				"task_id":   req.TaskID,
				"task_type": "command",
				"payload": map[string]any{
					"command": req.Command,
					"timeout": req.Timeout,
				},
			},
		})
		writeJSON(w, http.StatusOK, model.Envelope{
			Code: 0, Message: "ok", RequestID: requestID(),
		})
	}
}

func DebugDispatchControl(st *state.ServerState, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}
		var req model.DebugControlDispatch
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.AgentID == "" || req.Action == "" {
			writeBadRequest(w, "agent_id/action required")
			return
		}
		if req.Action != "refresh_token" && req.Action != "shutdown" && req.Action != "reload_config" && req.Action != "cancel_task" && req.Action != "cancel" {
			writeBadRequest(w, "invalid action")
			return
		}
		st.Enqueue(req.AgentID, map[string]any{
			"type":        "control",
			"delivery_id": "dly-" + randHex(8),
			"server_time": time.Now().Unix(),
			"data": map[string]any{
				"action":  req.Action,
				"payload": req.Payload,
			},
		})
		writeJSON(w, http.StatusOK, model.Envelope{
			Code: 0, Message: "ok", RequestID: requestID(),
		})
	}
}

func DebugState(st *state.ServerState, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}
		st.Mu.Lock()
		defer st.Mu.Unlock()
		writeJSON(w, http.StatusOK, model.Envelope{
			Code:      0,
			Message:   "ok",
			RequestID: requestID(),
			Data: map[string]any{
				"agents": len(st.Agents),
				"tasks":  len(st.Tasks),
			},
		})
	}
}
