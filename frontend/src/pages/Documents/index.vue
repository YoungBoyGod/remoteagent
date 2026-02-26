<script setup lang="ts">
import { ref, computed, watch, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import {
  Search,
  ArrowLeft,
  Download,
  Clock,
  User,
  Check,
  Edit,
  Share,
  CircleCheck,
  CircleClose,
  View,
  Document as DocumentIcon,
  List as ListIcon,
  Reading,
  Upload,
  MoreFilled,
  FolderOpened,
  ArrowRight as ArrowRightIcon,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useDocumentStore } from '@/stores/document'
import { marked } from 'marked'
import dayjs from 'dayjs'

const store = useDocumentStore()
const router = useRouter()

// ==================== 状态管理 ====================
const searchQuery = ref('')
const searchVisible = ref(false)
const feedbackVisible = ref(false)
const feedbackForm = ref<{ type: 'content' | 'missing' | 'other'; description: string }>({ type: 'content', description: '' })
const activeDocSlug = ref('')
const activeTab = ref('list')
const currentCategory = ref('all')
const tocItems = ref<{ id: string; text: string; level: number }[]>([])
const activeTocId = ref('')
const expandedCategories = ref<number[]>([])
const currentPage = ref(1)
const pageSize = ref(10)

// ==================== 计算属性 ====================

// 将 store 分类 + 文档组合成带 items 的导航结构
const categoryNav = computed(() => {
  return store.categories.map(cat => ({
    id: cat.id,
    name: cat.name,
    slug: cat.slug,
    icon: cat.icon || 'FolderOpened',
    color: cat.color || '#4096ff',
    items: store.documents.filter(doc => doc.category_id === cat.id),
  }))
})

const filteredDocuments = computed(() => {
  let result = store.documents
  if (currentCategory.value !== 'all') {
    const catId = Number(currentCategory.value)
    result = result.filter(doc => doc.category_id === catId)
  }
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(doc => doc.title.toLowerCase().includes(query))
  }
  return result
})

const paginatedDocuments = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredDocuments.value.slice(start, start + pageSize.value)
})

// 筛选变化时重置页码
watch([searchQuery, currentCategory], () => {
  currentPage.value = 1
})

// 当前选中的文档
const activeDoc = computed(() => {
  return store.documents.find(d => d.slug === activeDocSlug.value) || null
})

// 渲染 Markdown 为 HTML
const renderedContent = computed(() => {
  if (!activeDoc.value?.content) return ''
  return marked.parse(activeDoc.value.content) as string
})

// ==================== 方法 ====================
function toggleCategory(catId: number) {
  const idx = expandedCategories.value.indexOf(catId)
  if (idx > -1) {
    expandedCategories.value.splice(idx, 1)
  } else {
    expandedCategories.value.push(catId)
  }
}

function isCategoryExpanded(catId: number) {
  return expandedCategories.value.includes(catId)
}

function selectDoc(slug: string) {
  activeDocSlug.value = slug
  activeTab.value = 'detail'
  nextTick(() => {
    generateTOC()
  })
}

function generateTOC() {
  nextTick(() => {
    const container = document.querySelector('.doc-body')
    if (!container) return
    const headings = container.querySelectorAll('h1, h2, h3')
    tocItems.value = Array.from(headings).map((el, i) => {
      const id = `heading-${i}`
      el.id = id
      const level = parseInt(el.tagName[1]) - 2 // h2=0, h3=1
      return { id, text: el.textContent || '', level: Math.max(0, level) }
    })
  })
}

function scrollToSection(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeTocId.value = id
  }
}

function goBack() {
  activeTab.value = 'list'
}

async function downloadPDF() {
  if (!activeDoc.value) return
  try {
    const blob = await store.exportPdf(activeDoc.value.slug)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${activeDoc.value.title}.pdf`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.warning('PDF 导出功能暂未开放')
  }
}

function editDoc(slug: string) {
  router.push(`/documents/editor/${slug}`)
}

async function downloadDoc(slug: string, title: string) {
  try {
    const blob = await store.exportPdf(slug)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${title}.pdf`
    a.click()
    URL.revokeObjectURL(url)
  } catch {
    ElMessage.warning('下载功能暂未开放')
  }
}

function shareDoc(slug: string) {
  const url = `${window.location.origin}/documents/${slug}`
  navigator.clipboard.writeText(url).then(() => {
    ElMessage.success('文档链接已复制到剪贴板')
  })
}

function handleDropdownCommand(command: string, row: { slug: string; title: string }) {
  switch (command) {
    case 'download': downloadDoc(row.slug, row.title); break
    case 'share': shareDoc(row.slug); break
    case 'edit': editDoc(row.slug); break
  }
}

function filterByCategory(categoryId: string) {
  currentCategory.value = categoryId
}

async function submitFeedback() {
  if (!activeDoc.value || !feedbackForm.value.description) {
    ElMessage.warning('请填写反馈描述')
    return
  }
  try {
    await store.submitFeedback(activeDoc.value.slug, {
      type: feedbackForm.value.type,
      description: feedbackForm.value.description,
    })
    ElMessage.success('感谢您的反馈！')
    feedbackVisible.value = false
    feedbackForm.value = { type: 'content', description: '' }
  } catch {
    ElMessage.warning('反馈提交功能暂未开放')
    feedbackVisible.value = false
  }
}

function formatTime(ts: number) {
  return dayjs(ts * 1000).format('YYYY-MM-DD')
}

function getCategoryName(categoryId: number) {
  return store.categoryMap.get(categoryId)?.name || '-'
}

// ==================== 初始化 ====================
onMounted(async () => {
  await Promise.all([store.fetchDocuments(), store.fetchCategories()])
  // 默认展开所有分类
  expandedCategories.value = store.categories.map(c => c.id)

  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      searchVisible.value = true
    }
  })
})
</script>

<template>
  <div>
    <!-- 文档列表页 -->
    <template v-if="activeTab === 'list'">
      <h2 class="page-title">
        <el-icon size="28"><Reading /></el-icon>
        文档中心
      </h2>

      <el-row :gutter="20">
        <!-- 左侧分类导航 -->
        <el-col :span="4">
          <el-card shadow="hover" class="category-nav-card">
            <template #header>
              <div class="nav-header">
                <el-icon><FolderOpened /></el-icon>
                <span>文档分类</span>
              </div>
            </template>
            
            <div class="category-list">
              <div
                class="category-item"
                :class="{ active: currentCategory === 'all' }"
                @click="filterByCategory('all')"
              >
                <el-icon><DocumentIcon /></el-icon>
                <span>全部文档</span>
                <el-tag size="small" type="info" effect="plain">{{ store.totalDocs }}</el-tag>
              </div>

              <div v-for="cat in categoryNav" :key="cat.id" class="category-section">
                <div
                  class="category-title"
                  :class="{ expanded: isCategoryExpanded(cat.id) }"
                  @click="toggleCategory(cat.id)"
                >
                  <div class="category-title-left">
                    <el-icon :style="{ color: cat.color }">
                      <component :is="cat.icon" />
                    </el-icon>
                    <span>{{ cat.name }}</span>
                    <el-tag type="info" size="small" effect="plain">{{ cat.items.length }}</el-tag>
                  </div>
                  <el-icon class="category-arrow"><ArrowRightIcon /></el-icon>
                </div>

                <div v-show="isCategoryExpanded(cat.id)" class="category-docs">
                  <div
                    v-for="item in cat.items"
                    :key="item.slug"
                    class="doc-item"
                    :class="{ active: activeDocSlug === item.slug }"
                    @click="selectDoc(item.slug)"
                  >
                    <el-icon size="14"><DocumentIcon /></el-icon>
                    <span class="doc-item-text">{{ item.title }}</span>
                  </div>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <!-- 右侧内容区 -->
        <el-col :span="20">
          <!-- 统计概览 -->
          <el-row :gutter="16" class="stats-row">
            <el-col :xs="12" :sm="6">
              <div class="stat-mini">
                <div class="stat-mini-value">{{ store.totalDocs }}</div>
                <div class="stat-mini-label">总文档数</div>
              </div>
            </el-col>
            <el-col v-for="cat in categoryNav.slice(0, 3)" :key="cat.id" :xs="12" :sm="6">
              <div class="stat-mini">
                <div class="stat-mini-value" :style="{ color: cat.color }">{{ cat.items.length }}</div>
                <div class="stat-mini-label">{{ cat.name }}</div>
              </div>
            </el-col>
          </el-row>

          <!-- 工具栏 -->
          <div class="toolbar">
            <el-input
              v-model="searchQuery"
              placeholder="搜索文档标题..."
              clearable
              :prefix-icon="Search"
              style="width: 300px"
            />
            <div class="toolbar-spacer" />
            <el-button :icon="Upload" type="primary" @click="router.push('/documents/editor')">新建文档</el-button>
          </div>

          <!-- 文档表格 -->
          <el-table :data="paginatedDocuments" v-loading="store.loadingDocs" stripe border style="width: 100%">
            <el-table-column prop="title" label="文档名称" min-width="250" show-overflow-tooltip>
              <template #default="{ row }">
                <el-link type="primary" underline="never" @click="selectDoc(row.slug)">
                  <el-icon style="margin-right: 8px; color: var(--el-color-primary)"><DocumentIcon /></el-icon>
                  {{ row.title }}
                </el-link>
              </template>
            </el-table-column>
            <el-table-column label="分类" width="120" align="center">
              <template #default="{ row }">
                <span class="category-badge">{{ row.category_name || getCategoryName(row.category_id) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="90" align="center">
              <template #default="{ row }">
                <el-tag :type="row.status === 'published' ? 'success' : row.status === 'draft' ? 'info' : 'warning'" size="small">
                  {{ row.status === 'published' ? '已发布' : row.status === 'draft' ? '草稿' : '已归档' }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="author" label="作者" width="100" align="center" />
            <el-table-column label="更新时间" width="140" align="center">
              <template #default="{ row }">
                <el-icon style="margin-right: 4px; color: var(--el-text-color-secondary); font-size: 12px"><Clock /></el-icon>
                {{ formatTime(row.updated_at) }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="selectDoc(row.slug)">查看</el-button>
                <el-dropdown trigger="click" @command="(cmd: string) => handleDropdownCommand(cmd, row)">
                  <el-button link :icon="MoreFilled" />
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item :icon="Download" command="download">下载</el-dropdown-item>
                      <el-dropdown-item :icon="Share" command="share">分享</el-dropdown-item>
                      <el-dropdown-item :icon="Edit" command="edit" divided>编辑</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>

          <!-- 分页 -->
          <div class="pagination-wrapper">
            <el-pagination
              v-model:current-page="currentPage"
              v-model:page-size="pageSize"
              :total="filteredDocuments.length"
              :page-sizes="[10, 20, 50]"
              layout="total, sizes, prev, pager, next, jumper"
            />
          </div>
        </el-col>
      </el-row>
    </template>

    <!-- 文档详情页 -->
    <template v-else>
      <!-- 返回按钮 -->
      <div style="margin-bottom: 16px">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回列表
        </el-button>
      </div>

      <el-row :gutter="20">
        <!-- 左侧文档导航 -->
        <el-col :span="4">
          <el-card shadow="hover" class="doc-nav-card">
            <template #header>
              <div class="nav-header">
                <el-icon><FolderOpened /></el-icon>
                <span>文档导航</span>
              </div>
            </template>
            
            <div class="doc-nav">
              <div v-for="cat in categoryNav" :key="cat.id" class="nav-section">
                <div
                  class="nav-section-title"
                  :class="{ expanded: isCategoryExpanded(cat.id) }"
                  @click="toggleCategory(cat.id)"
                >
                  <div class="nav-section-left">
                    <el-icon :style="{ color: cat.color }">
                      <component :is="cat.icon" />
                    </el-icon>
                    <span>{{ cat.name }}</span>
                  </div>
                  <el-icon class="nav-arrow"><ArrowRightIcon /></el-icon>
                </div>

                <div v-show="isCategoryExpanded(cat.id)" class="nav-items">
                  <a
                    v-for="item in cat.items"
                    :key="item.slug"
                    class="nav-item"
                    :class="{ active: activeDocSlug === item.slug }"
                    @click="selectDoc(item.slug)"
                  >
                    {{ item.title }}
                  </a>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <!-- 中间内容区 -->
        <el-col :span="15">
          <el-card shadow="hover">
            <!-- 文档头部 -->
            <div class="doc-detail-header">
              <div class="doc-meta">
                <span class="doc-badge success" v-if="activeDoc?.status === 'published'">
                  <el-icon><Check /></el-icon>
                  已发布
                </span>
                <span class="doc-badge" v-else style="background: var(--el-color-info-light-9); color: var(--el-color-info);">
                  草稿
                </span>
                <span class="doc-meta-item">
                  <el-icon><Clock /></el-icon>
                  最后更新: {{ activeDoc ? formatTime(activeDoc.updated_at) : '-' }}
                </span>
                <span class="doc-meta-item">
                  <el-icon><User /></el-icon>
                  {{ activeDoc?.author || '-' }}
                </span>
                <div style="flex: 1" />
                <el-button size="small" @click="downloadPDF">
                  <el-icon><Download /></el-icon>
                  导出 PDF
                </el-button>
              </div>
              <h1 class="doc-title">{{ activeDoc?.title || '文档详情' }}</h1>
              <p class="doc-desc">{{ activeDoc?.category_name || '' }}</p>
            </div>

            <el-divider />

            <!-- 文档正文 -->
            <div class="doc-body" v-html="renderedContent" />

            <el-divider />

            <!-- 文档底部 -->
            <div class="doc-footer-actions">
              <span class="footer-label">这篇文档对您有帮助吗？</span>
              <el-button @click="ElMessage.success('感谢您的认可！')">
                <el-icon><CircleCheck /></el-icon>
                有帮助
              </el-button>
              <el-button @click="feedbackVisible = true">
                <el-icon><CircleClose /></el-icon>
                需要改进
              </el-button>
              <span class="view-count">
                <el-icon><View /></el-icon>
                {{ activeDoc?.view_count?.toLocaleString() || 0 }} 次阅读
              </span>
            </div>
          </el-card>
        </el-col>

        <!-- 右侧目录 -->
        <el-col :span="5">
          <div class="toc-sidebar">
            <el-card shadow="hover">
              <template #header>
                <div class="toc-header">
                  <el-icon><ListIcon /></el-icon>
                  <span>目录</span>
                </div>
              </template>
              <div class="toc-nav">
                <a
                  v-for="item in tocItems"
                  :key="item.id"
                  class="toc-item"
                  :class="{ 
                    active: activeTocId === item.id,
                    'toc-level-1': item.level === 1 
                  }"
                  @click="scrollToSection(item.id)"
                >
                  {{ item.text }}
                </a>
              </div>
            </el-card>
          </div>
        </el-col>
      </el-row>
    </template>

    <!-- 搜索对话框 -->
    <el-dialog
      v-model="searchVisible"
      title="搜索文档"
      width="600px"
      destroy-on-close
    >
      <el-input
        v-model="searchQuery"
        placeholder="搜索文档标题、内容..."
        clearable
        :prefix-icon="Search"
        style="margin-bottom: 20px"
      />
      <div v-if="searchQuery" class="search-results">
        <div
          v-for="doc in filteredDocuments"
          :key="doc.slug"
          class="search-result-item"
          @click="selectDoc(doc.slug); searchVisible = false"
        >
          <el-icon><DocumentIcon /></el-icon>
          <span>{{ doc.title }}</span>
        </div>
        <el-empty v-if="!filteredDocuments.length" description="未找到匹配文档" :image-size="60" />
      </div>
    </el-dialog>

    <!-- 反馈对话框 -->
    <el-dialog
      v-model="feedbackVisible"
      title="文档反馈"
      width="500px"
      destroy-on-close
    >
      <el-form label-position="top">
        <el-form-item label="反馈类型">
          <el-radio-group v-model="feedbackForm.type">
            <el-radio-button value="content">内容错误</el-radio-button>
            <el-radio-button value="missing">缺少信息</el-radio-button>
            <el-radio-button value="other">其他建议</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="详细描述">
          <el-input v-model="feedbackForm.description" type="textarea" :rows="4" placeholder="请描述您遇到的问题或建议..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="feedbackVisible = false">取消</el-button>
        <el-button type="primary" @click="submitFeedback">
          提交反馈
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* 左侧分类导航 */
.category-nav-card :deep(.el-card__header) {
  padding: 12px 16px;
}

.nav-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.category-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.category-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.category-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}

.category-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}

.category-item .el-tag {
  margin-left: auto;
}

.category-section {
  margin-top: 4px;
}

.category-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.category-title:hover {
  background: var(--el-fill-color-light);
}

.category-title-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.category-arrow {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  transition: transform 0.2s;
}

.category-title.expanded .category-arrow {
  transform: rotate(90deg);
}

.category-docs {
  padding-left: 12px;
}

.doc-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px 8px 24px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.doc-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}

.doc-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}

.doc-item-text {
  flex: 1;
}

/* 统计卡片 */
.stats-row {
  margin-bottom: 20px;
}

.stat-mini {
  background: #ffffff;
  border-radius: 12px;
  padding: 16px 20px;
  text-align: center;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
  border: 1px solid var(--el-border-color-light);
  transition: all 0.3s ease;
}

.stat-mini:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
}

.stat-mini-value {
  font-size: 24px;
  font-weight: 700;
  margin-bottom: 4px;
  color: var(--el-text-color-primary);
}

.stat-mini-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 表格样式 */
.category-badge {
  display: inline-block;
  padding: 2px 10px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.version-badge {
  display: inline-block;
  padding: 2px 8px;
  background: var(--el-color-primary-light-9);
  border-radius: 4px;
  font-size: 12px;
  color: var(--el-color-primary);
  font-weight: 500;
}

/* 文档详情页左侧导航 */
.doc-nav-card :deep(.el-card__header) {
  padding: 12px 16px;
}

.doc-nav {
  max-height: calc(100vh - 300px);
  overflow-y: auto;
}

.nav-section {
  margin-bottom: 4px;
}

.nav-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
  cursor: pointer;
  transition: all 0.2s;
  border-radius: 6px;
}

.nav-section-title:hover {
  background: var(--el-fill-color-light);
}

.nav-section-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.nav-arrow {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  transition: transform 0.2s;
}

.nav-section-title.expanded .nav-arrow {
  transform: rotate(90deg);
}

.nav-items {
  padding-left: 24px;
}

.nav-item {
  display: block;
  padding: 8px 12px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.2s;
  text-decoration: none;
}

.nav-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}

.nav-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}

/* 文档内容 */
.doc-detail-header {
  margin-bottom: 20px;
}

.doc-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.doc-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.doc-badge.success {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
  border: 1px solid var(--el-color-success-light-5);
}

.doc-meta-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.doc-title {
  font-size: 32px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  margin-bottom: 12px;
}

.doc-desc {
  font-size: 16px;
  color: var(--el-text-color-secondary);
}

.doc-body {
  font-size: 14px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
}

.doc-body :deep(h2) {
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-top: 32px;
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.doc-body :deep(h3) {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-top: 24px;
  margin-bottom: 12px;
}

.doc-body :deep(p) {
  margin-bottom: 16px;
}

.doc-body :deep(ul),
.doc-body :deep(ol) {
  margin-bottom: 16px;
  padding-left: 24px;
}

.doc-body :deep(li) {
  margin-bottom: 8px;
}

.doc-body :deep(a) {
  color: var(--el-color-primary);
  text-decoration: none;
}

.doc-body :deep(a:hover) {
  text-decoration: underline;
}

.code-block {
  background: #1e1e1e;
  border-radius: 8px;
  padding: 16px;
  margin: 16px 0;
  overflow-x: auto;
}

.code-block code {
  color: #d4d4d4;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre;
}

/* 文档底部 */
.doc-footer-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.footer-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.view-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 右侧目录 */
.toc-sidebar {
  position: sticky;
  top: 20px;
}

.toc-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.toc-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toc-item {
  display: block;
  padding: 6px 0;
  color: var(--el-text-color-regular);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  text-decoration: none;
  border-radius: 4px;
}

.toc-item:hover {
  color: var(--el-color-primary);
  background: var(--el-fill-color-light);
  padding-left: 8px;
}

.toc-item.active {
  color: var(--el-color-primary);
  font-weight: 500;
  background: var(--el-color-primary-light-9);
  padding-left: 8px;
}

.toc-level-1 {
  padding-left: 12px;
}

/* 分页 */
.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
  padding: 16px 0;
}

/* 搜索结果 */
.search-result-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  font-size: 14px;
  color: var(--el-text-color-regular);
  transition: all 0.2s;
}

.search-result-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}
</style>
