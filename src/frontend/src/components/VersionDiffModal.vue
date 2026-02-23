<script setup lang="ts">
import { ref, computed } from 'vue'
import {
  ArrowRight,
  Plus,
  Minus,
  Edit,
  Download,
} from '@element-plus/icons-vue'
import type { DocumentDiff, DocumentVersion } from '../pages/Documents/types'

const props = withDefaults(defineProps<{
  visible: boolean
  versions: DocumentVersion[]
  diffs: DocumentDiff[]
}>(), {
  diffs: () => [],
})

const emit = defineEmits<{
  'update:visible': [value: boolean]
  'export-report': []
}>()

const fromVersion = ref('')
const toVersion = ref('')

// 初始化版本选择
function initVersions() {
  if (props.versions.length >= 2) {
    fromVersion.value = props.versions[1]?.version || ''
    toVersion.value = props.versions[0]?.version || ''
  }
}

// 统计信息
const stats = computed(() => {
  const added = props.diffs.filter(d => d.type === 'added').length
  const modified = props.diffs.filter(d => d.type === 'modified').length
  const removed = props.diffs.filter(d => d.type === 'removed').length
  return { added, modified, removed, total: added + modified + removed }
})

const fromVersionInfo = computed(() =>
  props.versions.find(v => v.version === fromVersion.value)
)
const toVersionInfo = computed(() =>
  props.versions.find(v => v.version === toVersion.value)
)

function handleClose() {
  emit('update:visible', false)
}

function handleExport() {
  emit('export-report')
}

// 当 visible 变为 true 时初始化
defineExpose({ initVersions })
</script>

<template>
  <el-dialog
    :model-value="visible"
    title="文档变更对比"
    width="900px"
    destroy-on-close
    @update:model-value="emit('update:visible', $event)"
    @open="initVersions"
  >
    <!-- 版本选择器 -->
    <div class="diff-selectors">
      <el-select v-model="fromVersion" placeholder="选择旧版本" class="version-select">
        <el-option
          v-for="v in versions"
          :key="v.version"
          :label="v.version + ' (' + v.date + ')'"
          :value="v.version"
          :disabled="v.version === toVersion"
        />
      </el-select>
      <el-icon class="diff-arrow"><ArrowRight /></el-icon>
      <el-select v-model="toVersion" placeholder="选择新版本" class="version-select">
        <el-option
          v-for="v in versions"
          :key="v.version"
          :label="v.version + ' (' + v.date + ')'"
          :value="v.version"
          :disabled="v.version === fromVersion"
        />
      </el-select>
    </div>

    <!-- 版本信息头 -->
    <div class="diff-header">
      <div class="diff-version old">
        <div class="diff-label">旧版本</div>
        <div class="diff-name">{{ fromVersion }}</div>
        <div class="diff-date">{{ fromVersionInfo?.date || '' }}</div>
      </div>
      <el-icon class="diff-arrow-center"><ArrowRight /></el-icon>
      <div class="diff-version new">
        <div class="diff-label">新版本</div>
        <div class="diff-name">{{ toVersion }}</div>
        <div class="diff-date">{{ toVersionInfo?.date || '' }}</div>
      </div>
    </div>

    <!-- 统计信息 -->
    <div class="diff-stats">
      <span class="stat-item stat-added">
        <el-icon><Plus /></el-icon>
        {{ stats.added }} 新增
      </span>
      <span class="stat-item stat-modified">
        <el-icon><Edit /></el-icon>
        {{ stats.modified }} 修改
      </span>
      <span class="stat-item stat-removed">
        <el-icon><Minus /></el-icon>
        {{ stats.removed }} 删除
      </span>
      <span class="stat-total">共 {{ stats.total }} 项变更</span>
    </div>

    <!-- Diff 列表 -->
    <div class="diff-list">
      <div
        v-for="(item, idx) in diffs"
        :key="idx"
        class="diff-item"
        :class="item.type"
      >
        <div class="diff-badge">
          <el-icon>
            <Plus v-if="item.type === 'added'" />
            <Edit v-else-if="item.type === 'modified'" />
            <Minus v-else />
          </el-icon>
          {{ item.type === 'added' ? '新增' : item.type === 'modified' ? '修改' : '删除' }}
        </div>
        <div class="diff-title">{{ item.title }}</div>
        <p v-if="item.description && item.type !== 'modified'" class="diff-desc">{{ item.description }}</p>
        <div v-if="item.type === 'modified' && (item.oldContent || item.newContent)" class="diff-compare">
          <div class="compare-old">{{ item.oldContent }}</div>
          <div class="compare-new">{{ item.newContent }}</div>
        </div>
        <p v-else-if="item.type === 'modified'" class="diff-desc">{{ item.description }}</p>
      </div>

      <div v-if="diffs.length === 0" class="diff-empty">
        暂无变更记录
      </div>
    </div>

    <template #footer>
      <el-button @click="handleClose">关闭</el-button>
      <el-button type="primary" @click="handleExport">
        <el-icon><Download /></el-icon>
        导出变更报告
      </el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.diff-selectors {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 20px;
}

.version-select {
  flex: 1;
}

.diff-arrow {
  font-size: 18px;
  color: #64748b;
  flex-shrink: 0;
}

.diff-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
  padding: 16px;
  background: rgba(30, 41, 59, 0.5);
  border-radius: 12px;
}

.diff-version {
  flex: 1;
  padding: 16px;
  border-radius: 8px;
}

.diff-version.old {
  background: rgba(255, 255, 255, 0.05);
}

.diff-version.new {
  background: rgba(64, 150, 255, 0.1);
  border: 1px solid rgba(64, 150, 255, 0.3);
}

.diff-label {
  font-size: 12px;
  color: #64748b;
  margin-bottom: 4px;
}

.diff-name {
  font-size: 18px;
  font-weight: 600;
  color: #e2e8f0;
  margin-bottom: 4px;
}

.diff-version.new .diff-name {
  color: #60a5fa;
}

.diff-date {
  font-size: 13px;
  color: #94a3b8;
}

.diff-arrow-center {
  font-size: 24px;
  color: #64748b;
}

.diff-stats {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
  padding: 12px 16px;
  background: rgba(30, 41, 59, 0.3);
  border-radius: 8px;
}

.stat-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  font-weight: 500;
}

.stat-added { color: #52c41a; }
.stat-modified { color: #4096ff; }
.stat-removed { color: #ff4d4f; }

.stat-total {
  margin-left: auto;
  font-size: 13px;
  color: #94a3b8;
}

.diff-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.diff-item {
  padding: 16px;
  border-radius: 8px;
  border-left: 4px solid;
}

.diff-item.added {
  background: rgba(82, 196, 26, 0.1);
  border-color: #52c41a;
}

.diff-item.modified {
  background: rgba(64, 150, 255, 0.1);
  border-color: #4096ff;
}

.diff-item.removed {
  background: rgba(255, 77, 79, 0.1);
  border-color: #ff4d4f;
}

.diff-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  margin-bottom: 8px;
}

.diff-item.added .diff-badge { color: #52c41a; }
.diff-item.modified .diff-badge { color: #4096ff; }
.diff-item.removed .diff-badge { color: #ff4d4f; }

.diff-title {
  font-weight: 500;
  color: #e2e8f0;
  margin-bottom: 8px;
}

.diff-desc {
  font-size: 13px;
  color: #94a3b8;
  margin: 0;
}

.diff-compare {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-top: 12px;
}

.compare-old {
  font-size: 13px;
  color: #94a3b8;
  padding: 12px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 6px;
}

.compare-new {
  font-size: 13px;
  color: #93c5fd;
  padding: 12px;
  background: rgba(64, 150, 255, 0.1);
  border-radius: 6px;
}

.diff-empty {
  text-align: center;
  padding: 40px;
  color: #64748b;
  font-size: 14px;
}
</style>
