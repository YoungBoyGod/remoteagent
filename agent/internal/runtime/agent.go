package runtime

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"luoyi2026/agent/internal/config"
)

const (
	defaultHeartbeatInterval = 30 * time.Second
	maxCommandOutputBytes    = 256 * 1024
)

type State string

const (
	StateInit        State = "INIT"
	StateRegistering State = "REGISTERING"
	StateRunning     State = "RUNNING"
	StateAuthExpired State = "AUTH_EXPIRED"
	StateDraining    State = "DRAINING"
	StateStopped     State = "STOPPED"
)

type Agent struct {
	cfg        config.Config
	httpClient *http.Client

	agentID string
	token   string

	heartbeatInterval time.Duration
	pollTimeout       time.Duration

	stateMu sync.Mutex
	state   State

	mu       sync.Mutex
	tasks    map[string]*taskRecord
	pending  []queuedRequest
	running  map[string]*runningTask
	canceled map[string]struct{}
	taskWg   sync.WaitGroup

	reauthCh   chan struct{}
	shutdownCh chan string

	paths filePaths
}

type filePaths struct {
	agentIDPath string
	tasksPath   string
	pendingPath string
}

type apiEnvelope struct {
	Code      int             `json:"code"`
	Message   string          `json:"message"`
	RequestID string          `json:"request_id"`
	Data      json.RawMessage `json:"data"`
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

type registerData struct {
	Token             string `json:"token"`
	HeartbeatInterval int    `json:"heartbeat_interval"`
	PollTimeout       int    `json:"poll_timeout"`
	ServerTime        int64  `json:"server_time"`
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

type pollMessage struct {
	Type       string          `json:"type"`
	DeliveryID string          `json:"delivery_id"`
	ServerTime int64           `json:"server_time"`
	Data       json.RawMessage `json:"data"`
}

type taskPayload struct {
	TaskID   string         `json:"task_id"`
	TaskType string         `json:"task_type"`
	Payload  commandPayload `json:"payload"`
}

type commandPayload struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type controlPayload struct {
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload"`
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

type taskRecord struct {
	TaskID     string `json:"task_id"`
	Status     string `json:"status"`
	StartedAt  int64  `json:"started_at,omitempty"`
	FinishedAt int64  `json:"finished_at,omitempty"`
	Attempt    int    `json:"attempt"`
	Command    string `json:"command,omitempty"`
	Timeout    int    `json:"timeout,omitempty"`
	ExitCode   int    `json:"exit_code,omitempty"`
	LastError  string `json:"last_error,omitempty"`
	UpdatedAt  int64  `json:"updated_at"`
	Truncated  bool   `json:"truncated,omitempty"`
}

type queuedRequest struct {
	Path    string          `json:"path"`
	Body    json.RawMessage `json:"body"`
	AddedAt int64           `json:"added_at"`
}

type runningTask struct {
	Cancel func()
}

type taskStoreFile struct {
	Tasks []*taskRecord `json:"tasks"`
}

var errUnauthorized = errors.New("unauthorized")

type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e httpStatusError) Error() string {
	if e.Body == "" {
		return fmt.Sprintf("request failed with status %d", e.StatusCode)
	}
	return fmt.Sprintf("request failed with status %d: %s", e.StatusCode, e.Body)
}

func New(cfg config.Config) *Agent {
	client := &http.Client{Timeout: 35 * time.Second}
	paths := filePaths{
		agentIDPath: filepath.Join(cfg.DataDir, "agent.id"),
		tasksPath:   filepath.Join(cfg.DataDir, "tasks.db.json"),
		pendingPath: filepath.Join(cfg.DataDir, "pending_reports.json"),
	}
	return &Agent{
		cfg:               cfg,
		httpClient:        client,
		heartbeatInterval: defaultHeartbeatInterval,
		pollTimeout:       cfg.PollTimeout,
		state:             StateInit,
		tasks:             make(map[string]*taskRecord),
		pending:           make([]queuedRequest, 0),
		running:           make(map[string]*runningTask),
		canceled:          make(map[string]struct{}),
		reauthCh:          make(chan struct{}, 1),
		shutdownCh:        make(chan string, 1),
		paths:             paths,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	if err := a.initialize(); err != nil {
		a.setState(StateStopped)
		return err
	}
	if err := a.registerUntilSuccess(ctx); err != nil {
		a.setState(StateStopped)
		return err
	}
	a.setState(StateRunning)
	if err := a.flushPending(ctx); err != nil {
		log.Printf("flush pending skipped: %v", err)
	}

	serverDone := make(chan struct{})
	go a.startLocalServer(serverDone)

	loopCancel, loopsDone := a.startLoops(ctx)

	for {
		select {
		case <-ctx.Done():
			a.requestShutdown("context canceled")
		case <-a.reauthCh:
			if a.getState() == StateDraining || a.getState() == StateStopped {
				continue
			}
			a.setState(StateAuthExpired)
			loopCancel()
			<-loopsDone
			if err := a.registerUntilSuccess(ctx); err != nil {
				a.setState(StateStopped)
				return err
			}
			if err := a.flushPending(ctx); err != nil {
				log.Printf("flush pending failed after reauth: %v", err)
			}
			a.setState(StateRunning)
			loopCancel, loopsDone = a.startLoops(ctx)
		case reason := <-a.shutdownCh:
			log.Printf("agent draining: %s", reason)
			a.setState(StateDraining)
			loopCancel()
			<-loopsDone
			a.waitRunningTasks(30 * time.Second)
			if err := a.flushPending(ctx); err != nil {
				log.Printf("final flush pending failed: %v", err)
			}
			a.setState(StateStopped)
			close(serverDone)
			return nil
		}
	}
}

func (a *Agent) startLocalServer(done <-chan struct{}) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"service":   "luoyi-agent",
			"status":    "ok",
			"timestamp": time.Now().Unix(),
			"agent_id":  a.agentID,
			"state":     string(a.getState()),
		})
	})

	srv := &http.Server{Addr: a.cfg.LocalAddr, Handler: mux}
	go func() {
		<-done
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("luoyi-agent local endpoint listening on %s", a.cfg.LocalAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("agent local endpoint stopped: %v", err)
	}
}

func (a *Agent) startLoops(parent context.Context) (context.CancelFunc, <-chan struct{}) {
	loopCtx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		a.heartbeatLoop(loopCtx)
	}()
	go func() {
		defer wg.Done()
		a.pollLoop(loopCtx)
	}()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	return cancel, done
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	backoff := time.Second
	for {
		err := a.sendHeartbeat(ctx)
		if err == nil {
			backoff = time.Second
			if !sleepContext(ctx, a.heartbeatInterval) {
				return
			}
			continue
		}
		if errors.Is(err, errUnauthorized) {
			a.triggerReauth()
			return
		}
		log.Printf("heartbeat failed: %v", err)
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

func (a *Agent) pollLoop(ctx context.Context) {
	backoff := time.Second
	for {
		message, err := a.pollOnce(ctx)
		if err == nil {
			backoff = time.Second
			if message == nil {
				continue
			}
			a.handlePollMessage(message)
			continue
		}
		if errors.Is(err, errUnauthorized) {
			a.triggerReauth()
			return
		}
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("poll failed: %v", err)
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return
		}
		backoff = nextBackoff(backoff)
	}
}

func (a *Agent) handlePollMessage(message *pollMessage) {
	switch message.Type {
	case "task":
		var payload taskPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("invalid task payload: %v", err)
			return
		}
		if payload.TaskID == "" || payload.TaskType != "command" || payload.Payload.Command == "" {
			log.Printf("ignore invalid task message")
			return
		}
		go a.runTask(payload)
	case "control":
		var payload controlPayload
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Printf("invalid control payload: %v", err)
			return
		}
		a.handleControl(payload)
	default:
		log.Printf("ignore unknown message type: %s", message.Type)
	}
}

func (a *Agent) handleControl(payload controlPayload) {
	switch payload.Action {
	case "refresh_token":
		log.Printf("control received: refresh_token")
		a.triggerReauth()
	case "shutdown":
		log.Printf("control received: shutdown")
		a.requestShutdown("control shutdown")
	case "reload_config":
		log.Printf("control received: reload_config")
	case "cancel_task", "cancel":
		taskID := strings.TrimSpace(readStringMap(payload.Payload, "task_id"))
		if taskID == "" {
			log.Printf("control cancel ignored: missing task_id")
			return
		}
		if a.cancelTaskFromControl(taskID) {
			log.Printf("control cancel accepted: %s", taskID)
			return
		}
		log.Printf("control cancel ignored: task not running %s", taskID)
	default:
		log.Printf("control ignored: %s", payload.Action)
	}
}

func (a *Agent) runTask(payload taskPayload) {
	now := time.Now().Unix()

	a.mu.Lock()
	existing := a.tasks[payload.TaskID]
	if existing != nil {
		switch existing.Status {
		case "success", "failed", "canceled", "running":
			a.mu.Unlock()
			log.Printf("skip duplicate task %s with status %s", payload.TaskID, existing.Status)
			return
		}
	}

	attempt := 1
	if existing != nil && existing.Attempt > 0 {
		attempt = existing.Attempt + 1
	}

	record := &taskRecord{
		TaskID:    payload.TaskID,
		Status:    "running",
		StartedAt: now,
		Attempt:   attempt,
		Command:   payload.Payload.Command,
		Timeout:   payload.Payload.Timeout,
		UpdatedAt: now,
	}
	a.tasks[payload.TaskID] = record

	taskCtx, cancel := context.WithCancel(context.Background())
	a.running[payload.TaskID] = &runningTask{Cancel: cancel}
	a.taskWg.Add(1)
	_ = a.persistTasksLocked()
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		delete(a.running, payload.TaskID)
		a.mu.Unlock()
		a.taskWg.Done()
	}()

	statusRunning := taskStatusRequest{
		EventID:   newEventID(),
		AgentID:   a.agentID,
		TaskID:    payload.TaskID,
		Status:    "running",
		Timestamp: now,
		Attempt:   attempt,
	}
	a.sendOrQueue("/api/v1/agent/task/status", statusRunning)

	timeout := time.Duration(payload.Payload.Timeout) * time.Second
	if timeout <= 0 {
		timeout = a.cfg.DefaultTimeout
	}

	result, execErr := runCommand(taskCtx, payload.Payload.Command, timeout)
	finishedAt := time.Now().Unix()
	finalStatus := "success"
	lastError := ""
	if execErr != nil {
		if errors.Is(execErr, context.Canceled) {
			if a.takeCanceledMark(payload.TaskID) {
				finalStatus = "canceled"
				lastError = "canceled by control"
			} else {
				finalStatus = "failed"
				lastError = "canceled by draining"
			}
		} else {
			finalStatus = "failed"
			lastError = execErr.Error()
		}
	} else {
		a.clearCanceledMark(payload.TaskID)
	}

	a.mu.Lock()
	record.Status = finalStatus
	record.FinishedAt = finishedAt
	record.ExitCode = result.ExitCode
	record.LastError = lastError
	record.UpdatedAt = finishedAt
	record.Truncated = result.Truncated
	_ = a.persistTasksLocked()
	a.mu.Unlock()

	statusFinal := taskStatusRequest{
		EventID:   newEventID(),
		AgentID:   a.agentID,
		TaskID:    payload.TaskID,
		Status:    finalStatus,
		Timestamp: finishedAt,
		Attempt:   attempt,
	}
	a.sendOrQueue("/api/v1/agent/task/status", statusFinal)

	report := taskReportRequest{
		EventID:    newEventID(),
		AgentID:    a.agentID,
		TaskID:     payload.TaskID,
		Status:     finalStatus,
		StartedAt:  record.StartedAt,
		FinishedAt: finishedAt,
		Result: reportResult{
			ExitCode:  result.ExitCode,
			Stdout:    result.Stdout,
			Stderr:    result.Stderr,
			Truncated: result.Truncated,
		},
	}
	a.sendOrQueue("/api/v1/agent/task/report", report)
}

func (a *Agent) sendHeartbeat(ctx context.Context) error {
	runningTasks := a.runningTaskIDs()
	req := heartbeatRequest{
		AgentID:      a.agentID,
		Timestamp:    time.Now().Unix(),
		Metrics:      collectMetrics(),
		RunningTasks: runningTasks,
	}
	_, err := a.postAuthJSON(ctx, "/api/v1/agent/heartbeat", req)
	return err
}

func (a *Agent) pollOnce(ctx context.Context) (*pollMessage, error) {
	token := a.getToken()
	if token == "" {
		return nil, errUnauthorized
	}
	requestURL := strings.TrimRight(a.cfg.ServerAddr, "/") + "/api/v1/agent/poll?agent_id=" + a.agentID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errUnauthorized
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, fmt.Errorf("poll status %d", resp.StatusCode)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("poll bad status %d: %s", resp.StatusCode, string(body))
	}

	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, err
	}
	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, nil
	}
	var message pollMessage
	if err := json.Unmarshal(envelope.Data, &message); err != nil {
		return nil, err
	}
	return &message, nil
}

func (a *Agent) postAuthJSON(ctx context.Context, path string, payload any) (apiEnvelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return apiEnvelope{}, err
	}
	return a.postAuthRaw(ctx, path, body)
}

func (a *Agent) postAuthRaw(ctx context.Context, path string, body []byte) (apiEnvelope, error) {
	token := a.getToken()
	if token == "" {
		return apiEnvelope{}, errUnauthorized
	}
	requestURL := strings.TrimRight(a.cfg.ServerAddr, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return apiEnvelope{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return apiEnvelope{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return apiEnvelope{}, errUnauthorized
	}
	if resp.StatusCode >= http.StatusInternalServerError {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return apiEnvelope{}, httpStatusError{StatusCode: resp.StatusCode, Body: string(payload)}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return apiEnvelope{}, httpStatusError{StatusCode: resp.StatusCode, Body: string(payload)}
	}
	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return apiEnvelope{}, err
	}
	return envelope, nil
}

func (a *Agent) sendOrQueue(path string, payload any) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := a.postAuthJSONWithRetry(ctx, path, payload, 3)
	if err == nil {
		return
	}
	if errors.Is(err, errUnauthorized) {
		a.triggerReauth()
	}
	a.enqueuePending(path, payload)
}

func (a *Agent) enqueuePending(path string, payload any) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		log.Printf("pending marshal failed: %v", err)
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pending = append(a.pending, queuedRequest{
		Path:    path,
		Body:    encoded,
		AddedAt: time.Now().Unix(),
	})
	if len(a.pending) > 1000 {
		a.pending = a.pending[len(a.pending)-1000:]
	}
	if err := a.persistPendingLocked(); err != nil {
		log.Printf("persist pending failed: %v", err)
	}
}

func (a *Agent) flushPending(ctx context.Context) error {
	backoff := time.Second
	for {
		a.mu.Lock()
		if len(a.pending) == 0 {
			a.mu.Unlock()
			return nil
		}
		next := a.pending[0]
		a.mu.Unlock()

		_, err := a.postAuthRawWithRetry(ctx, next.Path, next.Body, 5)
		if err != nil {
			if errors.Is(err, errUnauthorized) {
				a.triggerReauth()
			}
			if !sleepContext(ctx, backoffWithJitter(backoff)) {
				return err
			}
			backoff = nextBackoff(backoff)
			continue
		}
		backoff = time.Second

		a.mu.Lock()
		if len(a.pending) > 0 {
			a.pending = a.pending[1:]
			if err := a.persistPendingLocked(); err != nil {
				a.mu.Unlock()
				return err
			}
		}
		a.mu.Unlock()
	}
}

func (a *Agent) postAuthJSONWithRetry(ctx context.Context, path string, payload any, maxAttempts int) (apiEnvelope, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return apiEnvelope{}, err
	}
	return a.postAuthRawWithRetry(ctx, path, body, maxAttempts)
}

func (a *Agent) postAuthRawWithRetry(ctx context.Context, path string, body []byte, maxAttempts int) (apiEnvelope, error) {
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	backoff := time.Second
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		response, err := a.postAuthRaw(ctx, path, body)
		if err == nil {
			return response, nil
		}
		if errors.Is(err, errUnauthorized) {
			return apiEnvelope{}, err
		}
		if !isRetryableHTTPError(err) {
			return apiEnvelope{}, err
		}
		lastErr = err
		if attempt == maxAttempts {
			break
		}
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return apiEnvelope{}, context.Canceled
		}
		backoff = nextBackoff(backoff)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("request retry exhausted")
	}
	return apiEnvelope{}, lastErr
}

func isRetryableHTTPError(err error) bool {
	if errors.Is(err, context.Canceled) {
		return false
	}
	var statusErr httpStatusError
	if errors.As(err, &statusErr) {
		if statusErr.StatusCode == http.StatusTooManyRequests {
			return true
		}
		return statusErr.StatusCode >= http.StatusInternalServerError
	}
	return true
}

func (a *Agent) registerUntilSuccess(ctx context.Context) error {
	a.setState(StateRegistering)
	backoff := time.Second
	for {
		err := a.registerOnce(ctx)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return err
		}
		log.Printf("register failed: %v", err)
		if !sleepContext(ctx, backoffWithJitter(backoff)) {
			return context.Canceled
		}
		backoff = nextBackoff(backoff)
	}
}

func (a *Agent) registerOnce(ctx context.Context) error {
	request := registerRequest{
		AgentID:      a.agentID,
		DeviceCode:   a.cfg.DeviceCode,
		AgentVersion: a.cfg.AgentVersion,
		TenantID:     a.cfg.TenantID,
		Device:       collectDeviceInfo(),
		Labels: map[string]string{
			"runtime": "go",
		},
		Capabilities: []string{"command_exec"},
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return err
	}
	requestURL := strings.TrimRight(a.cfg.ServerAddr, "/") + "/api/v1/agent/register"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Register-Token", a.cfg.RegisterToken)

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("register unauthorized")
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("register status %d: %s", resp.StatusCode, string(body))
	}

	var envelope apiEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	var data registerData
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		return err
	}
	if data.Token == "" {
		return fmt.Errorf("empty token in register response")
	}

	a.mu.Lock()
	a.token = data.Token
	if data.HeartbeatInterval > 0 {
		a.heartbeatInterval = time.Duration(data.HeartbeatInterval) * time.Second
	} else {
		a.heartbeatInterval = defaultHeartbeatInterval
	}
	if data.PollTimeout > 0 {
		a.pollTimeout = time.Duration(data.PollTimeout) * time.Second
	}
	a.mu.Unlock()

	log.Printf("registered success: agent_id=%s heartbeat=%ds poll_timeout=%ds", a.agentID, int(a.heartbeatInterval/time.Second), int(a.pollTimeout/time.Second))
	return nil
}

func (a *Agent) initialize() error {
	a.setState(StateInit)
	if err := os.MkdirAll(a.cfg.DataDir, 0o755); err != nil {
		return err
	}
	agentID, err := a.loadOrCreateAgentID()
	if err != nil {
		return err
	}
	a.agentID = agentID
	if err := a.loadTasks(); err != nil {
		return err
	}
	if err := a.loadPending(); err != nil {
		return err
	}
	a.recoverRunningTasks()
	log.Printf("agent initialized: agent_id=%s device_code=%s server=%s", a.agentID, a.cfg.DeviceCode, a.cfg.ServerAddr)
	return nil
}

func (a *Agent) recoverRunningTasks() {
	now := time.Now().Unix()
	for _, task := range a.tasks {
		if task.Status != "running" {
			continue
		}
		task.Status = "failed"
		task.FinishedAt = now
		task.ExitCode = -1
		task.LastError = "agent restarted while task running"
		task.UpdatedAt = now

		statusReq := taskStatusRequest{
			EventID:   newEventID(),
			AgentID:   a.agentID,
			TaskID:    task.TaskID,
			Status:    "failed",
			Timestamp: now,
			Attempt:   max(task.Attempt, 1),
		}
		a.enqueuePending("/api/v1/agent/task/status", statusReq)

		reportReq := taskReportRequest{
			EventID:    newEventID(),
			AgentID:    a.agentID,
			TaskID:     task.TaskID,
			Status:     "failed",
			StartedAt:  task.StartedAt,
			FinishedAt: now,
			Result: reportResult{
				ExitCode: -1,
				Stdout:   "",
				Stderr:   "agent restarted while task running",
			},
		}
		a.enqueuePending("/api/v1/agent/task/report", reportReq)
	}
	a.mu.Lock()
	_ = a.persistTasksLocked()
	a.mu.Unlock()
}

func (a *Agent) loadOrCreateAgentID() (string, error) {
	content, err := os.ReadFile(a.paths.agentIDPath)
	if err == nil {
		id := strings.TrimSpace(string(content))
		if id != "" {
			return id, nil
		}
	}
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	id := newUUIDLike()
	if err := writeFileAtomic(a.paths.agentIDPath, []byte(id+"\n"), 0o644); err != nil {
		return "", err
	}
	return id, nil
}

func (a *Agent) loadTasks() error {
	data, err := os.ReadFile(a.paths.tasksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var file taskStoreFile
	if err := json.Unmarshal(data, &file); err != nil {
		return err
	}
	for _, task := range file.Tasks {
		if task == nil || task.TaskID == "" {
			continue
		}
		a.tasks[task.TaskID] = task
	}
	return nil
}

func (a *Agent) loadPending() error {
	data, err := os.ReadFile(a.paths.pendingPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var pending []queuedRequest
	if err := json.Unmarshal(data, &pending); err != nil {
		return err
	}
	a.pending = pending
	return nil
}

func (a *Agent) persistTasksLocked() error {
	items := make([]*taskRecord, 0, len(a.tasks))
	for _, record := range a.tasks {
		cloned := *record
		items = append(items, &cloned)
	}
	sort.Slice(items, func(left int, right int) bool {
		return items[left].TaskID < items[right].TaskID
	})
	payload, err := json.MarshalIndent(taskStoreFile{Tasks: items}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(a.paths.tasksPath, payload, 0o644)
}

func (a *Agent) persistPendingLocked() error {
	payload, err := json.MarshalIndent(a.pending, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(a.paths.pendingPath, payload, 0o644)
}

func (a *Agent) runningTaskIDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	ids := make([]string, 0, len(a.running))
	for taskID := range a.running {
		ids = append(ids, taskID)
	}
	sort.Strings(ids)
	return ids
}

func (a *Agent) waitRunningTasks(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		a.taskWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return
	case <-time.After(timeout):
		log.Printf("draining timeout, force cancel running tasks")
		a.cancelAllRunningTasks()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
}

func (a *Agent) cancelAllRunningTasks() {
	a.mu.Lock()
	cancels := make([]func(), 0, len(a.running))
	for _, task := range a.running {
		cancels = append(cancels, task.Cancel)
	}
	a.mu.Unlock()
	for _, cancel := range cancels {
		cancel()
	}
}

func (a *Agent) cancelTaskFromControl(taskID string) bool {
	a.mu.Lock()
	running, ok := a.running[taskID]
	if !ok {
		a.mu.Unlock()
		return false
	}
	a.canceled[taskID] = struct{}{}
	cancel := running.Cancel
	a.mu.Unlock()
	cancel()
	return true
}

func (a *Agent) takeCanceledMark(taskID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	_, ok := a.canceled[taskID]
	if ok {
		delete(a.canceled, taskID)
	}
	return ok
}

func (a *Agent) clearCanceledMark(taskID string) {
	a.mu.Lock()
	delete(a.canceled, taskID)
	a.mu.Unlock()
}

func (a *Agent) requestShutdown(reason string) {
	select {
	case a.shutdownCh <- reason:
	default:
	}
}

func (a *Agent) triggerReauth() {
	select {
	case a.reauthCh <- struct{}{}:
	default:
	}
}

func (a *Agent) setState(state State) {
	a.stateMu.Lock()
	a.state = state
	a.stateMu.Unlock()
}

func (a *Agent) getState() State {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.state
}

func (a *Agent) getToken() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.token
}

func collectDeviceInfo() deviceInfo {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown"
	}
	return deviceInfo{
		Hostname: hostname,
		OS:       runtime.GOOS,
		Arch:     runtime.GOARCH,
		IP:       detectLocalIP(),
	}
}

func detectLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "127.0.0.1"
	}
	if udpAddr.IP == nil {
		return "127.0.0.1"
	}
	return udpAddr.IP.String()
}

func collectMetrics() metricsInfo {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	memMB := float64(memory.Alloc) / 1024.0 / 1024.0
	return metricsInfo{
		CPUPercent:  0,
		MemPercent:  memMB,
		DiskPercent: 0,
	}
}

type commandResult struct {
	ExitCode  int
	Stdout    string
	Stderr    string
	Truncated bool
}

func runCommand(parent context.Context, command string, timeout time.Duration) (commandResult, error) {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.Command("sh", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout := newLimitedBuffer(maxCommandOutputBytes)
	stderr := newLimitedBuffer(maxCommandOutputBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		return commandResult{ExitCode: -1, Stderr: err.Error()}, err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		killProcessGroup(cmd)
		waitErr = <-done
	}

	exitCode := 0
	if waitErr != nil {
		exitCode = extractExitCode(waitErr)
	}
	result := commandResult{
		ExitCode:  exitCode,
		Stdout:    stdout.String(),
		Stderr:    stderr.String(),
		Truncated: stdout.Truncated() || stderr.Truncated(),
	}

	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	return result, waitErr
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		return
	}
	_ = cmd.Process.Kill()
}

func extractExitCode(err error) int {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return -1
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
		return status.ExitStatus()
	}
	return -1
}

type limitedBuffer struct {
	max       int
	buf       bytes.Buffer
	truncated bool
}

func newLimitedBuffer(max int) *limitedBuffer {
	return &limitedBuffer{max: max}
}

func (b *limitedBuffer) Write(payload []byte) (int, error) {
	if b.max <= 0 {
		b.truncated = true
		return len(payload), nil
	}
	if b.buf.Len() >= b.max {
		b.truncated = true
		return len(payload), nil
	}
	remaining := b.max - b.buf.Len()
	if len(payload) > remaining {
		_, _ = b.buf.Write(payload[:remaining])
		b.truncated = true
		return len(payload), nil
	}
	_, _ = b.buf.Write(payload)
	return len(payload), nil
}

func (b *limitedBuffer) String() string {
	return b.buf.String()
}

func (b *limitedBuffer) Truncated() bool {
	return b.truncated
}

func writeFileAtomic(path string, content []byte, mode os.FileMode) error {
	tempPath := fmt.Sprintf("%s.tmp.%d", path, time.Now().UnixNano())
	if err := os.WriteFile(tempPath, content, mode); err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

func newUUIDLike() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(raw)
	return fmt.Sprintf("%s-%s-%s-%s-%s", encoded[0:8], encoded[8:12], encoded[12:16], encoded[16:20], encoded[20:32])
}

func newEventID() string {
	return "evt-" + randomHex(8)
}

func randomHex(length int) string {
	if length <= 0 {
		length = 8
	}
	raw := make([]byte, length)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw)
}

func nextBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > 60*time.Second {
		return 60 * time.Second
	}
	if next < time.Second {
		return time.Second
	}
	return next
}

func backoffWithJitter(delay time.Duration) time.Duration {
	if delay <= 0 {
		delay = time.Second
	}
	jitterRaw := make([]byte, 1)
	_, _ = rand.Read(jitterRaw)
	jitterMs := int(jitterRaw[0]) % 500
	return delay + time.Duration(jitterMs)*time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func max(current int, fallback int) int {
	if current <= 0 {
		return fallback
	}
	return current
}

func readStringMap(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
