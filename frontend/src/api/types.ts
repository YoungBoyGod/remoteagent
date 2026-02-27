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
  host_tags?: string[]
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
  target_agent_id?: string
}

// POST /api/v1/tasks/batch 批量创建
export interface TaskBatchCreateReq {
  tasks: TaskCreateReq[]
}

export interface TaskBatchCreateResp {
  tasks: TaskCreateResp[]
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
  submitter?: string
  attempt: number
  max_attempts: number
  leased_until?: number | null
  preempt_state: string
  error_code?: string
  error_message?: string
  exit_code?: number | null
  stdout?: string
  stderr?: string
  truncated?: boolean
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

export interface DashboardSummaryResp {
  agent_total: number
  agent_online: number
  host_total: number
  customer_total: number
  task_total: number
  task_status_count: Record<string, number>
  recent_tasks: TaskDetail[]
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

// ============================================================
// 主机管理
// ============================================================

// 受管主机信息
export interface ManagedHost {
  host_id: string
  name: string
  ip: string
  hostname: string
  port: number
  username: string
  auth_type: 'password' | 'key'
  password?: string
  status: 'online' | 'offline' | 'unknown'
  source: 'agent' | 'manual'
  vnc_addr: string
  jupyter_addr: string
  ext_ssh_addr: string
  ext_vnc_addr: string
  ext_jupyter_addr: string
  agent_id?: string
  assigned_to?: string
  customer_id?: string
  description?: string
  tags: string[]
  // Agent 上报信息
  agent_status?: string
  agent_hostname?: string
  agent_os?: string
  agent_arch?: string
  agent_version?: string
  external_ip?: string
  last_heartbeat_at?: number | null
  created_at: number
  updated_at: number
}

// 创建主机请求
export interface HostCreateReq {
  name: string
  ip: string
  hostname?: string
  port?: number
  username?: string
  auth_type: 'password' | 'key'
  password?: string
  ssh_key?: string
  vnc_addr?: string
  jupyter_addr?: string
  ext_ssh_addr?: string
  ext_vnc_addr?: string
  ext_jupyter_addr?: string
  assigned_to?: string
  description?: string
  tags?: string[]
}

// 更新主机请求
export interface HostUpdateReq {
  name?: string
  ip?: string
  hostname?: string
  port?: number
  username?: string
  auth_type?: 'password' | 'key'
  password?: string
  ssh_key?: string
  vnc_addr?: string
  jupyter_addr?: string
  ext_ssh_addr?: string
  ext_vnc_addr?: string
  ext_jupyter_addr?: string
  assigned_to?: string
  description?: string
  tags?: string[]
}

// 主机列表响应
export interface HostListResp {
  total: number
  page: number
  page_size: number
  items: ManagedHost[]
}

// ============================================================
// 客户管理
// ============================================================

export interface CustomerItem {
  customer_id: string
  name: string
  email: string
  phone: string
  company: string
  description?: string
  tags: string[]
  status: 'active' | 'inactive'
  host_count: number
  created_at: number
  updated_at: number
}

export interface CustomerCreateReq {
  name: string
  email?: string
  phone?: string
  company?: string
  description?: string
  tags?: string[]
}

export interface CustomerUpdateReq {
  name?: string
  email?: string
  phone?: string
  company?: string
  description?: string
  tags?: string[]
  status?: 'active' | 'inactive'
}

export interface CustomerListResp {
  total: number
  page: number
  page_size: number
  items: CustomerItem[]
}

export interface CustomerHostAssignReq {
  host_id: string
  note?: string
}

export interface CustomerHostItem {
  host_id: string
  host_name: string
  ip: string
  hostname: string
  status: string
  assigned_at: number
  note?: string
}

export interface CustomerHostListResp {
  items: CustomerHostItem[]
}

// ============================================================
// 操作日志
// ============================================================

export interface OperationLogItem {
  log_id: number
  resource_type: string
  resource_id: string
  action: string
  operator: string
  detail: Record<string, unknown>
  created_at: number
}

export interface OperationLogListResp {
  total: number
  page: number
  page_size: number
  items: OperationLogItem[]
}

// ============================================================
// 安全分发管理
// ============================================================

// POST /api/v1/distributions 创建分发记录
export interface DistributionCreateReq {
  file_name: string
  file_size: number
  sha256_original: string
  encryption_algo?: string
  customer_name: string
  customer_email: string
  release_notes?: string
  source_type?: 's3' | 'local'
  s3_key?: string
  scheduled_at?: number
}

// PUT /api/v1/distributions/:id 更新分发记录
export interface DistributionUpdateReq {
  encrypted_file_path?: string
  sha256_encrypted?: string
  session_key_hash?: string
  presigned_url?: string
  url_expires_at?: number | null
  release_notes?: string
  customer_name?: string
  customer_email?: string
}

// PATCH /api/v1/distributions/:id/status 更新状态
export interface DistributionStatusReq {
  status: string
  download_ip?: string
}

// 分发记录详情
export interface DistributionItem {
  id: number
  task_id: string
  file_name: string
  file_size: number
  encrypted_file_path?: string
  sha256_original: string
  sha256_encrypted?: string
  encryption_algo: string
  customer_name: string
  customer_email: string
  session_key_hash?: string
  presigned_url?: string
  url_expires_at?: number | null
  status: string
  download_ip?: string
  download_at?: number | null
  release_notes?: string
  scheduled_at?: number | null
  created_at: number
  updated_at: number
}

// GET /api/v1/distributions 分页响应
export interface DistributionListResp {
  total: number
  page: number
  page_size: number
  items: DistributionItem[]
}

// GET /api/v1/distributions/s3-objects 查询参数
export interface DistributionS3ListReq {
  prefix?: string
  page_size?: number
  continuation_token?: string
}

export interface DistributionS3ObjectItem {
  key: string
  size: number
  last_modified: number
}

export interface DistributionS3ListResp {
  items: DistributionS3ObjectItem[]
  next_token?: string
  has_more: boolean
}

// ============================================================
// 发布说明草稿
// ============================================================

export interface ReleaseNoteCreateReq {
  title: string
  content: string
  version?: string
  created_by?: string
}

export interface ReleaseNoteUpdateReq {
  title?: string
  content?: string
  version?: string
}

export interface ReleaseNoteItem {
  id: number
  title: string
  content: string
  version: string
  created_by: string
  created_at: number
  updated_at: number
}

export interface ReleaseNoteListResp {
  total: number
  page: number
  page_size: number
  items: ReleaseNoteItem[]
}
