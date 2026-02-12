<script setup lang="ts">
import { ref, onMounted } from 'vue'
import dayjs from 'dayjs'
import client from '../../api/client'
import StatusTag from '../../components/StatusTag.vue'
import type { DebugAgentItem, TaskItem, TaskListResp, HealthResp, SystemState } from '../../api/types'

const loading = ref(false)

// stats
const agentOnlineCount = ref(0)
const taskTotal = ref(0)
const runningTasks = ref(0)

// health
const health = ref<HealthResp | null>(null)
const healthLoading = ref(false)

// recent tasks
const recentTasks = ref<TaskItem[]>([])

function formatTime(t: number | null) {
  if (t == null) return '-'
  return dayjs.unix(t).format('YYYY-MM-DD HH:mm:ss')
}

async function fetchData() {
  loading.value = true
  try {
    const [stateResp, agentsResp, tasksResp] = await Promise.all([
      client.get('/api/v1/debug/state'),
      client.get('/api/v1/debug/agents'),
      client.get('/api/v1/debug/tasks', { params: { page: 1, page_size: 10 } }),
    ])

    // system state
    const state: SystemState = stateResp.data.data
    taskTotal.value = state.tasks

    // agents online count
    const agents: DebugAgentItem[] = agentsResp.data.data ?? []
    agentOnlineCount.value = agents.filter((a) => a.status === 'online').length

    // recent tasks
    const taskList: TaskListResp = tasksResp.data.data
    recentTasks.value = taskList.items ?? []
    runningTasks.value = recentTasks.value.filter((t) => t.status === 'running').length
  } catch {
    // errors handled by interceptor
  } finally {
    loading.value = false
  }
}

async function fetchHealth() {
  healthLoading.value = true
  try {
    const resp = await client.get('/healthz')
    health.value = resp.data.data
  } catch {
    health.value = null
  } finally {
    healthLoading.value = false
  }
}

onMounted(() => {
  fetchData()
  fetchHealth()
})
</script>

<template>
  <div v-loading="loading">
    <h2 style="margin-top: 0">Dashboard</h2>

    <!-- stat cards -->
    <el-row :gutter="16" style="margin-bottom: 20px">
      <el-col :span="8">
        <el-card shadow="hover">
          <el-statistic title="Agent 在线数" :value="agentOnlineCount" />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <el-statistic title="任务总数" :value="taskTotal" />
        </el-card>
      </el-col>
      <el-col :span="8">
        <el-card shadow="hover">
          <el-statistic title="运行中任务" :value="runningTasks" />
        </el-card>
      </el-col>
    </el-row>

    <!-- health check -->
    <el-card shadow="hover" style="margin-bottom: 20px" v-loading="healthLoading">
      <template #header>
        <span>服务健康状态</span>
      </template>
      <template v-if="health">
        <el-descriptions :column="3" border>
          <el-descriptions-item label="服务名称">
            {{ health.service }}
          </el-descriptions-item>
          <el-descriptions-item label="状态">
            <StatusTag :status="health.status" />
          </el-descriptions-item>
          <el-descriptions-item label="服务器时间">
            {{ formatTime(health.timestamp) }}
          </el-descriptions-item>
        </el-descriptions>
      </template>
      <template v-else>
        <span style="color: #999">无法获取健康状态</span>
      </template>
    </el-card>

    <!-- recent tasks -->
    <el-card shadow="hover">
      <template #header>
        <span>最近任务</span>
      </template>
      <el-table :data="recentTasks" stripe style="width: 100%">
        <el-table-column prop="task_id" label="Task ID" min-width="220" show-overflow-tooltip />
        <el-table-column prop="agent_id" label="Agent ID" min-width="220" show-overflow-tooltip />
        <el-table-column label="Status" width="120">
          <template #default="{ row }">
            <StatusTag :status="row.status ?? ''" />
          </template>
        </el-table-column>
        <el-table-column label="开始时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.started_at) }}
          </template>
        </el-table-column>
        <el-table-column label="完成时间" width="180">
          <template #default="{ row }">
            {{ formatTime(row.finished_at) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
