<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  Upload, Lock, UploadFilled, Promotion, Check,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import client from '../../api/client'
import { createDistribution, listDistributionS3Objects } from '../../api/distribution'
import { listCustomers } from '../../api/customer'
import type {
  DistributionS3ObjectItem,
  CustomerItem,
  ReleaseNoteItem,
  ReleaseNoteListResp,
  Envelope,
} from '../../api/types'
import EncryptionQueue from './components/EncryptionQueue.vue'
import ReleaseNoteCreator from './components/ReleaseNoteCreator.vue'
import ReleaseNotes from './components/ReleaseNotes.vue'
import DistributionRecords from './components/DistributionRecords.vue'

interface SelectedSourceFile {
  name: string
  size: string
  bytes: number
  sha256: string
  sourceType: 's3' | 'local'
  s3Key?: string
}

const sourceMode = ref<'s3' | 'local'>('s3')
const createTab = ref('new-task')
const flowTab = ref('encrypt-queue')
const s3Path = ref('s3://releases/')
const selectedFile = ref<SelectedSourceFile | null>(null)
const hashing = ref(false)
const browsingS3 = ref(false)
const loadingMoreS3 = ref(false)

const s3Files = ref<DistributionS3ObjectItem[]>([])
const s3NextToken = ref('')
const s3HasMore = ref(false)
const s3Error = ref('')

const queueRef = ref<InstanceType<typeof EncryptionQueue> | null>(null)

const customers = ref<CustomerItem[]>([])
const loadingCustomers = ref(false)
const releaseNotes = ref<ReleaseNoteItem[]>([])
const loadingReleaseNotes = ref(false)
const submittingDistribution = ref(false)

const distributionForm = ref({
  customerId: '',
  receiveMethod: 'email' as 'email' | 'portal',
  releaseNoteId: 0,
  scheduledAt: '',
})

const selectedCustomer = computed(() => customers.value.find(c => c.customer_id === distributionForm.value.customerId) || null)
const selectedReleaseNote = computed(() => releaseNotes.value.find(n => n.id === distributionForm.value.releaseNoteId) || null)
const canCreateDistributionTask = computed(() => !!selectedFile.value && !!selectedCustomer.value && !!selectedReleaseNote.value && !submittingDistribution.value)

async function fetchCustomers() {
  loadingCustomers.value = true
  try {
    const resp = await listCustomers({ page: 1, page_size: 200, status: 'active' })
    customers.value = resp.items ?? []
  } catch (err: any) {
    ElMessage.error('读取客户列表失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
    customers.value = []
  } finally {
    loadingCustomers.value = false
  }
}

async function fetchReleaseNotes() {
  loadingReleaseNotes.value = true
  try {
    const resp = await client.get<Envelope<ReleaseNoteListResp>>('/api/v1/release-notes', {
      params: { page: 1, page_size: 200, sort_by: 'updated_at', sort_dir: 'desc' },
    })
    releaseNotes.value = resp.data.data?.items ?? []
  } catch (err: any) {
    ElMessage.error('读取发布说明失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
    releaseNotes.value = []
  } finally {
    loadingReleaseNotes.value = false
  }
}

function setSourceMode(mode: 's3' | 'local') {
  sourceMode.value = mode
  selectedFile.value = null
  s3Error.value = ''
  if (mode === 's3') {
    browseS3(true)
  }
}

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 Bytes'
  const units = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + units[i]
}

function formatS3Time(unix: number): string {
  if (!unix || unix <= 0) return '-'
  return new Date(unix * 1000).toLocaleString()
}

function normalizeS3Prefix(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return 'releases/'
  let cleaned = trimmed
  if (cleaned.startsWith('s3://')) {
    cleaned = cleaned.slice(5)
    const slashIdx = cleaned.indexOf('/')
    cleaned = slashIdx >= 0 ? cleaned.slice(slashIdx + 1) : ''
  }
  cleaned = cleaned.replace(/^\/+/, '')
  if (!cleaned) cleaned = 'releases/'
  if (!cleaned.startsWith('releases/')) {
    cleaned = `releases/${cleaned}`
  }
  if (!cleaned.endsWith('/')) {
    cleaned += '/'
  }
  return cleaned
}

function normalizeAndApplyS3Prefix(): string {
  const normalized = normalizeS3Prefix(s3Path.value)
  s3Path.value = normalized
  return normalized
}

async function computeSHA256(file: File): Promise<string> {
  const buffer = await file.arrayBuffer()
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
  const hashArray = Array.from(new Uint8Array(hashBuffer))
  return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
}

function selectS3File(file: DistributionS3ObjectItem) {
  selectedFile.value = selectedFile.value?.s3Key === file.key
    ? null
    : {
      name: file.key.split('/').pop() || file.key,
      size: formatBytes(file.size),
      bytes: file.size,
      sha256: '',
      sourceType: 's3',
      s3Key: file.key,
    }
}

async function browseS3(reset = true) {
  if (sourceMode.value !== 's3') {
    return
  }
  if (reset) {
    browsingS3.value = true
    s3Error.value = ''
  } else {
    loadingMoreS3.value = true
  }
  try {
    const normalizedPrefix = normalizeAndApplyS3Prefix()
    const resp = await listDistributionS3Objects({
      prefix: normalizedPrefix,
      page_size: 50,
      continuation_token: reset ? '' : s3NextToken.value,
    })
    if (reset) {
      s3Files.value = resp.items ?? []
    } else {
      s3Files.value = [...s3Files.value, ...(resp.items ?? [])]
    }
    s3HasMore.value = !!resp.has_more
    s3NextToken.value = resp.next_token || ''
    if (reset) {
      selectedFile.value = null
    }
  } catch (err: any) {
    const msg = err?.response?.data?.message || err.message || '未知错误'
    s3Error.value = msg
    if (reset) {
      s3Files.value = []
      s3HasMore.value = false
      s3NextToken.value = ''
    }
    ElMessage.error('读取 S3 列表失败: ' + msg)
  } finally {
    browsingS3.value = false
    loadingMoreS3.value = false
  }
}

async function handleLocalUpload(uploadFile: any) {
  const f: File = uploadFile.raw || uploadFile
  const bytes = f.size
  const size = formatBytes(bytes)
  selectedFile.value = { name: f.name, size, bytes, sha256: '', sourceType: 'local' }

  hashing.value = true
  try {
    const hash = await computeSHA256(f)
    if (selectedFile.value?.name === f.name) {
      selectedFile.value.sha256 = hash
    }
  } catch {
    ElMessage.warning('SHA-256 计算失败')
  } finally {
    hashing.value = false
  }
  return false
}

async function submitDistributionTask() {
  if (!selectedFile.value) {
    ElMessage.warning('请先选择待分发文件')
    return
  }
  if (!selectedCustomer.value) {
    ElMessage.warning('请选择客户')
    return
  }
  if (!selectedReleaseNote.value) {
    ElMessage.warning('请选择发布说明')
    return
  }
  const releaseNote = selectedReleaseNote.value

  if (distributionForm.value.scheduledAt) {
    const selectedTime = new Date(distributionForm.value.scheduledAt).getTime()
    if (Number.isNaN(selectedTime)) {
      ElMessage.warning('计划分发时间格式无效')
      return
    }
    if (selectedTime <= Date.now()) {
      ElMessage.warning('计划分发时间必须晚于当前时间')
      return
    }
  }

  submittingDistribution.value = true
  try {
    const customerEmail = selectedCustomer.value.email || ''
    if (distributionForm.value.receiveMethod === 'email' && !customerEmail) {
      ElMessage.warning('当前客户未配置邮箱，无法使用邮箱接收')
      return
    }
    if (selectedFile.value.sourceType !== 's3' && !customerEmail) {
      ElMessage.warning('当前后端要求本地文件分发必须填写客户邮箱')
      return
    }

    const scheduledAtUnix = distributionForm.value.scheduledAt
      ? Math.floor(new Date(distributionForm.value.scheduledAt).getTime() / 1000)
      : undefined

    await createDistribution({
      file_name: selectedFile.value.name,
      file_size: selectedFile.value.bytes,
      sha256_original: selectedFile.value.sha256,
      encryption_algo: 'AES-256',
      customer_name: selectedCustomer.value.name,
      customer_email: customerEmail,
      release_notes: releaseNote.content,
      source_type: selectedFile.value.sourceType,
      s3_key: selectedFile.value.s3Key,
      scheduled_at: scheduledAtUnix,
    })

    ElMessage.success('分发任务已创建，可在加密队列查看进度')
    selectedFile.value = null
    distributionForm.value = {
      customerId: '',
      receiveMethod: 'email',
      releaseNoteId: 0,
      scheduledAt: '',
    }
    flowTab.value = 'encrypt-queue'
    queueRef.value?.refresh()
  } catch (err: any) {
    ElMessage.error('创建分发任务失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    submittingDistribution.value = false
  }
}

onMounted(() => {
  browseS3(true)
  fetchCustomers()
  fetchReleaseNotes()
})
</script>

<template>
  <div class="distribution-page">
    <h2 class="page-title">
      <el-icon :size="24" color="#409eff"><Lock /></el-icon>
      SecureRelease 分发中心
    </h2>

    <div class="distribution-tabs">
      <el-tabs v-model="createTab" class="create-tabs">
        <el-tab-pane name="new-task" label="新建加密任务">
          <div class="create-cards">
            <div class="source-task-card">
              <h3 class="distribution-task-title">选择分发文件</h3>
              <el-radio-group :model-value="sourceMode" size="default" style="margin-bottom: 12px" @change="setSourceMode">
                <el-radio-button value="s3"><el-icon><UploadFilled /></el-icon> S3 存储</el-radio-button>
                <el-radio-button value="local"><el-icon><Upload /></el-icon> 本地上传</el-radio-button>
              </el-radio-group>

            <div v-if="sourceMode === 's3'">
              <div class="s3-path-row">
                <el-input v-model="s3Path" placeholder="releases/" @blur="normalizeAndApplyS3Prefix" />
                <el-button type="primary" :loading="browsingS3" @click="browseS3(true)">浏览</el-button>
              </div>
              <div class="s3-hint">仅支持 releases/ 前缀，输入会自动规范化。</div>
              <div class="file-list" v-loading="browsingS3">
                <div
                  v-for="file in s3Files"
                  :key="file.key"
                  class="file-item"
                  :class="{ selected: selectedFile?.s3Key === file.key }"
                  @click="selectS3File(file)"
                >
                  <div class="file-info">
                    <el-icon :size="16" color="#e6a23c"><UploadFilled /></el-icon>
                    <div>
                      <div class="file-name">{{ file.key }}</div>
                      <div class="file-meta">{{ formatS3Time(file.last_modified) }} · {{ formatBytes(file.size) }}</div>
                    </div>
                  </div>
                  <el-icon v-if="selectedFile?.s3Key === file.key" color="#409eff"><Check /></el-icon>
                </div>
                <div v-if="!browsingS3 && !s3Error && s3Files.length === 0" class="file-empty">暂无可选文件</div>
                <div v-if="!browsingS3 && s3Error" class="file-error">{{ s3Error }}</div>
              </div>
              <div class="s3-more-row" v-if="s3HasMore">
                <el-button text type="primary" :loading="loadingMoreS3" @click="browseS3(false)">加载更多</el-button>
              </div>
            </div>

            <div v-else>
              <el-upload drag :auto-upload="false" :on-change="handleLocalUpload" :show-file-list="false">
                <el-icon :size="36" color="#c0c4cc"><UploadFilled /></el-icon>
                <div class="el-upload__text">拖拽文件到此处或<em>点击上传</em></div>
              </el-upload>
            </div>

            <div v-if="selectedFile" style="margin-top: 10px">
              <el-tag closable @close="selectedFile = null" size="large">
                {{ selectedFile.name }} ({{ selectedFile.size }})
              </el-tag>
              <div v-if="selectedFile.sourceType === 's3'" class="file-hash-row">
                <span class="hash-label">S3 Key:</span>
                <code class="hash-value">{{ selectedFile.s3Key }}</code>
              </div>
              <div v-if="selectedFile.sourceType === 'local' && (selectedFile.sha256 || hashing)" class="file-hash-row">
                <span class="hash-label">SHA-256:</span>
                <code v-if="selectedFile.sha256" class="hash-value">{{ selectedFile.sha256 }}</code>
                <span v-else style="color:#e6a23c">计算中...</span>
              </div>
            </div>

            <p class="task-hint">提交后在下方依次完成：编写发布说明 → 选择客户与接收方式 → 确认分发</p>
            </div>

            <div class="release-note-create-card">
              <ReleaseNoteCreator />
            </div>

            <div class="distribution-task-card">
            <h3 class="distribution-task-title">新建分发任务</h3>
            <el-form label-position="top">
              <el-form-item label="选择客户" required>
                <el-select
                  v-model="distributionForm.customerId"
                  placeholder="请选择客户"
                  filterable
                  clearable
                  :loading="loadingCustomers"
                  style="width: 100%"
                >
                  <el-option
                    v-for="customer in customers"
                    :key="customer.customer_id"
                    :label="customer.name + (customer.company ? `（${customer.company}）` : '')"
                    :value="customer.customer_id"
                  />
                </el-select>
              </el-form-item>

              <el-form-item label="接收方式" required>
                <el-radio-group v-model="distributionForm.receiveMethod">
                  <el-radio-button label="email">邮箱接收</el-radio-button>
                  <el-radio-button label="portal">平台站内接收</el-radio-button>
                </el-radio-group>
                <div v-if="distributionForm.receiveMethod === 'email'" class="distribution-hint">
                  {{ selectedCustomer?.email ? `将发送到：${selectedCustomer.email}` : '当前客户未配置邮箱，提交时会失败，请先维护客户邮箱。' }}
                </div>
              </el-form-item>

              <el-form-item label="关联发布说明" required>
                <el-select
                  v-model="distributionForm.releaseNoteId"
                  placeholder="请选择发布说明"
                  filterable
                  clearable
                  :loading="loadingReleaseNotes"
                  style="width: 100%"
                >
                  <el-option
                    v-for="note in releaseNotes"
                    :key="note.id"
                    :label="`${note.title}${note.version ? `（${note.version}）` : ''}`"
                    :value="note.id"
                  />
                </el-select>
                <div v-if="selectedReleaseNote" class="distribution-hint">{{ selectedReleaseNote.content }}</div>
              </el-form-item>

              <el-form-item label="计划分发时间">
                <el-date-picker
                  v-model="distributionForm.scheduledAt"
                  type="datetime"
                  clearable
                  style="width: 100%"
                  placeholder="不选则立即分发"
                />
                <div class="distribution-hint">可选：设置未来时间用于计划分发。到达设定时间后系统会自动下发加密任务。</div>
              </el-form-item>

              <div class="task-action-row">
                <el-button
                  type="primary"
                  :disabled="!canCreateDistributionTask"
                  :loading="submittingDistribution"
                  @click="submitDistributionTask"
                >
                  提交分发任务 <el-icon><Promotion /></el-icon>
                </el-button>
              </div>
            </el-form>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <el-tabs v-model="flowTab" class="flow-tabs">
        <el-tab-pane name="encrypt-queue" label="加密队列">
          <EncryptionQueue ref="queueRef" />
        </el-tab-pane>
        <el-tab-pane name="release-notes" label="发布说明记录">
          <ReleaseNotes />
        </el-tab-pane>
        <el-tab-pane name="distribution-records" label="分发记录">
          <DistributionRecords />
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<style scoped>
.distribution-page {
  width: 100%;
}

.page-title {
  font-size: 22px;
  font-weight: 600;
  margin: 0 0 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  color: #303133;
}

.distribution-tabs {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.create-cards {
  display: flex;
  flex-direction: column;
  gap: 28px;
  align-items: stretch;
}

.create-cards > * + * {
  position: relative;
}

.create-cards > * + *::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: -14px;
  border-top: 2px dashed #dcdfe6;
}

.source-task-card,
.release-note-create-card,
.distribution-task-card {
  background: #fff;
  border-radius: 10px;
  padding: 16px;
  border: 1px solid #dcdfe6;
  box-shadow: 0 6px 16px rgba(0, 0, 0, 0.06);
}

.source-task-card {
  min-height: 280px;
}

.create-tabs,
.flow-tabs {
  background: #fff;
  border-radius: 10px;
  padding: 8px 16px 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.task-panel {
  width: 100%;
}

.s3-path-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.s3-hint {
  color: #909399;
  font-size: 12px;
  margin-bottom: 8px;
}

.s3-more-row {
  display: flex;
  justify-content: center;
  margin-top: 8px;
}

.file-empty {
  padding: 14px;
  text-align: center;
  color: #909399;
  font-size: 12px;
}

.file-error {
  padding: 14px;
  text-align: center;
  color: #f56c6c;
  font-size: 12px;
}

.file-list {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  max-height: 240px;
  overflow-y: auto;
}

.file-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  cursor: pointer;
  border-bottom: 1px solid #f2f3f5;
  transition: all 0.2s;
}

.file-item:last-child {
  border-bottom: none;
}

.file-item:hover {
  background: #f5f7fa;
}

.file-item.selected {
  background: #ecf5ff;
  border-left: 3px solid #409eff;
  padding-left: 11px;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-name {
  font-size: 13px;
  font-weight: 500;
  color: #303133;
}

.file-meta {
  font-size: 11px;
  color: #909399;
  margin-top: 2px;
}

.file-hash-row {
  margin-top: 6px;
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.hash-label {
  color: #909399;
  flex-shrink: 0;
}

.hash-value {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 3px;
  word-break: break-all;
}

.task-hint {
  font-size: 13px;
  color: #909399;
  margin: 12px 0 0;
}

.distribution-task-title {
  margin: 0 0 12px;
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.distribution-hint {
  margin-top: 6px;
  font-size: 12px;
  color: #909399;
  line-height: 1.6;
  white-space: pre-wrap;
}

.task-action-row {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #f2f3f5;
}

@media (max-width: 960px) {
  .create-tabs,
  .flow-tabs {
    padding: 8px 12px 12px;
  }

  .create-cards {
    grid-template-columns: 1fr;
  }
}
</style>
