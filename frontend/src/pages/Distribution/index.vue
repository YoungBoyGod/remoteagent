<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  Upload, Lock, UploadFilled, Promotion,
  Check, CopyDocument, Link as LinkIcon, View,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import {
  createDistribution,
  getDistribution,
  updateDistributionStatus,
} from '../../api/distribution'
import type { DistributionItem } from '../../api/types'
import DistributionRecords from './components/DistributionRecords.vue'

// ---- State ----

const currentStep = ref(0)
const sourceMode = ref<'s3' | 'local'>('s3')
const s3Path = ref('s3://releases/2024/')
const selectedFile = ref<{ name: string; size: string } | null>(null)
const encrypting = ref(false)
const sending = ref(false)
const currentDistId = ref<number | null>(null)
const currentDist = ref<DistributionItem | null>(null)
let pollTimer: ReturnType<typeof setInterval> | null = null

const customerForm = ref({
  name: '',
  email: '',
  releaseNotes: '',
  separateKey: true,
  notifyDownload: false,
})

const recordsRef = ref<InstanceType<typeof DistributionRecords> | null>(null)

// Mock S3 file list (will be replaced by real API later)
const s3Files = ref([
  { name: 'release-v2.4.1-patch.zip', size: '2.4 GB', time: '2024-01-15 14:30', icon: 'archive' },
  { name: 'hotfix-security-2024-01.jar', size: '156 MB', time: '2024-01-14 09:15', icon: 'code' },
  { name: 'client-update-bundle.tar.gz', size: '4.1 GB', time: '2024-01-13 16:45', icon: 'archive' },
])

const steps = [
  { title: '选择文件', icon: Upload },
  { title: '加密处理', icon: Lock },
  { title: '上传存储', icon: UploadFilled },
  { title: '分发客户', icon: Promotion },
]

const encryptTasks = ref([
  { key: 'keygen', label: '生成加密密钥', desc: 'AES-256-GCM 对称加密', progress: 0, status: 'waiting' },
  { key: 'encrypt', label: '文件加密', desc: '流式加密处理大文件', progress: 0, status: 'waiting' },
  { key: 'hash', label: '计算 SHA-256 校验值', desc: '确保文件完整性', progress: 0, status: 'waiting' },
])

const canEncrypt = computed(() => !!selectedFile.value)
const showSuccess = ref(false)

// ---- Helpers ----

function formatExpiry(ts: number | null | undefined): string {
  if (!ts) return '24 小时'
  const hours = Math.max(0, Math.round((ts - Date.now() / 1000) / 3600))
  return `${hours} 小时`
}

// ---- Stage 1: File Selection ----

function selectFile(file: { name: string; size: string }) {
  if (selectedFile.value?.name === file.name) {
    selectedFile.value = null
  } else {
    selectedFile.value = file
  }
}

function handleLocalUpload(uploadFile: any) {
  const f = uploadFile.raw || uploadFile
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB']
  let bytes = f.size
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  const size = parseFloat((bytes / Math.pow(1024, i)).toFixed(2)) + ' ' + sizes[i]
  selectedFile.value = { name: f.name, size }
  return false // prevent auto upload
}

// ---- Stage 2: Encryption ----

async function startEncryption() {
  if (!selectedFile.value) return
  encrypting.value = true
  currentStep.value = 1

  // Reset encrypt tasks
  encryptTasks.value.forEach(t => { t.progress = 0; t.status = 'waiting' })

  try {
    // Create distribution via API
    const dist = await createDistribution({
      file_name: selectedFile.value.name,
      file_size: 0,
      sha256_original: '',
      encryption_algo: 'AES-256',
      customer_name: customerForm.value.name || '',
      customer_email: customerForm.value.email || '',
      release_notes: customerForm.value.releaseNotes,
    })

    if (dist) {
      currentDistId.value = dist.id
      currentDist.value = dist
      startPolling()
    }
  } catch (err: any) {
    ElMessage.error('创建分发任务失败: ' + (err.message || '未知错误'))
    encrypting.value = false
    currentStep.value = 0
  }
}

function startPolling() {
  if (pollTimer) clearInterval(pollTimer)
  encryptTasks.value[0].status = 'running'
  encryptTasks.value[0].progress = 50

  pollTimer = setInterval(async () => {
    if (!currentDistId.value) return
    try {
      const detail = await getDistribution(currentDistId.value)
      if (detail) {
        currentDist.value = detail
        updateEncryptProgress(detail.status)
      }
    } catch {
      stopPolling()
      ElMessage.error('查询任务状态失败')
    }
  }, 2000)
}

function stopPolling() {
  if (pollTimer) { clearInterval(pollTimer); pollTimer = null }
}

function updateEncryptProgress(status: string) {
  const tasks = encryptTasks.value
  switch (status) {
    case 'pending':
      tasks[0].progress = 50; tasks[0].status = 'running'
      break
    case 'encrypting':
      tasks[0].progress = 100; tasks[0].status = 'done'
      tasks[1].progress = 60; tasks[1].status = 'running'
      break
    case 'uploading':
      tasks[0].progress = 100; tasks[0].status = 'done'
      tasks[1].progress = 100; tasks[1].status = 'done'
      tasks[2].progress = 60; tasks[2].status = 'running'
      break
    case 'uploaded':
      tasks.forEach(t => { t.progress = 100; t.status = 'done' })
      stopPolling()
      encrypting.value = false
      setTimeout(() => { currentStep.value = 2 }, 800)
      break
    case 'sent':
      tasks.forEach(t => { t.progress = 100; t.status = 'done' })
      stopPolling()
      encrypting.value = false
      goToSuccess()
      break
    case 'failed':
      stopPolling()
      encrypting.value = false
      ElMessage.error('加密任务失败')
      break
  }
}

// ---- Stage 3: Upload ----

function goToDistribute() {
  currentStep.value = 3
}

// ---- Stage 4: Send ----

async function sendDistribution() {
  if (!customerForm.value.name || !customerForm.value.email) {
    ElMessage.warning('请填写客户名称和邮箱')
    return
  }
  if (!currentDistId.value) return

  sending.value = true
  try {
    await updateDistributionStatus(currentDistId.value, { status: 'sent' })
    // Refresh detail
    const detail = await getDistribution(currentDistId.value)
    if (detail) currentDist.value = detail
    goToSuccess()
    ElMessage.success('分发任务创建成功')
  } catch (err: any) {
    ElMessage.error('发送失败: ' + (err.message || '未知错误'))
  } finally {
    sending.value = false
  }
}

function goToSuccess() {
  showSuccess.value = true
  currentStep.value = 4
  recordsRef.value?.refresh()
}

// ---- Reset ----

function resetWorkflow() {
  selectedFile.value = null
  currentStep.value = 0
  showSuccess.value = false
  currentDistId.value = null
  currentDist.value = null
  encrypting.value = false
  sending.value = false
  customerForm.value = { name: '', email: '', releaseNotes: '', separateKey: true, notifyDownload: false }
  encryptTasks.value.forEach(t => { t.progress = 0; t.status = 'waiting' })
  stopPolling()
}

// ---- Clipboard ----

function copyUrl() {
  const url = currentDist.value?.presigned_url
  if (url) {
    navigator.clipboard.writeText(url).then(() => ElMessage.success('链接已复制'))
  }
}

// ---- Security Report Dialog ----

const showSecurityDialog = ref(false)
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

    <!-- Steps -->
    <el-card shadow="never" class="steps-card">
      <el-steps :active="currentStep" finish-status="success" align-center>
        <el-step v-for="s in steps" :key="s.title" :title="s.title" :icon="s.icon" />
      </el-steps>
    </el-card>

    <!-- Stage 1: File Selection -->
    <el-card v-if="currentStep === 0" shadow="never" class="stage-card">
      <template #header>
        <div class="stage-header">
          <el-tag type="primary" round size="small">1</el-tag>
          <span>选择 Release 包</span>
        </div>
      </template>

      <!-- Source Toggle -->
      <div class="source-toggle">
        <el-radio-group v-model="sourceMode" size="large">
          <el-radio-button value="s3">
            <el-icon><UploadFilled /></el-icon> 从 S3 存储选择
          </el-radio-button>
          <el-radio-button value="local">
            <el-icon><Upload /></el-icon> 本地上传
          </el-radio-button>
        </el-radio-group>
      </div>

      <!-- S3 Browser -->
      <div v-if="sourceMode === 's3'" class="s3-browser">
        <div class="s3-path-row">
          <el-input v-model="s3Path" placeholder="s3://releases/2024/" prefix-icon="Search" />
          <el-button type="primary">浏览</el-button>
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
              <el-icon :size="18" color="#e6a23c"><UploadFilled /></el-icon>
              <div>
                <div class="file-name">{{ file.name }}</div>
                <div class="file-meta">修改时间: {{ file.time }} | 大小: {{ file.size }}</div>
              </div>
            </div>
            <el-icon v-if="selectedFile?.name === file.name" color="#409eff"><Check /></el-icon>
          </div>
        </div>
      </div>

      <!-- Local Upload -->
      <div v-else class="local-upload">
        <el-upload
          drag
          :auto-upload="false"
          :on-change="handleLocalUpload"
          :show-file-list="false"
        >
          <el-icon :size="48" color="#c0c4cc"><UploadFilled /></el-icon>
          <div class="el-upload__text">拖拽文件到此处或<em>点击上传</em></div>
          <template #tip>
            <div class="el-upload__tip">支持单文件最大 10GB</div>
          </template>
        </el-upload>
      </div>

      <!-- Selected File -->
      <div v-if="selectedFile" class="selected-file">
        <el-tag closable @close="selectedFile = null" size="large">
          {{ selectedFile.name }} ({{ selectedFile.size }})
        </el-tag>
      </div>

      <div class="stage-actions">
        <el-button type="primary" :disabled="!canEncrypt" @click="startEncryption">
          开始加密处理 <el-icon><Promotion /></el-icon>
        </el-button>
      </div>
    </el-card>

    <!-- Stage 2: Encryption -->
    <el-card v-if="currentStep === 1" shadow="never" class="stage-card">
      <template #header>
        <div class="stage-header">
          <el-tag type="warning" round size="small">2</el-tag>
          <span>加密与完整性校验</span>
        </div>
      </template>

      <div class="encrypt-tasks">
        <div v-for="task in encryptTasks" :key="task.key" class="encrypt-task">
          <div class="task-header">
            <div class="task-info">
              <el-icon :size="20" :color="task.status === 'done' ? '#67c23a' : task.status === 'running' ? '#409eff' : '#c0c4cc'">
                <Check v-if="task.status === 'done'" />
                <Lock v-else />
              </el-icon>
              <div>
                <div class="task-label">{{ task.label }}</div>
                <div class="task-desc">{{ task.desc }}</div>
              </div>
            </div>
            <el-tag
              :type="task.status === 'done' ? 'success' : task.status === 'running' ? 'primary' : 'info'"
              size="small"
              effect="plain"
            >
              {{ task.status === 'done' ? '完成' : task.status === 'running' ? '处理中...' : '等待中' }}
            </el-tag>
          </div>
          <el-progress
            :percentage="task.progress"
            :status="task.status === 'done' ? 'success' : undefined"
            :striped="task.status === 'running'"
            :striped-flow="task.status === 'running'"
            :show-text="false"
            :stroke-width="6"
          />
        </div>
      </div>

      <!-- SHA-256 result -->
      <div v-if="currentDist?.sha256_original" class="hash-result">
        <el-tag type="success" effect="plain" size="small">SHA-256</el-tag>
        <code>{{ currentDist.sha256_original }}</code>
      </div>

      <el-alert type="warning" :closable="false" show-icon class="security-tip">
        <template #title>安全提示</template>
        加密密钥将通过独立通道发送给客户，请勿在同一封邮件中同时发送加密文件和密码。
      </el-alert>
    </el-card>

    <!-- Stage 3: Upload & Storage -->
    <el-card v-if="currentStep === 2" shadow="never" class="stage-card">
      <template #header>
        <div class="stage-header">
          <el-tag type="success" round size="small">3</el-tag>
          <span>安全存储与链接生成</span>
        </div>
      </template>

      <el-descriptions :column="2" border size="small" class="upload-info">
        <el-descriptions-item label="存储位置">
          <code>s3://secure-releases/encrypted/</code>
        </el-descriptions-item>
        <el-descriptions-item label="访问有效期">
          {{ formatExpiry(currentDist?.url_expires_at) }}
        </el-descriptions-item>
        <el-descriptions-item label="加密文件">
          {{ currentDist?.file_name }}.enc
        </el-descriptions-item>
        <el-descriptions-item label="加密算法">
          {{ currentDist?.encryption_algo || 'AES-256' }}
        </el-descriptions-item>
      </el-descriptions>

      <!-- Presigned URL -->
      <div v-if="currentDist?.presigned_url" class="presigned-url-box">
        <div class="url-header">
          <span>预签名下载链接 (Presigned URL)</span>
          <el-tag type="success" size="small" effect="plain">安全</el-tag>
        </div>
        <div class="url-content">
          <code>{{ currentDist.presigned_url }}</code>
        </div>
        <div class="url-actions">
          <el-button @click="copyUrl" :icon="CopyDocument">复制链接</el-button>
          <el-button :icon="LinkIcon">测试链接</el-button>
        </div>
      </div>

      <div class="stage-actions">
        <el-button type="primary" @click="goToDistribute">
          配置客户分发 <el-icon><Promotion /></el-icon>
        </el-button>
      </div>
    </el-card>

    <!-- Stage 4: Customer Distribution -->
    <el-card v-if="currentStep === 3 && !showSuccess" shadow="never" class="stage-card">
      <template #header>
        <div class="stage-header">
          <el-tag type="danger" round size="small">4</el-tag>
          <span>客户分发配置</span>
        </div>
      </template>

      <el-form label-position="top" class="customer-form">
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="客户公司名称">
              <el-input v-model="customerForm.name" placeholder="客户公司名称" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="接收邮箱">
              <el-input v-model="customerForm.email" placeholder="primary@company.com" type="email" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-form-item label="分发选项">
          <div class="dist-options">
            <el-checkbox v-model="customerForm.separateKey">
              分开发送解密密钥（通过独立邮件发送密码，提高安全性）
            </el-checkbox>
            <el-checkbox v-model="customerForm.notifyDownload">
              下载通知（客户成功下载后通知我）
            </el-checkbox>
          </div>
        </el-form-item>

        <el-form-item label="Release 说明（将包含在邮件中）">
          <el-input
            v-model="customerForm.releaseNotes"
            type="textarea"
            :rows="4"
            placeholder="v2.4.1 安全补丁更新..."
          />
        </el-form-item>
      </el-form>

      <div class="stage-actions" style="justify-content: space-between;">
        <el-button text type="primary" @click="showSecurityDialog = true">
          <el-icon><View /></el-icon> 查看安全报告
        </el-button>
        <el-button type="success" :loading="sending" @click="sendDistribution">
          <el-icon><Promotion /></el-icon> 确认分发
        </el-button>
      </div>
    </el-card>

    <!-- Success State -->
    <el-card v-if="showSuccess" shadow="never" class="stage-card success-card">
      <el-result icon="success" title="分发任务已创建" sub-title="邮件已加入发送队列">
        <template #extra>
          <el-descriptions :column="1" border size="small" class="success-info">
            <el-descriptions-item label="任务 ID">{{ currentDist?.task_id || '-' }}</el-descriptions-item>
            <el-descriptions-item label="加密文件">{{ currentDist?.file_name }}.enc</el-descriptions-item>
            <el-descriptions-item label="密钥发送方式">
              <el-tag :type="customerForm.separateKey ? 'success' : 'warning'" size="small">
                {{ customerForm.separateKey ? '独立邮件 (分开发送)' : '同一邮件' }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item label="链接有效期">24 小时</el-descriptions-item>
          </el-descriptions>
          <div class="success-actions">
            <el-button @click="resetWorkflow">分发新的 Release</el-button>
            <el-button type="primary" @click="recordsRef?.refresh()">查看分发记录</el-button>
          </div>
        </template>
      </el-result>
    </el-card>

    <!-- Distribution Records -->
    <DistributionRecords ref="recordsRef" style="margin-top: 20px;" />

    <!-- Security Report Dialog -->
    <el-dialog v-model="showSecurityDialog" title="安全分发报告" width="600px">
      <el-alert type="success" :closable="false" show-icon>
        <template #title>安全评级: A+ (企业级)</template>
        本次分发遵循行业最佳实践
      </el-alert>
      <el-descriptions :column="2" border size="small" style="margin-top: 16px;" title="加密详情">
        <el-descriptions-item label="算法">AES-256-GCM</el-descriptions-item>
        <el-descriptions-item label="密钥长度">256 bits</el-descriptions-item>
        <el-descriptions-item label="哈希算法">SHA-256</el-descriptions-item>
        <el-descriptions-item label="传输协议">TLS 1.3</el-descriptions-item>
      </el-descriptions>
      <div style="margin-top: 16px;">
        <h4 style="margin-bottom: 8px;">风险控制措施</h4>
        <ul class="security-list">
          <li>文件与密钥分开发送，降低单点泄露风险</li>
          <li>下载链接 24 小时自动过期，防止长期暴露</li>
          <li>IP 地址白名单限制（可选）</li>
          <li>下载次数限制（防止转发滥用）</li>
        </ul>
      </div>
    </el-dialog>
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

.steps-card {
  margin-bottom: 20px;
}

.stage-card {
  margin-bottom: 20px;
}

.stage-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 600;
}

.source-toggle {
  margin-bottom: 16px;
}

.s3-path-row {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
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
  padding: 12px 16px;
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
  gap: 12px;
}

.file-name {
  font-size: 14px;
  font-weight: 500;
}

.file-meta {
  font-size: 12px;
  color: #909399;
  margin-top: 2px;
}

.selected-file {
  margin-top: 12px;
}

.stage-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
}

.encrypt-tasks {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.encrypt-task {
  padding: 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
}

.task-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.task-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.task-label {
  font-size: 14px;
  font-weight: 500;
}

.task-desc {
  font-size: 12px;
  color: #909399;
}

.hash-result {
  margin-top: 16px;
  padding: 12px;
  background: #f0f9eb;
  border-radius: 8px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.hash-result code {
  font-size: 12px;
  word-break: break-all;
  color: #67c23a;
}

.security-tip {
  margin-top: 16px;
}

.presigned-url-box {
  margin-top: 16px;
  padding: 16px;
  border: 1px solid #ebeef5;
  border-radius: 8px;
}

.url-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  font-weight: 500;
  font-size: 14px;
}

.url-content {
  padding: 12px;
  background: #f5f7fa;
  border-radius: 6px;
  margin-bottom: 12px;
}

.url-content code {
  font-size: 12px;
  word-break: break-all;
  color: #606266;
}

.url-actions {
  display: flex;
  gap: 8px;
}

.customer-form {
  margin-bottom: 0;
}

.dist-options {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.success-card {
  text-align: center;
}

.success-info {
  text-align: left;
  margin-bottom: 16px;
}

.success-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  margin-top: 16px;
}

.security-list {
  padding-left: 20px;
  color: #606266;
  font-size: 14px;
  line-height: 2;
}

.security-list li {
  list-style-type: none;
  position: relative;
  padding-left: 20px;
}

.security-list li::before {
  content: '✓';
  position: absolute;
  left: 0;
  color: #67c23a;
  font-weight: bold;
}

.upload-info {
  margin-bottom: 16px;
}
</style>
