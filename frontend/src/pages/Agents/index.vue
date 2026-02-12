<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Refresh, Search } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import client from '../../api/client'
import type { Envelope, DebugAgentItem } from '../../api/types'

dayjs.extend(relativeTime)

const loading = ref(false)
const agents = ref<DebugAgentItem[]>([])
const expandedRows = ref<string[]>([])

const filter = reactive({
  search: '',
  status: '',
})

function formatHeartbeat(ts: number | null): string {
  if (!ts || ts <= 0) return '-'
  return dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
}

function formatLabels(labels: Record<string, string>): string {
  if (!labels || Object.keys(labels).length === 0) return '-'
  return Object.entries(labels)
    .map(([k, v]) => `${k}=${v}`)
    .join(', ')
}

async function fetchAgents() {
  loading.value = true
  try {
    const params: Record<string, string> = {}
    if (filter.status) params.status = filter.status
    if (filter.search) params.search = filter.search

    const resp = await client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents', { params })
    agents.value = (resp.data.data ?? []).sort((a, b) => a.device_code.localeCompare(b.device_code))
  } catch {
    agents.value = []
  } finally {
    loading.value = false
  }
}

function onSearch() {
  fetchAgents()
}

function onRefresh() {
  fetchAgents()
}

function handleExpandChange(_row: DebugAgentItem, expanded: DebugAgentItem[]) {
  expandedRows.value = expanded.map((r) => r.agent_id)
}

onMounted(() => {
  fetchAgents()
})
</script>

<template>
  <div>
    <h2 style="margin-top: 0">Agents</h2>

    <!-- Toolbar -->
    <el-row :gutter="12" style="margin-bottom: 16px" align="middle">
      <el-col :span="6">
        <el-input
          v-model="filter.search"
          placeholder="搜索 device_code"
          clearable
          :prefix-icon="Search"
          @clear="onSearch"
          @keyup.enter="onSearch"
        />
      </el-col>
      <el-col :span="4">
        <el-select
          v-model="filter.status"
          placeholder="状态筛选"
          clearable
          @change="onSearch"
        >
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
        </el-select>
      </el-col>
      <el-col :span="3">
        <el-button :icon="Refresh" @click="onRefresh" :loading="loading">
          刷新
        </el-button>
      </el-col>
    </el-row>

    <!-- Table -->
    <el-table
      :data="agents"
      v-loading="loading"
      row-key="agent_id"
      :expand-row-keys="expandedRows"
      @expand-change="handleExpandChange"
      border
      stripe
      style="width: 100%"
    >
      <el-table-column type="expand">
        <template #default="{ row }">
          <div style="padding: 12px 24px">
            <el-descriptions title="Agent 详情" :column="2" border>
              <el-descriptions-item label="Agent Version">
                {{ row.agent_version || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="Heartbeat Interval">
                {{ row.heartbeat_interval ? row.heartbeat_interval + 's' : '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="Labels" :span="2">
                {{ formatLabels(row.labels) }}
              </el-descriptions-item>
              <el-descriptions-item label="Capabilities" :span="2">
                <el-tag
                  v-for="cap in (row.capabilities || [])"
                  :key="cap"
                  size="small"
                  style="margin-right: 6px; margin-bottom: 4px"
                >
                  {{ cap }}
                </el-tag>
                <span v-if="!row.capabilities || row.capabilities.length === 0">-</span>
              </el-descriptions-item>
              <el-descriptions-item label="注册时间">
                {{ row.created_at ? dayjs.unix(row.created_at).format('YYYY-MM-DD HH:mm:ss') : '-' }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="agent_id" label="Agent ID" min-width="180" show-overflow-tooltip />
      <el-table-column prop="device_code" label="Device Code" min-width="140" show-overflow-tooltip />
      <el-table-column prop="hostname" label="Hostname" min-width="120" show-overflow-tooltip />
      <el-table-column prop="os" label="OS" width="100" />
      <el-table-column prop="arch" label="Arch" width="90" />
      <el-table-column prop="ip" label="IP" width="140" />
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <el-tag
            :type="row.status === 'online' ? 'success' : 'info'"
            size="small"
          >
            {{ row.status === 'online' ? '在线' : '离线' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="最后心跳" width="140">
        <template #default="{ row }">
          {{ formatHeartbeat(row.last_heartbeat_at) }}
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
