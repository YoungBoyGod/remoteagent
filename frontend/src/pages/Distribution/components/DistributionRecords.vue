<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import client from '../../../api/client'
import type { Envelope } from '../../../api/types'

// ---- Types (aligned with backend DistributionItem) ----

interface DistributionItem {
  id: number
  task_id: string
  file_name: string
  file_size: number
  encrypted_file_path?: string
  sha256_original: string
  sha256_encrypted?: string
  encryption_algo: string
  customer_name: string
  customer_email: string
  session_key_hash?: string
  presigned_url?: string
  url_expires_at?: number | null
  status: string
  download_ip?: string
  download_at?: number | null
  release_notes?: string
  scheduled_at?: number | null
  created_at: number
  updated_at: number
}

interface DistributionListResp {
  total: number
  page: number
  page_size: number
  items: DistributionItem[]
}

// ---- State ----

const loading = ref(false)
const records = ref<DistributionItem[]>([])
const total = ref(0)
const expandedRows = ref<number[]>([])
const activeStatus = ref('')

const pagination = reactive({
  page: 1,
  page_size: 10,
})

const statusTabs = [
  { label: '全部', value: '' },
  { label: '进行中', value: 'pending,encrypting,uploading' },
  { label: '已完成', value: 'uploaded,sent' },
  { label: '失败', value: 'failed' },
]

const statusMap: Record<string, { type: '' | 'success' | 'warning' | 'info' | 'danger'; label: string }> = {
  pending:    { type: 'info',    label: '排队中' },
  encrypting: { type: 'warning', label: '加密中' },
  uploading:  { type: 'warning', label: '上传中' },
  uploaded:   { type: 'success', label: '已上传' },
  sent:       { type: '',        label: '已发送' },
  expired:    { type: 'info',    label: '已过期' },
  failed:     { type: 'danger',  label: '失败' },
}

// ---- Helpers ----

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('YYYY-MM-DD HH:mm:ss')
}

function formatSize(bytes: number | null | undefined): string {
  if (!bytes || bytes <= 0) return '-'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(i > 0 ? 2 : 0)} ${units[i]}`
}

function getStatusTag(status: string) {
  return statusMap[status] || { type: 'info' as const, label: status }
}

// ---- API ----

async function fetchRecords() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.page,
      page_size: pagination.page_size,
      sort_by: 'created_at',
      sort_dir: 'desc',
    }
    if (activeStatus.value) {
      params.status = activeStatus.value
    }
    const resp = await client.get<Envelope<DistributionListResp>>('/api/v1/distributions', { params })
    const data = resp.data.data
    records.value = data?.items ?? []
    total.value = data?.total ?? 0
  } catch {
    records.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

async function resend(row: DistributionItem) {
  try {
    await ElMessageBox.confirm(
      `确定重新发送给 ${row.customer_name || row.customer_email}？`,
      '确认重新发送',
      { type: 'warning' },
    )
    await client.patch(`/api/v1/distributions/${row.id}/status`, { status: 'sent' })
    ElMessage.success('已重新发送')
    fetchRecords()
  } catch { /* cancelled or error */ }
}

// ---- Events ----

function handleTabChange() {
  pagination.page = 1
  expandedRows.value = []
  fetchRecords()
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchRecords()
}

function handleSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  fetchRecords()
}

function handleExpandChange(_row: DistributionItem, expanded: DistributionItem[]) {
  expandedRows.value = expanded.map((r) => r.id)
}

// ---- Expose ----

function refresh() {
  pagination.page = 1
  fetchRecords()
}

defineExpose({ refresh })

// ---- Init ----

onMounted(() => {
  fetchRecords()
})
</script>

<template>
  <div class="distribution-records">
    <!-- Header -->
    <div class="records-header">
      <h3 class="records-title">历史记录</h3>
      <el-button :icon="Refresh" @click="refresh" :loading="loading" size="small">刷新</el-button>
    </div>

    <!-- Status Tabs -->
    <el-tabs v-model="activeStatus" @tab-change="handleTabChange" class="status-tabs">
      <el-tab-pane
        v-for="tab in statusTabs"
        :key="tab.value"
        :label="tab.label"
        :name="tab.value"
      />
    </el-tabs>

    <!-- Table -->
    <el-table
      :data="records"
      v-loading="loading"
      row-key="id"
      :expand-row-keys="expandedRows"
      @expand-change="handleExpandChange"
      border
      stripe
      style="width: 100%"
    >
      <!-- Expand -->
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-detail">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="分发 ID">{{ row.id }}</el-descriptions-item>
              <el-descriptions-item label="任务 ID">{{ row.task_id || '-' }}</el-descriptions-item>
              <el-descriptions-item label="加密算法">{{ row.encryption_algo || 'AES-256' }}</el-descriptions-item>
              <el-descriptions-item label="文件大小">{{ formatSize(row.file_size) }}</el-descriptions-item>
              <el-descriptions-item label="原始文件 SHA-256" :span="2">
                <code class="hash-text">{{ row.sha256_original || '-' }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="加密文件 SHA-256" :span="2">
                <code class="hash-text">{{ row.sha256_encrypted || '-' }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="下载 IP">{{ row.download_ip || '-' }}</el-descriptions-item>
              <el-descriptions-item label="下载时间">{{ formatTime(row.download_at) }}</el-descriptions-item>
              <el-descriptions-item label="计划分发时间">{{ formatTime(row.scheduled_at) }}</el-descriptions-item>
              <el-descriptions-item label="更新时间">{{ formatTime(row.updated_at) }}</el-descriptions-item>
              <el-descriptions-item v-if="row.release_notes" label="Release 说明" :span="2">
                {{ row.release_notes }}
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </template>
      </el-table-column>

      <!-- Columns -->
      <el-table-column label="任务ID" min-width="100" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="task-id-text">{{ row.task_id || `#${row.id}` }}</span>
        </template>
      </el-table-column>

      <el-table-column prop="file_name" label="文件名" min-width="180" show-overflow-tooltip />

      <el-table-column prop="customer_name" label="客户名称" min-width="120" show-overflow-tooltip>
        <template #default="{ row }">{{ row.customer_name || '-' }}</template>
      </el-table-column>

      <el-table-column prop="customer_email" label="邮箱" min-width="180" show-overflow-tooltip>
        <template #default="{ row }">{{ row.customer_email || '-' }}</template>
      </el-table-column>

      <el-table-column label="时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>

      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="getStatusTag(row.status).type" size="small">
            {{ getStatusTag(row.status).label }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column label="操作" width="100" align="center" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status === 'sent' || row.status === 'expired' || row.status === 'uploaded'"
            size="small"
            type="primary"
            @click="resend(row)"
          >重发</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>
  </div>
</template>

<style scoped>
.distribution-records {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
}

.records-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.records-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: #303133;
}

.status-tabs {
  margin-bottom: 16px;
}

.status-tabs :deep(.el-tabs__item) {
  padding: 0 20px !important;
  font-weight: 500;
  height: 36px;
  line-height: 36px;
  min-width: 80px;
  text-align: center;
}

.status-tabs :deep(.el-tabs__nav) {
  border-radius: 6px;
  background: #f5f7fa;
  padding: 4px;
  height: 36px;
  display: flex;
  align-items: center;
}

.status-tabs :deep(.el-tabs__item.is-active) {
  background: #409eff;
  color: #fff;
  border-radius: 4px;
}

.status-tabs :deep(.el-tabs__item:hover:not(.is-active)) {
  background: #e8f0fe;
  color: #409eff;
}

.expand-detail {
  padding: 16px 24px;
  background: #fafafa;
  border-radius: 8px;
  margin: 8px;
}

.hash-text {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  word-break: break-all;
  color: #606266;
}

.task-id-text {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  color: #409eff;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
  padding: 8px 0;
}
</style>
