<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Refresh, Search, Connection } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import client from '../../api/client'
import type { Envelope, DebugAgentItem } from '../../api/types'

dayjs.extend(relativeTime)

const loading = ref(false)
const agents = ref<DebugAgentItem[]>([])
const expandedRows = ref<string[]>([])

// 全量统计（不受筛选影响）
const statsTotal = ref(0)
const statsOnline = ref(0)
const statsOffline = ref(0)

const filter = reactive({
  search: '',
  status: '',
})

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
  if (percent >= 100) return '#ff4d4f'
  if (percent >= 75) return '#faad14'
  return '#52c41a'
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

    const hasFilter = filter.status || filter.search

    // 有筛选时并行请求：筛选列表 + 全量统计
    const requests = [
      client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents', { params }),
    ]
    if (hasFilter) {
      requests.push(client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents'))
    }

    const results = await Promise.all(requests)

    // 筛选后的列表用于表格
    agents.value = (results[0].data.data ?? []).sort((a, b) => a.device_code.localeCompare(b.device_code))

    // 统计始终基于全量数据
    const allAgents = hasFilter ? (results[1].data.data ?? []) : agents.value
    statsTotal.value = allAgents.length
    statsOnline.value = allAgents.filter((a) => a.status === 'online').length
    statsOffline.value = allAgents.filter((a) => a.status === 'offline').length
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
    <h2 class="page-title">
      <el-icon size="28"><Connection /></el-icon>
      Agents
    </h2>

    <!-- 统计概览 -->
    <el-row :gutter="16" class="stats-row">
      <el-col :xs="12" :sm="8" :md="6">
        <div class="stat-mini">
          <div class="stat-mini-value">{{ statsTotal }}</div>
          <div class="stat-mini-label">总数</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6">
        <div class="stat-mini online">
          <div class="stat-mini-value" style="color: #52c41a">{{ statsOnline }}</div>
          <div class="stat-mini-label">
            <span class="status-dot online" />在线
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="6">
        <div class="stat-mini offline">
          <div class="stat-mini-value" style="color: #8c8c8c">{{ statsOffline }}</div>
          <div class="stat-mini-label">
            <span class="status-dot offline" />离线
          </div>
        </div>
      </el-col>
    </el-row>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-input
        v-model="filter.search"
        placeholder="搜索 device_code"
        clearable
        :prefix-icon="Search"
        @clear="onSearch"
        @keyup.enter="onSearch"
        style="width: 260px"
      />
      <el-select v-model="filter.status" placeholder="状态筛选" clearable @change="onSearch" style="width: 140px">
        <el-option label="在线" value="online" />
        <el-option label="离线" value="offline" />
      </el-select>
      <div class="toolbar-spacer" />
      <el-button :icon="Refresh" @click="fetchAgents" :loading="loading">刷新</el-button>
    </div>

    <!-- 表格 -->
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
          <div class="expand-content">
            <el-row :gutter="24">
              <!-- 基本信息 -->
              <el-col :span="12">
                <el-descriptions title="基本信息" :column="1" border size="small">
                  <el-descriptions-item label="Agent ID">
                    <code>{{ row.agent_id }}</code>
                  </el-descriptions-item>
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
                      <div class="slot-progress">
                        <el-progress
                          :percentage="slotPercent(row)"
                          :color="slotColor(slotPercent(row))"
                          :stroke-width="16"
                          :text-inside="true"
                          style="flex: 1"
                        />
                        <span class="slot-text">{{ slotText(row) }}</span>
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
      <el-table-column label="OS / Arch" width="140">
        <template #default="{ row }">
          <el-tag size="small" effect="plain">{{ row.os }}/{{ row.arch }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="ip" label="IP" min-width="130" />
      <el-table-column prop="external_ip" label="External IP" min-width="130" />
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <div class="status-cell">
            <span class="status-dot" :class="row.status === 'online' ? 'online' : 'offline'" />
            <span>{{ row.status === 'online' ? '在线' : '离线' }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="并发槽位" width="160" align="center">
        <template #default="{ row }">
          <template v-if="row.max_concurrent != null">
            <el-progress
              :percentage="slotPercent(row)"
              :color="slotColor(slotPercent(row))"
              :stroke-width="10"
              :text-inside="true"
              :format="() => slotText(row)"
              style="width: 100%"
            />
          </template>
          <span v-else style="color: var(--el-text-color-secondary); font-size: 12px">-</span>
        </template>
      </el-table-column>
      <el-table-column label="最后心跳" width="170">
        <template #default="{ row }">
          <span v-if="row.last_heartbeat_at">{{ formatTime(row.last_heartbeat_at) }}</span>
          <span v-else>-</span>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<style scoped>
.stats-row {
  margin-bottom: 24px;
}

.stat-mini {
  background: #ffffff;
  border-radius: 12px;
  padding: 16px 20px;
  text-align: center;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  border: 1px solid var(--el-border-color-light);
  transition: all 0.3s ease;
}

.stat-mini:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.stat-mini-value {
  font-size: 24px;
  font-weight: 700;
  margin-bottom: 4px;
}

.stat-mini-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.expand-content {
  padding: 20px 28px;
  background: #fafafa;
  border-radius: 8px;
  margin: 8px;
}

.slot-progress {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 200px;
}

.slot-text {
  white-space: nowrap;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.status-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}
</style>
