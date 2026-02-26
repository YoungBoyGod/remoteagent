<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { Search, Clock, Document, Delete, ArrowRight } from '@element-plus/icons-vue'
import type { DocumentSearchResult } from '../pages/Documents/types'

const props = defineProps<{
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  select: [doc: DocumentSearchResult]
}>()

// ==================== State ====================
const searchQuery = ref('')
const searchInputRef = ref<HTMLInputElement | null>(null)
const activeIndex = ref(-1)
const isSearching = ref(false)
const searchResults = ref<DocumentSearchResult[]>([])

// ==================== Search History (localStorage) ====================
const HISTORY_KEY = 'doc-search-history'
const MAX_HISTORY = 10

const searchHistory = ref<string[]>(loadHistory())

function loadHistory(): string[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function saveHistory(history: string[]) {
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history))
}

function addToHistory(query: string) {
  const trimmed = query.trim()
  if (!trimmed) return
  const filtered = searchHistory.value.filter(h => h !== trimmed)
  filtered.unshift(trimmed)
  searchHistory.value = filtered.slice(0, MAX_HISTORY)
  saveHistory(searchHistory.value)
}

function removeHistoryItem(index: number) {
  searchHistory.value.splice(index, 1)
  saveHistory(searchHistory.value)
}

function clearHistory() {
  searchHistory.value = []
  saveHistory([])
}

function clickHistory(query: string) {
  searchQuery.value = query
  doSearch(query)
}

// ==================== Hot Docs ====================
const hotDocs: DocumentSearchResult[] = [
  { id: 'quickstart', title: '快速开始', content: '在 30 分钟内完成产品部署', category: '产品文档' },
  { id: 'api-reference', title: 'API 参考', content: '完整的 API 接口文档', category: '技术文档' },
  { id: 'deployment', title: '部署指南', content: '生产环境部署最佳实践', category: '运维手册' },
]

// ==================== Mock Search (will be replaced by real API) ====================
const allDocs: DocumentSearchResult[] = [
  { id: 'whitepaper', title: '产品白皮书', content: '产品定位、核心功能与技术架构概述', category: '产品文档' },
  { id: 'architecture', title: '架构设计', content: '系统整体架构、模块划分与数据流', category: '产品文档' },
  { id: 'features', title: '功能清单', content: '完整的功能列表与版本对照', category: '产品文档' },
  { id: 'quickstart', title: '快速开始', content: '在 30 分钟内完成产品部署，从安装到首个 API 调用', category: '产品文档' },
  { id: 'release-notes', title: '发布说明', content: 'v2.4.1 版本新增功能与修复内容', category: 'v2.4.1 文档' },
  { id: 'upgrade-guide', title: '升级指南', content: '从旧版本升级到 v2.4.1 的步骤', category: 'v2.4.1 文档' },
  { id: 'compatibility', title: '兼容性说明', content: '硬件与软件兼容性矩阵', category: 'v2.4.1 文档' },
  { id: 'known-issues', title: '已知问题', content: '当前版本已知问题与临时解决方案', category: 'v2.4.1 文档' },
  { id: 'changelog', title: '变更日志', content: '详细的代码变更记录', category: 'v2.4.1 文档' },
  { id: 'api-reference', title: 'API 参考', content: 'RESTful API 完整接口文档', category: '技术文档' },
  { id: 'sdk', title: 'SDK 集成', content: 'Python、Java、Go SDK 使用指南', category: '技术文档' },
  { id: 'webhook', title: 'Webhook 配置', content: '事件通知与 Webhook 回调配置', category: '技术文档' },
  { id: 'error-codes', title: '错误代码', content: '错误代码对照表与排查建议', category: '技术文档' },
  { id: 'deployment', title: '部署指南', content: '生产环境部署、高可用配置', category: '运维手册' },
  { id: 'monitoring', title: '监控配置', content: 'Prometheus + Grafana 监控接入', category: '运维手册' },
  { id: 'backup', title: '备份恢复', content: '数据备份策略与灾难恢复流程', category: '运维手册' },
  { id: 'troubleshooting', title: '故障排查', content: '常见故障现象与排查步骤', category: '运维手册' },
]

function doSearch(query: string) {
  const q = query.trim().toLowerCase()
  if (!q) {
    searchResults.value = []
    isSearching.value = false
    return
  }
  isSearching.value = true
  // Mock: local filter. Will be replaced by API call.
  searchResults.value = allDocs.filter(
    doc => doc.title.toLowerCase().includes(q) || doc.content.toLowerCase().includes(q)
  )
  activeIndex.value = searchResults.value.length > 0 ? 0 : -1
  isSearching.value = false
}

// ==================== Debounce ====================
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(searchQuery, (val) => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    doSearch(val)
  }, 300)
})

// ==================== Grouped Results ====================
const groupedResults = computed(() => {
  const groups: Record<string, DocumentSearchResult[]> = {}
  for (const r of searchResults.value) {
    if (!groups[r.category]) groups[r.category] = []
    groups[r.category].push(r)
  }
  return groups
})

const flatResults = computed(() => searchResults.value)

// ==================== Highlight ====================
function highlightText(text: string, query: string): string {
  if (!query.trim()) return text
  const escaped = query.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const regex = new RegExp(`(${escaped})`, 'gi')
  return text.replace(regex, '<mark>$1</mark>')
}

// ==================== Keyboard Navigation ====================
function onKeydown(e: KeyboardEvent) {
  const total = flatResults.value.length
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    activeIndex.value = total > 0 ? (activeIndex.value + 1) % total : -1
    scrollActiveIntoView()
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    activeIndex.value = total > 0 ? (activeIndex.value - 1 + total) % total : -1
    scrollActiveIntoView()
  } else if (e.key === 'Enter') {
    e.preventDefault()
    if (activeIndex.value >= 0 && activeIndex.value < total) {
      selectResult(flatResults.value[activeIndex.value])
    }
  }
}

function scrollActiveIntoView() {
  nextTick(() => {
    const el = document.querySelector('.search-result-item.active')
    if (el) el.scrollIntoView({ block: 'nearest' })
  })
}

// ==================== Select ====================
function selectResult(doc: DocumentSearchResult) {
  addToHistory(searchQuery.value)
  emit('select', doc)
  close()
}

// ==================== Open / Close ====================
function close() {
  emit('update:visible', false)
  searchQuery.value = ''
  searchResults.value = []
  activeIndex.value = -1
}

watch(() => props.visible, (val) => {
  if (val) {
    nextTick(() => {
      searchInputRef.value?.focus()
    })
  }
})

// ==================== Show state ====================
const hasQuery = computed(() => searchQuery.value.trim().length > 0)
const noResults = computed(() => hasQuery.value && !isSearching.value && searchResults.value.length === 0)
</script>

<template>
  <Teleport to="body">
    <Transition name="search-fade">
      <div v-if="visible" class="search-overlay" @click.self="close">
        <div class="search-modal" @click.stop>
          <!-- Search Input -->
          <div class="search-input-wrapper">
            <el-icon class="search-icon"><Search /></el-icon>
            <input
              ref="searchInputRef"
              v-model="searchQuery"
              type="text"
              placeholder="搜索文档、API、错误代码..."
              class="search-input"
              @keydown="onKeydown"
            />
            <kbd class="esc-hint" @click="close">ESC</kbd>
          </div>

          <div class="search-body">
            <!-- No query: show history + hot docs -->
            <template v-if="!hasQuery">
              <!-- Search History -->
              <div v-if="searchHistory.length > 0" class="search-section">
                <div class="search-section-header">
                  <span class="search-section-title">最近搜索</span>
                  <button class="clear-btn" @click="clearHistory">清除</button>
                </div>
                <div
                  v-for="(item, idx) in searchHistory"
                  :key="'h-' + idx"
                  class="search-item history-item"
                  @click="clickHistory(item)"
                >
                  <el-icon class="item-icon"><Clock /></el-icon>
                  <span class="item-text">{{ item }}</span>
                  <button class="remove-btn" @click.stop="removeHistoryItem(idx)">
                    <el-icon><Delete /></el-icon>
                  </button>
                </div>
              </div>

              <!-- Hot Docs -->
              <div class="search-section">
                <div class="search-section-header">
                  <span class="search-section-title">热门文档</span>
                </div>
                <div
                  v-for="doc in hotDocs"
                  :key="'hot-' + doc.id"
                  class="search-item hot-item"
                  @click="selectResult(doc)"
                >
                  <el-icon class="item-icon doc-icon"><Document /></el-icon>
                  <div class="item-content">
                    <div class="item-title">{{ doc.title }}</div>
                    <div class="item-meta">{{ doc.category }}</div>
                  </div>
                  <el-icon class="item-arrow"><ArrowRight /></el-icon>
                </div>
              </div>
            </template>

            <!-- Has query: show results -->
            <template v-else-if="searchResults.length > 0">
              <div
                v-for="(docs, category) in groupedResults"
                :key="category"
                class="search-section"
              >
                <div class="search-section-header">
                  <span class="search-section-title">{{ category }}</span>
                  <span class="section-count">{{ docs.length }}</span>
                </div>
                <div
                  v-for="doc in docs"
                  :key="doc.id"
                  class="search-result-item"
                  :class="{ active: flatResults[activeIndex]?.id === doc.id }"
                  @click="selectResult(doc)"
                  @mouseenter="activeIndex = flatResults.findIndex(r => r.id === doc.id)"
                >
                  <el-icon class="item-icon doc-icon"><Document /></el-icon>
                  <div class="item-content">
                    <div class="item-title" v-html="highlightText(doc.title, searchQuery)" />
                    <div class="item-desc" v-html="highlightText(doc.content, searchQuery)" />
                    <div class="item-meta">{{ doc.category }}</div>
                  </div>
                  <el-icon class="item-arrow"><ArrowRight /></el-icon>
                </div>
              </div>
            </template>

            <!-- No results -->
            <div v-else-if="noResults" class="no-results">
              <div class="no-results-icon">
                <el-icon size="48"><Search /></el-icon>
              </div>
              <div class="no-results-title">未找到相关文档</div>
              <div class="no-results-tips">
                <p>搜索建议：</p>
                <ul>
                  <li>检查关键词是否有拼写错误</li>
                  <li>尝试使用更简短的关键词</li>
                  <li>尝试使用不同的关键词组合</li>
                </ul>
              </div>
              <div class="no-results-hot">
                <p>推荐文档：</p>
                <div class="hot-links">
                  <a
                    v-for="doc in hotDocs"
                    :key="'nr-' + doc.id"
                    class="hot-link"
                    @click="selectResult(doc)"
                  >
                    {{ doc.title }}
                  </a>
                </div>
              </div>
            </div>
          </div>

          <!-- Footer -->
          <div class="search-footer">
            <div class="footer-hints">
              <span class="hint"><kbd>&uarr;</kbd><kbd>&darr;</kbd> 导航</span>
              <span class="hint"><kbd>Enter</kbd> 选择</span>
              <span class="hint"><kbd>ESC</kbd> 关闭</span>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
/* Overlay */
.search-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(0, 0, 0, 0.6);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 10vh;
}

/* Modal */
.search-modal {
  width: 680px;
  max-height: 70vh;
  background: #1e293b;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Input */
.search-input-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.search-icon {
  font-size: 20px;
  color: #64748b;
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  font-size: 18px;
  color: #e2e8f0;
}

.search-input::placeholder {
  color: #64748b;
}

.esc-hint {
  background: rgba(255, 255, 255, 0.1);
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  color: #94a3b8;
  cursor: pointer;
  flex-shrink: 0;
}

.esc-hint:hover {
  background: rgba(255, 255, 255, 0.15);
}

/* Body */
.search-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 0;
}

/* Section */
.search-section {
  margin-bottom: 8px;
}

.search-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 20px;
}

.search-section-title {
  font-size: 12px;
  color: #64748b;
  text-transform: uppercase;
  font-weight: 500;
  letter-spacing: 0.5px;
}

.section-count {
  font-size: 11px;
  color: #475569;
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.clear-btn {
  background: none;
  border: none;
  color: #64748b;
  font-size: 12px;
  cursor: pointer;
  padding: 2px 6px;
  border-radius: 4px;
}

.clear-btn:hover {
  color: #94a3b8;
  background: rgba(255, 255, 255, 0.05);
}

/* Items */
.search-item,
.search-result-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 20px;
  cursor: pointer;
  transition: background 0.15s;
}

.search-item:hover,
.search-result-item:hover,
.search-result-item.active {
  background: rgba(255, 255, 255, 0.05);
}

.search-result-item.active {
  background: rgba(64, 150, 255, 0.1);
}

.item-icon {
  font-size: 16px;
  color: #64748b;
  flex-shrink: 0;
}

.item-icon.doc-icon {
  color: #4096ff;
}

.item-text {
  flex: 1;
  font-size: 14px;
  color: #cbd5e1;
}

.item-content {
  flex: 1;
  min-width: 0;
}

.item-title {
  font-size: 14px;
  font-weight: 500;
  color: #e2e8f0;
  margin-bottom: 2px;
}

.item-title :deep(mark) {
  background: rgba(245, 158, 11, 0.3);
  color: #fbbf24;
  padding: 0 2px;
  border-radius: 2px;
}

.item-desc {
  font-size: 13px;
  color: #94a3b8;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.item-desc :deep(mark) {
  background: rgba(245, 158, 11, 0.3);
  color: #fbbf24;
  padding: 0 2px;
  border-radius: 2px;
}

.item-meta {
  font-size: 12px;
  color: #475569;
  margin-top: 2px;
}

.item-arrow {
  font-size: 14px;
  color: #475569;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}

.search-item:hover .item-arrow,
.search-result-item:hover .item-arrow,
.search-result-item.active .item-arrow {
  opacity: 1;
}

.remove-btn {
  background: none;
  border: none;
  color: #475569;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  opacity: 0;
  transition: opacity 0.15s;
}

.history-item:hover .remove-btn {
  opacity: 1;
}

.remove-btn:hover {
  color: #ef4444;
  background: rgba(239, 68, 68, 0.1);
}

/* No Results */
.no-results {
  padding: 32px 20px;
  text-align: center;
}

.no-results-icon {
  color: #334155;
  margin-bottom: 16px;
}

.no-results-title {
  font-size: 16px;
  font-weight: 500;
  color: #94a3b8;
  margin-bottom: 16px;
}

.no-results-tips {
  text-align: left;
  max-width: 300px;
  margin: 0 auto 20px;
}

.no-results-tips p {
  font-size: 13px;
  color: #64748b;
  margin-bottom: 8px;
}

.no-results-tips ul {
  list-style: none;
  padding: 0;
}

.no-results-tips li {
  font-size: 13px;
  color: #94a3b8;
  padding: 4px 0;
}

.no-results-tips li::before {
  content: '\2022';
  color: #475569;
  margin-right: 8px;
}

.no-results-hot p {
  font-size: 13px;
  color: #64748b;
  margin-bottom: 8px;
}

.hot-links {
  display: flex;
  gap: 8px;
  justify-content: center;
  flex-wrap: wrap;
}

.hot-link {
  padding: 6px 12px;
  background: rgba(64, 150, 255, 0.1);
  border: 1px solid rgba(64, 150, 255, 0.2);
  border-radius: 6px;
  color: #60a5fa;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s;
  text-decoration: none;
}

.hot-link:hover {
  background: rgba(64, 150, 255, 0.2);
  border-color: rgba(64, 150, 255, 0.4);
}

/* Footer */
.search-footer {
  padding: 10px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(15, 23, 42, 0.5);
}

.footer-hints {
  display: flex;
  gap: 16px;
}

.hint {
  font-size: 12px;
  color: #475569;
  display: flex;
  align-items: center;
  gap: 4px;
}

.hint kbd {
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 11px;
  color: #94a3b8;
  min-width: 20px;
  text-align: center;
}

/* Transition */
.search-fade-enter-active,
.search-fade-leave-active {
  transition: opacity 0.2s ease;
}

.search-fade-enter-active .search-modal,
.search-fade-leave-active .search-modal {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.search-fade-enter-from,
.search-fade-leave-to {
  opacity: 0;
}

.search-fade-enter-from .search-modal,
.search-fade-leave-to .search-modal {
  transform: translateY(-20px) scale(0.98);
  opacity: 0;
}
</style>
