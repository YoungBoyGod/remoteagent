<script setup lang="ts">
import { ref, reactive, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import client from '../../api/client'
import type { Envelope, DebugAgentItem, TaskResult, TaskCreateResp } from '../../api/types'
import StatusTag from '../../components/StatusTag.vue'
import OutputViewer from '../../components/OutputViewer.vue'

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

// ── 当前 Tab ──
const activeTab = ref('v2')

// ============================================================
// Phase 2: 任务提交
// ============================================================
const v2Form = reactive({
  task_type: 'shell',
  command: '',
  args: '',
  workdir: '',
  timeout: 30,
  exec_mode: 'shared' as 'shared' | 'exclusive',
  priority: 50,
  preemptible: false,
  max_attempts: 3,
  target_agent_id: '',
  env_text: '',
})
const v2Sending = ref(false)
const v2Result = ref<TaskCreateResp | null>(null)

function parseEnv(text: string): Record<string, string> {
  const env: Record<string, string> = {}
  for (const line of text.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed || trimmed.startsWith('#')) continue
    const idx = trimmed.indexOf('=')
    if (idx > 0) {
      env[trimmed.slice(0, idx).trim()] = trimmed.slice(idx + 1).trim()
    }
  }
  return env
}

async function submitV2Task() {
  if (!v2Form.command.trim()) {
    ElMessage.warning('请输入命令')
    return
  }
  v2Sending.value = true
  v2Result.value = null
  try {
    const env = parseEnv(v2Form.env_text)
    const args = v2Form.args.trim() ? v2Form.args.trim().split(/\s+/) : undefined
    const body: Record<string, unknown> = {
      task_type: v2Form.task_type,
      payload: {
        command: v2Form.command,
        args,
        env: Object.keys(env).length > 0 ? env : undefined,
        workdir: v2Form.workdir || undefined,
        timeout: v2Form.timeout,
      },
      exec_mode: v2Form.exec_mode,
      priority: v2Form.priority,
      preemptible: v2Form.preemptible,
      max_attempts: v2Form.max_attempts,
    }
    if (v2Form.target_agent_id) {
      body.schedule = { target_agent_id: v2Form.target_agent_id }
    }
    const resp = await client.post<Envelope<TaskCreateResp>>('/api/v1/tasks', body)
    v2Result.value = resp.data.data
    ElMessage.success(`任务已创建: ${resp.data.data.task_id}`)
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
  <div style="max-width: 960px">
    <h2 class="page-title">任务分发</h2>

    <el-tabs v-model="activeTab" type="border-card">
      <!-- Phase 2 任务提交 -->
      <el-tab-pane label="任务提交" name="v2">
        <el-form label-width="100px" @submit.prevent="submitV2Task" style="margin-top: 12px">
          <el-form-item label="任务类型">
            <el-input v-model="v2Form.task_type" placeholder="shell" />
          </el-form-item>

          <el-form-item label="命令">
            <el-input v-model="v2Form.command" type="textarea" :rows="3" placeholder="要执行的命令..." />
          </el-form-item>

          <el-form-item label="参数">
            <el-input v-model="v2Form.args" placeholder="空格分隔的参数（可选）" />
          </el-form-item>

          <el-row :gutter="16">
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

          <el-row :gutter="16">
            <el-col :span="12">
              <el-form-item label="优先级">
                <el-slider v-model="v2Form.priority" :min="1" :max="100" show-input />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="超时">
                <el-input-number v-model="v2Form.timeout" :min="1" :max="3600" />
              </el-form-item>
            </el-col>
            <el-col :span="6">
              <el-form-item label="最大重试">
                <el-input-number v-model="v2Form.max_attempts" :min="1" :max="10" />
              </el-form-item>
            </el-col>
          </el-row>

          <el-form-item label="工作目录">
            <el-input v-model="v2Form.workdir" placeholder="/home/user（可选）" />
          </el-form-item>

          <el-form-item label="目标 Agent">
            <el-select v-model="v2Form.target_agent_id" placeholder="自动调度（不指定）" clearable style="width: 100%">
              <el-option label="自动调度" value="" />
              <el-option
                v-for="a in agents"
                :key="a.agent_id"
                :label="`${a.agent_id} (${a.hostname || a.ip || '-'})${a.status === 'offline' ? ' [离线]' : ''}`"
                :value="a.agent_id"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="环境变量">
            <el-input v-model="v2Form.env_text" type="textarea" :rows="2" placeholder="KEY=VALUE（每行一个，可选）" />
          </el-form-item>

          <el-form-item>
            <el-button type="primary" :loading="v2Sending" @click="submitV2Task">提交任务</el-button>
          </el-form-item>
        </el-form>

        <!-- V2 Result -->
        <el-alert
          v-if="v2Result"
          :title="`任务已创建: ${v2Result.task_id}`"
          type="success"
          :closable="true"
          show-icon
          style="margin-top: 12px"
        >
          <template #default>
            状态: {{ v2Result.status }}
          </template>
        </el-alert>
      </el-tab-pane>

      <!-- Phase 1 Debug 分发 -->
      <el-tab-pane label="Debug 分发" name="debug">
        <el-form label-width="100px" @submit.prevent="sendTask" style="margin-top: 12px">
          <el-form-item label="目标 Agent">
            <el-select v-model="form.agentId" placeholder="全部 Agent" clearable :loading="agentsLoading" style="width: 100%">
              <el-option label="全部在线 Agent" value="" />
              <el-option
                v-for="a in agents"
                :key="a.agent_id"
                :label="`${a.agent_id} (${a.hostname || a.ip || '-'})${a.status === 'offline' ? ' [离线]' : ''}`"
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
          <div v-for="[taskId, r] in results" :key="taskId" style="margin-bottom: 24px">
            <div style="margin-bottom: 8px; display: flex; align-items: center; gap: 12px">
              <span style="font-weight: 600">{{ r.agent_id }}</span>
              <StatusTag :status="r.status ?? ''" />
              <span v-if="r.exit_code !== 0" style="color: #f56c6c; font-size: 13px">exit_code: {{ r.exit_code }}</span>
              <span style="color: var(--el-text-color-secondary); font-size: 12px">{{ taskId }}</span>
            </div>
            <div v-if="r.stdout" style="margin-bottom: 8px">
              <div style="font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 4px">stdout</div>
              <OutputViewer :content="r.stdout" label="stdout" :truncated="r.truncated" :filename="`${taskId}-stdout.txt`" />
            </div>
            <div v-if="r.stderr">
              <div style="font-size: 12px; color: var(--el-text-color-secondary); margin-bottom: 4px">stderr</div>
              <OutputViewer :content="r.stderr" label="stderr" :truncated="r.truncated" :filename="`${taskId}-stderr.txt`" />
            </div>
            <div v-if="!r.stdout && !r.stderr && r.status !== 'pending' && r.status !== 'running'" style="color: var(--el-text-color-secondary); font-size: 13px">
              (无输出)
            </div>
          </div>
        </template>
      </el-tab-pane>

      <!-- 控制指令 -->
      <el-tab-pane label="控制指令" name="control">
        <el-form label-width="100px" @submit.prevent="sendControl" style="margin-top: 12px">
          <el-form-item label="目标 Agent">
            <el-select v-model="controlForm.agentId" placeholder="选择 Agent" style="width: 100%">
              <el-option
                v-for="a in agents"
                :key="a.agent_id"
                :label="`${a.agent_id} (${a.hostname || a.ip || '-'})${a.status === 'offline' ? ' [离线]' : ''}`"
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
