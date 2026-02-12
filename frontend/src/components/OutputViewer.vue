<script setup lang="ts">
import { computed } from 'vue'
import { Download, View } from '@element-plus/icons-vue'

const props = withDefaults(defineProps<{
  content: string
  label?: string
  truncated?: boolean
  filename?: string
}>(), {
  label: 'output',
  truncated: false,
  filename: 'output.txt',
})

const dialogVisible = defineModel<boolean>('dialogVisible', { default: false })

const INLINE_LINES = 100
const PREVIEW_LINES = 100

const lineCount = computed(() => props.content.split('\n').length)
const isLong = computed(() => lineCount.value > INLINE_LINES)

const preview = computed(() => {
  if (!isLong.value) return props.content
  return props.content.split('\n').slice(0, PREVIEW_LINES).join('\n')
})

function openDialog() {
  dialogVisible.value = true
}

function download() {
  const blob = new Blob([props.content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = props.filename
  a.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div v-if="content">
    <!-- 短输出：直接内联 -->
    <template v-if="!isLong">
      <pre class="output-block"><code>{{ content }}</code></pre>
    </template>

    <!-- 长输出：预览 + 操作按钮 -->
    <template v-else>
      <pre class="output-block output-preview"><code>{{ preview }}</code><span class="fade-out" /></pre>
      <div class="output-actions">
        <el-button size="small" :icon="View" @click="openDialog">查看完整输出</el-button>
        <el-button size="small" :icon="Download" @click="download">下载</el-button>
        <el-tag v-if="truncated" type="warning" size="small" style="margin-left: 8px">
          输出已截断 (超过 64KB)
        </el-tag>
      </div>
    </template>

    <!-- 截断提示（短输出时也显示） -->
    <el-tag v-if="truncated && !isLong" type="warning" size="small" style="margin-top: 4px">
      输出已截断 (超过 64KB)
    </el-tag>

    <!-- 全屏查看 Dialog -->
    <el-dialog
      v-model="dialogVisible"
      :title="label"
      width="90%"
      top="5vh"
      destroy-on-close
      append-to-body
    >
      <div style="display: flex; justify-content: flex-end; margin-bottom: 8px">
        <el-button size="small" :icon="Download" @click="download">下载</el-button>
      </div>
      <pre class="output-block output-full"><code>{{ content }}</code></pre>
      <el-tag v-if="truncated" type="warning" size="small" style="margin-top: 8px">
        输出已截断 (超过 64KB)
      </el-tag>
    </el-dialog>
  </div>
  <span v-else style="color: var(--el-text-color-secondary); font-size: 13px">(empty)</span>
</template>

<style scoped>
.output-block {
  background-color: #1e1e1e;
  color: #d4d4d4;
  padding: 12px 16px;
  border-radius: 4px;
  overflow-x: auto;
  overflow-y: auto;
  max-height: 400px;
  font-family: 'Courier New', Courier, monospace;
  font-size: 13px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-all;
  margin: 0;
}
.output-preview {
  position: relative;
  max-height: 1000px;
  overflow: hidden;
}
.fade-out {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 30px;
  background: linear-gradient(transparent, #1e1e1e);
}
.output-full {
  max-height: 70vh;
  overflow-y: auto;
}
.output-actions {
  margin-top: 8px;
  display: flex;
  align-items: center;
}
</style>
