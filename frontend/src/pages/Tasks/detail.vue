<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import dayjs from 'dayjs'
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
    <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 20px">
      <el-button text @click="router.push('/tasks')">← 返回列表</el-button>
      <h2 class="page-title" style="margin: 0">任务详情</h2>
    </div>

    <template v-if="task">
      <!-- 状态概览 -->
      <el-card shadow="never" style="margin-bottom: 16px">
        <div style="display: flex; align-items: center; gap: 16px; flex-wrap: wrap">
          <StatusTag :status="task.status" />
          <span v-if="polling" style="color: var(--el-text-color-secondary); font-size: 13px">
            <el-icon class="is-loading" style="vertical-align: middle"><i /></el-icon>
            等待执行...
          </span>
          <span v-if="task.exit_code != null" :style="{ color: task.exit_code === 0 ? '#67c23a' : '#f56c6c', fontWeight: 600 }">
            exit_code: {{ task.exit_code }}
          </span>
          <span v-if="task.agent_id" style="font-size: 13px">
            Agent: <code style="background: var(--el-fill-color-light); padding: 2px 6px; border-radius: 3px">{{ task.agent_id }}</code>
          </span>
        </div>
      </el-card>

      <!-- 基本信息 -->
      <el-card shadow="never" style="margin-bottom: 16px">
        <template #header><span>基本信息</span></template>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="Task ID">{{ task.task_id }}</el-descriptions-item>
          <el-descriptions-item label="类型">{{ task.task_type }}</el-descriptions-item>
          <el-descriptions-item label="命令">
            <code>{{ task.payload?.command || '-' }}</code>
          </el-descriptions-item>
          <el-descriptions-item label="参数">{{ task.payload?.args?.join(' ') || '-' }}</el-descriptions-item>
          <el-descriptions-item label="工作目录">{{ task.payload?.workdir || '-' }}</el-descriptions-item>
          <el-descriptions-item label="超时">{{ task.payload?.timeout || 30 }}s</el-descriptions-item>
          <el-descriptions-item label="执行模式">
            <el-tag :type="task.exec_mode === 'exclusive' ? 'danger' : 'info'" size="small" effect="plain">{{ task.exec_mode }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="优先级">{{ task.priority }}</el-descriptions-item>
          <el-descriptions-item label="尝试次数">{{ task.attempt }} / {{ task.max_attempts }}</el-descriptions-item>
          <el-descriptions-item label="幂等键">{{ task.idempotency_key || '-' }}</el-descriptions-item>
          <el-descriptions-item label="创建时间">{{ formatTime(task.created_at) }}</el-descriptions-item>
          <el-descriptions-item label="开始时间">{{ formatTime(task.started_at) }}</el-descriptions-item>
          <el-descriptions-item label="完成时间">{{ formatTime(task.finished_at) }}</el-descriptions-item>
          <el-descriptions-item v-if="task.error_code" label="错误码">{{ task.error_code }}</el-descriptions-item>
          <el-descriptions-item v-if="task.error_message" label="错误信息">{{ task.error_message }}</el-descriptions-item>
          <el-descriptions-item v-if="task.payload?.env && Object.keys(task.payload.env).length > 0" label="环境变量" :span="2">
            <el-tag v-for="(v, k) in task.payload.env" :key="k" size="small" style="margin-right: 6px">{{ k }}={{ v }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>
      </el-card>

      <!-- 执行输出 -->
      <el-card shadow="never">
        <template #header><span>执行输出</span></template>
        <template v-if="task.stdout || task.stderr">
          <div v-if="task.stdout" style="margin-bottom: 16px">
            <div style="font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 4px">stdout</div>
            <OutputViewer
              :content="task.stdout"
              label="stdout"
              :truncated="task.truncated ?? false"
              :filename="`${task.task_id}-stdout.txt`"
            />
          </div>
          <div v-if="task.stderr">
            <div style="font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 4px">stderr</div>
            <OutputViewer
              :content="task.stderr"
              label="stderr"
              :truncated="task.truncated ?? false"
              :filename="`${task.task_id}-stderr.txt`"
            />
          </div>
        </template>
        <div v-else-if="polling" style="color: var(--el-text-color-secondary); padding: 20px 0; text-align: center">
          等待任务执行完成...
        </div>
        <div v-else style="color: var(--el-text-color-secondary); padding: 20px 0; text-align: center">
          (无输出)
        </div>
      </el-card>
    </template>

    <el-empty v-else-if="!loading" description="任务不存在" />
  </div>
</template>
