<script setup lang="ts">
import { computed } from 'vue'
import {
  ArrowLeft,
  ArrowRight,
  CircleCheck,
  CircleClose,
  View,
} from '@element-plus/icons-vue'
import type { DocumentItem, DocumentCategory } from '../pages/Documents/types'

const props = defineProps<{
  activeDocId: string
  categories: DocumentCategory[]
  viewCount?: number
}>()

const emit = defineEmits<{
  'rate': [helpful: boolean]
  'navigate': [docId: string]
}>()

// 扁平化所有文档项，用于计算上一篇/下一篇
const flatDocs = computed(() => {
  const docs: DocumentItem[] = []
  for (const cat of props.categories) {
    for (const item of cat.items) {
      docs.push(item)
    }
  }
  return docs
})

const currentIndex = computed(() =>
  flatDocs.value.findIndex(d => d.id === props.activeDocId)
)

const prevDoc = computed(() =>
  currentIndex.value > 0 ? flatDocs.value[currentIndex.value - 1] : null
)

const nextDoc = computed(() =>
  currentIndex.value < flatDocs.value.length - 1 ? flatDocs.value[currentIndex.value + 1] : null
)

const displayCount = computed(() => {
  const count = props.viewCount ?? 0
  if (count >= 1000) {
    return (count / 1000).toFixed(1).replace(/\.0$/, '') + 'k'
  }
  return count.toString()
})
</script>

<template>
  <footer class="doc-footer">
    <div class="footer-actions">
      <span class="footer-label">这篇文档对您有帮助吗？</span>
      <el-button @click="emit('rate', true)">
        <el-icon><CircleCheck /></el-icon>
        <span>有帮助</span>
      </el-button>
      <el-button @click="emit('rate', false)">
        <el-icon><CircleClose /></el-icon>
        <span>需要改进</span>
      </el-button>
      <span class="view-count">
        <el-icon><View /></el-icon>
        {{ displayCount }} 次阅读
      </span>
    </div>

    <div class="footer-nav">
      <a
        v-if="prevDoc"
        href="#"
        class="nav-prev"
        @click.prevent="emit('navigate', prevDoc.id)"
      >
        <el-icon><ArrowLeft /></el-icon>
        <span>上一篇: {{ prevDoc.title }}</span>
      </a>
      <span v-else />
      <a
        v-if="nextDoc"
        href="#"
        class="nav-next"
        @click.prevent="emit('navigate', nextDoc.id)"
      >
        <span>下一篇: {{ nextDoc.title }}</span>
        <el-icon><ArrowRight /></el-icon>
      </a>
    </div>
  </footer>
</template>

<style scoped>
.doc-footer {
  margin-top: 48px;
  padding-top: 32px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 32px;
}

.footer-label {
  font-size: 14px;
  color: #94a3b8;
}

.view-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  font-size: 13px;
  color: #64748b;
}

.footer-nav {
  display: flex;
  justify-content: space-between;
}

.footer-nav a {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: #94a3b8;
  font-size: 14px;
  text-decoration: none;
  transition: color 0.2s;
}

.footer-nav a:hover {
  color: #e2e8f0;
}
</style>
