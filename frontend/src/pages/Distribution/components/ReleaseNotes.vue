<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Plus, Edit, Delete, Refresh } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import client from '../../../api/client'
import type { Envelope, ReleaseNoteItem, ReleaseNoteListResp } from '../../../api/types'

const loading = ref(false)
const records = ref<ReleaseNoteItem[]>([])
const total = ref(0)
const pagination = reactive({ page: 1, page_size: 10 })

// 编辑弹窗
const dialogVisible = ref(false)
const dialogTitle = ref('新建发布说明')
const editingId = ref<number | null>(null)
const form = ref({ title: '', content: '', version: '', created_by: '' })
const saving = ref(false)

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('YYYY-MM-DD HH:mm')
}

async function fetchList() {
  loading.value = true
  try {
    const resp = await client.get<Envelope<ReleaseNoteListResp>>('/api/v1/release-notes', {
      params: { page: pagination.page, page_size: pagination.page_size, sort_by: 'created_at', sort_dir: 'desc' },
    })
    records.value = resp.data.data?.items ?? []
    total.value = resp.data.data?.total ?? 0
  } catch {
    records.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function openCreate() {
  editingId.value = null
  dialogTitle.value = '新建发布说明'
  form.value = { title: '', content: '', version: '', created_by: '' }
  dialogVisible.value = true
}

function openEdit(row: ReleaseNoteItem) {
  editingId.value = row.id
  dialogTitle.value = '编辑发布说明'
  form.value = { title: row.title, content: row.content, version: row.version, created_by: row.created_by }
  dialogVisible.value = true
}

async function saveForm() {
  if (!form.value.title.trim()) { ElMessage.warning('请填写标题'); return }
  if (!form.value.content.trim()) { ElMessage.warning('请填写内容'); return }
  saving.value = true
  try {
    if (editingId.value) {
      await client.put(`/api/v1/release-notes/${editingId.value}`, {
        title: form.value.title,
        content: form.value.content,
        version: form.value.version,
      })
      ElMessage.success('已更新')
    } else {
      await client.post('/api/v1/release-notes', form.value)
      ElMessage.success('已创建')
    }
    dialogVisible.value = false
    fetchList()
  } catch (err: any) {
    ElMessage.error('保存失败: ' + (err?.response?.data?.message || err.message || '未知错误'))
  } finally {
    saving.value = false
  }
}

async function handleDelete(row: ReleaseNoteItem) {
  try {
    await ElMessageBox.confirm(`确定删除「${row.title}」？`, '确认删除', { type: 'warning' })
    await client.delete(`/api/v1/release-notes/${row.id}`)
    ElMessage.success('已删除')
    fetchList()
  } catch { /* cancelled */ }
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchList()
}

function handleSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  fetchList()
}

// 供父组件选择用
function getContent(row: ReleaseNoteItem): string {
  return row.content
}

defineExpose({ refresh: fetchList, getContent })

onMounted(() => { fetchList() })
</script>

<template>
  <div class="release-notes-section">
    <div class="section-header">
      <h3 class="section-title">发布说明</h3>
      <div class="header-actions">
        <el-button :icon="Refresh" @click="fetchList" :loading="loading" size="small">刷新</el-button>
        <el-button type="primary" :icon="Plus" @click="openCreate" size="small">新建</el-button>
      </div>
    </div>

    <el-table :data="records" v-loading="loading" border stripe style="width: 100%;" size="small">
      <el-table-column prop="title" label="标题" min-width="180" show-overflow-tooltip />
      <el-table-column prop="version" label="版本号" width="120" show-overflow-tooltip>
        <template #default="{ row }">{{ row.version || '-' }}</template>
      </el-table-column>
      <el-table-column label="内容预览" min-width="240" show-overflow-tooltip>
        <template #default="{ row }">
          <span class="content-preview">{{ row.content }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="created_by" label="创建人" width="100" show-overflow-tooltip>
        <template #default="{ row }">{{ row.created_by || '-' }}</template>
      </el-table-column>
      <el-table-column label="时间" width="150">
        <template #default="{ row }">{{ formatTime(row.updated_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="140" align="center" fixed="right">
        <template #default="{ row }">
          <el-button size="small" type="primary" text :icon="Edit" @click="openEdit(row)">编辑</el-button>
          <el-button size="small" type="danger" text :icon="Delete" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="total > pagination.page_size" class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>

    <!-- 编辑弹窗 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="600px" :close-on-click-modal="false">
      <el-form label-position="top">
        <el-form-item label="标题">
          <el-input v-model="form.title" placeholder="如：v2.4.1 安全补丁更新" />
        </el-form-item>
        <el-row :gutter="12">
          <el-col :span="12">
            <el-form-item label="版本号">
              <el-input v-model="form.version" placeholder="如：v2.4.1" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item v-if="!editingId" label="创建人">
              <el-input v-model="form.created_by" placeholder="姓名" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="发布说明内容">
          <el-input v-model="form.content" type="textarea" :rows="6"
            placeholder="详细描述本次发布的变更内容、修复的问题、新增功能等..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="saveForm">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.release-notes-section {
  background: #fff;
  border-radius: 8px;
  padding: 20px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-title {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: #303133;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.content-preview {
  font-size: 12px;
  color: #606266;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
