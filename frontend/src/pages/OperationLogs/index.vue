<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
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

function actionTagType(action: string): string {
  switch (action) {
    case 'create': return 'success'
    case 'delete': return 'danger'
    case 'update': return 'warning'
    case 'assign_host': return ''
    case 'unassign_host': return 'info'
    default: return 'info'
  }
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
    <h2 class="page-title">操作日志</h2>

    <!-- Toolbar -->
    <el-row :gutter="12" style="margin-bottom: 16px" align="middle">
      <el-col :span="5">
        <el-select v-model="filter.resource_type" placeholder="资源类型" clearable @change="onSearch">
          <el-option v-for="o in resourceTypeOptions" :key="o.value" :label="o.label" :value="o.value" />
        </el-select>
      </el-col>
      <el-col :span="5">
        <el-select v-model="filter.action" placeholder="操作类型" clearable @change="onSearch">
          <el-option v-for="o in actionOptions" :key="o.value" :label="o.label" :value="o.value" />
        </el-select>
      </el-col>
      <el-col :span="2">
        <el-button :icon="Refresh" @click="fetchLogs" :loading="loading">刷新</el-button>
      </el-col>
    </el-row>

    <!-- Table -->
    <el-table :data="logs" v-loading="loading" row-key="log_id" border stripe style="width: 100%">
      <el-table-column prop="log_id" label="日志ID" width="80" align="center" />
      <el-table-column label="资源类型" width="120" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="row.resource_type === 'customer' ? 'primary' : 'success'" effect="plain">
            {{ resourceTypeLabel(row.resource_type) }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="resource_id" label="资源ID" min-width="140" show-overflow-tooltip />
      <el-table-column label="操作类型" width="120" align="center">
        <template #default="{ row }">
          <el-tag :type="actionTagType(row.action)" size="small" effect="plain">{{ actionLabel(row.action) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作人" width="120" show-overflow-tooltip>
        <template #default="{ row }">{{ row.operator || '-' }}</template>
      </el-table-column>
      <el-table-column label="操作详情" min-width="120">
        <template #default="{ row }">
          <el-popover trigger="click" width="480" placement="left">
            <template #reference>
              <el-button link type="primary" size="small">查看详情</el-button>
            </template>
            <pre class="detail-json">{{ formatDetail(row.detail) }}</pre>
          </el-popover>
        </template>
      </el-table-column>
      <el-table-column label="操作时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <div style="margin-top: 16px; display: flex; justify-content: flex-end">
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
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  line-height: 1.5;
  max-height: 400px;
  overflow: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
