<script setup lang="ts">
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useEditor, EditorContent } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import Image from '@tiptap/extension-image'
import Placeholder from '@tiptap/extension-placeholder'
import Link from '@tiptap/extension-link'
import Underline from '@tiptap/extension-underline'
import TextAlign from '@tiptap/extension-text-align'
import Highlight from '@tiptap/extension-highlight'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import { Table } from '@tiptap/extension-table'
import TableRow from '@tiptap/extension-table-row'
import TableCell from '@tiptap/extension-table-cell'
import TableHeader from '@tiptap/extension-table-header'
import CodeBlockLowlight from '@tiptap/extension-code-block-lowlight'
import { Markdown } from 'tiptap-markdown'
import { common, createLowlight } from 'lowlight'
import {
  ArrowLeft,
  Document,
  Check,
  EditPen,
  Promotion,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useDocumentStore } from '@/stores/document'

const route = useRoute()
const router = useRouter()
const store = useDocumentStore()

const slug = computed(() => route.params.slug as string | undefined)
const isEdit = computed(() => !!slug.value)

// ==================== 表单数据 ====================
const form = ref({
  title: '',
  slug: '',
  category_id: undefined as number | undefined,
  language: 'zh',
  status: 'draft' as 'draft' | 'published' | 'archived',
  content: '',
})

// ==================== 版本发布 ====================
const versionDialogVisible = ref(false)
const versionForm = ref({
  version: '',
  changelog: '',
})

const languageOptions = [
  { value: 'zh', label: '中文' },
  { value: 'en', label: 'English' },
]

const categoryOptions = computed(() =>
  store.categories.map(c => ({ value: c.id, label: c.name }))
)

// ==================== Tiptap 编辑器 ====================
const lowlight = createLowlight(common)

const editor = useEditor({
  extensions: [
    StarterKit.configure({ codeBlock: false }),
    CodeBlockLowlight.configure({ lowlight }),
    Image.configure({ inline: true, allowBase64: true }),
    Placeholder.configure({ placeholder: '开始编写文档内容...' }),
    Link.configure({ openOnClick: false }),
    Underline,
    TextAlign.configure({ types: ['heading', 'paragraph'] }),
    Highlight,
    TaskList,
    TaskItem.configure({ nested: true }),
    Table.configure({ resizable: true }),
    TableRow,
    TableCell,
    TableHeader,
    Markdown,
  ],
  content: '',
  onUpdate: ({ editor: e }) => {
    form.value.content = (e.storage as Record<string, any>).markdown.getMarkdown()
  },
})

// ==================== slug 自动生成 ====================
watch(() => form.value.title, (title) => {
  if (!isEdit.value && title) {
    form.value.slug = title
      .toLowerCase()
      .replace(/[\s]+/g, '-')
      .replace(/[^\w\u4e00-\u9fa5-]/g, '')
      .substring(0, 80)
  }
})

// ==================== 数据加载 ====================
onMounted(async () => {
  await store.fetchCategories()
  if (slug.value) {
    const doc = await store.fetchDocument(slug.value)
    if (doc) {
      form.value = {
        title: doc.title,
        slug: doc.slug,
        category_id: doc.category_id,
        language: doc.language,
        status: doc.status,
        content: doc.content,
      }
      editor.value?.commands.setContent(doc.content)
    }
  }
})

onBeforeUnmount(() => {
  editor.value?.destroy()
})

// ==================== 工具栏操作 ====================
function insertImage() {
  const url = window.prompt('输入图片 URL')
  if (url) {
    editor.value?.chain().focus().setImage({ src: url }).run()
  }
}

function insertLink() {
  const url = window.prompt('输入链接 URL')
  if (url) {
    editor.value?.chain().focus().setLink({ href: url }).run()
  }
}

function insertTable() {
  editor.value?.chain().focus().insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()
}

// ==================== 图片上传（拖拽/粘贴） ====================
async function handleDrop(event: DragEvent) {
  const files = event.dataTransfer?.files
  if (!files?.length) return
  const images = Array.from(files).filter(f => f.type.startsWith('image/'))
  if (!images.length) return
  event.preventDefault()
  for (const file of images) {
    const url = await uploadImage(file)
    editor.value?.chain().focus().setImage({ src: url }).run()
  }
}

async function handlePaste(event: ClipboardEvent) {
  const items = event.clipboardData?.items
  if (!items) return
  for (const item of Array.from(items)) {
    if (item.type.startsWith('image/')) {
      event.preventDefault()
      const file = item.getAsFile()
      if (!file) continue
      const url = await uploadImage(file)
      editor.value?.chain().focus().setImage({ src: url }).run()
    }
  }
}

async function uploadImage(file: File): Promise<string> {
  if (slug.value) {
    try {
      const attachment = await store.uploadAttachment(slug.value, file)
      return attachment.url
    } catch {
      // fallback
    }
  }
  ElMessage.info('图片上传功能将在对接 S3 后启用')
  return URL.createObjectURL(file)
}

// ==================== 保存 ====================
async function saveDraft() {
  if (!form.value.title) {
    ElMessage.warning('请输入文档标题')
    return
  }
  try {
    if (isEdit.value && slug.value) {
      await store.updateDocument(slug.value, {
        title: form.value.title,
        slug: form.value.slug,
        content: form.value.content,
        category_id: form.value.category_id,
        language: form.value.language,
        status: 'draft',
      })
    } else {
      const doc = await store.createDocument({
        title: form.value.title,
        slug: form.value.slug,
        content: form.value.content,
        category_id: form.value.category_id!,
        language: form.value.language,
        status: 'draft',
      })
      router.replace(`/documents/editor/${doc.slug}`)
    }
    form.value.status = 'draft'
    ElMessage.success('草稿已保存')
  } catch {
    // error handled by interceptor
  }
}

async function saveAndPublish() {
  if (!form.value.title) {
    ElMessage.warning('请输入文档标题')
    return
  }
  if (!form.value.category_id) {
    ElMessage.warning('请选择文档分类')
    return
  }
  try {
    if (isEdit.value && slug.value) {
      await store.updateDocument(slug.value, {
        title: form.value.title,
        slug: form.value.slug,
        content: form.value.content,
        category_id: form.value.category_id,
        language: form.value.language,
        status: 'published',
      })
    } else {
      const doc = await store.createDocument({
        title: form.value.title,
        slug: form.value.slug,
        content: form.value.content,
        category_id: form.value.category_id,
        language: form.value.language,
        status: 'published',
      })
      router.replace(`/documents/editor/${doc.slug}`)
    }
    form.value.status = 'published'
    ElMessage.success('文档已发布')
  } catch {
    // error handled by interceptor
  }
}

// ==================== 版本发布 ====================
function openVersionDialog() {
  if (!isEdit.value) {
    ElMessage.warning('请先保存文档后再发布版本')
    return
  }
  versionDialogVisible.value = true
}

async function publishVersion() {
  if (!versionForm.value.version) {
    ElMessage.warning('请输入版本号')
    return
  }
  if (!slug.value) return
  try {
    await store.createVersion(slug.value, {
      version: versionForm.value.version,
      changelog: versionForm.value.changelog,
    })
    ElMessage.success(`版本 ${versionForm.value.version} 已发布`)
    versionDialogVisible.value = false
    versionForm.value = { version: '', changelog: '' }
  } catch {
    // error handled by interceptor
  }
}

function goBack() {
  router.push('/documents/admin')
}
</script>

<template>
  <div class="editor-page">
    <!-- 顶部工具栏 -->
    <header class="editor-header">
      <div class="header-left">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          <span>返回列表</span>
        </el-button>
        <div class="header-divider" />
        <div class="header-title">
          <el-icon><Document /></el-icon>
          <span>{{ isEdit ? '编辑文档' : '新建文档' }}</span>
        </div>
        <el-tag v-if="form.status === 'published'" type="success" size="small">已发布</el-tag>
        <el-tag v-else-if="form.status === 'draft'" type="info" size="small">草稿</el-tag>
        <el-tag v-else type="warning" size="small">已归档</el-tag>
      </div>
      <div class="header-actions">
        <el-button @click="saveDraft" :loading="store.saving">
          <el-icon><EditPen /></el-icon>
          <span>保存草稿</span>
        </el-button>
        <el-button type="primary" @click="saveAndPublish" :loading="store.saving">
          <el-icon><Check /></el-icon>
          <span>保存并发布</span>
        </el-button>
        <el-button type="warning" @click="openVersionDialog">
          <el-icon><Promotion /></el-icon>
          <span>发布版本</span>
        </el-button>
      </div>
    </header>

    <!-- 主体内容 -->
    <div class="editor-body">
      <div class="editor-main" v-loading="store.loadingDoc">
        <!-- 元数据表单 -->
        <div class="meta-form">
          <el-form :model="form" label-position="top" inline>
            <el-form-item label="标题" class="meta-title">
              <el-input v-model="form.title" placeholder="输入文档标题" size="large" />
            </el-form-item>
            <el-form-item label="Slug">
              <el-input v-model="form.slug" placeholder="url-friendly-slug" />
            </el-form-item>
            <el-form-item label="分类">
              <el-select v-model="form.category_id" placeholder="选择分类">
                <el-option
                  v-for="opt in categoryOptions"
                  :key="opt.value"
                  :label="opt.label"
                  :value="opt.value"
                />
              </el-select>
            </el-form-item>
            <el-form-item label="语言">
              <el-select v-model="form.language" placeholder="选择语言">
                <el-option
                  v-for="opt in languageOptions"
                  :key="opt.value"
                  :label="opt.label"
                  :value="opt.value"
                />
              </el-select>
            </el-form-item>
          </el-form>
        </div>

        <!-- Tiptap 工具栏 -->
        <div class="tiptap-toolbar" v-if="editor">
          <div class="toolbar-group">
            <button :class="{ active: editor.isActive('bold') }" @click="editor.chain().focus().toggleBold().run()" title="粗体">
              <strong>B</strong>
            </button>
            <button :class="{ active: editor.isActive('italic') }" @click="editor.chain().focus().toggleItalic().run()" title="斜体">
              <em>I</em>
            </button>
            <button :class="{ active: editor.isActive('underline') }" @click="editor.chain().focus().toggleUnderline().run()" title="下划线">
              <u>U</u>
            </button>
            <button :class="{ active: editor.isActive('strike') }" @click="editor.chain().focus().toggleStrike().run()" title="删除线">
              <s>S</s>
            </button>
            <button :class="{ active: editor.isActive('highlight') }" @click="editor.chain().focus().toggleHighlight().run()" title="高亮">
              <span class="icon-highlight">H</span>
            </button>
          </div>

          <div class="toolbar-divider" />

          <div class="toolbar-group">
            <button :class="{ active: editor.isActive('heading', { level: 1 }) }" @click="editor.chain().focus().toggleHeading({ level: 1 }).run()">
              H1
            </button>
            <button :class="{ active: editor.isActive('heading', { level: 2 }) }" @click="editor.chain().focus().toggleHeading({ level: 2 }).run()">
              H2
            </button>
            <button :class="{ active: editor.isActive('heading', { level: 3 }) }" @click="editor.chain().focus().toggleHeading({ level: 3 }).run()">
              H3
            </button>
          </div>

          <div class="toolbar-divider" />

          <div class="toolbar-group">
            <button :class="{ active: editor.isActive('bulletList') }" @click="editor.chain().focus().toggleBulletList().run()" title="无序列表">
              ≡
            </button>
            <button :class="{ active: editor.isActive('orderedList') }" @click="editor.chain().focus().toggleOrderedList().run()" title="有序列表">
              1.
            </button>
            <button :class="{ active: editor.isActive('taskList') }" @click="editor.chain().focus().toggleTaskList().run()" title="任务列表">
              ☑
            </button>
            <button :class="{ active: editor.isActive('blockquote') }" @click="editor.chain().focus().toggleBlockquote().run()" title="引用">
              "
            </button>
          </div>

          <div class="toolbar-divider" />

          <div class="toolbar-group">
            <button :class="{ active: editor.isActive('codeBlock') }" @click="editor.chain().focus().toggleCodeBlock().run()" title="代码块">
              &lt;/&gt;
            </button>
            <button @click="editor.chain().focus().setHorizontalRule().run()" title="分割线">
              ―
            </button>
            <button @click="insertLink" :class="{ active: editor.isActive('link') }" title="链接">
              🔗
            </button>
            <button @click="insertImage" title="图片">
              🖼
            </button>
            <button @click="insertTable" title="表格">
              ⊞
            </button>
          </div>

          <div class="toolbar-divider" />

          <div class="toolbar-group">
            <button @click="editor.chain().focus().undo().run()" :disabled="!editor.can().undo()" title="撤销">
              ↩
            </button>
            <button @click="editor.chain().focus().redo().run()" :disabled="!editor.can().redo()" title="重做">
              ↪
            </button>
          </div>
        </div>

        <!-- Tiptap 编辑区 -->
        <div class="editor-container" @drop="handleDrop" @paste="handlePaste">
          <EditorContent :editor="editor" class="tiptap-editor" />
        </div>
      </div>
    </div>

    <!-- 版本发布弹窗 -->
    <el-dialog
      v-model="versionDialogVisible"
      title="发布新版本"
      width="500px"
      destroy-on-close
    >
      <el-form :model="versionForm" label-position="top">
        <el-form-item label="版本号" required>
          <el-input v-model="versionForm.version" placeholder="例如: v2.5.0" />
        </el-form-item>
        <el-form-item label="变更日志">
          <el-input
            v-model="versionForm.changelog"
            type="textarea"
            :rows="6"
            placeholder="描述本次版本的主要变更..."
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="versionDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="publishVersion">
          <el-icon><Promotion /></el-icon>
          发布版本
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.editor-page {
  height: calc(100vh - 48px);
  display: flex;
  flex-direction: column;
  margin: -24px -28px;
  background: var(--el-bg-color-page);
}

/* Header */
.editor-header {
  height: 56px;
  background: #ffffff;
  border-bottom: 1px solid var(--el-border-color-light);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-divider {
  width: 1px;
  height: 24px;
  background: var(--el-border-color);
}

.header-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Body */
.editor-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

.editor-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Meta Form */
.meta-form {
  padding: 16px 20px;
  background: #ffffff;
  border-bottom: 1px solid var(--el-border-color-light);
  flex-shrink: 0;
}

.meta-form .el-form {
  display: flex;
  align-items: flex-end;
  gap: 16px;
  flex-wrap: wrap;
}

.meta-form .el-form-item {
  margin-bottom: 0;
}

.meta-form .meta-title {
  flex: 1;
  min-width: 300px;
}

.meta-form .meta-title .el-input {
  font-size: 16px;
}

/* Tiptap Toolbar */
.tiptap-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 16px;
  background: #ffffff;
  border-bottom: 1px solid var(--el-border-color-light);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.toolbar-group {
  display: flex;
  align-items: center;
  gap: 2px;
}

.toolbar-divider {
  width: 1px;
  height: 24px;
  background: var(--el-border-color-lighter);
  margin: 0 6px;
}

.tiptap-toolbar button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: #475569;
  transition: all 0.15s;
}

.tiptap-toolbar button:hover {
  background: #f1f5f9;
  color: #1e293b;
}

.tiptap-toolbar button.active {
  background: #e0e7ff;
  color: #4f46e5;
}

.tiptap-toolbar button:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.icon-highlight {
  background: #fef08a;
  padding: 0 4px;
  border-radius: 2px;
  font-weight: 600;
  font-size: 12px;
}

/* Editor */
.editor-container {
  flex: 1;
  overflow-y: auto;
  background: #ffffff;
}

.tiptap-editor {
  height: 100%;
}

.tiptap-editor :deep(.tiptap) {
  padding: 32px 48px;
  min-height: 100%;
  outline: none;
  font-size: 15px;
  line-height: 1.8;
  color: #1e293b;
}

.tiptap-editor :deep(.tiptap p.is-editor-empty:first-child::before) {
  content: attr(data-placeholder);
  float: left;
  color: #94a3b8;
  pointer-events: none;
  height: 0;
}

.tiptap-editor :deep(.tiptap h1) {
  font-size: 2em;
  font-weight: 700;
  margin: 1em 0 0.5em;
  color: #0f172a;
}

.tiptap-editor :deep(.tiptap h2) {
  font-size: 1.5em;
  font-weight: 600;
  margin: 0.8em 0 0.4em;
  padding-bottom: 0.3em;
  border-bottom: 1px solid #e2e8f0;
  color: #1e293b;
}

.tiptap-editor :deep(.tiptap h3) {
  font-size: 1.25em;
  font-weight: 600;
  margin: 0.6em 0 0.3em;
  color: #334155;
}

.tiptap-editor :deep(.tiptap p) {
  margin: 0.5em 0;
}

.tiptap-editor :deep(.tiptap ul),
.tiptap-editor :deep(.tiptap ol) {
  padding-left: 1.5em;
  margin: 0.5em 0;
}

.tiptap-editor :deep(.tiptap li) {
  margin: 0.25em 0;
}

.tiptap-editor :deep(.tiptap blockquote) {
  border-left: 4px solid #6366f1;
  padding: 0.5em 1em;
  margin: 1em 0;
  background: #f8fafc;
  color: #475569;
}

.tiptap-editor :deep(.tiptap pre) {
  background: #1e293b;
  color: #e2e8f0;
  padding: 16px 20px;
  border-radius: 8px;
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 13px;
  line-height: 1.6;
  overflow-x: auto;
  margin: 1em 0;
}

.tiptap-editor :deep(.tiptap pre code) {
  background: none;
  color: inherit;
  padding: 0;
  font-size: inherit;
}

.tiptap-editor :deep(.tiptap code) {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', monospace;
  font-size: 0.9em;
  color: #e11d48;
}

.tiptap-editor :deep(.tiptap a) {
  color: #4f46e5;
  text-decoration: underline;
  cursor: pointer;
}

.tiptap-editor :deep(.tiptap img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 1em 0;
}

.tiptap-editor :deep(.tiptap hr) {
  border: none;
  border-top: 1px solid #e2e8f0;
  margin: 1.5em 0;
}

.tiptap-editor :deep(.tiptap mark) {
  background: #fef08a;
  padding: 0 2px;
  border-radius: 2px;
}

/* Task List */
.tiptap-editor :deep(.tiptap ul[data-type="taskList"]) {
  list-style: none;
  padding-left: 0;
}

.tiptap-editor :deep(.tiptap ul[data-type="taskList"] li) {
  display: flex;
  align-items: flex-start;
  gap: 8px;
}

.tiptap-editor :deep(.tiptap ul[data-type="taskList"] li label) {
  margin-top: 3px;
}

/* Table */
.tiptap-editor :deep(.tiptap table) {
  border-collapse: collapse;
  width: 100%;
  margin: 1em 0;
}

.tiptap-editor :deep(.tiptap table td),
.tiptap-editor :deep(.tiptap table th) {
  border: 1px solid #e2e8f0;
  padding: 8px 12px;
  text-align: left;
}

.tiptap-editor :deep(.tiptap table th) {
  background: #f8fafc;
  font-weight: 600;
}

.tiptap-editor :deep(.tiptap table .selectedCell) {
  background: #e0e7ff;
}
</style>
