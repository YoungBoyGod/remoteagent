<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import dayjs from 'dayjs'
import { Monitor, Connection, DataLine, CircleCheck, CircleClose } from '@element-plus/icons-vue'
import client from '../../api/client'
import type { Envelope, HealthResp, DebugAgentItem } from '../../api/types'

/* ---- Health Check ---- */
const health = ref<HealthResp | null>(null)
const healthLoading = ref(false)
const healthCheckedAt = ref('')

async function fetchHealth() {
  healthLoading.value = true
  try {
    const resp = await client.get('/healthz')
    health.value = resp.data.data
    healthCheckedAt.value = dayjs().format('YYYY-MM-DD HH:mm:ss')
  } catch {
    health.value = null
  } finally {
    healthLoading.value = false
  }
}

/* ---- Agent Heartbeat ---- */
const agents = ref<DebugAgentItem[]>([])
const agentsLoading = ref(false)

async function fetchAgents() {
  agentsLoading.value = true
  try {
    const resp = await client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents')
    agents.value = (resp.data.data ?? []).sort((a, b) => a.device_code.localeCompare(b.device_code))
  } catch {
    agents.value = []
  } finally {
    agentsLoading.value = false
  }
}

function formatTs(ts: number | null | undefined): string {
  if (!ts) return '-'
  return dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
}

/* ---- Grafana ---- */
const grafanaUrl = import.meta.env.VITE_GRAFANA_URL || 'http://localhost:3002'

/* ---- Lifecycle ---- */
let healthTimer: ReturnType<typeof setInterval>
let agentsTimer: ReturnType<typeof setInterval>

onMounted(() => {
  fetchHealth()
  fetchAgents()
  healthTimer = setInterval(fetchHealth, 10_000)
  agentsTimer = setInterval(fetchAgents, 30_000)
})

onUnmounted(() => {
  clearInterval(healthTimer)
  clearInterval(agentsTimer)
})
</script>

<template>
  <div>
    <h2 class="page-title">
      <el-icon size="28"><DataLine /></el-icon>
      监控中心
    </h2>

    <!-- 健康检查 -->
    <el-card shadow="hover" style="margin-bottom: 20px">
      <template #header>
        <div class="card-header">
          <div class="header-title">
            <el-icon><Monitor /></el-icon>
            <span>健康检查</span>
          </div>
          <el-tag v-if="healthCheckedAt" size="small" type="info" effect="plain">
            检查时间: {{ healthCheckedAt }}
          </el-tag>
        </div>
      </template>
      <div v-loading="healthLoading">
        <template v-if="health">
          <el-descriptions :column="3" border>
            <el-descriptions-item label="服务名称">
              <span style="font-weight: 600">{{ health.service }}</span>
            </el-descriptions-item>
            <el-descriptions-item label="状态">
              <span class="health-status" :class="health.status">
                <el-icon v-if="health.status === 'ok'" size="12"><CircleCheck /></el-icon>
                <el-icon v-else size="12"><CircleClose /></el-icon>
                {{ health.status }}
              </span>
            </el-descriptions-item>
            <el-descriptions-item label="服务器时间">
              {{ formatTs(health.timestamp) }}
            </el-descriptions-item>
          </el-descriptions>
        </template>
        <el-empty v-else description="暂无数据" :image-size="80" />
      </div>
    </el-card>

    <!-- Agent Heartbeat -->
    <el-card shadow="hover" style="margin-bottom: 20px">
      <template #header>
        <div class="card-header">
          <div class="header-title">
            <el-icon><Connection /></el-icon>
            <span>Agent 心跳状态</span>
          </div>
          <el-tag type="primary" size="small" effect="plain">{{ agents.length }} 个 Agent</el-tag>
        </div>
      </template>
      <el-table v-loading="agentsLoading" :data="agents" stripe border style="width: 100%">
        <el-table-column prop="agent_id" label="Agent ID" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <code>{{ row.agent_id }}</code>
          </template>
        </el-table-column>
        <el-table-column prop="device_code" label="Device Code" min-width="150" show-overflow-tooltip />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <div class="status-cell">
              <span class="status-dot" :class="row.status === 'online' ? 'online' : 'offline'" />
              <span>{{ row.status }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="最后心跳时间" min-width="180">
          <template #default="{ row }">
            {{ formatTs(row.last_heartbeat_at) }}
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- Grafana Dashboard -->
    <el-card shadow="hover">
      <template #header>
        <div class="header-title">
          <el-icon><DataLine /></el-icon>
          <span>Grafana Dashboard</span>
        </div>
      </template>
      <iframe
        :src="grafanaUrl"
        width="100%"
        height="600"
        frameborder="0"
        style="border: none; display: block; border-radius: 8px"
      />
    </el-card>
  </div>
</template>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.status-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

/* 健康状态 - 无边框 */
.health-status {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.health-status.ok {
  color: #52c41a;
  background: #f6ffed;
}

.health-status:not(.ok) {
  color: #ff4d4f;
  background: #fff2f0;
}
</style>
