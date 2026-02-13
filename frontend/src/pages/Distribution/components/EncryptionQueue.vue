<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { Refresh, CopyDocument, Promotion, Loading } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'
import client from '../../../api/client'
import type { Envelope, DistributionItem, DistributionListResp } from '../../../api/types'

// ---- State ----

const loading = ref(false)
const items = ref<DistributionItem[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

// 分发 dialog
const showDistDialog = ref(false)
const distTarget = ref<DistributionItem | null>(null)
const distForm = ref({ name: '', email: '', releaseNotes: '' })
const sending = ref(false)

// ---- Helpers ----

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('MM-DD HH:mm:ss')
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    pending: '排队中',
    encrypting: '加密中',
    uploaded: '待分发',
  }
  return map[status] || status
}

function statusType(status: string): '' | 'success' | 'warning' | 'info' | 'danger' {
  const map: Record<string, '' | 'success' | 'warning' | 'info' | 'danger'> = {
    pending: 'info',
    encrypting: 'warning',
    uploaded: 'success',
  }
  return map[status] || 'info'
}

// ---- API ----

async function fetchQueue() {
  loading.value = true
  try {
    const resp = await client.get<Envelope<DistributionListResp>>('/api/v1/distributions', {
      params: { status: 'pending,encrypting,uploaded', page: 1, page_size: 50, sort_by: 'created_at', sort_dir: 'desc' },
    })
    items.value = resp.data.data?.items ?? []
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

// ---- 分发操作 ----

function openDistDialog(row: DistributionItem) {
  distTarget.value = row
  distForm.value = {
    name: row.customer_name || '',
    email: row.customer_email || '',
    releaseNotes: row.release_notes || '',
  }
  showDistDialog.value = true
}

async function confirmDist() {
  if (!distTarget.value) return
  if (!distForm.value.name || !distForm.value.email) {
    ElMessage.warning('请填写客户名称和邮箱')
    return
  }
  sending.value = true
  try {
    // 先更新客户信息
    await client.put(`/api/v1/distributions/${distTarget.value.id}`, {
      customer_name: distForm.value.name,
      customer_email: distForm.value.email,
      release_notes: distForm.value.releaseNotes,
    })
    // 推进状态到 sent
    await client.patch(`/api/v1/distributions/${distTarget.value.id}/status`, { status: 'sent' })
    ElMessage.success('分发成功')
    showDistDialog.value = false
    fetchQueue()
  } catch (err: any) {
    ElMessage.error('分发失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    sending.value = false
  }
}

function copyUrl(row: DistributionItem) {
  if (row.presigned_url) {
    navigator.clipboard.writeText(row.presigned_url).then(() => ElMessage.success('链接已复制'))
  } else {
    ElMessage.info('暂无下载链接')
  }
}

// ---- Polling ----

function startPolling() {
  if (pollTimer) clearInterval(pollTimer)
  pollTimer = setInterval(fetchQueue, 3000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

// ---- Expose ----

function refresh() { fetchQueue() }
defineExpose({ refresh })

// ---- Lifecycle ----

onMounted(() => {
  fetchQueue()
  startPolling()
})

onUnmounted(() => {
  stopPolling()
})
</script>

<template>
  <div class="encryption-queue" v-if="items.length > 0 || loading">
    <div class="queue-header">
      <h3 class="queue-title">加密队列</h3>
      <el-button :icon="Refresh" @click="refresh" :loading="loading" size="small" text>刷新</el-button>
    </div>

    <div class="queue-list" v-loading="loading">
      <div v-for="item in items" :key="item.id" class="queue-item" :class="'status-' + item.status">
        <div class="item-main">
          <div class="item-left">
            <div class="item-file">
              <span class="file-name">{{ item.file_name }}</span>
              <el-tag :type="statusType(item.status)" size="small" effect="plain">
                <el-icon v-if="item.status === 'encrypting' || item.status === 'pending'" class="is-loading"><Loading /></el-icon>
                {{ statusLabel(item.status) }}
              </el-tag>
            </div>
            <div class="item-meta">
              <span class="task-id">{{ item.task_id }}</span>
              <span v-if="item.customer_name" class="customer">{{ item.customer_name }}</span>
              <span class="time">{{ formatTime(item.created_at) }}</span>
            </div>
          </div>
          <div class="item-actions">
            <template v-if="item.status === 'uploaded'">
              <el-button size="small" text :icon="CopyDocument" @click="copyUrl(item)">复制链接</el-button>
              <el-button size="small" type="primary" :icon="Promotion" @click="openDistDialog(item)">配置分发</el-button>
            </template>
            <template v-else>
              <el-tag type="info" size="small" effect="plain">{{ item.encryption_algo || 'AES-256' }}</el-tag>
            </template>
          </div>
        </div>
        <!-- 加密进度条 -->
        <el-progress
          v-if="item.status === 'pending' || item.status === 'encrypting'"
          :percentage="item.status === 'pending' ? 20 : 60"
          :striped="true"
          :striped-flow="true"
          :show-text="false"
          :stroke-width="4"
          style="margin-top: 8px;"
        />
      </div>
    </div>

    <!-- 分发 Dialog -->
    <el-dialog v-model="showDistDialog" title="配置客户分发" width="500px" :close-on-click-modal="false">
      <el-form label-position="top">
        <el-form-item label="加密文件">
          <el-input :model-value="distTarget?.file_name + '.enc'" disabled />
        </el-form-item>
        <el-form-item label="客户公司名称" required>
          <el-input v-model="distForm.name" placeholder="客户公司名称" />
        </el-form-item>
        <el-form-item label="接收邮箱" required>
          <el-input v-model="distForm.email" placeholder="primary@company.com" type="email" />
        </el-form-item>
        <el-form-item label="Release 说明">
          <el-input v-model="distForm.releaseNotes" type="textarea" :rows="3" placeholder="v2.4.1 安全补丁更新..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showDistDialog = false">取消</el-button>
        <el-button type="primary" :loading="sending" @click="confirmDist">
          <el-icon><Promotion /></el-icon> 确认分发
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.encryption-queue {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
}

.queue-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.queue-title {
  font-size: 16px;
  font-weight: 600;
  margin: 0;
  color: #303133;
}

.queue-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.queue-item {
  padding: 12px 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  transition: border-color 0.2s;
}

.queue-item.status-uploaded {
  border-color: #b3e19d;
  background: #f0f9eb;
}

.queue-item.status-encrypting {
  border-color: #f3d19e;
  background: #fdf6ec;
}

.item-main {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.item-left {
  flex: 1;
  min-width: 0;
}

.item-file {
  display: flex;
  align-items: center;
  gap: 8px;
}

.file-name {
  font-size: 14px;
  font-weight: 500;
  color: #303133;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-meta {
  display: flex;
  gap: 12px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.task-id {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  color: #409eff;
}

.item-actions {
  display: flex;
  gap: 4px;
  flex-shrink: 0;
  margin-left: 16px;
}
</style>
