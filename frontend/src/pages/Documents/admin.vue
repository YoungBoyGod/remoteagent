<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import {
  Search,
  Plus,
  Edit,
  Delete,
  Promotion,
  FolderOpened,
  Clock,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDocumentStore } from '@/stores/document'
import dayjs from 'dayjs'

const router = useRouter()
const store = useDocumentStore()

// ==================== 筛选 ====================
const searchQuery = ref('')
const filterCategory = ref<number | ''>('')
const filterStatus = ref('')
const currentPage = ref(1)
const pageSize = ref(10)

const statusOptions = [
  { value: 'draft', label: '草稿' },
  { value: 'published', label: '已发布' },
  { value: 'archived', label: '已归档' },
]

// ==================== 加载数据 ====================
onMounted(async () => {
  await Promise.all([
    store.fetchDocuments(),
    store.fetchCategories(),
  ])
})

// ==================== 分类选项（从 store 获取） ====================
const categoryOptions = computed(() =>
  store.categories.map(c => ({ value: c.id, label: c.name }))
)

// ==================== 过滤 ====================
const filteredDocuments = computed(() => {
  return store.documents.filter(doc => {
    const matchSearch = !searchQuery.value ||
      doc.title.toLowerCase().includes(searchQuery.value.toLowerCase()) ||
      doc.slug.toLowerCase().includes(searchQuery.value.toLowerCase())
    const matchCategory = !filterCategory.value || doc.category_id === filterCategory.value
    const matchStatus = !filterStatus.value || doc.status === filterStatus.value
    return matchSearch && matchCategory && matchStatus
  })
})

const paginatedDocuments = computed(() => {
  const start = (currentPage.value - 1) * pageSize.value
  return filteredDocuments.value.slice(start, start + pageSize.value)
})

watch([searchQuery, filterCategory, filterStatus], () => {
  currentPage.value = 1
})

// ==================== 操作 ====================
function createDoc() {
  router.push('/documents/editor')
}

function editDoc(slug: string) {
  router.push(`/documents/editor/${slug}`)
}

async function deleteDoc(doc: { title: string; slug: string }) {
  try {
    await ElMessageBox.confirm(
      `确定要删除文档「${doc.title}」吗？此操作不可恢复。`,
      '删除确认',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
    await store.deleteDocument(doc.slug)
    ElMessage.success('文档已删除')
  } catch { /* cancelled */ }
}

async function togglePublish(doc: { slug: string; status: string }) {
  const newStatus = doc.status === 'published' ? 'archived' : 'published'
  await store.updateDocument(doc.slug, { status: newStatus as 'published' | 'archived' })
  ElMessage.success(newStatus === 'published' ? '文档已发布' : '文档已归档')
}

function getCategoryName(categoryId: number) {
  return store.categoryMap.get(categoryId)?.name || '-'
}

function formatTime(ts: number) {
  return dayjs(ts * 1000).format('YYYY-MM-DD HH:mm')
}

function getStatusType(status: string) {
  switch (status) {
    case 'published': return 'success'
    case 'draft': return 'info'
    case 'archived': return 'warning'
    default: return 'info'
  }
}

function getStatusLabel(status: string) {
  switch (status) {
    case 'published': return '已发布'
    case 'draft': return '草稿'
    case 'archived': return '已归档'
    default: return status
  }
}
</script>

<template>
  <div class="admin-page">
    <h2 class="page-title">文档管理</h2>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-input
        v-model="searchQuery"
        placeholder="搜索文档标题或 slug..."
        :prefix-icon="Search"
        clearable
      />
      <el-select v-model="filterCategory" placeholder="全部分类" clearable>
        <el-option
          v-for="opt in categoryOptions"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-select>
      <el-select v-model="filterStatus" placeholder="全部状态" clearable>
        <el-option
          v-for="opt in statusOptions"
          :key="opt.value"
          :label="opt.label"
          :value="opt.value"
        />
      </el-select>
      <div class="toolbar-spacer" />
      <el-button type="primary" @click="createDoc">
        <el-icon><Plus /></el-icon>
        <span>新建文档</span>
      </el-button>
    </div>

    <!-- 文档表格 -->
    <el-card shadow="never">
      <el-table :data="paginatedDocuments" v-loading="store.loadingDocs" stripe style="width: 100%">
        <el-table-column prop="title" label="标题" min-width="200">
          <template #default="{ row }">
            <div class="doc-title-cell">
              <span class="doc-title-text">{{ row.title }}</span>
              <span class="doc-slug">{{ row.slug }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="分类" width="120">
          <template #default="{ row }">
            <span class="category-text">
              <el-icon style="margin-right: 4px;"><FolderOpened /></el-icon>
              {{ getCategoryName(row.category_id) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getStatusType(row.status)" size="small">
              {{ getStatusLabel(row.status) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="language" label="语言" width="80" align="center">
          <template #default="{ row }">
            <span>{{ row.language === 'zh' ? '中文' : 'EN' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="author" label="作者" width="100" />
        <el-table-column label="更新时间" width="170">
          <template #default="{ row }">
            <div class="time-cell">
              <el-icon><Clock /></el-icon>
              <span>{{ formatTime(row.updated_at) }}</span>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-button link type="primary" @click="editDoc(row.slug)">
              <el-icon><Edit /></el-icon>
              编辑
            </el-button>
            <el-button link :type="row.status === 'published' ? 'warning' : 'success'" @click="togglePublish(row)">
              <el-icon><Promotion /></el-icon>
              {{ row.status === 'published' ? '归档' : '发布' }}
            </el-button>
            <el-button link type="danger" @click="deleteDoc(row)">
              <el-icon><Delete /></el-icon>
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
      <div class="pagination-wrapper">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :total="filteredDocuments.length"
          :page-sizes="[10, 20, 50]"
          layout="total, sizes, prev, pager, next, jumper"
        />
      </div>
    </el-card>
  </div>
</template>

<style scoped>
.admin-page {
  /* 使用全局 page-title 和 toolbar 样式 */
}

.doc-title-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.doc-title-text {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.doc-slug {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-family: 'JetBrains Mono', monospace;
}

.time-cell {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.category-text {
  display: inline-flex;
  align-items: center;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
