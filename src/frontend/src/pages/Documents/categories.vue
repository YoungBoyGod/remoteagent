<script setup lang="ts">
import { ref, onMounted } from 'vue'
import {
  Plus,
  Edit,
  Delete,
  FolderAdd,
} from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDocumentStore } from '@/stores/document'
import type { DocCategoryRecord } from './types'

const store = useDocumentStore()

// ==================== 弹窗 ====================
const dialogVisible = ref(false)
const dialogTitle = ref('添加分类')
const editingId = ref<number | null>(null)
const parentId = ref<number | null>(null)

const form = ref({
  name: '',
  slug: '',
  icon: '',
  color: '#4096ff',
})

const iconOptions = [
  'Box', 'Document', 'Setting', 'Monitor', 'Connection',
  'Cpu', 'Upload', 'DataLine', 'FolderOpened', 'Reading',
]

const colorPresets = [
  '#4096ff', '#722ed1', '#fa8c16', '#52c41a',
  '#ff4d4f', '#13c2c2', '#eb2f96', '#1677ff',
]

// ==================== 加载数据 ====================
onMounted(() => {
  store.fetchCategories()
})

// ==================== 操作 ====================
function openAddDialog(pId?: number) {
  dialogTitle.value = pId ? '添加子分类' : '添加分类'
  editingId.value = null
  parentId.value = pId ?? null
  form.value = { name: '', slug: '', icon: 'Document', color: '#4096ff' }
  dialogVisible.value = true
}

function openEditDialog(cat: DocCategoryRecord) {
  dialogTitle.value = '编辑分类'
  editingId.value = cat.id
  parentId.value = null
  form.value = {
    name: cat.name,
    slug: cat.slug,
    icon: cat.icon,
    color: cat.color,
  }
  dialogVisible.value = true
}

async function saveCategory() {
  if (!form.value.name) {
    ElMessage.warning('请输入分类名称')
    return
  }
  if (!form.value.slug) {
    ElMessage.warning('请输入分类 slug')
    return
  }

  try {
    if (editingId.value) {
      await store.updateCategory(editingId.value, {
        name: form.value.name,
        slug: form.value.slug,
        icon: form.value.icon,
        color: form.value.color,
      })
      ElMessage.success('分类已更新')
    } else {
      await store.createCategory({
        name: form.value.name,
        slug: form.value.slug,
        icon: form.value.icon,
        color: form.value.color,
        parent_id: parentId.value,
      })
      ElMessage.success('分类已添加')
    }
    dialogVisible.value = false
  } catch {
    // error handled by interceptor
  }
}

async function handleDelete(cat: DocCategoryRecord) {
  try {
    await ElMessageBox.confirm(
      `确定要删除分类「${cat.name}」吗？其下的子分类也会被删除。`,
      '删除确认',
      { confirmButtonText: '删除', cancelButtonText: '取消', type: 'warning' }
    )
    await store.deleteCategory(cat.id)
    ElMessage.success('分类已删除')
  } catch { /* cancelled */ }
}

const defaultProps = {
  children: 'children',
  label: 'name',
}
</script>

<template>
  <div class="categories-page">
    <h2 class="page-title">分类管理</h2>

    <div class="toolbar">
      <span class="toolbar-hint">拖拽节点可调整分类层级</span>
      <div class="toolbar-spacer" />
      <el-button type="primary" @click="openAddDialog()">
        <el-icon><Plus /></el-icon>
        <span>添加分类</span>
      </el-button>
    </div>

    <el-card shadow="never">
      <el-tree
        :data="store.categoryTree"
        :props="defaultProps"
        node-key="id"
        default-expand-all
        :expand-on-click-node="false"
        draggable
        v-loading="store.loadingCategories"
        class="category-tree"
      >
        <template #default="{ data }">
          <div class="tree-node">
            <div class="node-info">
              <span class="node-color" :style="{ background: data.color }" />
              <span class="node-label">{{ data.name }}</span>
              <span class="node-slug">{{ data.slug }}</span>
            </div>
            <div class="node-actions">
              <el-button link size="small" type="primary" @click.stop="openAddDialog(data.id)">
                <el-icon><FolderAdd /></el-icon>
              </el-button>
              <el-button link size="small" type="primary" @click.stop="openEditDialog(data)">
                <el-icon><Edit /></el-icon>
              </el-button>
              <el-button link size="small" type="danger" @click.stop="handleDelete(data)">
                <el-icon><Delete /></el-icon>
              </el-button>
            </div>
          </div>
        </template>
      </el-tree>
    </el-card>

    <!-- 添加/编辑弹窗 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="480px"
      destroy-on-close
    >
      <el-form :model="form" label-position="top">
        <el-form-item label="分类名称" required>
          <el-input v-model="form.name" placeholder="例如: 产品文档" />
        </el-form-item>
        <el-form-item label="Slug" required>
          <el-input v-model="form.slug" placeholder="例如: product" />
        </el-form-item>
        <el-form-item label="图标">
          <el-select v-model="form.icon" placeholder="选择图标">
            <el-option v-for="icon in iconOptions" :key="icon" :label="icon" :value="icon" />
          </el-select>
        </el-form-item>
        <el-form-item label="颜色">
          <div class="color-picker">
            <div
              v-for="color in colorPresets"
              :key="color"
              class="color-swatch"
              :class="{ active: form.color === color }"
              :style="{ background: color }"
              @click="form.color = color"
            />
            <el-color-picker v-model="form.color" size="small" />
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveCategory">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.toolbar-hint {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.category-tree {
  background: transparent;
}

.category-tree :deep(.el-tree-node__content) {
  height: 48px;
  border-radius: 8px;
  margin: 2px 0;
  padding-right: 8px;
}

.category-tree :deep(.el-tree-node__content:hover) {
  background: var(--el-bg-color-hover);
}

.tree-node {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding-right: 8px;
}

.node-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.node-color {
  width: 12px;
  height: 12px;
  border-radius: 3px;
  flex-shrink: 0;
}

.node-label {
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.node-slug {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  font-family: 'JetBrains Mono', monospace;
  background: var(--el-fill-color);
  padding: 2px 8px;
  border-radius: 4px;
}

.node-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s;
}

.category-tree :deep(.el-tree-node__content:hover) .node-actions {
  opacity: 1;
}

/* Color Picker */
.color-picker {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.color-swatch {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  cursor: pointer;
  border: 2px solid transparent;
  transition: all 0.2s;
}

.color-swatch:hover {
  transform: scale(1.1);
}

.color-swatch.active {
  border-color: var(--el-text-color-primary);
  box-shadow: 0 0 0 2px var(--el-bg-color), 0 0 0 4px currentColor;
}
</style>
