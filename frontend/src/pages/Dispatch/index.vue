<script setup lang="ts">
import { ref, reactive, computed, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Promotion } from '@element-plus/icons-vue'
import client from '../../api/client'
import type { Envelope, DebugAgentItem, TaskResult, TaskCreateResp, TaskBatchCreateResp } from '../../api/types'
import StatusTag from '../../components/StatusTag.vue'
import OutputViewer from '../../components/OutputViewer.vue'

const router = useRouter()

// ── Agent list ──
const agents = ref<DebugAgentItem[]>([])
const agentsLoading = ref(false)

async function fetchAgents() {
  agentsLoading.value = true
  try {
    const resp = await client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents')
    agents.value = resp.data.data ?? []
  } catch { /* interceptor handles error */ } finally {
    agentsLoading.value = false
  }
}
fetchAgents()

// Agent 按状态分组
const onlineAgents = computed(() => agents.value.filter(a => a.status === 'online'))
const offlineAgents = computed(() => agents.value.filter(a => a.status !== 'online'))

// ── 标签快选 ──
interface TagGroup {
  label: string
  agentIds: string[]
}

// Agent Labels 分组: key=value → agentIds
const labelGroups = computed<TagGroup[]>(() => {
  const map = new Map<string, string[]>()
  for (const a of onlineAgents.value) {
    if (!a.labels) continue
    for (const [k, v] of Object.entries(a.labels)) {
      const label = `${k}=${v}`
      const ids = map.get(label) || []
      ids.push(a.agent_id)
      map.set(label, ids)
    }
  }
  return Array.from(map.entries()).map(([label, agentIds]) => ({ label, agentIds }))
})

// Host Tags 分组: tag → agentIds
const hostTagGroups = computed<TagGroup[]>(() => {
  const map = new Map<string, string[]>()
  for (const a of onlineAgents.value) {
    if (!a.host_tags) continue
    for (const tag of a.host_tags) {
      const ids = map.get(tag) || []
      ids.push(a.agent_id)
      map.set(tag, ids)
    }
  }
  return Array.from(map.entries()).map(([label, agentIds]) => ({ label, agentIds }))
})

// 当前选中的标签
const selectedLabels = ref<Set<string>>(new Set())
const selectedHostTags = ref<Set<string>>(new Set())

function toggleLabel(group: TagGroup) {
  const sel = selectedLabels.value
  if (sel.has(group.label)) {
    sel.delete(group.label)
    // 移除该标签独有的 agentIds（不被其他选中标签覆盖的）
    const otherIds = new Set<string>()
    for (const l of sel) {
      const g = labelGroups.value.find(x => x.label === l)
      if (g) g.agentIds.forEach(id => otherIds.add(id))
    }
    for (const l of selectedHostTags.value) {
      const g = hostTagGroups.value.find(x => x.label === l)
      if (g) g.agentIds.forEach(id => otherIds.add(id))
    }
    v2Form.target_agent_ids = v2Form.target_agent_ids.filter(id => otherIds.has(id))
  } else {
    sel.add(group.label)
    // 合并该标签的 agentIds
    const current = new Set(v2Form.target_agent_ids)
    group.agentIds.forEach(id => current.add(id))
    v2Form.target_agent_ids = Array.from(current)
  }
}

function toggleHostTag(group: TagGroup) {
  const sel = selectedHostTags.value
  if (sel.has(group.label)) {
    sel.delete(group.label)
    const otherIds = new Set<string>()
    for (const l of selectedLabels.value) {
      const g = labelGroups.value.find(x => x.label === l)
      if (g) g.agentIds.forEach(id => otherIds.add(id))
    }
    for (const l of sel) {
      const g = hostTagGroups.value.find(x => x.label === l)
      if (g) g.agentIds.forEach(id => otherIds.add(id))
    }
    v2Form.target_agent_ids = v2Form.target_agent_ids.filter(id => otherIds.has(id))
  } else {
    sel.add(group.label)
    const current = new Set(v2Form.target_agent_ids)
    group.agentIds.forEach(id => current.add(id))
    v2Form.target_agent_ids = Array.from(current)
  }
}

function agentLabel(a: DebugAgentItem): string {
  const parts = [a.agent_id]
  if (a.hostname) parts.push(a.hostname)
  else if (a.ip) parts.push(a.ip)
  return parts.join(' / ')
}

// ── 当前 Tab ──
const activeTab = ref('v2')

// ============================================================
// Phase 2: 任务提交
// ============================================================
const v2Form = reactive({
  task_type: 'shell',
  command: '',
  timeout: 30,
  exec_mode: 'shared' as 'shared' | 'exclusive',
  priority: 50,
  preemptible: false,
  max_attempts: 3,
  target_agent_ids: [] as string[],
})
const v2Sending = ref(false)
const v2Results = ref<TaskCreateResp[]>([])

// 常用命令预设
const presets = [
  { label: '系统信息', icon: '💻', type: 'shell', cmd: 'uname -a && hostname && uptime' },
  { label: 'CPU 状态', icon: '⚡', type: 'shell', cmd: 'top -bn1 | head -20' },
  { label: '内存', icon: '📊', type: 'shell', cmd: 'free -h' },
  { label: '磁盘', icon: '💾', type: 'shell', cmd: 'df -h' },
  { label: '进程 Top10', icon: '📋', type: 'shell', cmd: 'ps aux --sort=-%cpu | head -11' },
  { label: '网络连接', icon: '🌐', type: 'shell', cmd: 'ss -tunlp' },
  { label: 'IP 地址', icon: '🔗', type: 'shell', cmd: 'ip addr' },
  { label: '登录记录', icon: '🔒', type: 'shell', cmd: 'lastb | head -30' },
  { label: 'GPU 状态', icon: '🎮', type: 'shell', cmd: 'nvidia-smi' },
  { label: 'Docker 容器', icon: '🐳', type: 'shell', cmd: 'docker ps --format "table {{.Names}}\\t{{.Status}}\\t{{.Ports}}"' },
  { label: 'Python 版本', icon: '🐍', type: 'shell', cmd: 'python3 --version && pip3 --version' },
  { label: '系统采集', icon: '📈', type: 'python', cmd: 'import json, os, socket, platform\ninfo = {\n  "hostname": socket.gethostname(),\n  "cpu_count": os.cpu_count(),\n  "platform": platform.platform(),\n  "python": platform.python_version(),\n}\nprint(json.dumps(info, indent=2))' },
]

function applyPreset(p: typeof presets[0]) {
  v2Form.task_type = p.type
  v2Form.command = p.cmd
}

async function submitV2Task() {
  if (!v2Form.command.trim()) {
    ElMessage.warning('请输入命令')
    return
  }
  v2Sending.value = true
  v2Results.value = []
  try {
    const targets = v2Form.target_agent_ids.length > 0 ? v2Form.target_agent_ids : ['']
    const baseTask = {
      task_type: v2Form.task_type,
      payload: { command: v2Form.command, timeout: v2Form.timeout },
      exec_mode: v2Form.exec_mode,
      priority: v2Form.priority,
      preemptible: v2Form.preemptible,
      max_attempts: v2Form.max_attempts,
    }

    let created: TaskCreateResp[]

    if (targets.length === 1 && !targets[0]) {
      // 不指定 agent，单条创建
      const resp = await client.post<Envelope<TaskCreateResp>>('/api/v1/tasks', baseTask)
      created = [resp.data.data]
    } else if (targets.length === 1) {
      // 单个 agent，单条创建
      const resp = await client.post<Envelope<TaskCreateResp>>('/api/v1/tasks', {
        ...baseTask,
        schedule: { target_agent_id: targets[0] },
      })
      created = [resp.data.data]
    } else {
      // 多个 agent，批量创建
      const tasks = targets.map(agentId => ({
        ...baseTask,
        schedule: { target_agent_id: agentId },
      }))
      const resp = await client.post<Envelope<TaskBatchCreateResp>>('/api/v1/tasks/batch', { tasks })
      created = resp.data.data.tasks
    }

    v2Results.value = created
    if (created.length === 1) {
      ElMessage.success(`任务已创建: ${created[0].task_id}`)
      router.push(`/tasks/${created[0].task_id}`)
    } else {
      ElMessage.success(`已创建 ${created.length} 个任务`)
    }
  } catch { /* interceptor handles error */ } finally {
    v2Sending.value = false
  }
}


// ============================================================
// Phase 1: Debug 任务分发（保留）
// ============================================================
function genTaskId() {
  return `task-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

const form = reactive({
  agentId: '' as string,
  taskId: genTaskId(),
  command: '',
  timeout: 30,
})

function resetTaskId() {
  form.taskId = genTaskId()
}

const sending = ref(false)
const results = reactive<Map<string, TaskResult>>(new Map())
const pollingTimers: ReturnType<typeof setInterval>[] = []

function clearTimers() {
  pollingTimers.forEach(clearInterval)
  pollingTimers.length = 0
}
onUnmounted(clearTimers)

async function sendTask() {
  if (!form.command.trim()) {
    ElMessage.warning('请输入命令')
    return
  }
  const targetAgents = form.agentId
    ? [form.agentId]
    : agents.value.filter((a) => a.status === 'online').map((a) => a.agent_id)

  if (targetAgents.length === 0) {
    ElMessage.warning('没有可用的 Agent')
    return
  }

  sending.value = true
  results.clear()
  clearTimers()

  const baseTaskId = form.taskId
  const taskIds: { agentId: string; taskId: string }[] = []

  for (const agentId of targetAgents) {
    const taskId = targetAgents.length === 1 ? baseTaskId : `${baseTaskId}-${agentId}`
    taskIds.push({ agentId, taskId })
    try {
      await client.post('/api/v1/debug/dispatch/task', {
        agent_id: agentId,
        task_id: taskId,
        command: form.command,
        timeout: form.timeout,
      })
      results.set(taskId, {
        task_id: taskId, agent_id: agentId, status: 'pending',
        exit_code: 0, stdout: '', stderr: '', truncated: false,
        started_at: null, finished_at: null,
      })
    } catch {
      results.set(taskId, {
        task_id: taskId, agent_id: agentId, status: 'failed',
        exit_code: -1, stdout: '', stderr: 'Failed to dispatch task',
        truncated: false, started_at: null, finished_at: null,
      })
    }
  }

  sending.value = false
  resetTaskId()

  for (const { taskId } of taskIds) {
    const current = results.get(taskId)
    if (current && current.status === 'failed') continue
    startPolling(taskId)
  }
}

function startPolling(taskId: string) {
  const timer = setInterval(async () => {
    try {
      const resp = await client.get<Envelope<TaskResult>>(`/api/v1/debug/task/${taskId}`)
      const data = resp.data.data
      if (data) {
        results.set(taskId, data)
        if (data.status !== 'pending' && data.status !== 'running') {
          clearInterval(timer)
        }
      }
    } catch {
      clearInterval(timer)
    }
  }, 2000)
  pollingTimers.push(timer)
}

// ── Control panel ──
const controlForm = reactive({ agentId: '', action: '' })
const controlActions = [
  { label: 'Refresh Token', value: 'refresh_token' },
  { label: 'Shutdown', value: 'shutdown' },
  { label: 'Reload Config', value: 'reload_config' },
  { label: 'Cancel Task', value: 'cancel_task' },
  { label: 'Cancel', value: 'cancel' },
]
const controlSending = ref(false)

async function sendControl() {
  if (!controlForm.agentId || !controlForm.action) {
    ElMessage.warning('请选择 Agent 和 Action')
    return
  }
  controlSending.value = true
  try {
    await client.post('/api/v1/debug/dispatch/control', {
      agent_id: controlForm.agentId,
      action: controlForm.action,
    })
    ElMessage.success('控制指令已发送')
  } catch { /* interceptor handles error */ } finally {
    controlSending.value = false
  }
}
</script>

<template>
  <div style="max-width: 1000px">
    <h2 class="page-title">
      <el-icon size="28"><Promotion /></el-icon>
      任务分发
    </h2>

    <el-tabs v-model="activeTab" type="border-card" class="dispatch-tabs">
      <!-- Phase 2 任务提交 -->
      <el-tab-pane label="任务提交" name="v2">
        <el-form label-width="100px" @submit.prevent="submitV2Task" style="margin-top: 8px">
          <el-form-item label="任务类型">
            <el-radio-group v-model="v2Form.task_type">
              <el-radio-button value="shell">Shell</el-radio-button>
              <el-radio-button value="python">Python</el-radio-button>
            </el-radio-group>
            <div style="font-size: 12px; color: var(--el-text-color-secondary); margin-top: 8px">
              <template v-if="v2Form.task_type === 'shell'">通过 <code>sh -c</code> 执行 Shell 命令</template>
              <template v-else>
                <p style="margin: 0">通过 <code>python3 -c</code> 执行 Python 代码，需目标机器已安装 python3</p>
                <p style="margin: 4px 0 0">如需安装依赖包，可在代码中内联安装：<code>subprocess.check_call(['pip3', 'install', '-q', '包名'])</code></p>
              </template>
            </div>
          </el-form-item>

          <el-form-item label="常用命令">
            <div class="preset-buttons">
              <el-button
                v-for="p in presets"
                :key="p.label"
                size="small"
                plain
                @click="applyPreset(p)"
              >{{ p.icon }} {{ p.label }}</el-button>
            </div>
          </el-form-item>

          <el-form-item label="命令">
            <el-input
              v-model="v2Form.command"
              type="textarea"
              :rows="6"
              :placeholder="v2Form.task_type === 'python'
                ? 'import os\nprint(os.uname())'
                : 'hostname && date'"
            />
          </el-form-item>

          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="执行模式">
                <el-radio-group v-model="v2Form.exec_mode">
                  <el-radio value="shared">共享 (shared)</el-radio>
                  <el-radio value="exclusive">独占 (exclusive)</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-col>
            <el-col :span="12">
              <el-form-item label="可抢占">
                <el-switch v-model="v2Form.preemptible" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-row :gutter="20">
            <el-col :span="12">
              <el-form-item label="优先级">
                <el-slider v-model="v2Form.priority" :min="1" :max="100" show-input />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="超时">
                <el-input-number v-model="v2Form.timeout" :min="1" :max="3600" style="width: 100%" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="最大重试">
                <el-input-number v-model="v2Form.max_attempts" :min="1" :max="10" style="width: 100%" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item v-if="labelGroups.length > 0 || hostTagGroups.length > 0" label="标签快选">
            <div class="tag-quick-select">
              <div v-if="labelGroups.length > 0" class="tag-group-row">
                <span class="tag-group-label">Agent Labels:</span>
                <el-check-tag
                  v-for="g in labelGroups"
                  :key="'l-' + g.label"
                  :checked="selectedLabels.has(g.label)"
                  @change="toggleLabel(g)"
                >{{ g.label }} ({{ g.agentIds.length }})</el-check-tag>
              </div>
              <div v-if="hostTagGroups.length > 0" class="tag-group-row">
                <span class="tag-group-label">Host Tags:</span>
                <el-check-tag
                  v-for="g in hostTagGroups"
                  :key="'h-' + g.label"
                  :checked="selectedHostTags.has(g.label)"
                  @change="toggleHostTag(g)"
                >{{ g.label }} ({{ g.agentIds.length }})</el-check-tag>
              </div>
            </div>
          </el-form-item>

          <el-form-item label="目标 Agent">
            <el-select
              v-model="v2Form.target_agent_ids"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              placeholder="自动调度（不指定）"
              clearable
              style="width: 100%"
              @clear="selectedLabels.clear(); selectedHostTags.clear()"
            >
              <el-option-group label="在线">
                <el-option
                  v-for="a in onlineAgents"
                  :key="a.agent_id"
                  :label="agentLabel(a)"
                  :value="a.agent_id"
                />
              </el-option-group>
              <el-option-group v-if="offlineAgents.length > 0" label="离线">
                <el-option
                  v-for="a in offlineAgents"
                  :key="a.agent_id"
                  :label="agentLabel(a)"
                  :value="a.agent_id"
                  disabled
                />
              </el-option-group>
            </el-select>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="v2Sending" @click="submitV2Task" size="large">
              <el-icon style="margin-right: 6px"><Promotion /></el-icon>
              提交任务
            </el-button>
          </el-form-item>
        </el-form>

        <!-- V2 Results -->
        <template v-if="v2Results.length > 0">
          <el-divider />
          <div class="result-list">
            <div v-for="r in v2Results" :key="r.task_id" class="result-item">
              <el-link type="primary" underline="never" style="font-weight: 600" @click="router.push(`/tasks/${r.task_id}`)">
                {{ r.task_id }}
              </el-link>
              <StatusTag :status="r.status" />
              <el-button size="small" text type="primary" @click="router.push(`/tasks/${r.task_id}`)">查看详情 →</el-button>
            </div>
          </div>
        </template>
      </el-tab-pane>

      <!-- Phase 1 Debug 分发 -->
      <el-tab-pane label="Debug 分发" name="debug">
        <el-form label-width="100px" @submit.prevent="sendTask" style="margin-top: 8px">
          <el-form-item label="目标 Agent">
            <el-select v-model="form.agentId" placeholder="全部 Agent" clearable :loading="agentsLoading" style="width: 100%">
              <el-option label="全部在线 Agent" value="" />
              <el-option
                v-for="a in agents"
                :key="a.agent_id"
                :label="`${a.agent_id.slice(0, 20)} (${a.hostname || a.ip || '-'})${a.status === 'offline' ? ' [离线]' : ''}`"
                :value="a.agent_id"
                :disabled="a.status === 'offline'"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="任务 ID">
            <el-input v-model="form.taskId">
              <template #append>
                <el-button @click="resetTaskId">重新生成</el-button>
              </template>
            </el-input>
          </el-form-item>

          <el-form-item label="命令">
            <el-input v-model="form.command" type="textarea" :rows="4" placeholder="输入要执行的命令..." />
          </el-form-item>

          <el-form-item label="超时时间">
            <el-input-number v-model="form.timeout" :min="1" :max="3600" />
            <span style="margin-left: 8px; color: var(--el-text-color-secondary)">秒</span>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="sending" @click="sendTask">发送任务</el-button>
          </el-form-item>
        </el-form>

        <!-- Results -->
        <template v-if="results.size > 0">
          <el-divider />
          <div v-for="[taskId, r] in results" :key="taskId" class="result-panel">
            <div class="result-header">
              <span style="font-weight: 600">{{ r.agent_id }}</span>
              <StatusTag :status="r.status ?? ''" />
              <span v-if="r.exit_code !== 0" style="color: #ff4d4f; font-size: 13px">exit_code: {{ r.exit_code }}</span>
              <span style="color: var(--el-text-color-secondary); font-size: 12px; margin-left: auto">{{ taskId }}</span>
            </div>
            <div v-if="r.stdout" style="margin-bottom: 12px">
              <div class="output-label">stdout</div>
              <OutputViewer :content="r.stdout" label="stdout" :truncated="r.truncated" :filename="`${taskId}-stdout.txt`" />
            </div>
            <div v-if="r.stderr">
              <div class="output-label">stderr</div>
              <OutputViewer :content="r.stderr" label="stderr" :truncated="r.truncated" :filename="`${taskId}-stderr.txt`" />
            </div>
            <div v-if="!r.stdout && !r.stderr && r.status !== 'pending' && r.status !== 'running'" style="color: var(--el-text-color-secondary); font-size: 13px; padding: 12px 0">
              (无输出)
            </div>
          </div>
        </template>
      </el-tab-pane>

      <!-- 控制指令 -->
      <el-tab-pane label="控制指令" name="control">
        <el-form label-width="100px" @submit.prevent="sendControl" style="margin-top: 8px">
          <el-form-item label="目标 Agent">
            <el-select v-model="controlForm.agentId" placeholder="选择 Agent" style="width: 100%">
              <el-option
                v-for="a in agents"
                :key="a.agent_id"
                :label="`${a.agent_id.slice(0, 20)} (${a.hostname || a.ip || '-'})${a.status === 'offline' ? ' [离线]' : ''}`"
                :value="a.agent_id"
                :disabled="a.status === 'offline'"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="Action">
            <el-select v-model="controlForm.action" placeholder="选择操作" style="width: 100%">
              <el-option v-for="act in controlActions" :key="act.value" :label="act.label" :value="act.value" />
            </el-select>
          </el-form-item>

          <el-form-item>
            <el-button type="warning" :loading="controlSending" @click="sendControl">发送控制指令</el-button>
          </el-form-item>
        </el-form>
      </el-tab-pane>
    </el-tabs>
  </div>
</template>

<style scoped>
.dispatch-tabs :deep(.el-tabs__content) {
  padding: 24px;
}

.preset-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.result-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
}

.result-panel {
  margin-bottom: 24px;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  border: 1px solid var(--el-border-color-light);
}

.result-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.output-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
  font-weight: 500;
}

.tag-quick-select {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.tag-group-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.tag-group-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  white-space: nowrap;
  min-width: 80px;
}
</style>
