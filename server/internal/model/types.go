package model

import (
	"time"
)

type Envelope struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
	Data      any    `json:"data,omitempty"`
}

type HealthResp struct {
	Service   string `json:"service"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

type RegisterRequest struct {
	AgentID      string            `json:"agent_id"`
	DeviceCode   string            `json:"device_code"`
	AgentVersion string            `json:"agent_version"`
	TenantID     string            `json:"tenant_id"`
	Device       DeviceInfo        `json:"device"`
	Labels       map[string]string `json:"labels"`
	Capabilities []string          `json:"capabilities"`
}

type DeviceInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	IP       string `json:"ip"`
}

type HeartbeatRequest struct {
	AgentID      string      `json:"agent_id"`
	Timestamp    int64       `json:"timestamp"`
	Metrics      MetricsInfo `json:"metrics"`
	RunningTasks []string    `json:"running_tasks"`
}

type MetricsInfo struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	DiskPercent float64 `json:"disk_percent"`
}

type TaskStatusRequest struct {
	EventID   string `json:"event_id"`
	AgentID   string `json:"agent_id"`
	TaskID    string `json:"task_id"`
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
	Attempt   int    `json:"attempt"`
}

type TaskReportRequest struct {
	EventID    string       `json:"event_id"`
	AgentID    string       `json:"agent_id"`
	TaskID     string       `json:"task_id"`
	Status     string       `json:"status"`
	StartedAt  int64        `json:"started_at"`
	FinishedAt int64        `json:"finished_at"`
	Result     ReportResult `json:"result"`
}

type ReportResult struct {
	ExitCode  int    `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
}

type DebugTaskDispatch struct {
	AgentID string `json:"agent_id"`
	TaskID  string `json:"task_id"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type DebugControlDispatch struct {
	AgentID string         `json:"agent_id"`
	Action  string         `json:"action"`
	Payload map[string]any `json:"payload"`
}

type AgentRecord struct {
	AgentID            string
	DeviceCode         string
	Token              string
	TokenExpiresAt     time.Time
	HeartbeatInterval  int
	PollTimeoutSeconds int
	LastHeartbeatAt    time.Time
	RunningTasks       map[string]struct{}
}

type TaskRecord struct {
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

type TokenRecord struct {
	AgentID   string
	ExpiresAt time.Time
}
