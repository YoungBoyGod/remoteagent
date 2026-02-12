<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import dayjs from 'dayjs'
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
    <!-- Health Check -->
    <el-card shadow="never" style="margin-bottom: 20px">
      <template #header>
        <span>健康检查</span>
      </template>
      <div v-loading="healthLoading">
        <template v-if="health">
          <el-descriptions :column="3" border>
            <el-descriptions-item label="服务名称">{{ health.service }}</el-descriptions-item>
            <el-descriptions-item label="状态">
              <el-tag :type="health.status === 'ok' ? 'success' : 'danger'">
                {{ health.status }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="上次检查时间">{{ healthCheckedAt }}</el-descriptions-item>
          </el-descriptions>
        </template>
        <el-empty v-else description="暂无数据" />
      </div>
    </el-card>

    <!-- Agent Heartbeat -->
    <el-card shadow="never" style="margin-bottom: 20px">
      <template #header>
        <span>Agent 心跳状态</span>
      </template>
      <el-table v-loading="agentsLoading" :data="agents" stripe border style="width: 100%">
        <el-table-column prop="agent_id" label="Agent ID" min-width="200" show-overflow-tooltip />
        <el-table-column prop="device_code" label="Device Code" min-width="150" show-overflow-tooltip />
        <el-table-column label="状态" width="100" align="center">
          <template #default="{ row }">
            <span>
              <span class="status-dot" :class="row.status === 'online' ? 'online' : 'offline'" />
              {{ row.status }}
            </span>
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
    <el-card shadow="never">
      <template #header>
        <span>Grafana Dashboard</span>
      </template>
      <iframe
        :src="grafanaUrl"
        width="100%"
        height="600"
        frameborder="0"
        style="border: none; display: block"
      />
    </el-card>
  </div>
</template>
