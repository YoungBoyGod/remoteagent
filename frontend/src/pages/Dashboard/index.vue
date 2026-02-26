<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import dayjs from 'dayjs'
import client from '../../api/client'
import StatusTag from '../../components/StatusTag.vue'
import type { DashboardSummaryResp, HealthResp, TaskDetail } from '../../api/types'
import { Monitor, Clock, CircleCheck } from '@element-plus/icons-vue'

const loading = ref(false)

// stats
const agentOnlineCount = ref(0)
const agentTotal = ref(0)
const hostTotal = ref(0)
const customerTotal = ref(0)
const taskTotal = ref(0)

// Phase 2 任务状态分布
const taskStatusCounts = ref<Record<string, number>>({})
// health
const health = ref<HealthResp | null>(null)
const healthLoading = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | null = null

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

function statValueStyle(kind: 'agent' | 'default') {
  if (kind === 'agent') {
    return {
      color: '#4096ff',
      fontSize: '32px',
      fontWeight: '700',
      lineHeight: '1',
    }
  }
  return {
    fontSize: '32px',
    fontWeight: '700',
    lineHeight: '1',
  }
}

async function fetchData() {
  loading.value = true
  try {
    const resp = await client.get('/api/v1/dashboard/summary', {
      params: { recent_limit: 10 },
    })
    const summary: DashboardSummaryResp = resp.data.data
    agentTotal.value = summary?.agent_total ?? 0
    agentOnlineCount.value = summary?.agent_online ?? 0
    hostTotal.value = summary?.host_total ?? 0
    customerTotal.value = summary?.customer_total ?? 0
    taskTotal.value = summary?.task_total ?? 0
    taskStatusCounts.value = summary?.task_status_count ?? {}
    recentTasks.value = summary?.recent_tasks ?? []
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
  refreshTimer = setInterval(() => {
    fetchData()
    fetchHealth()
  }, 15_000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
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
      <el-col :xs="24" :sm="12" :md="8">
        <el-card shadow="hover" class="stat-card stat-agents">
          <el-statistic title="在线 Agent" :value="agentOnlineCount" :value-style="statValueStyle('agent')">
            <template #suffix>
              <span class="stat-suffix"> / {{ agentTotal }}</span>
            </template>
          </el-statistic>
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :md="8">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="受管主机" :value="hostTotal" />
        </el-card>
      </el-col>
      <el-col :xs="24" :sm="12" :md="8">
        <el-card shadow="hover" class="stat-card">
          <el-statistic title="客户数量" :value="customerTotal" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 任务状态分布 + 健康状态 -->
    <el-row :gutter="20" class="section-row">
      <el-col :xs="24" :lg="12">
        <el-card shadow="hover" class="section-card">
          <template #header>
            <div class="section-header">
              <div class="section-title">
                <el-icon><CircleCheck /></el-icon>
                <span>任务状态分布</span>
              </div>
              <el-tag type="info" effect="plain">任务总数 {{ taskTotal }}</el-tag>
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
        <el-card shadow="hover" class="section-card" v-loading="healthLoading">
          <template #header>
            <div class="section-title">
              <el-icon><Monitor /></el-icon>
              <span>服务健康状态</span>
            </div>
          </template>
          <template v-if="health">
            <el-descriptions :column="1" border>
              <el-descriptions-item label="服务名称">
                <span class="service-name">{{ health.service }}</span>
              </el-descriptions-item>
              <el-descriptions-item label="状态">
                <StatusTag :status="health.status" />
              </el-descriptions-item>
              <el-descriptions-item label="服务器时间">
                <el-icon class="time-icon"><Clock /></el-icon>
                {{ formatTime(health.timestamp) }}
              </el-descriptions-item>
            </el-descriptions>
          </template>
          <el-empty v-else description="无法获取健康状态" :image-size="80" />
        </el-card>
      </el-col>
    </el-row>

    <!-- 最近任务 -->
    <el-card shadow="hover" class="section-card">
      <template #header>
        <div class="section-header">
          <div class="section-title">
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
            <el-link type="primary" underline="never" @click="$router.push(`/tasks/${row.task_id}`)">
              {{ row.task_id.slice(0, 16) }}...
            </el-link>
          </template>
        </el-table-column>
        <el-table-column prop="task_type" label="类型" width="90">
          <template #default="{ row }">
            <el-tag size="small" effect="plain" :type="row.task_type === 'shell' ? 'info' : 'warning'">
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
            <span class="priority-value" :style="{ color: priorityColor(row.priority) }">{{ row.priority }}</span>
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

.section-row {
  margin-bottom: 24px;
}

.section-card {
  border-radius: 12px;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.stat-suffix {
  font-size: 16px;
  color: var(--el-text-color-secondary);
  margin-left: 2px;
}

.service-name {
  font-weight: 600;
}

.time-icon {
  margin-right: 4px;
  color: var(--el-text-color-secondary);
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

.priority-value {
  font-weight: 600;
  font-size: 14px;
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
