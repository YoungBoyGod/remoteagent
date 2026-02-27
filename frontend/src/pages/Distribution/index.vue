<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  Upload, Lock, UploadFilled, Promotion, Check,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { createDistribution, listDistributionS3Objects } from '../../api/distribution'
import type { DistributionS3ObjectItem } from '../../api/types'
import EncryptionQueue from './components/EncryptionQueue.vue'
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
const submitting = ref(false)
const hashing = ref(false)
const browsingS3 = ref(false)
const loadingMoreS3 = ref(false)

const s3Files = ref<DistributionS3ObjectItem[]>([])
const s3NextToken = ref('')
const s3HasMore = ref(false)

const queueRef = ref<InstanceType<typeof EncryptionQueue> | null>(null)

const canSubmit = computed(() => !!selectedFile.value && !browsingS3.value)

function setSourceMode(mode: 's3' | 'local') {
  sourceMode.value = mode
  selectedFile.value = null
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
  if (reset) {
    browsingS3.value = true
  } else {
    loadingMoreS3.value = true
  }
  try {
    const resp = await listDistributionS3Objects({
      prefix: s3Path.value,
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
    ElMessage.error('读取 S3 列表失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
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

async function submitEncryption() {
  if (!selectedFile.value) return
  submitting.value = true
  try {
    await createDistribution({
      file_name: selectedFile.value.name,
      file_size: selectedFile.value.bytes,
      sha256_original: selectedFile.value.sha256,
      encryption_algo: 'AES-256',
      customer_name: '',
      customer_email: '',
      release_notes: '',
      source_type: selectedFile.value.sourceType,
      s3_key: selectedFile.value.s3Key,
    })
    ElMessage.success('加密任务已提交，可在加密队列查看进度')
    selectedFile.value = null
    flowTab.value = 'encrypt-queue'
    queueRef.value?.refresh()
  } catch (err: any) {
    ElMessage.error('创建分发任务失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    submitting.value = false
  }
}

browseS3(true)
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
          <div class="task-panel">
            <el-radio-group :model-value="sourceMode" size="default" style="margin-bottom: 12px" @change="setSourceMode">
              <el-radio-button value="s3"><el-icon><UploadFilled /></el-icon> S3 存储</el-radio-button>
              <el-radio-button value="local"><el-icon><Upload /></el-icon> 本地上传</el-radio-button>
            </el-radio-group>

            <div v-if="sourceMode === 's3'">
              <div class="s3-path-row">
                <el-input v-model="s3Path" placeholder="s3://releases/" />
                <el-button type="primary" :loading="browsingS3" @click="browseS3(true)">浏览</el-button>
              </div>
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
                <div v-if="!browsingS3 && s3Files.length === 0" class="file-empty">暂无可选文件</div>
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

            <p class="task-hint">提交后在下方队列依次完成：编写发布说明 → 选择客户 → 确认分发</p>

            <div class="task-action-row">
              <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitEncryption">
                提交加密任务 <el-icon><Promotion /></el-icon>
              </el-button>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>

      <el-tabs v-model="flowTab" class="flow-tabs">
        <el-tab-pane name="encrypt-queue" label="加密队列">
          <EncryptionQueue ref="queueRef" />
        </el-tab-pane>
        <el-tab-pane name="release-notes" label="发布说明">
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

.create-tabs,
.flow-tabs {
  background: #fff;
  border-radius: 10px;
  padding: 8px 16px 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.05);
}

.task-panel {
  width: 100%;
  min-height: 240px;
}

.s3-path-row {
  display: flex;
  gap: 8px;
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
}
</style>
