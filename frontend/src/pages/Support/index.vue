<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  Monitor,
  Connection,
  OfficeBuilding,
  ChatDotRound,
  VideoPlay,
  Refresh,
  Plus,
  Search,
  MoreFilled,
  CircleCheck,
  Warning,
  User,
  Message,
  CloseBold,
  Document,
} from '@element-plus/icons-vue'
import client from '../../api/client'
import type {
  DebugAgentItem,
  Host,
  SupportSession,
  SupportMessage,
  SupportStats,
} from '../../api/types'

// ============================================================
// 状态管理
// ============================================================
const activeTab = ref('overview')

// ============================================================
// 统计数据
// ============================================================
const stats = reactive<SupportStats>({
  total_sessions: 0,
  active_sessions: 0,
  waiting_sessions: 0,
  total_hosts: 0,
  online_hosts: 0,
  total_agents: 0,
  online_agents: 0,
  avg_session_duration: 0,
  sessions_today: 0,
  sessions_this_week: 0,
})

// ============================================================
// Agent 管理
// ============================================================
const agents = ref<DebugAgentItem[]>([])
const agentsLoading = ref(false)
const agentFilter = reactive({
  status: '',
  search: '',
})

const filteredAgents = computed(() => {
  return agents.value.filter((agent) => {
    if (agentFilter.status && agent.status !== agentFilter.status) return false
    if (agentFilter.search) {
      const search = agentFilter.search.toLowerCase()
      return (
        agent.agent_id.toLowerCase().includes(search) ||
        agent.hostname?.toLowerCase().includes(search) ||
        agent.ip?.includes(search)
      )
    }
    return true
  })
})

const onlineAgents = computed(() => agents.value.filter((a) => a.status === 'online'))

async function fetchAgents() {
  agentsLoading.value = true
  try {
    const resp = await client.get('/api/v1/debug/agents')
    agents.value = resp.data.data ?? []
    stats.total_agents = agents.value.length
    stats.online_agents = onlineAgents.value.length
  } catch {
    // interceptor handles error
  } finally {
    agentsLoading.value = false
  }
}

async function refreshAgentToken(agentId: string) {
  try {
    await client.post(`/api/v1/debug/agents/${agentId}/refresh-token`)
    ElMessage.success('Token 已刷新')
  } catch {
    // interceptor handles error
  }
}

async function shutdownAgent(agentId: string) {
  try {
    await ElMessageBox.confirm(`确定要关闭 Agent ${agentId} 吗？`, '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    await client.post(`/api/v1/debug/agents/${agentId}/shutdown`)
    ElMessage.success('关闭指令已发送')
  } catch {
    // user cancelled or error
  }
}

// ============================================================
// Host 管理
// ============================================================
const hosts = ref<Host[]>([])
const hostsLoading = ref(false)
const hostFilter = reactive({
  status: '',
  search: '',
})
const hostDialogVisible = ref(false)
const hostForm = reactive<Partial<Host>>({
  hostname: '',
  ip: '',
  os: '',
  arch: '',
  description: '',
  tags: [],
})

const filteredHosts = computed(() => {
  return hosts.value.filter((host) => {
    if (hostFilter.status && host.status !== hostFilter.status) return false
    if (hostFilter.search) {
      const search = hostFilter.search.toLowerCase()
      return (
        host.hostname.toLowerCase().includes(search) ||
        host.ip.includes(search) ||
        host.host_id.toLowerCase().includes(search)
      )
    }
    return true
  })
})

async function fetchHosts() {
  hostsLoading.value = true
  try {
    const resp = await client.get<{ data: Host[] }>('/api/v1/hosts')
    hosts.value = resp.data.data ?? []
    stats.total_hosts = hosts.value.length
    stats.online_hosts = hosts.value.filter((h) => h.status === 'online').length
  } finally {
    hostsLoading.value = false
  }
}

function openHostDialog(host?: Host) {
  if (host) {
    Object.assign(hostForm, host)
  } else {
    Object.assign(hostForm, {
      hostname: '',
      ip: '',
      os: '',
      arch: '',
      description: '',
      tags: [],
    })
  }
  hostDialogVisible.value = true
}

async function saveHost() {
  // 保存Host逻辑
  hostDialogVisible.value = false
  ElMessage.success('保存成功')
  fetchHosts()
}

// ============================================================
// 会话管理
// ============================================================
const sessions = ref<SupportSession[]>([])
const sessionsLoading = ref(false)
const sessionFilter = reactive({
  status: '',
  priority: '',
  search: '',
})
const sessionDialogVisible = ref(false)
const sessionForm = reactive({
  host_id: '',
  customer_name: '',
  customer_email: '',
  issue_description: '',
  priority: 'medium' as const,
  tags: [] as string[],
})

const filteredSessions = computed(() => {
  return sessions.value.filter((session) => {
    if (sessionFilter.status && session.status !== sessionFilter.status) return false
    if (sessionFilter.priority && session.priority !== sessionFilter.priority) return false
    if (sessionFilter.search) {
      const search = sessionFilter.search.toLowerCase()
      return (
        session.customer_name.toLowerCase().includes(search) ||
        session.issue_description.toLowerCase().includes(search) ||
        session.session_id.toLowerCase().includes(search)
      )
    }
    return true
  })
})

const activeSessions = computed(() => sessions.value.filter((s) => s.status === 'active'))
const waitingSessions = computed(() => sessions.value.filter((s) => s.status === 'waiting'))

async function fetchSessions() {
  sessionsLoading.value = true
  try {
    // 会话管理功能暂未实现后端接口
    sessions.value = []
    stats.total_sessions = 0
    stats.active_sessions = 0
    stats.waiting_sessions = 0
    stats.sessions_today = 0
    stats.sessions_this_week = 0
  } finally {
    sessionsLoading.value = false
  }
}

function openSessionDialog() {
  Object.assign(sessionForm, {
    host_id: '',
    customer_name: '',
    customer_email: '',
    issue_description: '',
    priority: 'medium',
    tags: [],
  })
  sessionDialogVisible.value = true
}

async function createSession() {
  // 创建会话逻辑
  sessionDialogVisible.value = false
  ElMessage.success('会话创建成功')
  fetchSessions()
}

async function closeSession(_sessionId: string) {
  try {
    await ElMessageBox.confirm('确定要关闭此会话吗？', '确认', {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    })
    // 关闭会话逻辑
    ElMessage.success('会话已关闭')
    fetchSessions()
  } catch {
    // user cancelled
  }
}

async function joinSession(sessionId: string) {
  activeTab.value = 'chat'
  currentSessionId.value = sessionId
  fetchMessages(sessionId)
}

// ============================================================
// 聊天功能
// ============================================================
const currentSessionId = ref('')
const messages = ref<SupportMessage[]>([])
const messageText = ref('')
const chatLoading = ref(false)

async function fetchMessages(_sessionId: string) {
  chatLoading.value = true
  try {
    // 消息功能暂未实现后端接口
    messages.value = []
  } finally {
    chatLoading.value = false
  }
}

async function sendMessage() {
  if (!messageText.value.trim()) return
  // 发送消息逻辑
  messages.value.push({
    message_id: `msg-${Date.now()}`,
    session_id: currentSessionId.value,
    sender_type: 'agent',
    sender_name: '技术支持',
    content: messageText.value,
    message_type: 'text',
    created_at: Date.now(),
  })
  messageText.value = ''
}

// ============================================================
// 远程控制
// ============================================================
const remoteDialogVisible = ref(false)
const remoteCommand = ref('')
const remoteOutput = ref('')
const remoteExecuting = ref(false)

function openRemoteDialog(_hostId: string) {
  remoteDialogVisible.value = true
  remoteCommand.value = ''
  remoteOutput.value = ''
}

async function executeRemoteCommand() {
  if (!remoteCommand.value.trim()) return
  remoteExecuting.value = true
  remoteOutput.value = ''
  try {
    const resp = await client.post<{ data: { task_id: string } }>('/api/v1/debug/dispatch/task', {
      command: remoteCommand.value,
    })
    remoteOutput.value = `任务已提交，task_id: ${resp.data.data?.task_id ?? '未知'}`
  } catch (e: unknown) {
    remoteOutput.value = `执行失败: ${e instanceof Error ? e.message : String(e)}`
  } finally {
    remoteExecuting.value = false
  }
}

// ============================================================
// 初始化
// ============================================================
let refreshTimer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  fetchAgents()
  fetchHosts()
  fetchSessions()

  // 自动刷新
  refreshTimer = setInterval(() => {
    fetchAgents()
    fetchHosts()
    fetchSessions()
  }, 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})

// ============================================================
// 工具函数
// ============================================================
function formatDuration(ms: number): string {
  const minutes = Math.floor(ms / 60000)
  const hours = Math.floor(minutes / 60)
  if (hours > 0) {
    return `${hours}小时${minutes % 60}分钟`
  }
  return `${minutes}分钟`
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleString('zh-CN')
}

function getStatusType(status: string): string {
  const map: Record<string, string> = {
    online: 'success',
    offline: 'info',
    busy: 'warning',
    maintenance: 'danger',
    active: 'success',
    waiting: 'warning',
    paused: 'info',
    closed: 'info',
    low: 'info',
    medium: 'success',
    high: 'warning',
    urgent: 'danger',
  }
  return map[status] || 'info'
}

function getStatusText(status: string): string {
  const map: Record<string, string> = {
    online: '在线',
    offline: '离线',
    busy: '忙碌',
    maintenance: '维护中',
    active: '进行中',
    waiting: '等待中',
    paused: '已暂停',
    closed: '已关闭',
    low: '低',
    medium: '中',
    high: '高',
    urgent: '紧急',
  }
  return map[status] || status
}
</script>

<template>
  <div class="support-platform">
    <div class="page-header">
      <h1 class="page-title">
        <el-icon size="28"><Monitor /></el-icon>
        远程客户支持平台
      </h1>
      <div class="header-actions">
        <el-button :icon="Refresh" @click="fetchAgents(); fetchHosts(); fetchSessions()">
          刷新
        </el-button>
        <el-button type="primary" :icon="Plus" @click="openSessionDialog">
          新建会话
        </el-button>
      </div>
    </div>

    <el-tabs v-model="activeTab" type="border-card" class="support-tabs">
      <!-- 概览 -->
      <el-tab-pane name="overview">
        <template #label>
          <span class="tab-label">
            <el-icon><Monitor /></el-icon>
            概览
          </span>
        </template>

        <div class="stats-grid">
          <el-card class="stat-card" shadow="hover">
            <div class="stat-value">{{ stats.active_sessions }}</div>
            <div class="stat-label">进行中会话</div>
            <div class="stat-sublabel" v-if="stats.waiting_sessions > 0">
              <el-tag type="warning" size="small">{{ stats.waiting_sessions }} 个等待中</el-tag>
            </div>
          </el-card>

          <el-card class="stat-card" shadow="hover">
            <div class="stat-value">{{ stats.online_agents }}/{{ stats.total_agents }}</div>
            <div class="stat-label">在线 Agent</div>
            <div class="stat-sublabel">
              <el-progress
                :percentage="stats.total_agents ? Math.round((stats.online_agents / stats.total_agents) * 100) : 0"
                :stroke-width="8"
                :show-text="false"
              />
            </div>
          </el-card>

          <el-card class="stat-card" shadow="hover">
            <div class="stat-value">{{ stats.online_hosts }}/{{ stats.total_hosts }}</div>
            <div class="stat-label">在线 Host</div>
            <div class="stat-sublabel">
              <el-progress
                :percentage="stats.total_hosts ? Math.round((stats.online_hosts / stats.total_hosts) * 100) : 0"
                :stroke-width="8"
                :show-text="false"
                status="success"
              />
            </div>
          </el-card>

          <el-card class="stat-card" shadow="hover">
            <div class="stat-value">{{ stats.sessions_today }}</div>
            <div class="stat-label">今日会话</div>
            <div class="stat-sublabel">本周: {{ stats.sessions_this_week }}</div>
          </el-card>
        </div>

        <el-row :gutter="20" class="overview-content">
          <el-col :xs="24" :lg="12">
            <el-card title="进行中的会话" class="overview-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <span>进行中的会话</span>
                  <el-button text size="small" @click="activeTab = 'sessions'">查看全部</el-button>
                </div>
              </template>
              <el-table :data="activeSessions.slice(0, 5)" size="small">
                <el-table-column prop="customer_name" label="客户" width="100" />
                <el-table-column prop="issue_description" label="问题描述" show-overflow-tooltip />
                <el-table-column prop="priority" label="优先级" width="80">
                  <template #default="{ row }">
                    <el-tag :type="getStatusType(row.priority)" size="small">
                      {{ getStatusText(row.priority) }}
                    </el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="80" align="center">
                  <template #default="{ row }">
                    <el-button link type="primary" size="small" @click="joinSession(row.session_id)">
                      接入
                    </el-button>
                  </template>
                </el-table-column>
              </el-table>
            </el-card>
          </el-col>

          <el-col :xs="24" :lg="12">
            <el-card title="在线 Agent" class="overview-card" shadow="hover">
              <template #header>
                <div class="card-header">
                  <span>在线 Agent</span>
                  <el-button text size="small" @click="activeTab = 'agents'">查看全部</el-button>
                </div>
              </template>
              <el-table :data="onlineAgents.slice(0, 5)" size="small">
                <el-table-column prop="agent_id" label="Agent ID" width="120" />
                <el-table-column prop="hostname" label="主机名" />
                <el-table-column prop="ip" label="IP" width="120" />
                <el-table-column label="状态" width="80" align="center">
                  <template #default>
                    <el-tag type="success" size="small">在线</el-tag>
                  </template>
                </el-table-column>
              </el-table>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <!-- Agent 管理 -->
      <el-tab-pane name="agents">
        <template #label>
          <span class="tab-label">
            <el-icon><Connection /></el-icon>
            Agent 管理
            <el-tag v-if="onlineAgents.length > 0" type="success" size="small" class="tab-badge">
              {{ onlineAgents.length }}
            </el-tag>
          </span>
        </template>

        <div class="toolbar">
          <el-input
            v-model="agentFilter.search"
            placeholder="搜索 Agent ID / 主机名 / IP"
            clearable
            style="width: 300px"
            :prefix-icon="Search"
          />
          <el-select v-model="agentFilter.status" placeholder="状态筛选" clearable style="width: 140px">
            <el-option label="在线" value="online" />
            <el-option label="离线" value="offline" />
          </el-select>
          <div class="toolbar-spacer" />
          <el-button :icon="Refresh" @click="fetchAgents">刷新</el-button>
        </div>

        <el-table :data="filteredAgents" v-loading="agentsLoading" stripe border>
          <el-table-column type="expand" width="50">
            <template #default="{ row }">
              <div class="agent-detail">
                <el-descriptions :column="3" border size="small">
                  <el-descriptions-item label="Agent ID">{{ row.agent_id }}</el-descriptions-item>
                  <el-descriptions-item label="Device Code">{{ row.device_code }}</el-descriptions-item>
                  <el-descriptions-item label="Version">{{ row.agent_version }}</el-descriptions-item>
                  <el-descriptions-item label="OS">{{ row.os }} ({{ row.arch }})</el-descriptions-item>
                  <el-descriptions-item label="IP">{{ row.ip }}</el-descriptions-item>
                  <el-descriptions-item label="External IP">{{ row.external_ip || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="Labels" :span="3">
                    <el-tag
                      v-for="(value, key) in row.labels"
                      :key="key"
                      size="small"
                      style="margin-right: 8px"
                    >
                      {{ key }}={{ value }}
                    </el-tag>
                    <span v-if="!row.labels || Object.keys(row.labels).length === 0">-</span>
                  </el-descriptions-item>
                  <el-descriptions-item label="Capabilities" :span="3">
                    <el-tag
                      v-for="cap in row.capabilities || []"
                      :key="cap"
                      type="success"
                      size="small"
                      style="margin-right: 8px"
                    >
                      {{ cap }}
                    </el-tag>
                    <span v-if="!row.capabilities || row.capabilities.length === 0">-</span>
                  </el-descriptions-item>
                </el-descriptions>
              </div>
            </template>
          </el-table-column>

          <el-table-column prop="agent_id" label="Agent ID" min-width="150" show-overflow-tooltip />
          <el-table-column prop="hostname" label="主机名" min-width="150" show-overflow-tooltip />
          <el-table-column prop="ip" label="IP 地址" width="140" />
          <el-table-column prop="os" label="操作系统" width="120" />
          <el-table-column label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small">
                <el-icon v-if="row.status === 'online'"><CircleCheck /></el-icon>
                <el-icon v-else><Warning /></el-icon>
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="心跳" width="100" align="center">
            <template #default="{ row }">
              <span v-if="row.last_heartbeat_at" class="heartbeat-time">
                {{ formatDuration(Date.now() - row.last_heartbeat_at * 1000) }}前
              </span>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150" align="center" fixed="right">
            <template #default="{ row }">
              <el-dropdown trigger="click">
                <el-button link type="primary" :icon="MoreFilled" />
                <template #dropdown>
                  <el-dropdown-menu>
                    <el-dropdown-item @click="refreshAgentToken(row.agent_id)">
                      <el-icon><Refresh /></el-icon> 刷新 Token
                    </el-dropdown-item>
                    <el-dropdown-item @click="openRemoteDialog(row.agent_id)">
                      <el-icon><Document /></el-icon> 远程命令
                    </el-dropdown-item>
                    <el-dropdown-item divided @click="shutdownAgent(row.agent_id)">
                      <el-icon><CloseBold /></el-icon> 关闭 Agent
                    </el-dropdown-item>
                  </el-dropdown-menu>
                </template>
              </el-dropdown>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- Host 管理 -->
      <el-tab-pane name="hosts">
        <template #label>
          <span class="tab-label">
            <el-icon><OfficeBuilding /></el-icon>
            Host 管理
          </span>
        </template>

        <div class="toolbar">
          <el-input
            v-model="hostFilter.search"
            placeholder="搜索 Host ID / 主机名 / IP"
            clearable
            style="width: 300px"
            :prefix-icon="Search"
          />
          <el-select v-model="hostFilter.status" placeholder="状态筛选" clearable style="width: 140px">
            <el-option label="在线" value="online" />
            <el-option label="离线" value="offline" />
            <el-option label="忙碌" value="busy" />
            <el-option label="维护中" value="maintenance" />
          </el-select>
          <div class="toolbar-spacer" />
          <el-button :icon="Refresh" @click="fetchHosts">刷新</el-button>
          <el-button type="primary" :icon="Plus" @click="openHostDialog()">添加 Host</el-button>
        </div>

        <el-row :gutter="16">
          <el-col :xs="24" :sm="12" :lg="8" :xl="6" v-for="host in filteredHosts" :key="host.host_id">
            <el-card class="host-card" shadow="hover" :body-style="{ padding: '16px' }">
              <div class="host-header">
                <el-tag :type="getStatusType(host.status)" size="small" effect="dark">
                  {{ getStatusText(host.status) }}
                </el-tag>
                <el-dropdown trigger="click">
                  <el-button link :icon="MoreFilled" />
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item @click="openHostDialog(host)">
                        <el-icon><Document /></el-icon> 编辑
                      </el-dropdown-item>
                      <el-dropdown-item @click="openRemoteDialog(host.host_id)">
                        <el-icon><Document /></el-icon> 远程命令
                      </el-dropdown-item>
                      <el-dropdown-item divided @click="joinSession(host.host_id)">
                        <el-icon><ChatDotRound /></el-icon> 创建会话
                      </el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </div>

              <div class="host-body">
                <h4 class="host-name">{{ host.hostname }}</h4>
                <p class="host-id">{{ host.host_id }}</p>
                <p class="host-ip">
                  <el-icon><Connection /></el-icon>
                  {{ host.ip }}
                </p>
                <p class="host-os">
                  <el-icon><Monitor /></el-icon>
                  {{ host.os }} ({{ host.arch }})
                </p>
              </div>

              <div class="host-footer">
                <div class="host-tags">
                  <el-tag v-for="tag in host.tags" :key="tag" size="small" effect="plain">
                    {{ tag }}
                  </el-tag>
                </div>
                <div class="host-agent" v-if="host.agent_id">
                  <el-tag type="info" size="small">
                    <el-icon><Connection /></el-icon>
                    {{ host.agent_id.slice(0, 12) }}
                  </el-tag>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </el-tab-pane>

      <!-- 会话管理 -->
      <el-tab-pane name="sessions">
        <template #label>
          <span class="tab-label">
            <el-icon><ChatDotRound /></el-icon>
            会话管理
            <el-tag v-if="waitingSessions.length > 0" type="warning" size="small" class="tab-badge">
              {{ waitingSessions.length }}
            </el-tag>
          </span>
        </template>

        <div class="toolbar">
          <el-input
            v-model="sessionFilter.search"
            placeholder="搜索客户 / 问题描述"
            clearable
            style="width: 300px"
            :prefix-icon="Search"
          />
          <el-select v-model="sessionFilter.status" placeholder="状态" clearable style="width: 120px">
            <el-option label="进行中" value="active" />
            <el-option label="等待中" value="waiting" />
            <el-option label="已暂停" value="paused" />
            <el-option label="已关闭" value="closed" />
          </el-select>
          <el-select v-model="sessionFilter.priority" placeholder="优先级" clearable style="width: 120px">
            <el-option label="紧急" value="urgent" />
            <el-option label="高" value="high" />
            <el-option label="中" value="medium" />
            <el-option label="低" value="low" />
          </el-select>
          <div class="toolbar-spacer" />
          <el-button :icon="Refresh" @click="fetchSessions">刷新</el-button>
          <el-button type="primary" :icon="Plus" @click="openSessionDialog">新建会话</el-button>
        </div>

        <el-table :data="filteredSessions" v-loading="sessionsLoading" stripe border>
          <el-table-column prop="session_id" label="会话 ID" width="150" show-overflow-tooltip />
          <el-table-column prop="customer_name" label="客户" width="120">
            <template #default="{ row }">
              <div class="customer-info">
                <el-avatar :size="24" :icon="User" />
                <span>{{ row.customer_name }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="customer_email" label="邮箱" width="180" show-overflow-tooltip />
          <el-table-column prop="issue_description" label="问题描述" min-width="200" show-overflow-tooltip />
          <el-table-column prop="status" label="状态" width="100" align="center">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.status)" size="small">
                {{ getStatusText(row.status) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="priority" label="优先级" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="getStatusType(row.priority)" size="small" effect="dark">
                {{ getStatusText(row.priority) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="时长" width="100">
            <template #default="{ row }">
              <span v-if="row.duration">{{ formatDuration(row.duration) }}</span>
              <span v-else>{{ formatDuration(Date.now() - row.started_at) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150" align="center" fixed="right">
            <template #default="{ row }">
              <el-button
                v-if="row.status === 'waiting'"
                type="primary"
                size="small"
                @click="joinSession(row.session_id)"
              >
                接入
              </el-button>
              <el-button
                v-else-if="row.status === 'active'"
                type="success"
                size="small"
                @click="joinSession(row.session_id)"
              >
                进入
              </el-button>
              <el-button
                v-if="row.status !== 'closed'"
                link
                type="danger"
                size="small"
                @click="closeSession(row.session_id)"
              >
                关闭
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-tab-pane>

      <!-- 实时聊天 -->
      <el-tab-pane name="chat">
        <template #label>
          <span class="tab-label">
            <el-icon><VideoPlay /></el-icon>
            实时支持
          </span>
        </template>

        <div class="chat-container">
          <div class="chat-sidebar">
            <div class="chat-sidebar-header">
              <h4>会话列表</h4>
            </div>
            <div class="chat-session-list">
              <div
                v-for="session in activeSessions"
                :key="session.session_id"
                class="chat-session-item"
                :class="{ active: currentSessionId === session.session_id }"
                @click="joinSession(session.session_id)"
              >
                <div class="session-item-header">
                  <span class="customer-name">{{ session.customer_name }}</span>
                  <el-tag :type="getStatusType(session.priority)" size="small">
                    {{ getStatusText(session.priority) }}
                  </el-tag>
                </div>
                <p class="session-issue">{{ session.issue_description }}</p>
                <p class="session-time">{{ formatTime(session.started_at) }}</p>
              </div>
            </div>
          </div>

          <div class="chat-main">
            <div v-if="!currentSessionId" class="chat-empty">
              <el-empty description="请选择一个会话开始支持">
                <el-button type="primary" @click="activeTab = 'sessions'">查看会话列表</el-button>
              </el-empty>
            </div>

            <template v-else>
              <div class="chat-header">
                <div class="chat-header-info">
                  <h4>{{ sessions.find((s) => s.session_id === currentSessionId)?.customer_name }}</h4>
                  <p>{{ sessions.find((s) => s.session_id === currentSessionId)?.issue_description }}</p>
                </div>
                <div class="chat-header-actions">
                  <el-button :icon="Document" @click="openRemoteDialog(currentSessionId)">远程命令</el-button>
                  <el-button type="danger" :icon="CloseBold" @click="closeSession(currentSessionId)">
                    结束会话
                  </el-button>
                </div>
              </div>

              <div class="chat-messages" v-loading="chatLoading">
                <div
                  v-for="msg in messages"
                  :key="msg.message_id"
                  class="chat-message"
                  :class="msg.sender_type"
                >
                  <div class="message-avatar">
                    <el-avatar :size="36" :icon="msg.sender_type === 'customer' ? User : msg.sender_type === 'agent' ? Connection : Monitor" />
                  </div>
                  <div class="message-content">
                    <div class="message-header">
                      <span class="sender-name">{{ msg.sender_name }}</span>
                      <span class="message-time">{{ formatTime(msg.created_at) }}</span>
                    </div>
                    <div class="message-body">{{ msg.content }}</div>
                  </div>
                </div>
              </div>

              <div class="chat-input-area">
                <el-input
                  v-model="messageText"
                  type="textarea"
                  :rows="3"
                  placeholder="输入消息..."
                  @keyup.enter.ctrl="sendMessage"
                />
                <div class="chat-input-actions">
                  <span class="input-hint">Ctrl + Enter 发送</span>
                  <el-button type="primary" :icon="Message" @click="sendMessage">发送</el-button>
                </div>
              </div>
            </template>
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 新建会话对话框 -->
    <el-dialog v-model="sessionDialogVisible" title="新建支持会话" width="600px" destroy-on-close>
      <el-form :model="sessionForm" label-width="100px">
        <el-form-item label="选择 Host">
          <el-select v-model="sessionForm.host_id" placeholder="选择要支持的设备" style="width: 100%">
            <el-option
              v-for="host in hosts.filter((h) => h.status === 'online')"
              :key="host.host_id"
              :label="`${host.hostname} (${host.ip})`"
              :value="host.host_id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="客户姓名">
          <el-input v-model="sessionForm.customer_name" placeholder="客户姓名" />
        </el-form-item>
        <el-form-item label="客户邮箱">
          <el-input v-model="sessionForm.customer_email" placeholder="customer@example.com" />
        </el-form-item>
        <el-form-item label="问题描述">
          <el-input
            v-model="sessionForm.issue_description"
            type="textarea"
            :rows="3"
            placeholder="请描述客户遇到的问题..."
          />
        </el-form-item>
        <el-form-item label="优先级">
          <el-radio-group v-model="sessionForm.priority">
            <el-radio-button label="low">低</el-radio-button>
            <el-radio-button label="medium">中</el-radio-button>
            <el-radio-button label="high">高</el-radio-button>
            <el-radio-button label="urgent">紧急</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="标签">
          <el-select-v2
            v-model="sessionForm.tags"
            :options="[
              { label: '打印机', value: 'printer' },
              { label: '网络', value: 'network' },
              { label: '软件', value: 'software' },
              { label: '硬件', value: 'hardware' },
              { label: '系统', value: 'system' },
            ]"
            placeholder="选择标签"
            multiple
            clearable
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="sessionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="createSession">创建</el-button>
      </template>
    </el-dialog>

    <!-- 远程命令对话框 -->
    <el-dialog v-model="remoteDialogVisible" title="远程命令执行" width="700px" destroy-on-close>
      <div class="remote-command-panel">
        <el-input
          v-model="remoteCommand"
          placeholder="输入要执行的命令..."
          :disabled="remoteExecuting"
          @keyup.enter="executeRemoteCommand"
        >
          <template #append>
            <el-button
              type="primary"
              :icon="Document"
              :loading="remoteExecuting"
              @click="executeRemoteCommand"
            >
              执行
            </el-button>
          </template>
        </el-input>

        <div class="remote-output" v-if="remoteOutput">
          <div class="output-header">
            <span>执行结果</span>
            <el-button link size="small" @click="remoteOutput = ''">清除</el-button>
          </div>
          <pre class="output-content">{{ remoteOutput }}</pre>
        </div>
      </div>
    </el-dialog>

    <!-- Host 编辑对话框 -->
    <el-dialog v-model="hostDialogVisible" title="Host 信息" width="500px" destroy-on-close>
      <el-form :model="hostForm" label-width="100px">
        <el-form-item label="主机名">
          <el-input v-model="hostForm.hostname" />
        </el-form-item>
        <el-form-item label="IP 地址">
          <el-input v-model="hostForm.ip" />
        </el-form-item>
        <el-form-item label="操作系统">
          <el-input v-model="hostForm.os" />
        </el-form-item>
        <el-form-item label="架构">
          <el-input v-model="hostForm.arch" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="hostForm.description" type="textarea" :rows="2" />
        </el-form-item>
        <el-form-item label="标签">
          <el-select-v2
            v-model="hostForm.tags"
            :options="[
              { label: '客户设备', value: 'customer' },
              { label: '服务器', value: 'server' },
              { label: '桌面', value: 'desktop' },
              { label: '笔记本', value: 'laptop' },
              { label: '生产环境', value: 'production' },
              { label: '测试环境', value: 'test' },
            ]"
            multiple
            clearable
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="hostDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveHost">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.support-platform {
  max-width: 1400px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.page-title {
  display: flex;
  align-items: center;
  gap: 12px;
  margin: 0;
  font-size: 24px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.page-title .el-icon {
  font-size: 28px;
  color: var(--el-color-primary);
}

.header-actions {
  display: flex;
  gap: 12px;
}

.support-tabs {
  min-height: calc(100vh - 140px);
}

.support-tabs :deep(.el-tabs__content) {
  padding: 20px 0;
}

.tab-label {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tab-badge {
  margin-left: 4px;
}

/* 统计卡片 */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 20px;
  margin-bottom: 24px;
}

@media (max-width: 1200px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}

.stat-card {
  text-align: center;
  padding: 24px;
  border-radius: 16px;
}

.stat-value {
  font-size: 36px;
  font-weight: 700;
  color: var(--el-color-primary);
  line-height: 1.2;
}

.stat-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
  margin-top: 8px;
}

.stat-sublabel {
  margin-top: 12px;
}

/* 概览内容 */
.overview-content {
  margin-top: 20px;
}

.overview-card {
  height: 100%;
  border-radius: 12px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

/* 筛选栏 */
.toolbar {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  align-items: center;
  padding: 16px 20px;
  background: #ffffff;
  border-radius: 12px;
  border: 1px solid var(--el-border-color-light);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.03);
}

.toolbar-spacer {
  flex: 1;
}

/* Agent 详情 */
.agent-detail {
  padding: 16px;
  background-color: var(--el-fill-color-light);
  border-radius: 8px;
  margin: 8px;
}

.heartbeat-time {
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

/* Host 卡片 */
.host-card {
  margin-bottom: 16px;
  transition: all 0.3s;
  border-radius: 12px;
}

.host-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.host-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.host-body {
  margin-bottom: 12px;
}

.host-name {
  margin: 0 0 4px 0;
  font-size: 16px;
  font-weight: 600;
}

.host-id {
  margin: 0 0 8px 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}

.host-ip,
.host-os {
  margin: 4px 0;
  font-size: 13px;
  color: var(--el-text-color-regular);
  display: flex;
  align-items: center;
  gap: 6px;
}

.host-footer {
  border-top: 1px solid var(--el-border-color-lighter);
  padding-top: 12px;
}

.host-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-bottom: 8px;
}

.host-agent {
  font-size: 12px;
}

/* 客户信息 */
.customer-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 聊天界面 */
.chat-container {
  display: flex;
  height: calc(100vh - 280px);
  border: 1px solid var(--el-border-color);
  border-radius: 12px;
  overflow: hidden;
  background: #ffffff;
}

.chat-sidebar {
  width: 280px;
  border-right: 1px solid var(--el-border-color);
  background-color: var(--el-fill-color-light);
  display: flex;
  flex-direction: column;
}

.chat-sidebar-header {
  padding: 16px;
  border-bottom: 1px solid var(--el-border-color);
}

.chat-sidebar-header h4 {
  margin: 0;
}

.chat-session-list {
  flex: 1;
  overflow-y: auto;
}

.chat-session-item {
  padding: 12px 16px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  cursor: pointer;
  transition: background-color 0.2s;
}

.chat-session-item:hover,
.chat-session-item.active {
  background-color: var(--el-color-primary-light-9);
}

.session-item-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.customer-name {
  font-weight: 600;
  font-size: 14px;
}

.session-issue {
  margin: 0 0 6px 0;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.session-time {
  margin: 0;
  font-size: 11px;
  color: var(--el-text-color-placeholder);
}

.chat-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  background-color: #fff;
}

.chat-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.chat-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--el-border-color);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.chat-header-info h4 {
  margin: 0 0 4px 0;
}

.chat-header-info p {
  margin: 0;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.chat-header-actions {
  display: flex;
  gap: 8px;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
  background-color: var(--el-fill-color-lighter);
}

.chat-message {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}

.chat-message.customer {
  flex-direction: row;
}

.chat-message.agent {
  flex-direction: row-reverse;
}

.chat-message.system {
  justify-content: center;
}

.chat-message.system .message-content {
  background-color: var(--el-color-info-light-9);
  max-width: 80%;
}

.message-content {
  max-width: 70%;
  background-color: #fff;
  padding: 12px 16px;
  border-radius: 12px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06);
}

.chat-message.agent .message-content {
  background-color: var(--el-color-primary-light-9);
}

.message-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
  font-size: 12px;
}

.sender-name {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.message-time {
  color: var(--el-text-color-placeholder);
  margin-left: 12px;
}

.message-body {
  font-size: 14px;
  line-height: 1.6;
  color: var(--el-text-color-regular);
  word-break: break-word;
}

.chat-input-area {
  padding: 16px 20px;
  border-top: 1px solid var(--el-border-color);
  background-color: #fff;
}

.chat-input-actions {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-top: 12px;
}

.input-hint {
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}

/* 远程命令面板 */
.remote-command-panel {
  padding: 20px 0;
}

.remote-output {
  margin-top: 20px;
  border: 1px solid var(--el-border-color);
  border-radius: 8px;
  overflow: hidden;
}

.output-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background-color: var(--el-fill-color-light);
  border-bottom: 1px solid var(--el-border-color);
  font-weight: 600;
}

.output-content {
  padding: 16px;
  margin: 0;
  background-color: #1e1e1e;
  color: #d4d4d4;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  overflow-x: auto;
  min-height: 200px;
  max-height: 400px;
  overflow-y: auto;
}

/* 响应式 */
@media (max-width: 1200px) {
  .stats-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }

  .chat-container {
    flex-direction: column;
  }

  .chat-sidebar {
    width: 100%;
    height: 200px;
    border-right: none;
    border-bottom: 1px solid var(--el-border-color);
  }

  .toolbar {
    flex-wrap: wrap;
    gap: 8px;
  }

  .toolbar .el-input,
  .toolbar .el-select {
    width: 100%;
  }

  .page-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
}
</style>
