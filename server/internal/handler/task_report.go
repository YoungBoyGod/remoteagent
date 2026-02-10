package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"luoyi2026/server/internal/model"
	"luoyi2026/server/internal/state"
	"luoyi2026/server/internal/store"
)

func TaskReport(st *state.ServerState) http.HandlerFunc {
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
		var req model.TaskReportRequest
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
		if req.Status != "success" && req.Status != "failed" && req.Status != "canceled" {
			writeBadRequest(w, "invalid status")
			return
		}

		if err := ProcessTaskReport(st, req); err != nil {
			log.Printf("task report process failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, model.Envelope{
			Code: 0, Message: "ok", RequestID: requestID(),
		})
	}
}

func ProcessTaskReport(st *state.ServerState, req model.TaskReportRequest) error {
	if err := store.UpsertTaskReport(st.DB, req); err != nil {
		return err
	}

	inserted, err := store.InsertTaskEvent(
		st.DB, req.EventID, req.TaskID, req.AgentID, "report", req.Status, req,
	)
	if err != nil {
		return err
	}
	if !inserted {
		return nil
	}

	st.Mu.Lock()
	defer st.Mu.Unlock()
	task := st.Tasks[req.TaskID]
	if task == nil {
		task = &model.TaskRecord{TaskID: req.TaskID, AgentID: req.AgentID}
		st.Tasks[req.TaskID] = task
	}
	task.Status = req.Status
	task.StartedAt = req.StartedAt
	task.FinishedAt = req.FinishedAt
	task.ExitCode = req.Result.ExitCode
	task.Stdout = req.Result.Stdout
	task.Stderr = req.Result.Stderr
	task.IsTruncated = req.Result.Truncated
	if st.Agents[req.AgentID] != nil {
		delete(st.Agents[req.AgentID].RunningTasks, req.TaskID)
	}
	return nil
}
