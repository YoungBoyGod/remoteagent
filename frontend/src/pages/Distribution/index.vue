<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  Upload, Lock, UploadFilled, Promotion, Check,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { createDistribution } from '../../api/distribution'
import EncryptionQueue from './components/EncryptionQueue.vue'
import ReleaseNotes from './components/ReleaseNotes.vue'
import DistributionRecords from './components/DistributionRecords.vue'

// ---- State ----

const sourceMode = ref<'s3' | 'local'>('s3')
const s3Path = ref('s3://releases/2024/')
const selectedFile = ref<{ name: string; size: string; bytes: number; sha256: string } | null>(null)
const submitting = ref(false)
const hashing = ref(false)

const queueRef = ref<InstanceType<typeof EncryptionQueue> | null>(null)
const recordsRef = ref<InstanceType<typeof DistributionRecords> | null>(null)

// Mock S3 file list
const s3Files = ref([
  { name: 'release-v2.4.1-patch.zip', size: '2.4 GB', time: '2024-01-15 14:30' },
  { name: 'hotfix-security-2024-01.jar', size: '156 MB', time: '2024-01-14 09:15' },
  { name: 'client-update-bundle.tar.gz', size: '4.1 GB', time: '2024-01-13 16:45' },
])

const canSubmit = computed(() => !!selectedFile.value)

// ---- Helpers ----

function formatBytes(bytes: number): string {
  if (bytes <= 0) return '0 Bytes'
  const units = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + units[i]
}

async function computeSHA256(file: File): Promise<string> {
  const buffer = await file.arrayBuffer()
  const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
  const hashArray = Array.from(new Uint8Array(hashBuffer))
  return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
}

// ---- File Selection ----

function selectFile(file: { name: string; size: string }) {
  selectedFile.value = selectedFile.value?.name === file.name
    ? null
    : { ...file, bytes: 0, sha256: '' }
}

async function handleLocalUpload(uploadFile: any) {
  const f: File = uploadFile.raw || uploadFile
  const bytes = f.size
  const size = formatBytes(bytes)
  selectedFile.value = { name: f.name, size, bytes, sha256: '' }

  // 异步计算 SHA-256
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

// ---- Submit ----

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
    })
    ElMessage.success('加密任务已提交，可在下方队列查看进度')
    selectedFile.value = null
    queueRef.value?.refresh()
  } catch (err: any) {
    ElMessage.error('创建分发任务失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="distribution-page">
    <!-- Header -->
    <div class="page-header">
      <div>
        <h2 class="page-title">
          <el-icon :size="24" color="#409eff"><Lock /></el-icon>
          SecureRelease 分发中心
        </h2>
        <p class="page-desc">企业级加密补丁分发系统</p>
      </div>
      <div class="header-badges">
        <el-tag type="success" effect="plain"><el-icon><Lock /></el-icon> AES-256 加密</el-tag>
        <el-tag type="success" effect="plain"><el-icon><Check /></el-icon> SHA-256 校验</el-tag>
      </div>
    </div>

    <!-- 提交区 -->
    <el-card shadow="never" class="submit-card">
      <template #header>
        <div class="card-header">
          <span>新建加密分发任务</span>
        </div>
      </template>

      <div class="source-toggle">
        <el-radio-group v-model="sourceMode" size="default">
          <el-radio-button value="s3">
            <el-icon><UploadFilled /></el-icon> S3 存储
          </el-radio-button>
          <el-radio-button value="local">
            <el-icon><Upload /></el-icon> 本地上传
          </el-radio-button>
        </el-radio-group>
      </div>

      <!-- S3 Browser -->
      <div v-if="sourceMode === 's3'" class="s3-browser">
        <div class="s3-path-row">
          <el-input v-model="s3Path" placeholder="s3://releases/2024/" size="default" />
          <el-button type="primary" size="default">浏览</el-button>
        </div>
        <div class="file-list">
          <div
            v-for="file in s3Files"
            :key="file.name"
            class="file-item"
            :class="{ selected: selectedFile?.name === file.name }"
            @click="selectFile(file)"
          >
            <div class="file-info">
              <el-icon :size="16" color="#e6a23c"><UploadFilled /></el-icon>
              <div>
                <div class="file-name">{{ file.name }}</div>
                <div class="file-meta">{{ file.time }} | {{ file.size }}</div>
              </div>
            </div>
            <el-icon v-if="selectedFile?.name === file.name" color="#409eff"><Check /></el-icon>
          </div>
        </div>
      </div>

      <!-- Local Upload -->
      <div v-else class="local-upload">
        <el-upload drag :auto-upload="false" :on-change="handleLocalUpload" :show-file-list="false">
          <el-icon :size="36" color="#c0c4cc"><UploadFilled /></el-icon>
          <div class="el-upload__text">拖拽文件到此处或<em>点击上传</em></div>
        </el-upload>
      </div>

      <div v-if="selectedFile" class="selected-file-card">
        <div class="file-detail-row">
          <el-tag closable @close="selectedFile = null" size="large">
            {{ selectedFile.name }} ({{ selectedFile.size }})
          </el-tag>
        </div>
        <div v-if="selectedFile.sha256 || hashing" class="file-hash-row">
          <span class="hash-label">SHA-256:</span>
          <code v-if="selectedFile.sha256" class="hash-value">{{ selectedFile.sha256 }}</code>
          <span v-else class="hash-computing">计算中...</span>
        </div>
      </div>

      <p class="submit-hint">提交后可在下方队列中依次完成：编写发布说明 → 选择客户 → 确认分发</p>

      <div class="submit-actions">
        <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitEncryption">
          提交加密任务 <el-icon><Promotion /></el-icon>
        </el-button>
      </div>
    </el-card>

    <!-- 加密队列 -->
    <EncryptionQueue ref="queueRef" style="margin-top: 16px;" />

    <!-- 发布说明 -->
    <ReleaseNotes style="margin-top: 16px;" />

    <!-- 历史记录 -->
    <DistributionRecords ref="recordsRef" style="margin-top: 16px;" />
  </div>
</template>

<style scoped>
.distribution-page {
  max-width: 960px;
  margin: 0 auto;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.page-title {
  font-size: 22px;
  font-weight: 600;
  margin: 0 0 4px 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.page-desc {
  color: #909399;
  font-size: 14px;
  margin: 0;
}

.header-badges {
  display: flex;
  gap: 8px;
}

.submit-card {
  margin-bottom: 0;
}

.card-header {
  font-size: 16px;
  font-weight: 600;
}

.source-toggle {
  margin-bottom: 12px;
}

.s3-path-row {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.file-list {
  border: 1px solid #ebeef5;
  border-radius: 8px;
  max-height: 200px;
  overflow-y: auto;
}

.file-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  cursor: pointer;
  transition: background-color 0.2s;
  border-bottom: 1px solid #f2f3f5;
}

.file-item:last-child {
  border-bottom: none;
}

.file-item:hover {
  background-color: #f5f7fa;
}

.file-item.selected {
  background-color: #ecf5ff;
}

.file-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-name {
  font-size: 13px;
  font-weight: 500;
}

.file-meta {
  font-size: 11px;
  color: #909399;
  margin-top: 2px;
}

.selected-file-card {
  margin-top: 10px;
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
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 11px;
  color: #606266;
  word-break: break-all;
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 3px;
}

.hash-computing {
  color: #e6a23c;
}

.submit-hint {
  font-size: 13px;
  color: #909399;
  margin: 12px 0 0 0;
}

.submit-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #f2f3f5;
}
</style>
