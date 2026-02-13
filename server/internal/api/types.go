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
	Hostname   string `json:"hostname"`
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	IP         string `json:"ip"`
	ExternalIP string `json:"external_ip,omitempty"`
}

type HeartbeatRequest struct {
	AgentID           string      `json:"agent_id"`
	Timestamp         int64       `json:"timestamp"`
	Metrics           MetricsInfo `json:"metrics"`
	RunningTasks      []string    `json:"running_tasks"`
	PrometheusMetrics string      `json:"prometheus_metrics,omitempty"`
	ExternalIP        string      `json:"external_ip,omitempty"`
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

type PreemptRequest struct {
	Reason             string `json:"reason"`
	GracePeriodSeconds int    `json:"grace_period_seconds"`
	RequestedBy        string `json:"requested_by,omitempty"`
}

type PreemptAckRequest struct {
	EventID      string `json:"event_id"`
	AgentID      string `json:"agent_id"`
	TaskID       string `json:"task_id"`
	Timestamp    int64  `json:"timestamp"`
	PreemptState string `json:"preempt_state"`
}

type PreemptResponseData struct {
	TaskID          string `json:"task_id"`
	PreemptState    string `json:"preempt_state"`
	PreemptDeadline int64  `json:"preempt_deadline"`
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
	ExternalIP        string            `json:"external_ip"`
	Labels            map[string]string `json:"labels"`
	Capabilities      []string          `json:"capabilities"`
	HeartbeatInterval int               `json:"heartbeat_interval"`
	LastHeartbeatAt   *int64            `json:"last_heartbeat_at"`
	CreatedAt         *int64            `json:"created_at"`
	HostTags []string `json:"host_tags,omitempty"`
	// Phase 2: 并发槽位信息（来自 Redis 容量缓存）
	MaxConcurrent    *int  `json:"max_concurrent,omitempty"`
	RunningShared    *int  `json:"running_shared,omitempty"`
	RunningExclusive *bool `json:"running_exclusive,omitempty"`
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

// ============================================================
// Phase 2: 任务调度系统
// ============================================================

// POST /v1/tasks — 创建任务
type TaskCreateRequest struct {
	IdempotencyKey string        `json:"idempotency_key"`
	TaskType       string        `json:"task_type" binding:"required"`
	Payload        TaskPayload   `json:"payload" binding:"required"`
	ExecMode       string        `json:"exec_mode" binding:"required,oneof=shared exclusive"`
	Priority       int           `json:"priority"`     // 1-100，默认 50
	Preemptible    bool          `json:"preemptible"`
	MaxAttempts    int           `json:"max_attempts"` // 默认 3
	Schedule       *TaskSchedule `json:"schedule,omitempty"`
}

// TaskPayload 任务执行载荷
type TaskPayload struct {
	Command string            `json:"command" binding:"required"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
	Workdir string            `json:"workdir,omitempty"`
	Timeout int               `json:"timeout"` // 秒，默认 30
}

// TaskSchedule 调度约束（可选）
type TaskSchedule struct {
	TargetAgentID string            `json:"target_agent_id,omitempty"`
	TargetLabels  map[string]string `json:"target_labels,omitempty"`
}

// POST /v1/tasks 响应
type TaskCreateResponse struct {
	TaskID        string `json:"task_id"`
	Status        string `json:"status"`
	TargetAgentID string `json:"target_agent_id,omitempty"`
}

// POST /v1/tasks/batch — 批量创建任务
type TaskBatchCreateRequest struct {
	Tasks []TaskCreateRequest `json:"tasks" binding:"required,min=1,max=50"`
}

type TaskBatchCreateResponse struct {
	Tasks []TaskCreateResponse `json:"tasks"`
}

// PATCH /v1/tasks/{id}/priority — 调整优先级
type TaskPriorityRequest struct {
	Priority int `json:"priority" binding:"required,min=1,max=100"`
}

// POST /v1/agents/{id}/poll — Agent 拉取候选任务
type TaskPollRequest struct {
	AgentID        string            `json:"agent_id" binding:"required"`
	MaxConcurrent  int               `json:"max_concurrent"`
	RunningShared  int               `json:"running_shared"`
	RunningExcl    bool              `json:"running_exclusive"`
	Capabilities   map[string]any    `json:"capabilities,omitempty"`
	Labels         map[string]string `json:"labels,omitempty"`
}

// TaskPollResponse poll 返回的候选任务列表
type TaskPollResponse struct {
	Tasks []TaskCandidate `json:"tasks"`
}

// TaskCandidate 候选任务摘要
type TaskCandidate struct {
	TaskID   string      `json:"task_id"`
	TaskType string      `json:"task_type"`
	ExecMode string      `json:"exec_mode"`
	Priority int         `json:"priority"`
	Payload  TaskPayload `json:"payload"`
}

// POST /v1/tasks/{id}/claim — 认领任务
type TaskClaimRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type TaskClaimResponse struct {
	TaskID      string      `json:"task_id"`
	Status      string      `json:"status"`
	LeasedUntil int64       `json:"leased_until"` // unix 毫秒
	Payload     TaskPayload `json:"payload"`
}

// POST /v1/tasks/{id}/heartbeat — 续租
type TaskHeartbeatRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
}

type TaskHeartbeatResponse struct {
	LeasedUntil    int64           `json:"leased_until"` // unix 毫秒
	PreemptCommand *PreemptCommand `json:"preempt_command,omitempty"`
}

// PreemptCommand 抢占指令，通过 heartbeat/poll 下发
type PreemptCommand struct {
	TaskID             string `json:"task_id"`
	Reason             string `json:"reason"`
	GracePeriodSeconds int    `json:"grace_period_seconds"`
	Deadline           int64  `json:"deadline"` // unix 毫秒
}

// POST /v1/tasks/{id}/complete — 上报执行结果
type TaskCompleteRequest struct {
	AgentID    string `json:"agent_id" binding:"required"`
	Status     string `json:"status" binding:"required,oneof=success failed timeout canceled"`
	Attempt    int    `json:"attempt"`
	ExitCode   int    `json:"exit_code"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	Truncated  bool   `json:"truncated"`
	ErrorCode  string `json:"error_code,omitempty"`
	ErrorMsg   string `json:"error_message,omitempty"`
}

// POST /v1/tasks/{id}/cancel — 取消任务
type TaskCancelRequest struct {
	Reason string `json:"reason,omitempty"`
}

// GET /v1/tasks — 任务列表查询
type TaskListRequest struct {
	Status   string   `form:"status"`
	Statuses []string // 解析后的多状态列表（controller 填充）
	ExecMode string   `form:"exec_mode"`
	AgentID  string   `form:"agent_id"`
	Page     int      `form:"page"`
	PageSize int      `form:"page_size"`
}

// TaskDetail 任务详情（列表/单条查询通用）
type TaskDetail struct {
	TaskID             string      `json:"task_id"`
	IdempotencyKey     string      `json:"idempotency_key,omitempty"`
	TaskType           string      `json:"task_type"`
	Payload            TaskPayload `json:"payload"`
	ExecMode           string      `json:"exec_mode"`
	Priority           int         `json:"priority"`
	Preemptible        bool        `json:"preemptible"`
	Status             string      `json:"status"`
	AgentID            string      `json:"agent_id,omitempty"`
	TargetAgentID      string      `json:"target_agent_id,omitempty"`
	Attempt            int         `json:"attempt"`
	MaxAttempts        int         `json:"max_attempts"`
	LeasedUntil        *int64      `json:"leased_until,omitempty"`
	PreemptState       string      `json:"preempt_state"`
	ErrorCode          string      `json:"error_code,omitempty"`
	ErrorMessage       string      `json:"error_message,omitempty"`
	ExitCode           *int        `json:"exit_code,omitempty"`
	Stdout             string      `json:"stdout,omitempty"`
	Stderr             string      `json:"stderr,omitempty"`
	Truncated          bool        `json:"truncated,omitempty"`
	CreatedAt          int64       `json:"created_at"`
	UpdatedAt          int64       `json:"updated_at"`
	StartedAt          *int64      `json:"started_at,omitempty"`
	FinishedAt         *int64      `json:"finished_at,omitempty"`
}

// TaskListResponse 任务列表分页响应
type TaskListResponse struct {
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Items    []TaskDetail `json:"items"`
}

// AgentCapacity Agent 容量快照，用于调度决策
type AgentCapacity struct {
	MaxConcurrent    int  `json:"max_concurrent"`
	RunningShared    int  `json:"running_shared"`
	RunningExclusive bool `json:"running_exclusive"`
}

// AgentCapabilityInfo Agent 硬件能力信息，用于调度匹配
type AgentCapabilityInfo struct {
	CPUCores        int      `json:"cpu_cores"`
	MemoryBytes     int64    `json:"memory_bytes"`
	DiskBytes       int64    `json:"disk_bytes"`
	GPUList         []string `json:"gpu_list,omitempty"`
	DockerAvailable bool     `json:"docker_available"`
	CUDAVersion     string   `json:"cuda_version,omitempty"`
}

// ============================================================
// 主机管理
// ============================================================

// HostCreateRequest POST /v1/hosts — 手动创建主机
type HostCreateRequest struct {
	Name           string   `json:"name" binding:"required"`
	IP             string   `json:"ip" binding:"required"`
	Hostname       string   `json:"hostname"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	AuthType       string   `json:"auth_type" binding:"required,oneof=password key"`
	Password       string   `json:"password"`
	SSHKey         string   `json:"ssh_key"`
	VNCAddr        string   `json:"vnc_addr"`
	JupyterAddr    string   `json:"jupyter_addr"`
	ExtSSHAddr     string   `json:"ext_ssh_addr"`
	ExtVNCAddr     string   `json:"ext_vnc_addr"`
	ExtJupyterAddr string   `json:"ext_jupyter_addr"`
	AssignedTo     string   `json:"assigned_to"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
}

// HostAutoCreateRequest Agent 注册时自动创建主机（内部使用）
type HostAutoCreateRequest struct {
	AgentID  string
	IP       string
	Hostname string
}

// HostUpdateRequest PUT /v1/hosts/:host_id — 更新主机
type HostUpdateRequest struct {
	Name           string   `json:"name"`
	IP             string   `json:"ip"`
	Hostname       string   `json:"hostname"`
	Port           *int     `json:"port"`
	Username       string   `json:"username"`
	AuthType       string   `json:"auth_type"`
	Password       string   `json:"password"`
	SSHKey         string   `json:"ssh_key"`
	VNCAddr        string   `json:"vnc_addr"`
	JupyterAddr    string   `json:"jupyter_addr"`
	ExtSSHAddr     string   `json:"ext_ssh_addr"`
	ExtVNCAddr     string   `json:"ext_vnc_addr"`
	ExtJupyterAddr string   `json:"ext_jupyter_addr"`
	AssignedTo     *string  `json:"assigned_to"`
	Description    string   `json:"description"`
	Tags           []string `json:"tags"`
}

// HostItem 主机详情
type HostItem struct {
	HostID         string   `json:"host_id"`
	Name           string   `json:"name"`
	IP             string   `json:"ip"`
	Hostname       string   `json:"hostname"`
	Port           int      `json:"port"`
	Username       string   `json:"username"`
	AuthType       string   `json:"auth_type"`
	Password       string   `json:"password,omitempty"`
	Status         string   `json:"status"`
	Source         string   `json:"source"`
	VNCAddr        string   `json:"vnc_addr"`
	JupyterAddr    string   `json:"jupyter_addr"`
	ExtSSHAddr     string   `json:"ext_ssh_addr"`
	ExtVNCAddr     string   `json:"ext_vnc_addr"`
	ExtJupyterAddr string   `json:"ext_jupyter_addr"`
	AgentID        string   `json:"agent_id,omitempty"`
	AssignedTo     string   `json:"assigned_to,omitempty"`
	CustomerID     string   `json:"customer_id,omitempty"`
	Description    string   `json:"description,omitempty"`
	Tags           []string `json:"tags"`
	AgentStatus    string   `json:"agent_status,omitempty"`
	AgentHostname  string   `json:"agent_hostname,omitempty"`
	AgentOS        string   `json:"agent_os,omitempty"`
	AgentArch      string   `json:"agent_arch,omitempty"`
	AgentVersion   string   `json:"agent_version,omitempty"`
	ExternalIP     string   `json:"external_ip,omitempty"`
	LastHeartbeat  *int64   `json:"last_heartbeat_at,omitempty"`
	CreatedAt      int64    `json:"created_at"`
	UpdatedAt      int64    `json:"updated_at"`
}

// HostVerifyRequest POST /v1/hosts/:host_id/verify — 校验主机连接
type HostVerifyRequest struct {
	Methods []string `json:"methods"` // "ssh", "vnc", "jupyter"
}

// HostVerifyResult 单项校验结果
type HostVerifyResult struct {
	Method  string `json:"method"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Latency int    `json:"latency_ms"`
}

// HostListResponse GET /v1/hosts 分页响应
type HostListResponse struct {
	Total    int        `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	Items    []HostItem `json:"items"`
}

// HostListRequest GET /v1/hosts 查询参数
type HostListRequest struct {
	Status   string `form:"status"`
	Search   string `form:"search"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
