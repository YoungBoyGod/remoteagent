package model

import (
	"time"
)

type AgentRecord struct {
	AgentID            string
	DeviceCode         string
	AgentVersion       string
	Hostname           string
	OS                 string
	Arch               string
	IP                 string
	ExternalIP         string
	Labels             map[string]string
	Capabilities       []string
	Token              string
	TokenExpiresAt     time.Time
	HeartbeatInterval  int
	PollTimeoutSeconds int
	LastHeartbeatAt    time.Time
	CreatedAt          time.Time
	RunningTasks       map[string]struct{}
	PrometheusMetrics  string
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
