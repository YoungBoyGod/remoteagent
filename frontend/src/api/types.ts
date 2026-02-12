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
  labels: Record<string, string>
  capabilities: string[]
  heartbeat_interval: number
  last_heartbeat_at: number | null
  created_at: number | null
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
