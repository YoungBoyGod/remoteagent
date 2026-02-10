package handler

import (
	"net/http"

	"luoyi2026/server/internal/config"
	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/state"
)

func Poll(st *state.ServerState, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthFailed(w)
			return
		}
		authAgentID, ok := st.Auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" || agentID != authAgentID {
			writeBadRequest(w, "agent_id mismatch")
			return
		}
		data := st.WaitPoll(agentID, cfg.PollTimeout)
		writeJSON(w, http.StatusOK, model.Envelope{
			Code: 0, Message: "ok", RequestID: requestID(), Data: data,
		})
	}
}
