<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  Upload, Lock, UploadFilled, Promotion, Check,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { createDistribution } from '../../api/distribution'
import EncryptionQueue from './components/EncryptionQueue.vue'
import DistributionRecords from './components/DistributionRecords.vue'

// ---- State ----

const sourceMode = ref<'s3' | 'local'>('s3')
const s3Path = ref('s3://releases/2024/')
const selectedFile = ref<{ name: string; size: string } | null>(null)
const submitting = ref(false)

const customerForm = ref({
  name: '',
  email: '',
  releaseNotes: '',
})

const queueRef = ref<InstanceType<typeof EncryptionQueue> | null>(null)
const recordsRef = ref<InstanceType<typeof DistributionRecords> | null>(null)

// Mock S3 file list
const s3Files = ref([
  { name: 'release-v2.4.1-patch.zip', size: '2.4 GB', time: '2024-01-15 14:30' },
  { name: 'hotfix-security-2024-01.jar', size: '156 MB', time: '2024-01-14 09:15' },
  { name: 'client-update-bundle.tar.gz', size: '4.1 GB', time: '2024-01-13 16:45' },
])

const canSubmit = computed(() => !!selectedFile.value)

// ---- File Selection ----

function selectFile(file: { name: string; size: string }) {
  selectedFile.value = selectedFile.value?.name === file.name ? null : file
}

function handleLocalUpload(uploadFile: any) {
  const f = uploadFile.raw || uploadFile
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  const bytes = f.size
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const size = parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + sizes[i]
  selectedFile.value = { name: f.name, size }
  return false
}

// ---- Submit ----

async function submitEncryption() {
  if (!selectedFile.value) return
  submitting.value = true
  try {
    await createDistribution({
      file_name: selectedFile.value.name,
      file_size: 0,
      sha256_original: '',
      encryption_algo: 'AES-256',
      customer_name: customerForm.value.name || '',
      customer_email: customerForm.value.email || '',
      release_notes: customerForm.value.releaseNotes,
    })
    ElMessage.success('加密任务已提交，可在下方队列查看进度')
    // 重置表单
    selectedFile.value = null
    customerForm.value = { name: '', email: '', releaseNotes: '' }
    // 刷新队列
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

      <el-row :gutter="20">
        <!-- 左侧：文件选择 -->
        <el-col :span="14">
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

          <div v-if="selectedFile" class="selected-file">
            <el-tag closable @close="selectedFile = null" size="large">
              {{ selectedFile.name }} ({{ selectedFile.size }})
            </el-tag>
          </div>
        </el-col>

        <!-- 右侧：客户信息 -->
        <el-col :span="10">
          <el-form label-position="top" size="default">
            <el-form-item label="客户公司名称">
              <el-input v-model="customerForm.name" placeholder="可选，稍后在队列中填写" />
            </el-form-item>
            <el-form-item label="接收邮箱">
              <el-input v-model="customerForm.email" placeholder="可选，稍后在队列中填写" type="email" />
            </el-form-item>
            <el-form-item label="Release 说明">
              <el-input v-model="customerForm.releaseNotes" type="textarea" :rows="3" placeholder="v2.4.1 安全补丁..." />
            </el-form-item>
          </el-form>
        </el-col>
      </el-row>

      <div class="submit-actions">
        <el-button type="primary" :disabled="!canSubmit" :loading="submitting" @click="submitEncryption">
          提交加密任务 <el-icon><Promotion /></el-icon>
        </el-button>
      </div>
    </el-card>

    <!-- 加密队列 -->
    <EncryptionQueue ref="queueRef" style="margin-top: 16px;" />

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

.selected-file {
  margin-top: 10px;
}

.submit-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid #f2f3f5;
}
</style>
