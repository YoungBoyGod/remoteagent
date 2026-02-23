<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Refresh, Document, Clock } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import client from '../../api/client'
import type { Envelope, OperationLogItem, OperationLogListResp } from '../../api/types'

const loading = ref(false)
const logs = ref<OperationLogItem[]>([])
const total = ref(0)

const filter = reactive({
  resource_type: '',
  action: '',
})

const pagination = reactive({
  page: 1,
  page_size: 20,
})

const resourceTypeOptions = [
  { label: '客户', value: 'customer' },
  { label: '主机分配', value: 'host_assign' },
]

const actionOptions = [
  { label: '创建', value: 'create' },
  { label: '更新', value: 'update' },
  { label: '删除', value: 'delete' },
  { label: '分配主机', value: 'assign_host' },
  { label: '回收主机', value: 'unassign_host' },
]

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('YYYY-MM-DD HH:mm:ss')
}

function resourceTypeLabel(val: string): string {
  const found = resourceTypeOptions.find((o) => o.value === val)
  return found ? found.label : val
}

function actionLabel(val: string): string {
  const found = actionOptions.find((o) => o.value === val)
  return found ? found.label : val
}



function formatDetail(detail: Record<string, unknown>): string {
  try {
    return JSON.stringify(detail, null, 2)
  } catch {
    return String(detail)
  }
}

async function fetchLogs() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (filter.resource_type) params.resource_type = filter.resource_type
    if (filter.action) params.action = filter.action

    const resp = await client.get<Envelope<OperationLogListResp>>('/api/v1/operation-logs', { params })
    const data = resp.data.data
    logs.value = data?.items ?? []
    total.value = data?.total ?? 0
  } catch {
    logs.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onSearch() {
  pagination.page = 1
  fetchLogs()
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchLogs()
}

function handleSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  fetchLogs()
}

onMounted(() => {
  fetchLogs()
})
</script>

<template>
  <div>
    <h2 class="page-title">
      <el-icon size="28"><Document /></el-icon>
      操作日志
    </h2>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-select v-model="filter.resource_type" placeholder="资源类型" clearable @change="onSearch" style="width: 160px">
        <el-option v-for="o in resourceTypeOptions" :key="o.value" :label="o.label" :value="o.value" />
      </el-select>
      <el-select v-model="filter.action" placeholder="操作类型" clearable @change="onSearch" style="width: 160px">
        <el-option v-for="o in actionOptions" :key="o.value" :label="o.label" :value="o.value" />
      </el-select>
      <div class="toolbar-spacer" />
      <el-button :icon="Refresh" @click="fetchLogs" :loading="loading">刷新</el-button>
    </div>

    <!-- 表格 -->
    <el-table :data="logs" v-loading="loading" row-key="log_id" border stripe style="width: 100%">
      <el-table-column prop="log_id" label="日志ID" width="80" align="center" />
      <el-table-column label="资源类型" width="100" align="center">
        <template #default="{ row }">
          <span class="resource-type" :class="row.resource_type">
            {{ resourceTypeLabel(row.resource_type) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column prop="resource_id" label="资源ID" min-width="140" show-overflow-tooltip />
      <el-table-column label="操作类型" width="90" align="center">
        <template #default="{ row }">
          <span class="action-type" :class="row.action">
            {{ actionLabel(row.action) }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="操作人" width="120" show-overflow-tooltip>
        <template #default="{ row }">{{ row.operator || '-' }}</template>
      </el-table-column>
      <el-table-column label="操作详情" min-width="120">
        <template #default="{ row }">
          <el-popover trigger="click" width="520" placement="left">
            <template #reference>
              <el-button link type="primary" size="small">查看详情</el-button>
            </template>
            <pre class="detail-json">{{ formatDetail(row.detail) }}</pre>
          </el-popover>
        </template>
      </el-table-column>
      <el-table-column label="操作时间" width="170">
        <template #default="{ row }">
          <div class="time-cell">
            <el-icon style="color: var(--el-text-color-secondary)"><Clock /></el-icon>
            {{ formatTime(row.created_at) }}
          </div>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>
  </div>
</template>

<style scoped>
.detail-json {
  margin: 0;
  padding: 16px;
  background: #1e1e1e;
  color: #d4d4d4;
  border-radius: 8px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 12px;
  line-height: 1.6;
  max-height: 400px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.time-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* 资源类型 - 无边框 */
.resource-type {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.resource-type.customer {
  color: #4096ff;
  background: #eaf6ff;
}

.resource-type.host_assign {
  color: #52c41a;
  background: #f6ffed;
}

/* 操作类型 - 无边框 */
.action-type {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.action-type.create {
  color: #52c41a;
  background: #f6ffed;
}

.action-type.update {
  color: #faad14;
  background: #fffbe6;
}

.action-type.delete {
  color: #ff4d4f;
  background: #fff2f0;
}

.action-type.assign_host {
  color: #4096ff;
  background: #eaf6ff;
}

.action-type.unassign_host {
  color: #8c8c8c;
  background: #f5f5f5;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
  padding: 16px 0;
}
</style>
