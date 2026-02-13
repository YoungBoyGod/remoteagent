<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import dayjs from 'dayjs'
import client from '../../api/client'
import StatusTag from '../../components/StatusTag.vue'
import type { DebugAgentItem, TaskDetail, TaskDetailListResp, HealthResp } from '../../api/types'
import { Monitor, Clock, Loading, CircleCheck } from '@element-plus/icons-vue'

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
  if (p >= 80) return '#ff4d4f'
  if (p >= 60) return '#faad14'
  if (p >= 40) return '#4096ff'
  return '#8c8c8c'
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
    <h2 class="page-title">
      <el-icon size="28"><Monitor /></el-icon>
      Dashboard
    </h2>

    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card stat-agents">
          <el-statistic title="在线 Agent">
            <template #default>
              <div style="display: flex; align-items: baseline; gap: 4px">
                <span style="font-size: 32px; font-weight: 700; color: #4096ff">{{ agentOnlineCount }}</span>
                <span style="font-size: 16px; color: var(--el-text-color-secondary)">/ {{ agentTotal }}</span>
              </div>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card stat-total">
          <el-statistic title="任务总数" :value="taskTotal" />
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card stat-running">
          <el-statistic title="运行中">
            <template #default>
              <div style="display: flex; align-items: center; gap: 8px">
                <el-icon size="24" style="color: #fa8c16"><Loading /></el-icon>
                <span style="font-size: 32px; font-weight: 700; color: #fa8c16">{{ runningCount + leasedCount }}</span>
              </div>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :md="6">
        <el-card shadow="hover" class="stat-card stat-pending">
          <el-statistic title="排队中">
            <template #default>
              <div style="display: flex; align-items: center; gap: 8px">
                <el-icon size="24" style="color: #eb2f96"><Clock /></el-icon>
                <span style="font-size: 32px; font-weight: 700; color: #eb2f96">{{ pendingCount }}</span>
              </div>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
    </el-row>

    <!-- 任务状态分布 + 健康状态 -->
    <el-row :gutter="20" style="margin-bottom: 24px">
      <el-col :xs="24" :lg="12">
        <el-card shadow="hover">
          <template #header>
            <div style="display: flex; align-items: center; gap: 8px">
              <el-icon><CircleCheck /></el-icon>
              <span>任务状态分布</span>
            </div>
          </template>
          <div v-if="Object.keys(taskStatusCounts).length > 0" class="status-distribution">
            <div 
              v-for="(count, status) in taskStatusCounts" 
              :key="status" 
              class="status-item"
            >
              <div class="status-count">{{ count }}</div>
              <StatusTag :status="String(status)" />
            </div>
          </div>
          <el-empty v-else description="暂无任务数据" :image-size="80" />
        </el-card>
      </el-col>
      <el-col :xs="24" :lg="12">
        <el-card shadow="hover" v-loading="healthLoading">
          <template #header>
            <div style="display: flex; align-items: center; gap: 8px">
              <el-icon><Monitor /></el-icon>
              <span>服务健康状态</span>
            </div>
          </template>
          <template v-if="health">
            <el-descriptions :column="1" border>
              <el-descriptions-item label="服务名称">
                <span style="font-weight: 600">{{ health.service }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="状态">
                <StatusTag :status="health.status" />
              </el-descriptions-item>
              <el-descriptions-item label="服务器时间">
                <el-icon style="margin-right: 4px; color: var(--el-text-color-secondary)"><Clock /></el-icon>
                {{ formatTime(health.timestamp) }}
              </el-descriptions-item>
            </el-descriptions>
          </template>
          <el-empty v-else description="无法获取健康状态" :image-size="80" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近任务 -->
    <el-card shadow="hover">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <div style="display: flex; align-items: center; gap: 8px">
            <el-icon><Monitor /></el-icon>
            <span>最近任务</span>
          </div>
          <el-button text type="primary" size="small" @click="$router.push('/tasks')">
            查看全部 →
          </el-button>
        </div>
      </template>
      <el-table :data="recentTasks" stripe style="width: 100%">
        <el-table-column prop="task_id" label="Task ID" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            <el-link type="primary" :underline="false" @click="$router.push(`/tasks/${row.task_id}`)">
              {{ row.task_id.slice(0, 16) }}...
            </el-link>
          </template>
        </el-table-column>
        <el-table-column prop="task_type" label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="row.task_type === 'shell' ? '' : 'warning'">
              {{ row.task_type }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <StatusTag :status="row.status" />
          </template>
        </el-table-column>
        <el-table-column label="模式" width="70" align="center">
          <template #default="{ row }">
            <span class="exec-mode" :class="row.exec_mode">
              {{ row.exec_mode === 'exclusive' ? '独占' : '共享' }}
            </span>
          </template>
        </el-table-column>
        <el-table-column label="优先级" width="80" align="center">
          <template #default="{ row }">
            <span :style="{ color: priorityColor(row.priority), fontWeight: 600, fontSize: '14px' }">{{ row.priority }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="agent_id" label="Agent" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">{{ row.agent_id?.slice(0, 12) || '-' }}</template>
        </el-table-column>
        <el-table-column label="创建时间" width="170">
          <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<style scoped>
.stats-row {
  margin-bottom: 24px;
}

.status-distribution {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
  padding: 16px 8px;
}

.status-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  min-width: 80px;
  padding: 16px 20px;
  background: var(--el-fill-color-light);
  border-radius: 12px;
  transition: all 0.3s ease;
}

.status-item:hover {
  transform: translateY(-2px);
  background: var(--el-fill-color);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.status-count {
  font-size: 28px;
  font-weight: 700;
  color: var(--el-text-color-primary);
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
