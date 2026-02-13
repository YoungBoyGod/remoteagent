<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import dayjs from 'dayjs'
import { ArrowLeft, Loading, Document } from '@element-plus/icons-vue'
import client from '../../api/client'
import type { Envelope, TaskDetail } from '../../api/types'
import StatusTag from '../../components/StatusTag.vue'
import OutputViewer from '../../components/OutputViewer.vue'

const route = useRoute()
const router = useRouter()
const taskId = route.params.task_id as string

const task = ref<TaskDetail | null>(null)
const loading = ref(true)
const polling = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

const terminalStatuses = ['success', 'failed', 'timeout', 'canceled']

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('YYYY-MM-DD HH:mm:ss')
}

async function fetchTask() {
  try {
    const resp = await client.get<Envelope<TaskDetail>>(`/api/v1/tasks/${taskId}`)
    task.value = resp.data.data
    if (task.value && terminalStatuses.includes(task.value.status)) {
      stopPolling()
    }
  } catch {
    stopPolling()
  } finally {
    loading.value = false
  }
}

function startPolling() {
  polling.value = true
  timer = setInterval(fetchTask, 2000)
}

function stopPolling() {
  polling.value = false
  if (timer) {
    clearInterval(timer)
    timer = null
  }
}

onMounted(() => {
  fetchTask().then(() => {
    if (task.value && !terminalStatuses.includes(task.value.status)) {
      startPolling()
    }
  })
})

onUnmounted(stopPolling)
</script>

<template>
  <div v-loading="loading">
    <div class="page-header">
      <el-button text @click="router.push('/tasks')" class="back-btn">
        <el-icon><ArrowLeft /></el-icon>
        返回列表
      </el-button>
      <h2 class="page-title" style="margin: 0; border: none; padding: 0">
        <el-icon size="28"><Document /></el-icon>
        任务详情
      </h2>
    </div>

    <template v-if="task">
      <!-- 状态概览 -->
      <el-card shadow="hover" style="margin-bottom: 16px">
        <div class="status-overview">
          <StatusTag :status="task.status" />
          <span v-if="polling" class="polling-indicator">
            <el-icon class="is-loading"><Loading /></el-icon>
            等待执行...
          </span>
          <span v-if="task.exit_code != null" :class="['exit-code', task.exit_code === 0 ? 'success' : 'error']">
            exit_code: {{ task.exit_code }}
          </span>
          <span v-if="task.agent_id" class="agent-info">
            Agent: <code>{{ task.agent_id }}</code>
          </span>
        </div>
      </el-card>

      <!-- 基本信息 -->
      <el-card shadow="hover" style="margin-bottom: 16px">
        <template #header>
          <div class="card-header-title">
            <el-icon><Document /></el-icon>
            <span>基本信息</span>
          </div>
        </template>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="Task ID">
            <code>{{ task.task_id }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="类型">
            <span class="task-type" :class="task.task_type">{{ task.task_type }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="命令" :span="2">
            <code class="command-code">{{ task.payload?.command || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="参数">{{ task.payload?.args?.join(' ') || '-' }}</el-descriptions-item>
          <el-descriptions-item label="工作目录">{{ task.payload?.workdir || '-' }}</el-descriptions-item>
          <el-descriptions-item label="超时">{{ task.payload?.timeout || 30 }}s</el-descriptions-item>
          <el-descriptions-item label="执行模式">
            <span class="exec-mode" :class="task.exec_mode">
              {{ task.exec_mode === 'exclusive' ? '独占' : '共享' }}
            </span>
          </el-descriptions-item>
          <el-descriptions-item label="优先级">{{ task.priority }}</el-descriptions-item>
          <el-descriptions-item label="尝试次数">{{ task.attempt }} / {{ task.max_attempts }}</el-descriptions-item>
          <el-descriptions-item label="幂等键">{{ task.idempotency_key || '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(task.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ formatTime(task.started_at) }}</el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ formatTime(task.finished_at) }}</el-descriptions-item>
          <el-descriptions-item v-if="task.error_code" label="错误码">{{ task.error_code }}</el-descriptions-item>
          <el-descriptions-item v-if="task.error_message" label="错误信息" :span="2">{{ task.error_message }}</el-descriptions-item>
          <el-descriptions-item v-if="task.payload?.env && Object.keys(task.payload.env).length > 0" label="环境变量" :span="2">
            <el-tag v-for="(v, k) in task.payload.env" :key="k" size="small" style="margin-right: 6px">{{ k }}={{ v }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 执行输出 -->
      <el-card shadow="hover">
        <template #header>
          <div class="card-header-title">
            <el-icon><Document /></el-icon>
            <span>执行输出</span>
          </div>
        </template>
        <template v-if="task.stdout || task.stderr">
          <div v-if="task.stdout" style="margin-bottom: 20px">
            <div class="output-label">stdout</div>
            <OutputViewer
              :content="task.stdout"
              label="stdout"
              :truncated="task.truncated ?? false"
              :filename="`${task.task_id}-stdout.txt`"
            />
          </div>
          <div v-if="task.stderr">
            <div class="output-label">stderr</div>
            <OutputViewer
              :content="task.stderr"
              label="stderr"
              :truncated="task.truncated ?? false"
              :filename="`${task.task_id}-stderr.txt`"
            />
          </div>
        </template>
        <div v-else-if="polling" class="polling-status">
          <el-icon class="is-loading" size="32"><Loading /></el-icon>
          <p>等待任务执行完成...</p>
        </div>
        <el-empty v-else description="(无输出)" :image-size="80" />
      </el-card>
    </template>

    <el-empty v-else-if="!loading" description="任务不存在" :image-size="100" />
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
}

.back-btn {
  padding: 8px 12px;
  border-radius: 8px;
}

.status-overview {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.polling-indicator {
  color: var(--el-text-color-secondary);
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 6px;
}

.exit-code {
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 13px;
}

.exit-code.success {
  color: #52c41a;
  background: #f6ffed;
}

.exit-code.error {
  color: #ff4d4f;
  background: #fff2f0;
}

.agent-info {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.agent-info code {
  background: var(--el-fill-color-light);
  padding: 4px 10px;
  border-radius: 6px;
  font-size: 13px;
}

.card-header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.command-code {
  display: block;
  padding: 12px 16px;
  background: #1e1e1e;
  color: #d4d4d4;
  border-radius: 8px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  max-width: 100%;
  overflow-x: auto;
}

.output-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
  font-weight: 600;
}

.polling-status {
  padding: 40px 0;
  text-align: center;
  color: var(--el-text-color-secondary);
}

.polling-status p {
  margin-top: 12px;
}

/* 任务类型 - 无边框 */
.task-type {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.task-type.shell {
  color: #4096ff;
  background: #eaf6ff;
}

.task-type.python {
  color: #faad14;
  background: #fffbe6;
}

/* 执行模式 - 无边框 */
.exec-mode {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.exec-mode.exclusive {
  color: #ff4d4f;
  background: #fff2f0;
}

.exec-mode.shared {
  color: #8c8c8c;
  background: #f5f5f5;
}
</style>
