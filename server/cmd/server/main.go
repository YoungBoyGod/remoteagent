package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"

	"luoyi2026/server/internal/config"
)

type envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      any    `json:"data,omitempty"`
}

type healthResp struct {
	Service   string `json:"service"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type registerRequest struct {
	AgentID      string            `json:"agent_id"`
	DeviceCode   string            `json:"device_code"`
	AgentVersion string            `json:"agent_version"`
	TenantID     string            `json:"tenant_id"`
	Device       deviceInfo        `json:"device"`
	Labels       map[string]string `json:"labels"`
	Capabilities []string          `json:"capabilities"`
}

type deviceInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	IP       string `json:"ip"`
}

type heartbeatRequest struct {
	AgentID      string      `json:"agent_id"`
	Timestamp    int64       `json:"timestamp"`
	Metrics      metricsInfo `json:"metrics"`
	RunningTasks []string    `json:"running_tasks"`
}

type metricsInfo struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	DiskPercent float64 `json:"disk_percent"`
}

type taskStatusRequest struct {
	EventID   string `json:"event_id"`
	AgentID   string `json:"agent_id"`
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	Attempt   int    `json:"attempt"`
}

type taskReportRequest struct {
	EventID    string       `json:"event_id"`
	AgentID    string       `json:"agent_id"`
	TaskID     string       `json:"task_id"`
	Status     string       `json:"status"`
	StartedAt  int64        `json:"started_at"`
	FinishedAt int64        `json:"finished_at"`
	Result     reportResult `json:"result"`
}

type reportResult struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
}

type debugTaskDispatch struct {
	AgentID string `json:"agent_id"`
	TaskID  string `json:"task_id"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type debugControlDispatch struct {
	AgentID string         `json:"agent_id"`
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload"`
}

type agentRecord struct {
	AgentID            string
	DeviceCode         string
	Token              string
	TokenExpiresAt     time.Time
	HeartbeatInterval  int
	PollTimeoutSeconds int
	LastHeartbeatAt    time.Time
	RunningTasks       map[string]struct{}
}

type taskRecord struct {
	TaskID      string
	AgentID     string
	Status      string
	Attempt     int
	StartedAt   int64
	FinishedAt  int64
	ExitCode    int
	Stdout      string
	Stderr      string
	IsTruncated bool
}

type tokenRecord struct {
	AgentID   string
	ExpiresAt time.Time
}

type serverState struct {
	mu      sync.Mutex
	db      *sql.DB
	agents  map[string]*agentRecord
	tokens  map[string]tokenRecord
	tasks   map[string]*taskRecord
	pending map[string][]any
}

func newServerState(db *sql.DB) *serverState {
	return &serverState{
		db:      db,
		agents:  make(map[string]*agentRecord),
		tokens:  make(map[string]tokenRecord),
		tasks:   make(map[string]*taskRecord),
		pending: make(map[string][]any),
	}
}

func (s *serverState) enqueue(agentID string, payload any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[agentID] = append(s.pending[agentID], payload)
}

func (s *serverState) pop(agentID string) (any, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	queue := s.pending[agentID]
	if len(queue) == 0 {
		return nil, false
	}
	item := queue[0]
	s.pending[agentID] = queue[1:]
	return item, true
}

func (s *serverState) waitPoll(agentID string, timeout time.Duration) any {
	deadline := time.Now().Add(timeout)
	for {
		if item, ok := s.pop(agentID); ok {
			return item
		}
		if time.Now().After(deadline) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func main() {
	cfg := config.Load()

	db, err := sql.Open("postgres", cfg.PostgresDSN())
	if err != nil {
		log.Fatalf("open postgres failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.DBConnectTimeoutS)*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping postgres failed: %v", err)
	}
	log.Printf("postgres connected: %s:%d/%s", cfg.DBHost, cfg.DBPort, cfg.DBName)

	state := newServerState(db)

	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, envelope{
			Code:      0,
			Message:   "ok",
			RequestID: requestID(),
			Data: healthResp{
				Service:   "luoyi-server",
				Status:    "ok",
				Timestamp: time.Now().Unix(),
			},
		})
	})

	mux.HandleFunc("/api/v1/agent/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}

		var req registerRequest
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

		state.mu.Lock()
		state.tokens[token] = tokenRecord{AgentID: req.AgentID, ExpiresAt: expiresAt}
		record, ok := state.agents[req.AgentID]
		if !ok {
			record = &agentRecord{AgentID: req.AgentID, RunningTasks: make(map[string]struct{})}
			state.agents[req.AgentID] = record
		}
		record.DeviceCode = req.DeviceCode
		record.Token = token
		record.TokenExpiresAt = expiresAt
		record.HeartbeatInterval = heartbeatInterval
		record.PollTimeoutSeconds = pollTimeout
		state.mu.Unlock()

		if err := upsertAgent(state.db, req, heartbeatInterval, pollTimeout); err != nil {
			log.Printf("register persist failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, envelope{
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
	})

	mux.HandleFunc("/api/v1/agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthFailed(w)
			return
		}
		agentID, ok := state.auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}

		var req heartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.AgentID == "" || req.AgentID != agentID {
			writeBadRequest(w, "agent_id mismatch")
			return
		}

		state.mu.Lock()
		record := state.agents[agentID]
		record.LastHeartbeatAt = time.Now()
		record.RunningTasks = toTaskSet(req.RunningTasks)
		state.mu.Unlock()

		if err := updateHeartbeat(state.db, req.AgentID, req.Timestamp); err != nil {
			log.Printf("heartbeat persist failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID(), Data: map[string]any{"server_time": time.Now().Unix()}})
	})

	mux.HandleFunc("/api/v1/agent/poll", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthFailed(w)
			return
		}
		authAgentID, ok := state.auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}
		agentID := r.URL.Query().Get("agent_id")
		if agentID == "" || agentID != authAgentID {
			writeBadRequest(w, "agent_id mismatch")
			return
		}
		data := state.waitPoll(agentID, cfg.PollTimeout)
		writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID(), Data: data})
	})

	mux.HandleFunc("/api/v1/agent/task/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthFailed(w)
			return
		}
		authAgentID, ok := state.auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}
		var req taskStatusRequest
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

		inserted, err := insertTaskEvent(state.db, req.EventID, req.TaskID, req.AgentID, "status", req.Status, req)
		if err != nil {
			log.Printf("task status event persist failed: %v", err)
			writeServerError(w)
			return
		}
		if !inserted {
			writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID()})
			return
		}

		state.mu.Lock()
		task := state.tasks[req.TaskID]
		if task == nil {
			task = &taskRecord{TaskID: req.TaskID, AgentID: req.AgentID, Attempt: max(req.Attempt, 1)}
			state.tasks[req.TaskID] = task
		}
		task.Status = req.Status
		if req.Status == "running" {
			task.StartedAt = req.Timestamp
			state.agents[req.AgentID].RunningTasks[req.TaskID] = struct{}{}
		} else {
			task.FinishedAt = req.Timestamp
			delete(state.agents[req.AgentID].RunningTasks, req.TaskID)
		}
		state.mu.Unlock()

		if err := upsertTaskStatus(state.db, req); err != nil {
			log.Printf("task status persist failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID()})
	})

	mux.HandleFunc("/api/v1/agent/task/report", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		token, err := bearerToken(r.Header.Get("Authorization"))
		if err != nil {
			writeAuthFailed(w)
			return
		}
		authAgentID, ok := state.auth(token)
		if !ok {
			writeAuthFailed(w)
			return
		}
		var req taskReportRequest
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

		inserted, err := insertTaskEvent(state.db, req.EventID, req.TaskID, req.AgentID, "report", req.Status, req)
		if err != nil {
			log.Printf("task report event persist failed: %v", err)
			writeServerError(w)
			return
		}
		if !inserted {
			writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID()})
			return
		}

		state.mu.Lock()
		task := state.tasks[req.TaskID]
		if task == nil {
			task = &taskRecord{TaskID: req.TaskID, AgentID: req.AgentID}
			state.tasks[req.TaskID] = task
		}
		task.Status = req.Status
		task.StartedAt = req.StartedAt
		task.FinishedAt = req.FinishedAt
		task.ExitCode = req.Result.ExitCode
		task.Stdout = req.Result.Stdout
		task.Stderr = req.Result.Stderr
		task.IsTruncated = req.Result.Truncated
		delete(state.agents[req.AgentID].RunningTasks, req.TaskID)
		state.mu.Unlock()

		if err := upsertTaskReport(state.db, req); err != nil {
			log.Printf("task report persist failed: %v", err)
			writeServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID()})
	})

	mux.HandleFunc("/api/v1/debug/dispatch/task", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}
		var req debugTaskDispatch
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
		state.enqueue(req.AgentID, map[string]any{
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
		writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID()})
	})

	mux.HandleFunc("/api/v1/debug/dispatch/control", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}
		var req debugControlDispatch
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeBadRequest(w, "invalid json")
			return
		}
		if req.AgentID == "" || req.Action == "" {
			writeBadRequest(w, "agent_id/action required")
			return
		}
		if req.Action != "refresh_token" && req.Action != "shutdown" && req.Action != "reload_config" {
			writeBadRequest(w, "invalid action")
			return
		}
		state.enqueue(req.AgentID, map[string]any{
			"type":        "control",
			"delivery_id": "dly-" + randHex(8),
			"server_time": time.Now().Unix(),
			"data": map[string]any{
				"action":  req.Action,
				"payload": req.Payload,
			},
		})
		writeJSON(w, http.StatusOK, envelope{Code: 0, Message: "ok", RequestID: requestID()})
	})

	mux.HandleFunc("/api/v1/debug/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeMethodNotAllowed(w)
			return
		}
		if r.Header.Get("X-Register-Token") != cfg.RegisterToken {
			writeAuthFailed(w)
			return
		}
		state.mu.Lock()
		defer state.mu.Unlock()
		writeJSON(w, http.StatusOK, envelope{
			Code:      0,
			Message:   "ok",
			RequestID: requestID(),
			Data: map[string]any{
				"agents": len(state.agents),
				"tasks":  len(state.tasks),
			},
		})
	})

	log.Printf("luoyi-server listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, mux); err != nil {
		log.Fatal(err)
	}
}

func upsertAgent(db *sql.DB, req registerRequest, heartbeatInterval int, pollTimeout int) error {
	tenantID := req.TenantID
	if tenantID == "" {
		tenantID = "default"
	}
	labels, err := json.Marshal(req.Labels)
	if err != nil {
		return err
	}
	capabilities, err := json.Marshal(req.Capabilities)
	if err != nil {
		return err
	}
	var ipValue any
	if req.Device.IP == "" {
		ipValue = nil
	} else {
		ipValue = req.Device.IP
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = db.ExecContext(
		ctx,
		`insert into agents (
			agent_id, tenant_id, device_code, agent_version, status,
			hostname, os, arch, ip, labels, capabilities,
			heartbeat_interval, poll_timeout, last_heartbeat_at, updated_at
		)
		values ($1,$2,$3,$4,'online',$5,$6,$7,$8,$9::jsonb,$10::jsonb,$11,$12,now(),now())
		on conflict (agent_id) do update set
			tenant_id = excluded.tenant_id,
			device_code = excluded.device_code,
			agent_version = excluded.agent_version,
			status = 'online',
			hostname = excluded.hostname,
			os = excluded.os,
			arch = excluded.arch,
			ip = excluded.ip,
			labels = excluded.labels,
			capabilities = excluded.capabilities,
			heartbeat_interval = excluded.heartbeat_interval,
			poll_timeout = excluded.poll_timeout,
			last_heartbeat_at = now(),
			updated_at = now()`,
		req.AgentID,
		tenantID,
		req.DeviceCode,
		req.AgentVersion,
		req.Device.Hostname,
		req.Device.OS,
		req.Device.Arch,
		ipValue,
		string(labels),
		string(capabilities),
		heartbeatInterval,
		pollTimeout,
	)
	return err
}

func updateHeartbeat(db *sql.DB, agentID string, timestamp int64) error {
	heartbeatTime := time.Unix(timestamp, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(
		ctx,
		`update agents
		 set status = 'online',
		     last_heartbeat_at = $2,
		     updated_at = now()
		 where agent_id = $1`,
		agentID,
		heartbeatTime,
	)
	return err
}

func insertTaskEvent(db *sql.DB, eventID string, taskID string, agentID string, eventType string, status string, body any) (bool, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := db.ExecContext(
		ctx,
		`insert into task_events(event_id, task_id, agent_id, event_type, status, body)
		 values($1,$2,$3,$4,$5,$6::jsonb)
		 on conflict(event_id) do nothing`,
		eventID,
		taskID,
		agentID,
		eventType,
		status,
		string(encoded),
	)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

func upsertTaskStatus(db *sql.DB, req taskStatusRequest) error {
	attempt := max(req.Attempt, 1)
	startedAt := nullableTime(req.Status == "running", req.Timestamp)
	finishedAt := nullableTime(req.Status != "running", req.Timestamp)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := db.ExecContext(
		ctx,
		`insert into tasks(
			task_id, tenant_id, agent_id, task_type, payload,
			status, attempt, started_at, finished_at
		)
		values($1,'default',$2,'command','{}'::jsonb,$3,$4,$5,$6)
		on conflict(task_id) do update set
			status = excluded.status,
			attempt = excluded.attempt,
			started_at = coalesce(tasks.started_at, excluded.started_at),
			finished_at = excluded.finished_at`,
		req.TaskID,
		req.AgentID,
		req.Status,
		attempt,
		startedAt,
		finishedAt,
	)
	return err
}

func upsertTaskReport(db *sql.DB, req taskReportRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startedAt := time.Unix(req.StartedAt, 0)
	finishedAt := time.Unix(req.FinishedAt, 0)

	_, err := db.ExecContext(
		ctx,
		`insert into tasks(
			task_id, tenant_id, agent_id, task_type, payload,
			status, attempt, started_at, finished_at
		)
		values($1,'default',$2,'command','{}'::jsonb,$3,1,$4,$5)
		on conflict(task_id) do update set
			status = excluded.status,
			finished_at = excluded.finished_at,
			started_at = coalesce(tasks.started_at, excluded.started_at)`,
		req.TaskID,
		req.AgentID,
		req.Status,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return err
	}

	_, err = db.ExecContext(
		ctx,
		`insert into task_results(
			task_id, exit_code, stdout, stderr, truncated, started_at, finished_at
		)
		values($1,$2,$3,$4,$5,$6,$7)
		on conflict(task_id) do update set
			exit_code = excluded.exit_code,
			stdout = excluded.stdout,
			stderr = excluded.stderr,
			truncated = excluded.truncated,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at`,
		req.TaskID,
		req.Result.ExitCode,
		req.Result.Stdout,
		req.Result.Stderr,
		req.Result.Truncated,
		startedAt,
		finishedAt,
	)
	return err
}

func nullableTime(enabled bool, unixSec int64) any {
	if !enabled {
		return nil
	}
	if unixSec <= 0 {
		return time.Now()
	}
	return time.Unix(unixSec, 0)
}

func (s *serverState) auth(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.tokens[token]
	if !ok {
		return "", false
	}
	if time.Now().After(record.ExpiresAt) {
		delete(s.tokens, token)
		return "", false
	}
	return record.AgentID, true
}

func writeJSON(w http.ResponseWriter, status int, payload envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, envelope{Code: 405, Message: "method not allowed", RequestID: requestID()})
}

func writeBadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, envelope{Code: 400, Message: message, RequestID: requestID()})
}

func writeAuthFailed(w http.ResponseWriter) {
	writeJSON(w, http.StatusUnauthorized, envelope{Code: 401, Message: "unauthorized", RequestID: requestID()})
}

func writeServerError(w http.ResponseWriter) {
	writeJSON(w, http.StatusInternalServerError, envelope{Code: 500, Message: "internal error", RequestID: requestID()})
}

func bearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("empty authorization")
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", errors.New("invalid authorization")
	}
	return parts[1], nil
}

func requestID() string {
	return "req-" + randHex(6)
}

func randHex(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return time.Now().Format("20060102150405")
	}
	return hex.EncodeToString(buf)
}

func toTaskSet(tasks []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tasks))
	for _, taskID := range tasks {
		if taskID == "" {
			continue
		}
		set[taskID] = struct{}{}
	}
	return set
}

func isTaskStatus(status string) bool {
	return status == "running" || status == "success" || status == "failed" || status == "canceled"
}

func max(current int, fallback int) int {
	if current <= 0 {
		return fallback
	}
	return current
}
