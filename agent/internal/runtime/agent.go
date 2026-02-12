package runtime

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"luoyi2026/agent/internal/config"
	"luoyi2026/agent/internal/observability"
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
	canceled  map[string]struct{}
	preempted map[string]struct{}
	taskWg    sync.WaitGroup

	reauthCh   chan struct{}
	shutdownCh chan string

	db  *sql.DB
	obs *observability.Metrics
	cc  *concurrencyController

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
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	IP         string `json:"ip"`
	ExternalIP string `json:"external_ip,omitempty"`
}

type heartbeatRequest struct {
	AgentID           string      `json:"agent_id"`
	Timestamp         int64       `json:"timestamp"`
	Metrics           metricsInfo `json:"metrics"`
	RunningTasks      []string    `json:"running_tasks"`
	PrometheusMetrics string      `json:"prometheus_metrics,omitempty"`
	ExternalIP        string      `json:"external_ip,omitempty"`
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

func (errorValue httpStatusError) Error() string {
	if errorValue.Body == "" {
		return fmt.Sprintf("request failed with status %d", errorValue.StatusCode)
	}
	return fmt.Sprintf("request failed with status %d: %s", errorValue.StatusCode, errorValue.Body)
}

func (a *Agent) ReloadConfig() {
	if err := a.cfg.ReloadFrom(); err != nil {
		log.Printf("reload config failed: %v", err)
		return
	}
	a.pollTimeout = a.cfg.PollTimeout
	log.Printf("config reloaded: poll_timeout=%s default_timeout=%s", a.cfg.PollTimeout, a.cfg.DefaultTimeout)
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
		preempted:         make(map[string]struct{}),
		reauthCh:          make(chan struct{}, 1),
		shutdownCh:        make(chan string, 1),
		obs:               observability.NewMetrics(),
		cc:                newConcurrencyController(cfg.MaxConcurrent),
		paths:             paths,
	}
}
