<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
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

const onlineCount = computed(() => agents.value.filter((a) => a.status === 'online').length)
const offlineCount = computed(() => agents.value.filter((a) => a.status === 'offline').length)

function formatTime(ts: number | null): string {
  if (!ts || ts <= 0) return '-'
  return dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
}

function relativeTimeStr(ts: number | null): string {
  if (!ts || ts <= 0) return ''
  return dayjs.unix(ts).fromNow()
}

// 能力标签颜色映射
function capabilityType(cap: string): '' | 'success' | 'warning' | 'danger' | 'info' {
  if (cap === 'command_exec') return 'success'
  if (cap.startsWith('gpu') || cap.startsWith('cuda')) return 'danger'
  if (cap.startsWith('docker')) return 'warning'
  return 'info'
}

// 槽位使用率百分比
function slotPercent(agent: DebugAgentItem): number {
  if (agent.max_concurrent == null || agent.max_concurrent <= 0) return 0
  if (agent.running_exclusive) return 100
  return Math.round(((agent.running_shared ?? 0) / agent.max_concurrent) * 100)
}

// 槽位进度条颜色
function slotColor(percent: number): string {
  if (percent >= 100) return '#f56c6c'
  if (percent >= 75) return '#e6a23c'
  return '#67c23a'
}

function slotText(agent: DebugAgentItem): string {
  if (agent.max_concurrent == null) return '未上报'
  if (agent.running_exclusive) return '独占中'
  return `${agent.running_shared ?? 0} / ${agent.max_concurrent}`
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

function handleExpandChange(_row: DebugAgentItem, expanded: DebugAgentItem[]) {
  expandedRows.value = expanded.map((r) => r.agent_id)
}

onMounted(() => {
  fetchAgents()
})
</script>

<template>
  <div>
    <h2 class="page-title">Agents</h2>

    <!-- 统计概览 -->
    <el-row :gutter="12" style="margin-bottom: 16px">
      <el-col :span="4">
        <el-statistic title="总数" :value="agents.length" />
      </el-col>
      <el-col :span="4">
        <el-statistic title="在线">
          <template #default>
            <span style="color: #22c55e; font-weight: 600; font-size: 24px">{{ onlineCount }}</span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="4">
        <el-statistic title="离线">
          <template #default>
            <span style="color: #64748b; font-weight: 600; font-size: 24px">{{ offlineCount }}</span>
          </template>
        </el-statistic>
      </el-col>
    </el-row>

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
        <el-button :icon="Refresh" @click="fetchAgents" :loading="loading">
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
            <el-row :gutter="24">
              <!-- 基本信息 -->
              <el-col :span="12">
                <el-descriptions title="基本信息" :column="1" border size="small">
                  <el-descriptions-item label="Agent ID">{{ row.agent_id }}</el-descriptions-item>
                  <el-descriptions-item label="Agent Version">{{ row.agent_version || '-' }}</el-descriptions-item>
                  <el-descriptions-item label="Heartbeat Interval">
                    {{ row.heartbeat_interval ? row.heartbeat_interval + 's' : '-' }}
                  </el-descriptions-item>
                  <el-descriptions-item label="注册时间">
                    {{ formatTime(row.created_at) }}
                  </el-descriptions-item>
                  <el-descriptions-item label="最后心跳">
                    {{ formatTime(row.last_heartbeat_at) }}
                    <span v-if="row.last_heartbeat_at" style="color: var(--el-text-color-secondary); margin-left: 8px">
                      ({{ relativeTimeStr(row.last_heartbeat_at) }})
                    </span>
                  </el-descriptions-item>
                </el-descriptions>
              </el-col>

              <!-- 能力 & 槽位 -->
              <el-col :span="12">
                <el-descriptions title="能力 & 并发" :column="1" border size="small">
                  <el-descriptions-item label="能力标签">
                    <el-tag
                      v-for="cap in (row.capabilities || [])"
                      :key="cap"
                      :type="capabilityType(cap)"
                      size="small"
                      style="margin-right: 6px; margin-bottom: 4px"
                    >
                      {{ cap }}
                    </el-tag>
                    <span v-if="!row.capabilities || row.capabilities.length === 0">-</span>
                  </el-descriptions-item>
                  <el-descriptions-item label="并发槽位">
                    <template v-if="row.max_concurrent != null">
                      <div style="display: flex; align-items: center; gap: 12px; min-width: 200px">
                        <el-progress
                          :percentage="slotPercent(row)"
                          :color="slotColor(slotPercent(row))"
                          :stroke-width="16"
                          :text-inside="true"
                          style="flex: 1"
                        />
                        <span style="white-space: nowrap; font-size: 13px">{{ slotText(row) }}</span>
                      </div>
                    </template>
                    <span v-else style="color: var(--el-text-color-secondary)">未上报</span>
                  </el-descriptions-item>
                  <el-descriptions-item label="Labels">
                    <template v-if="row.labels && Object.keys(row.labels).length > 0">
                      <el-tag
                        v-for="(v, k) in row.labels"
                        :key="k"
                        size="small"
                        type="info"
                        effect="plain"
                        style="margin-right: 6px; margin-bottom: 4px"
                      >
                        {{ k }}={{ v }}
                      </el-tag>
                    </template>
                    <span v-else>-</span>
                  </el-descriptions-item>
                </el-descriptions>
              </el-col>
            </el-row>
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="device_code" label="Device Code" min-width="140" show-overflow-tooltip />
      <el-table-column prop="hostname" label="Hostname" min-width="120" show-overflow-tooltip />
      <el-table-column label="OS / Arch" width="130">
        <template #default="{ row }">{{ row.os }}/{{ row.arch }}</template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" width="140" />
      <el-table-column prop="external_ip" label="External IP" width="140" />
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <span>
            <span class="status-dot" :class="row.status === 'online' ? 'online' : 'offline'" />
            {{ row.status === 'online' ? '在线' : '离线' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="并发槽位" width="160" align="center">
        <template #default="{ row }">
          <template v-if="row.max_concurrent != null">
            <el-progress
              :percentage="slotPercent(row)"
              :color="slotColor(slotPercent(row))"
              :stroke-width="12"
              :text-inside="true"
              :format="() => slotText(row)"
              style="width: 100%"
            />
          </template>
          <span v-else style="color: var(--el-text-color-secondary); font-size: 12px">-</span>
        </template>
      </el-table-column>
      <el-table-column label="最后心跳" width="140">
        <template #default="{ row }">
          <el-tooltip v-if="row.last_heartbeat_at" :content="formatTime(row.last_heartbeat_at)" placement="top">
            <span>{{ relativeTimeStr(row.last_heartbeat_at) }}</span>
          </el-tooltip>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>
