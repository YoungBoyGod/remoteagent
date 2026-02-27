<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { CopyDocument, Promotion, Loading, Check } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import dayjs from 'dayjs'
import client from '../../../api/client'
import type { Envelope, DistributionItem, DistributionListResp } from '../../../api/types'

// ---- State ----

const loading = ref(false)
const items = ref<DistributionItem[]>([])
let pollTimer: ReturnType<typeof setInterval> | null = null

const editingId = ref<number | null>(null)
const editForm = ref({ releaseNotes: '', name: '', email: '' })
const saving = ref(false)

// ---- 立即发布 ----
const publishVisible = ref(false)
const publishForm = ref({ itemId: null as number | null, releaseNotes: '', name: '', email: '' })
const publishing = ref(false)

// 所有已上传的文件（含已写发布说明的）
const allUploadedItems = computed(() =>
  items.value.filter(i => i.status === 'uploaded')
)

function openPublishDialog() {
  publishForm.value = { itemId: null, releaseNotes: '', name: '', email: '' }
  publishVisible.value = true
}

async function submitPublish() {
  const { itemId, releaseNotes, name, email } = publishForm.value
  if (!itemId) { ElMessage.warning('请选择文件'); return }
  if (!releaseNotes.trim()) { ElMessage.warning('请填写发布说明'); return }
  if (!name || !email) { ElMessage.warning('请填写客户名称和邮箱'); return }
  publishing.value = true
  try {
    await client.put(`/api/v1/distributions/${itemId}`, {
      release_notes: releaseNotes,
      customer_name: name,
      customer_email: email,
    })
    await client.patch(`/api/v1/distributions/${itemId}/status`, { status: 'sent' })
    ElMessage.success('发布成功')
    publishVisible.value = false
    fetchQueue()
  } catch (err: any) {
    ElMessage.error('发布失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    publishing.value = false
  }
}

// ---- 按阶段分组 ----

const encryptingItems = computed(() =>
  items.value.filter(i => i.status === 'pending' || i.status === 'encrypting')
)
const uploadedItems = computed(() =>
  items.value.filter(i => i.status === 'uploaded' && !i.release_notes)
)
const releaseItems = computed(() =>
  items.value.filter(i => i.status === 'uploaded' && !!i.release_notes)
)

// ---- Helpers ----

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('MM-DD HH:mm:ss')
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

// ---- API ----

async function fetchQueue(showLoading = false) {
  if (showLoading) loading.value = true
  try {
    const resp = await client.get<Envelope<DistributionListResp>>('/api/v1/distributions', {
      params: { status: 'pending,encrypting,uploaded', page: 1, page_size: 50, sort_by: 'created_at', sort_dir: 'desc' },
    })
    items.value = resp.data.data?.items ?? []
  } catch {
    items.value = []
  } finally {
    if (showLoading) loading.value = false
  }
}

// ---- 操作 ----

function startEditRelease(row: DistributionItem) {
  editingId.value = row.id
  editForm.value = { releaseNotes: row.release_notes || '', name: row.customer_name || '', email: row.customer_email || '' }
}

async function saveRelease(row: DistributionItem) {
  if (!editForm.value.releaseNotes.trim()) {
    ElMessage.warning('请填写发布说明')
    return
  }
  saving.value = true
  try {
    await client.put(`/api/v1/distributions/${row.id}`, { release_notes: editForm.value.releaseNotes })
    ElMessage.success('发布说明已保存')
    editingId.value = null
    fetchQueue()
  } catch (err: any) {
    ElMessage.error('保存失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

function startEditCustomer(row: DistributionItem) {
  editingId.value = row.id
  editForm.value = { releaseNotes: row.release_notes || '', name: row.customer_name || '', email: row.customer_email || '' }
}

async function confirmSend(row: DistributionItem) {
  if (!editForm.value.name || !editForm.value.email) {
    ElMessage.warning('请填写客户名称和邮箱')
    return
  }
  saving.value = true
  try {
    await client.put(`/api/v1/distributions/${row.id}`, {
      customer_name: editForm.value.name,
      customer_email: editForm.value.email,
    })
    await client.patch(`/api/v1/distributions/${row.id}/status`, { status: 'sent' })
    ElMessage.success('分发成功')
    editingId.value = null
    fetchQueue()
  } catch (err: any) {
    ElMessage.error('分发失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

function copyUrl(row: DistributionItem) {
  if (row.presigned_url) {
    navigator.clipboard.writeText(row.presigned_url).then(() => ElMessage.success('链接已复制'))
  } else {
    ElMessage.info('暂无下载链接')
  }
}

function cancelEdit() { editingId.value = null }

function scrollToUploadQueue() {
  document.getElementById('upload-queue-section')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
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

function refresh() { fetchQueue(true) }
defineExpose({ refresh })

onMounted(() => { fetchQueue(true); startPolling() })
onUnmounted(() => { stopPolling() })
</script>

<template>
  <div>
  <div class="queue-sections" v-if="items.length > 0 || loading" v-loading="loading">

    <!-- ========== 1. 加密队列 ========== -->
    <div v-if="encryptingItems.length > 0 || uploadedItems.length > 0" class="queue-section encrypt-section">
      <div class="section-header">
        <h3 class="section-title">
          <el-icon :class="encryptingItems.length > 0 ? 'is-loading' : ''" color="#e6a23c"><Loading /></el-icon>
          加密队列
        </h3>
        <el-tag type="warning" size="small" effect="plain">{{ encryptingItems.length + uploadedItems.length }} 项</el-tag>
      </div>
      <!-- 进行中 -->
      <div v-for="item in encryptingItems" :key="item.id" class="encrypt-row">
        <span class="encrypt-name">{{ item.file_name }}</span>
        <el-tag :type="item.status === 'pending' ? 'info' : 'warning'" size="small" effect="plain">
          {{ item.status === 'pending' ? '排队中' : '加密中' }}
        </el-tag>
        <el-progress
          :percentage="item.status === 'pending' ? 20 : 60"
          :striped="true" :striped-flow="true" :show-text="false" :stroke-width="4"
          class="encrypt-progress"
        />
      </div>
      <!-- 已完成 -->
      <div v-for="item in uploadedItems" :key="'done-' + item.id" class="encrypt-done-row">
        <div class="done-top">
          <span class="encrypt-name">{{ item.file_name }}</span>
          <el-tag type="success" size="small" effect="plain">加密完成</el-tag>
        </div>
        <div class="done-meta">
          <span v-if="item.file_size" class="done-detail">{{ formatSize(item.file_size) }}</span>
          <span class="done-detail">{{ item.encryption_algo || 'AES-256' }}</span>
          <span class="done-detail">{{ formatTime(item.created_at) }}</span>
        </div>
        <div v-if="item.sha256_original" class="done-hash">
          <span class="hash-label">SHA-256:</span>
          <code class="hash-value">{{ item.sha256_original }}</code>
        </div>
        <div class="done-action">
          <el-button size="small" type="primary" @click="scrollToUploadQueue">
            编写发布说明
          </el-button>
          <el-button v-if="item.presigned_url" size="small" type="info" @click="copyUrl(item)">复制链接</el-button>
        </div>
      </div>
    </div>

    <!-- ========== 2. 上传队列（待编写发布说明） ========== -->
    <div v-if="uploadedItems.length > 0" id="upload-queue-section" class="queue-section">
      <div class="section-header">
        <h3 class="section-title">
          <el-icon color="#67c23a"><Check /></el-icon>
          上传队列
        </h3>
        <el-tag type="success" size="small" effect="plain">{{ uploadedItems.length }} 项</el-tag>
      </div>
      <div class="section-list">
        <div v-for="item in uploadedItems" :key="item.id" class="queue-card uploaded">
          <div class="card-top">
            <span class="file-name">{{ item.file_name }}</span>
            <div class="card-top-actions">
              <el-button v-if="item.presigned_url" size="small" text :icon="CopyDocument" @click="copyUrl(item)">复制链接</el-button>
              <el-tag type="success" size="small" effect="plain">加密完成</el-tag>
            </div>
          </div>
          <div class="card-meta">
            <span class="task-id">{{ item.task_id }}</span>
            <span v-if="item.file_size" class="file-size">{{ formatSize(item.file_size) }}</span>
            <span class="time">{{ formatTime(item.created_at) }}</span>
          </div>
          <div v-if="item.sha256_original" class="card-hash">
            <span class="hash-label">SHA-256:</span>
            <code class="hash-value">{{ item.sha256_original }}</code>
          </div>
          <!-- 编写发布说明 -->
          <div class="card-action">
            <template v-if="editingId === item.id">
              <el-input v-model="editForm.releaseNotes" type="textarea" :rows="2"
                placeholder="请填写发布说明，如：v2.4.1 安全补丁更新..." style="margin-bottom: 8px;" />
              <div class="action-btns">
                <el-button size="small" @click="cancelEdit">取消</el-button>
                <el-button size="small" type="primary" :loading="saving" @click="saveRelease(item)">保存</el-button>
              </div>
            </template>
            <template v-else>
              <el-button size="small" type="primary" @click="startEditRelease(item)">编写发布说明</el-button>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- ========== 3. 编写发布队列（待选择客户分发） ========== -->
    <div v-if="releaseItems.length > 0" class="queue-section">
      <div class="section-header">
        <h3 class="section-title">
          <el-icon color="#409eff"><Promotion /></el-icon>
          发布队列
        </h3>
        <el-tag type="primary" size="small" effect="plain">{{ releaseItems.length }} 项</el-tag>
      </div>
      <div class="section-list">
        <div v-for="item in releaseItems" :key="item.id" class="queue-card release">
          <div class="card-top">
            <span class="file-name">{{ item.file_name }}</span>
            <div class="card-top-actions">
              <el-button v-if="item.presigned_url" size="small" type="info" @click="copyUrl(item)">复制链接</el-button>
              <el-tag type="primary" size="small" effect="plain">待分发</el-tag>
            </div>
          </div>
          <div class="card-meta">
            <span class="task-id">{{ item.task_id }}</span>
            <span v-if="item.file_size" class="file-size">{{ formatSize(item.file_size) }}</span>
            <span class="time">{{ formatTime(item.created_at) }}</span>
          </div>
          <div v-if="item.sha256_original" class="card-hash">
            <span class="hash-label">SHA-256:</span>
            <code class="hash-value">{{ item.sha256_original }}</code>
          </div>
          <div class="release-preview">
            <span class="release-label">发布说明：</span>{{ item.release_notes }}
          </div>
          <!-- 选择客户并分发 -->
          <div class="card-action">
            <template v-if="editingId === item.id">
              <el-row :gutter="12" style="margin-bottom: 8px;">
                <el-col :span="10">
                  <el-input v-model="editForm.name" placeholder="客户公司名称" size="small" />
                </el-col>
                <el-col :span="10">
                  <el-input v-model="editForm.email" placeholder="接收邮箱" size="small" type="email" />
                </el-col>
                <el-col :span="4">
                  <el-button size="small" @click="cancelEdit">取消</el-button>
                </el-col>
              </el-row>
              <div class="action-btns">
                <el-button size="small" type="primary" :loading="saving" @click="confirmSend(item)">确认分发</el-button>
              </div>
            </template>
            <template v-else>
              <el-button size="small" type="primary" @click="startEditCustomer(item)">选择客户并分发</el-button>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- 操作栏 -->
    <div class="queue-refresh">
      <el-button type="info" @click="refresh" :loading="loading" size="small">刷新队列</el-button>
      <el-button type="primary" @click="openPublishDialog" size="small"
        :disabled="allUploadedItems.length === 0">立即发布</el-button>
    </div>
  </div>

  <!-- 立即发布弹窗 -->
  <el-dialog v-model="publishVisible" title="立即发布" width="560px" :close-on-click-modal="false">
    <el-form label-position="top">
      <el-form-item label="选择加密文件">
        <el-select v-model="publishForm.itemId" placeholder="请选择已加密完成的文件" style="width: 100%;">
          <el-option
            v-for="item in allUploadedItems"
            :key="item.id"
            :value="item.id"
            :label="item.file_name"
          >
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <span>{{ item.file_name }}</span>
              <span style="font-size: 12px; color: #909399;">{{ formatSize(item.file_size) }}</span>
            </div>
          </el-option>
        </el-select>
      </el-form-item>
      <el-form-item label="客户名称">
        <el-input v-model="publishForm.name" placeholder="客户公司名称" />
      </el-form-item>
      <el-form-item label="客户邮箱">
        <el-input v-model="publishForm.email" placeholder="接收邮箱" type="email" />
      </el-form-item>
      <el-form-item label="发布说明">
        <el-input v-model="publishForm.releaseNotes" type="textarea" :rows="3"
          placeholder="请填写发布说明，如：v2.4.1 安全补丁更新..." />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="publishVisible = false">取消</el-button>
      <el-button type="primary" :loading="publishing" :icon="Promotion" @click="submitPublish">确认发布</el-button>
    </template>
  </el-dialog>
  </div>
</template>

<style scoped>
.queue-sections {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.queue-section {
  background: #fff;
  border-radius: 8px;
  padding: 16px 20px;
  box-shadow: 0 2px 4px rgba(0,0,0,0.05);
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.section-title {
  font-size: 15px;
  font-weight: 600;
  margin: 0;
  color: #303133;
  display: flex;
  align-items: center;
  gap: 6px;
}

.section-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* 加密队列简化样式 */
.encrypt-section {
  padding-bottom: 12px;
}

.encrypt-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 6px 0;
}

.encrypt-row + .encrypt-row {
  border-top: 1px solid #f2f3f5;
}

.encrypt-name {
  font-size: 13px;
  color: #303133;
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.encrypt-progress {
  width: 120px;
  flex-shrink: 0;
}

/* 加密完成项 */
.encrypt-done-row {
  padding: 10px 0;
  border-top: 1px solid #f2f3f5;
}

.done-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.done-meta {
  display: flex;
  gap: 12px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.done-detail {
  white-space: nowrap;
}

.done-hash {
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
}

.done-action {
  margin-top: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}

/* 卡片 */
.queue-card {
  padding: 12px 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  transition: all 0.2s;
}

.queue-card:hover {
  border-color: #dcdfe6;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
}

.queue-card.encrypting {
  border-left: 3px solid #e6a23c;
  background: #fffbf0;
}

.queue-card.uploaded {
  border-left: 3px solid #67c23a;
  background: #f0f9eb;
}

.queue-card.release {
  border-left: 3px solid #409eff;
  background: #ecf5ff;
}

.card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-top-actions {
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

.card-meta {
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

.card-hash {
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 11px;
}

.hash-label {
  color: #909399;
  flex-shrink: 0;
}

.hash-value {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 11px;
  color: #606266;
  word-break: break-all;
  background: rgba(0,0,0,0.04);
  padding: 1px 4px;
  border-radius: 3px;
}

.release-preview {
  font-size: 13px;
  color: #606266;
  background: rgba(0,0,0,0.03);
  padding: 6px 10px;
  border-radius: 4px;
  margin-top: 8px;
}

.release-label {
  color: #909399;
  font-size: 12px;
}

.card-action {
  margin-top: 10px;
  padding-top: 10px;
  border-top: 1px dashed #ebeef5;
}

.action-btns {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.queue-refresh {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-top: 8px;
}
</style>
