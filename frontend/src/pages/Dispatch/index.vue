<script setup lang="ts">
import { ref, reactive, onUnmounted } from 'vue'
import { ElMessage } from 'element-plus'
import client from '../../api/client'
import type { Envelope, DebugAgentItem, TaskResult } from '../../api/types'
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

// ── Task form ──
function genTaskId() {
  return `task-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
}

const form = reactive({
  agentId: '' as string, // '' means all agents
  taskId: genTaskId(),
  command: '',
  timeout: 30,
})

function resetTaskId() {
  form.taskId = genTaskId()
}

// ── Send task & poll results ──
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
        task_id: taskId,
        agent_id: agentId,
        status: 'pending',
        exit_code: 0,
        stdout: '',
        stderr: '',
        truncated: false,
        started_at: null,
        finished_at: null,
      })
    } catch {
      results.set(taskId, {
        task_id: taskId,
        agent_id: agentId,
        status: 'failed',
        exit_code: -1,
        stdout: '',
        stderr: 'Failed to dispatch task',
        truncated: false,
        started_at: null,
        finished_at: null,
      })
    }
  }

  sending.value = false
  resetTaskId()

  // Start polling for each dispatched task
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
const controlForm = reactive({
  agentId: '',
  action: '',
})
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
    <h2>任务分发</h2>

    <!-- Task dispatch form -->
    <el-card shadow="never" style="margin-bottom: 20px">
      <el-form label-width="100px" @submit.prevent="sendTask">
        <el-form-item label="目标 Agent">
          <el-select
            v-model="form.agentId"
            placeholder="全部 Agent"
            clearable
            :loading="agentsLoading"
            style="width: 100%"
          >
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
          <el-input
            v-model="form.command"
            type="textarea"
            :rows="4"
            placeholder="输入要执行的命令..."
          />
        </el-form-item>

        <el-form-item label="超时时间">
          <el-input-number v-model="form.timeout" :min="1" :max="3600" />
          <span style="margin-left: 8px; color: #909399">秒</span>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="sending" @click="sendTask">
            发送任务
          </el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- Results -->
    <el-card v-if="results.size > 0" shadow="never" style="margin-bottom: 20px">
      <template #header>
        <span>执行结果</span>
      </template>

      <div
        v-for="[taskId, r] in results"
        :key="taskId"
        style="margin-bottom: 24px"
      >
        <div style="margin-bottom: 8px; display: flex; align-items: center; gap: 12px">
          <span style="font-weight: 600">{{ r.agent_id }}</span>
          <StatusTag :status="r.status ?? ''" />
          <span v-if="r.exit_code !== 0" style="color: #f56c6c; font-size: 13px">
            exit_code: {{ r.exit_code }}
          </span>
          <span style="color: #909399; font-size: 12px">{{ taskId }}</span>
        </div>

        <div v-if="r.stdout" style="margin-bottom: 8px">
          <div style="font-size: 12px; color: #909399; margin-bottom: 4px">stdout</div>
          <OutputViewer :content="r.stdout" label="stdout" :truncated="r.truncated" :filename="`${taskId}-stdout.txt`" />
        </div>

        <div v-if="r.stderr">
          <div style="font-size: 12px; color: #909399; margin-bottom: 4px">stderr</div>
          <OutputViewer :content="r.stderr" label="stderr" :truncated="r.truncated" :filename="`${taskId}-stderr.txt`" />
        </div>

        <div
          v-if="!r.stdout && !r.stderr && r.status !== 'pending' && r.status !== 'running'"
          style="color: #909399; font-size: 13px"
        >
          (无输出)
        </div>
      </div>
    </el-card>

    <!-- Control panel -->
    <el-collapse style="margin-bottom: 20px">
      <el-collapse-item title="控制指令" name="control">
        <el-form label-width="100px" @submit.prevent="sendControl">
          <el-form-item label="目标 Agent">
            <el-select
              v-model="controlForm.agentId"
              placeholder="选择 Agent"
              style="width: 100%"
            >
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
            <el-select
              v-model="controlForm.action"
              placeholder="选择操作"
              style="width: 100%"
            >
              <el-option
                v-for="act in controlActions"
                :key="act.value"
                :label="act.label"
                :value="act.value"
              />
            </el-select>
          </el-form-item>

          <el-form-item>
            <el-button type="warning" :loading="controlSending" @click="sendControl">
              发送控制指令
            </el-button>
          </el-form-item>
        </el-form>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>
