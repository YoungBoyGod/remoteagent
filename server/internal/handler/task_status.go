package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/state"
	"luoyi2026/server/internal/store"
)

func TaskStatus(st *state.ServerState) http.HandlerFunc {
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
		authAgentID, ok := st.Auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}
		var req model.TaskStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.EventID == "" || req.AgentID == "" || req.TaskID == "" || req.Status == "" {
			writeBadRequest(w, "event_id/agent_id/task_id/status required")
			return
		}
		if req.AgentID != authAgentID {
			writeBadRequest(w, "agent_id mismatch")
			return
		}
		if !isTaskStatus(req.Status) {
			writeBadRequest(w, "invalid status")
			return
		}

		if err := ProcessTaskStatus(st, req); err != nil {
			log.Printf("task status process failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, model.Envelope{
			Code: 0, Message: "ok", RequestID: requestID(),
		})
	}
}

func ProcessTaskStatus(st *state.ServerState, req model.TaskStatusRequest) error {
	if err := store.UpsertTaskStatus(st.DB, req); err != nil {
		return err
	}

	inserted, err := store.InsertTaskEvent(
		st.DB, req.EventID, req.TaskID, req.AgentID, "status", req.Status, req,
	)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	attempt := req.Attempt
	if attempt <= 0 {
		attempt = 1
	}

	st.Mu.Lock()
	defer st.Mu.Unlock()
	task := st.Tasks[req.TaskID]
	if task == nil {
		task = &model.TaskRecord{
			TaskID: req.TaskID, AgentID: req.AgentID, Attempt: attempt,
		}
		st.Tasks[req.TaskID] = task
	}
	task.Status = req.Status
	if req.Status == "running" {
		task.StartedAt = req.Timestamp
		if st.Agents[req.AgentID] != nil {
			st.Agents[req.AgentID].RunningTasks[req.TaskID] = struct{}{}
		}
	} else {
		task.FinishedAt = req.Timestamp
		if st.Agents[req.AgentID] != nil {
			delete(st.Agents[req.AgentID].RunningTasks, req.TaskID)
		}
	}
	return nil
}
