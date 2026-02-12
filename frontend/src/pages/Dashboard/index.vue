<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import dayjs from 'dayjs'
import client from '../../api/client'
import StatusTag from '../../components/StatusTag.vue'
import type { DebugAgentItem, TaskDetail, TaskDetailListResp, HealthResp } from '../../api/types'

const loading = ref(false)

// stats
const agentOnlineCount = ref(0)
const agentTotal = ref(0)
const taskTotal = ref(0)

// Phase 2 任务状态分布
const taskStatusCounts = ref<Record<string, number>>({})
const pendingCount = computed(() => taskStatusCounts.value['pending'] || 0)
const runningCount = computed(() => taskStatusCounts.value['running'] || 0)
const leasedCount = computed(() => taskStatusCounts.value['leased'] || 0)
// health
const health = ref<HealthResp | null>(null)
const healthLoading = ref(false)

// recent tasks (Phase 2)
const recentTasks = ref<TaskDetail[]>([])

function formatTime(t: number | null | undefined) {
  if (t == null || t <= 0) return '-'
  return dayjs.unix(t).format('YYYY-MM-DD HH:mm:ss')
}

function priorityColor(p: number): string {
  if (p >= 80) return '#f56c6c'
  if (p >= 60) return '#e6a23c'
  if (p >= 40) return '#409eff'
  return '#909399'
}

async function fetchData() {
  loading.value = true
  try {
    const [agentsResp, tasksResp] = await Promise.all([
      client.get('/api/v1/debug/agents'),
      client.get('/api/v1/tasks', { params: { page: 1, page_size: 10 } }),
    ])

    // agents
    const agents: DebugAgentItem[] = agentsResp.data.data ?? []
    agentTotal.value = agents.length
    agentOnlineCount.value = agents.filter((a) => a.status === 'online').length

    // Phase 2 tasks
    const taskList: TaskDetailListResp = tasksResp.data.data
    recentTasks.value = taskList?.items ?? []
    taskTotal.value = taskList?.total ?? 0

    // 统计各状态数量（从最近的任务中粗略统计，精确统计需要后端支持）
    const counts: Record<string, number> = {}
    for (const t of recentTasks.value) {
      counts[t.status] = (counts[t.status] || 0) + 1
    }
    taskStatusCounts.value = counts
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
    <h2 class="page-title">Dashboard</h2>

    <!-- stat cards -->
    <el-row :gutter="16" style="margin-bottom: 20px">
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-agents">
          <el-statistic title="Agent 在线" :value="agentOnlineCount">
            <template #suffix>
              <span style="font-size: 14px; color: var(--el-text-color-secondary)"> / {{ agentTotal }}</span>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-total">
          <el-statistic title="任务总数" :value="taskTotal" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-running">
          <el-statistic title="运行中" :value="runningCount + leasedCount" />
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="stat-card stat-pending">
          <el-statistic title="排队中" :value="pendingCount" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 任务状态分布 -->
    <el-row :gutter="16" style="margin-bottom: 20px">
      <el-col :span="12">
        <el-card shadow="never">
          <template #header><span>任务状态分布</span></template>
          <div style="display: flex; gap: 16px; flex-wrap: wrap">
            <div v-for="(count, status) in taskStatusCounts" :key="status" style="text-align: center; min-width: 80px">
              <div style="font-size: 24px; font-weight: 600; margin-bottom: 4px">{{ count }}</div>
              <StatusTag :status="String(status)" />
            </div>
            <div v-if="Object.keys(taskStatusCounts).length === 0" style="color: var(--el-text-color-secondary)">
              暂无任务数据
            </div>
          </div>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card shadow="never" v-loading="healthLoading">
          <template #header><span>服务健康状态</span></template>
          <template v-if="health">
            <el-descriptions :column="1" border>
              <el-descriptions-item label="服务名称">{{ health.service }}</el-descriptions-item>
              <el-descriptions-item label="状态">
                <StatusTag :status="health.status" />
              </el-descriptions-item>
              <el-descriptions-item label="服务器时间">{{ formatTime(health.timestamp) }}</el-descriptions-item>
            </el-descriptions>
          </template>
          <template v-else>
            <span style="color: var(--el-text-color-secondary)">无法获取健康状态</span>
          </template>
        </el-card>
      </el-col>
    </el-row>

    <!-- recent tasks (Phase 2) -->
    <el-card shadow="never">
      <template #header><span>最近任务</span></template>
      <el-table :data="recentTasks" stripe style="width: 100%">
        <el-table-column prop="task_id" label="Task ID" min-width="140" show-overflow-tooltip />
        <el-table-column prop="task_type" label="类型" width="90" />
        <el-table-column label="状态" width="100">
          <template #default="{ row }">
            <StatusTag :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column label="模式" width="90" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.exec_mode === 'exclusive'" type="danger" size="small" effect="plain">
              {{ row.exec_mode }}
            </el-tag>
            <el-tag v-else size="small" effect="plain">
              {{ row.exec_mode }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="优先级" width="70" align="center">
          <template #default="{ row }">
            <span :style="{ color: priorityColor(row.priority), fontWeight: 600 }">{{ row.priority }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="agent_id" label="Agent" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.agent_id || '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>
