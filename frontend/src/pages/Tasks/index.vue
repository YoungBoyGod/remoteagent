<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh, List, Loading, SuccessFilled, CircleClose } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import client from '../../api/client'
import type { Envelope, TaskDetail, TaskDetailListResp, DebugAgentItem } from '../../api/types'
import StatusTag from '../../components/StatusTag.vue'
import OutputViewer from '../../components/OutputViewer.vue'

const router = useRouter()

const loading = ref(false)
const tasks = ref<TaskDetail[]>([])
const total = ref(0)
const agents = ref<DebugAgentItem[]>([])
const expandedRows = ref<string[]>([])
const activeTab = ref('all')

// 各状态任务数量
const statusCounts = reactive({
  all: 0,
  pending: 0,
  running: 0,
  success: 0,
  failed: 0,
})

const filter = reactive({
  agent_id: '',
  exec_mode: '',
})

const pagination = reactive({
  page: 1,
  page_size: 20,
})

const execModeOptions = ['shared', 'exclusive']

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('YYYY-MM-DD HH:mm:ss')
}

async function fetchAgents() {
  try {
    const resp = await client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents')
    agents.value = resp.data.data ?? []
  } catch { /* ignore */ }
}

// 根据 tab 获取对应的 status 参数
function getStatusParam(tab: string): string | undefined {
  switch (tab) {
    case 'pending': return 'pending'
    case 'running': return 'leased,running'
    case 'success': return 'success'
    case 'failed': return 'failed,timeout,canceled'
    default: return undefined
  }
}

async function fetchTasks() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (filter.agent_id) params.agent_id = filter.agent_id
    if (filter.exec_mode) params.exec_mode = filter.exec_mode

    const statusParam = getStatusParam(activeTab.value)
    if (statusParam) params.status = statusParam

    const resp = await client.get<Envelope<TaskDetailListResp>>('/api/v1/tasks', { params })
    const data = resp.data.data
    tasks.value = data?.items ?? []
    total.value = data?.total ?? 0
  } catch {
    tasks.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 获取各状态任务数量
async function fetchStatusCounts() {
  try {
    const [allResp, pendingResp, runningResp, successResp, failedResp] = await Promise.all([
      client.get<Envelope<TaskDetailListResp>>('/api/v1/tasks', { params: { page: 1, page_size: 1 } }),
      client.get<Envelope<TaskDetailListResp>>('/api/v1/tasks', { params: { page: 1, page_size: 1, status: 'pending' } }),
      client.get<Envelope<TaskDetailListResp>>('/api/v1/tasks', { params: { page: 1, page_size: 1, status: 'leased,running' } }),
      client.get<Envelope<TaskDetailListResp>>('/api/v1/tasks', { params: { page: 1, page_size: 1, status: 'success' } }),
      client.get<Envelope<TaskDetailListResp>>('/api/v1/tasks', { params: { page: 1, page_size: 1, status: 'failed,timeout,canceled' } }),
    ])

    statusCounts.all = allResp.data.data?.total ?? 0
    statusCounts.pending = pendingResp.data.data?.total ?? 0
    statusCounts.running = runningResp.data.data?.total ?? 0
    statusCounts.success = successResp.data.data?.total ?? 0
    statusCounts.failed = failedResp.data.data?.total ?? 0
  } catch { /* ignore */ }
}

function onSearch() {
  pagination.page = 1
  fetchTasks()
}

function handleTabChange() {
  pagination.page = 1
  fetchTasks()
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchTasks()
}

function handleSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  fetchTasks()
}

function handleExpandChange(_row: TaskDetail, expanded: TaskDetail[]) {
  expandedRows.value = expanded.map((r) => r.task_id)
}

// 取消任务
async function cancelTask(taskId: string) {
  try {
    await ElMessageBox.confirm('确定取消该任务？', '确认', { type: 'warning' })
    await client.post(`/api/v1/tasks/${taskId}/cancel`, { reason: '用户手动取消' })
    ElMessage.success('任务已取消')
    fetchTasks()
  } catch { /* user cancelled or error */ }
}

// 调整优先级
const priorityDialogVisible = ref(false)
const editingTaskId = ref('')
const editingPriority = ref(50)

function openPriorityDialog(task: TaskDetail) {
  editingTaskId.value = task.task_id
  editingPriority.value = task.priority
  priorityDialogVisible.value = true
}

async function savePriority() {
  try {
    await client.patch(`/api/v1/tasks/${editingTaskId.value}/priority`, {
      priority: editingPriority.value,
    })
    ElMessage.success('优先级已更新')
    priorityDialogVisible.value = false
    fetchTasks()
  } catch { /* error handled by interceptor */ }
}

function priorityColor(p: number): string {
  if (p >= 80) return '#ff4d4f'
  if (p >= 60) return '#faad14'
  if (p >= 40) return '#4096ff'
  return '#8c8c8c'
}

onMounted(() => {
  fetchAgents()
  fetchStatusCounts()
  fetchTasks()
})
</script>

<template>
  <div>
    <h2 class="page-title">
      <el-icon size="28"><List /></el-icon>
      任务管理
    </h2>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-select v-model="filter.agent_id" placeholder="按 Agent 筛选" clearable @change="onSearch" style="width: 200px">
        <el-option v-for="a in agents" :key="a.agent_id" :label="a.agent_id.slice(0, 20)" :value="a.agent_id" />
      </el-select>
      <el-select v-model="filter.exec_mode" placeholder="执行模式" clearable @change="onSearch" style="width: 140px">
        <el-option v-for="m in execModeOptions" :key="m" :label="m" :value="m" />
      </el-select>
      <div class="toolbar-spacer" />
      <el-button :icon="Refresh" @click="() => { fetchStatusCounts(); fetchTasks(); }" :loading="loading">刷新</el-button>
    </div>

    <!-- 状态 Tabs -->
    <el-tabs v-model="activeTab" @tab-change="handleTabChange" class="status-tabs">
      <el-tab-pane name="all">
        <template #label>
          <span class="tab-label">
            <el-icon><List /></el-icon>
            全部
            <el-badge :value="statusCounts.all" :hidden="statusCounts.all === 0" class="tab-badge" />
          </span>
        </template>
      </el-tab-pane>
      <el-tab-pane name="pending">
        <template #label>
          <span class="tab-label">
            <el-icon><Loading /></el-icon>
            排队中
            <el-badge :value="statusCounts.pending" :hidden="statusCounts.pending === 0" type="warning" class="tab-badge" />
          </span>
        </template>
      </el-tab-pane>
      <el-tab-pane name="running">
        <template #label>
          <span class="tab-label">
            <el-icon class="is-loading"><Loading /></el-icon>
            运行中
            <el-badge :value="statusCounts.running" :hidden="statusCounts.running === 0" type="primary" class="tab-badge" />
          </span>
        </template>
      </el-tab-pane>
      <el-tab-pane name="success">
        <template #label>
          <span class="tab-label">
            <el-icon><SuccessFilled /></el-icon>
            已完成
            <el-badge :value="statusCounts.success" :hidden="statusCounts.success === 0" type="success" class="tab-badge" />
          </span>
        </template>
      </el-tab-pane>
      <el-tab-pane name="failed">
        <template #label>
          <span class="tab-label">
            <el-icon><CircleClose /></el-icon>
            失败
            <el-badge :value="statusCounts.failed" :hidden="statusCounts.failed === 0" type="danger" class="tab-badge" />
          </span>
        </template>
      </el-tab-pane>
    </el-tabs>

    <!-- 表格 -->
    <el-table
      :data="tasks"
      v-loading="loading"
      row-key="task_id"
      :expand-row-keys="expandedRows"
      @expand-change="handleExpandChange"
      border
      stripe
      style="width: 100%"
    >
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-content">
            <el-descriptions :column="2" border size="small">
              <el-descriptions-item label="Task ID">{{ row.task_id }}</el-descriptions-item>
              <el-descriptions-item label="幂等键">{{ row.idempotency_key || '-' }}</el-descriptions-item>
              <el-descriptions-item label="命令" :span="2">
                <code>{{ row.payload?.command || '-' }}</code>
              </el-descriptions-item>
              <el-descriptions-item label="参数">
                {{ row.payload?.args?.join(' ') || '-' }}
              </el-descriptions-item>
              <el-descriptions-item label="工作目录">{{ row.payload?.workdir || '-' }}</el-descriptions-item>
              <el-descriptions-item label="超时">{{ row.payload?.timeout || 30 }}s</el-descriptions-item>
              <el-descriptions-item label="尝试次数">{{ row.attempt }} / {{ row.max_attempts }}</el-descriptions-item>
              <el-descriptions-item label="抢占状态">{{ row.preempt_state || '-' }}</el-descriptions-item>
              <el-descriptions-item v-if="row.exit_code != null" label="退出码">
                <el-tag :type="row.exit_code === 0 ? 'success' : 'danger'" size="small">{{ row.exit_code }}</el-tag>
              </el-descriptions-item>
              <el-descriptions-item v-if="row.error_code" label="错误码">{{ row.error_code }}</el-descriptions-item>
              <el-descriptions-item v-if="row.error_message" label="错误信息" :span="2">{{ row.error_message }}</el-descriptions-item>
              <el-descriptions-item label="环境变量" :span="2">
                <template v-if="row.payload?.env && Object.keys(row.payload.env).length > 0">
                  <el-tag v-for="(v, k) in row.payload.env" :key="k" size="small" style="margin-right: 6px">
                    {{ k }}={{ v }}
                  </el-tag>
                </template>
                <span v-else>-</span>
              </el-descriptions-item>
            </el-descriptions>

            <!-- 执行输出 -->
            <template v-if="row.stdout || row.stderr">
              <el-divider content-position="left" style="margin: 20px 0 16px">
                <span style="font-size: 14px; font-weight: 600">执行输出</span>
              </el-divider>
              <div v-if="row.stdout" style="margin-bottom: 16px">
                <div style="font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 8px; font-weight: 500">stdout</div>
                <OutputViewer
                  :content="row.stdout"
                  label="stdout"
                  :truncated="row.truncated ?? false"
                  :filename="`${row.task_id}-stdout.txt`"
                />
              </div>
              <div v-if="row.stderr">
                <div style="font-size: 13px; color: var(--el-text-color-secondary); margin-bottom: 8px; font-weight: 500">stderr</div>
                <OutputViewer
                  :content="row.stderr"
                  label="stderr"
                  :truncated="row.truncated ?? false"
                  :filename="`${row.task_id}-stderr.txt`"
                />
              </div>
            </template>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="Task ID" min-width="160" show-overflow-tooltip>
        <template #default="{ row }">
          <el-link type="primary" underline="never" @click="router.push(`/tasks/${row.task_id}`)">
            {{ row.task_id.slice(0, 20) }}...
          </el-link>
        </template>
      </el-table-column>
      <el-table-column prop="task_type" label="类型" width="70" align="center">
        <template #default="{ row }">
          <span class="task-type" :class="row.task_type">
            {{ row.task_type }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100" align="center">
        <template #default="{ row }">
          <StatusTag :status="row.status" />
        </template>
      </el-table-column>
      <el-table-column label="模式" width="70" align="center">
        <template #default="{ row }">
          <span class="exec-mode" :class="row.exec_mode">
            {{ row.exec_mode === 'exclusive' ? '独占' : '共享' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="优先级" width="80" align="center">
        <template #default="{ row }">
          <span :style="{ color: priorityColor(row.priority), fontWeight: 600, fontSize: '14px' }">{{ row.priority }}</span>
        </template>
      </el-table-column>
      <el-table-column prop="agent_id" label="Agent" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">{{ row.agent_id?.slice(0, 15) || '-' }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="140" align="center" fixed="right">
        <template #default="{ row }">
          <el-button
            v-if="row.status === 'pending'"
            size="small"
            type="warning"
            text
            @click="cancelTask(row.task_id)"
          >取消</el-button>
          <el-button
            v-if="row.status === 'pending'"
            size="small"
            type="primary"
            text
            @click="openPriorityDialog(row)"
          >优先级</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="pagination.page"
        v-model:page-size="pagination.page_size"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>

    <!-- 优先级调整对话框 -->
    <el-dialog v-model="priorityDialogVisible" title="调整优先级" width="420px">
      <el-form label-width="80px">
        <el-form-item label="优先级">
          <el-slider v-model="editingPriority" :min="1" :max="100" show-input />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="priorityDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="savePriority">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.status-tabs :deep(.el-tabs__item) {
  padding: 0 20px;
}

.tab-label {
  display: flex;
  align-items: center;
  gap: 6px;
}

.tab-badge {
  margin-left: 4px;
}

.expand-content {
  padding: 20px 28px;
  background: #fafafa;
  border-radius: 8px;
  margin: 8px;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
  padding: 16px 0;
}

/* 任务类型 - 无边框 */
.task-type {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.task-type.shell {
  color: #4096ff;
  background: #eaf6ff;
}

.task-type.python {
  color: #faad14;
  background: #fffbe6;
}

/* 执行模式 - 无边框 */
.exec-mode {
  font-size: 12px;
  font-weight: 500;
  padding: 2px 8px;
  border-radius: 4px;
}

.exec-mode.exclusive {
  color: #ff4d4f;
  background: #fff2f0;
}

.exec-mode.shared {
  color: #8c8c8c;
  background: #f5f5f5;
}
</style>
