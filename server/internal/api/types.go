package api

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
	AgentID           string      `json:"agent_id"`
	Timestamp         int64       `json:"timestamp"`
	Metrics           MetricsInfo `json:"metrics"`
	RunningTasks      []string    `json:"running_tasks"`
	PrometheusMetrics string      `json:"prometheus_metrics,omitempty"`
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

// DebugAgentItem agent 列表中的单个 agent 信息
type DebugAgentItem struct {
	AgentID           string            `json:"agent_id"`
	DeviceCode        string            `json:"device_code"`
	AgentVersion      string            `json:"agent_version"`
	Status            string            `json:"status"`
	Hostname          string            `json:"hostname"`
	OS                string            `json:"os"`
	Arch              string            `json:"arch"`
	IP                string            `json:"ip"`
	Labels            map[string]string `json:"labels"`
	Capabilities      []string          `json:"capabilities"`
	HeartbeatInterval int               `json:"heartbeat_interval"`
	LastHeartbeatAt   *int64            `json:"last_heartbeat_at"`
	CreatedAt         *int64            `json:"created_at"`
}

// DebugTaskItem 任务列表中的单个任务信息
type DebugTaskItem struct {
	TaskID     string `json:"task_id"`
	AgentID    string `json:"agent_id"`
	Status     string `json:"status"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Truncated  bool   `json:"truncated"`
	StartedAt  *int64 `json:"started_at"`
	FinishedAt *int64 `json:"finished_at"`
	CreatedAt  *int64 `json:"created_at"`
}

// DebugTaskListData 任务列表分页响应
type DebugTaskListData struct {
	Total    int             `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Items    []DebugTaskItem `json:"items"`
}
