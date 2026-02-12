export interface Envelope<T = unknown> {
  code: number
  message: string
  request_id: string
  data: T
}

// GET /healthz
export interface HealthResp {
  service: string
  status: string
  timestamp: number
}

// GET /api/v1/debug/state
export interface SystemState {
  agents: number
  tasks: number
}

// GET /api/v1/debug/agents 列表项
export interface DebugAgentItem {
  agent_id: string
  device_code: string
  agent_version: string
  status: string
  hostname: string
  os: string
  arch: string
  ip: string
  external_ip: string
  labels: Record<string, string>
  capabilities: string[]
  heartbeat_interval: number
  last_heartbeat_at: number | null
  created_at: number | null
  // Phase 2: 并发槽位
  max_concurrent?: number
  running_shared?: number
  running_exclusive?: boolean
}

// GET /api/v1/debug/tasks 列表项
export interface TaskItem {
  task_id: string
  agent_id: string
  status: string
  exit_code: number
  stdout: string
  stderr: string
  truncated: boolean
  started_at: number | null
  finished_at: number | null
  created_at: number | null
}

// GET /api/v1/debug/tasks 分页响应
export interface TaskListResp {
  total: number
  page: number
  page_size: number
  items: TaskItem[]
}

// GET /api/v1/debug/task/:task_id
export interface TaskResult {
  task_id: string
  agent_id: string
  status: string
  exit_code: number
  stdout: string
  stderr: string
  truncated: boolean
  started_at: number | null
  finished_at: number | null
}

// POST /api/v1/debug/dispatch/task
export interface DispatchTaskReq {
  agent_id: string
  task_id: string
  command: string
  timeout: number
}

// POST /api/v1/debug/dispatch/control
export interface DispatchControlReq {
  agent_id: string
  action: string
  payload?: Record<string, unknown>
}

// ============================================================
// Phase 2: 任务调度系统
// ============================================================

// POST /api/v1/tasks 创建任务请求
export interface TaskCreateReq {
  idempotency_key?: string
  task_type: string
  payload: TaskPayload
  exec_mode: 'shared' | 'exclusive'
  priority?: number
  preemptible?: boolean
  max_attempts?: number
  schedule?: TaskSchedule
}

export interface TaskPayload {
  command: string
  args?: string[]
  env?: Record<string, string>
  workdir?: string
  timeout?: number
}

export interface TaskSchedule {
  target_agent_id?: string
  target_labels?: Record<string, string>
}

// POST /api/v1/tasks 响应
export interface TaskCreateResp {
  task_id: string
  status: string
}

// GET /api/v1/tasks 任务详情
export interface TaskDetail {
  task_id: string
  idempotency_key?: string
  task_type: string
  payload: TaskPayload
  exec_mode: string
  priority: number
  preemptible: boolean
  status: string
  agent_id?: string
  attempt: number
  max_attempts: number
  leased_until?: number | null
  preempt_state: string
  error_code?: string
  error_message?: string
  created_at: number
  updated_at: number
  started_at?: number | null
  finished_at?: number | null
}

// GET /api/v1/tasks 分页响应
export interface TaskDetailListResp {
  total: number
  page: number
  page_size: number
  items: TaskDetail[]
}

// PATCH /api/v1/tasks/:id/priority
export interface TaskPriorityReq {
  priority: number
}

// POST /api/v1/tasks/:id/cancel
export interface TaskCancelReq {
  reason?: string
}

// ============================================================
// 远程客户支持平台
// ============================================================

// Host/设备信息
export interface Host {
  host_id: string
  hostname: string
  ip: string
  os: string
  arch: string
  status: 'online' | 'offline' | 'busy' | 'maintenance'
  agent_id?: string
  tags: string[]
  description?: string
  created_at: number
  updated_at: number
  last_seen_at?: number
}

// 支持会话
export interface SupportSession {
  session_id: string
  host_id: string
  agent_id?: string
  customer_name: string
  customer_email?: string
  issue_description: string
  status: 'waiting' | 'active' | 'paused' | 'closed'
  priority: 'low' | 'medium' | 'high' | 'urgent'
  started_at: number
  ended_at?: number
  duration?: number
  notes?: string
  tags: string[]
}

// 支持消息
export interface SupportMessage {
  message_id: string
  session_id: string
  sender_type: 'agent' | 'customer' | 'system'
  sender_name: string
  content: string
  message_type: 'text' | 'file' | 'command' | 'screenshot'
  created_at: number
  metadata?: Record<string, unknown>
}

// 远程命令执行记录
export interface RemoteCommand {
  command_id: string
  session_id: string
  host_id: string
  command: string
  output?: string
  exit_code?: number
  executed_by: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  created_at: number
  completed_at?: number
}

// 客户信息
export interface Customer {
  customer_id: string
  name: string
  email?: string
  phone?: string
  company?: string
  tags: string[]
  notes?: string
  created_at: number
  total_sessions: number
}

// 支持统计
export interface SupportStats {
  total_sessions: number
  active_sessions: number
  waiting_sessions: number
  total_hosts: number
  online_hosts: number
  total_agents: number
  online_agents: number
  avg_session_duration: number
  sessions_today: number
  sessions_this_week: number
}
