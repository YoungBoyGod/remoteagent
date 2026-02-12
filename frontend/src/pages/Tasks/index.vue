<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Refresh } from '@element-plus/icons-vue'
import dayjs from 'dayjs'
import client from '../../api/client'
import type { Envelope, TaskItem, TaskListResp, DebugAgentItem } from '../../api/types'
import OutputViewer from '../../components/OutputViewer.vue'

const loading = ref(false)
const tasks = ref<TaskItem[]>([])
const total = ref(0)
const agents = ref<DebugAgentItem[]>([])
const expandedRows = ref<string[]>([])

const filter = reactive({
  agent_id: '',
  status: '',
  task_id: '',
})

const pagination = reactive({
  page: 1,
  page_size: 20,
})

const statusOptions = ['pending', 'running', 'finished', 'failed']

const statusColorMap: Record<string, string> = {
  pending: '',       // primary (default blue)
  running: 'warning',
  finished: 'success',
  failed: 'danger',
}

function formatTime(val: number | null | undefined): string {
  if (val == null || val <= 0) return '-'
  return dayjs.unix(val).format('YYYY-MM-DD HH:mm:ss')
}

async function fetchAgents() {
  try {
    const resp = await client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents')
    agents.value = resp.data.data ?? []
  } catch {
    // ignore — agent list is optional for filtering
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
    if (filter.status) params.status = filter.status
    if (filter.task_id) params.task_id = filter.task_id

    const resp = await client.get<Envelope<TaskListResp>>('/api/v1/debug/tasks', { params })
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

function onSearch() {
  pagination.page = 1
  fetchTasks()
}

function onRefresh() {
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

function handleExpandChange(_row: TaskItem, expanded: TaskItem[]) {
  expandedRows.value = expanded.map((r) => r.task_id)
}

onMounted(() => {
  fetchAgents()
  fetchTasks()
})
</script>

<template>
  <div>
    <h2>Tasks</h2>

    <!-- Toolbar -->
    <el-row :gutter="12" style="margin-bottom: 16px" align="middle">
      <el-col :span="5">
        <el-select
          v-model="filter.agent_id"
          placeholder="Filter by Agent"
          clearable
          @change="onSearch"
        >
          <el-option
            v-for="a in agents"
            :key="a.agent_id"
            :label="a.agent_id"
            :value="a.agent_id"
          />
        </el-select>
      </el-col>
      <el-col :span="4">
        <el-select
          v-model="filter.status"
          placeholder="Filter by Status"
          clearable
          @change="onSearch"
        >
          <el-option
            v-for="s in statusOptions"
            :key="s"
            :label="s"
            :value="s"
          />
        </el-select>
      </el-col>
      <el-col :span="5">
        <el-input
          v-model="filter.task_id"
          placeholder="Search task_id"
          clearable
          @clear="onSearch"
          @keyup.enter="onSearch"
        />
      </el-col>
      <el-col :span="2">
        <el-button :icon="Refresh" @click="onRefresh" :loading="loading">
          Refresh
        </el-button>
      </el-col>
    </el-row>

    <!-- Table -->
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
          <div style="padding: 12px 24px">
            <p><strong>stdout:</strong></p>
            <OutputViewer :content="row.stdout" label="stdout" :truncated="row.truncated" :filename="`${row.task_id}-stdout.txt`" />
            <p style="margin-top: 12px"><strong>stderr:</strong></p>
            <OutputViewer :content="row.stderr" label="stderr" :truncated="row.truncated" :filename="`${row.task_id}-stderr.txt`" />
          </div>
        </template>
      </el-table-column>

      <el-table-column prop="task_id" label="Task ID" min-width="180" show-overflow-tooltip />
      <el-table-column prop="agent_id" label="Agent ID" min-width="160" show-overflow-tooltip />
      <el-table-column prop="status" label="Status" width="110" align="center">
        <template #default="{ row }">
          <el-tag :type="(statusColorMap[row.status] as any) || 'info'" size="small">
            {{ row.status }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column prop="exit_code" label="Exit Code" width="100" align="center" />
      <el-table-column label="Started At" width="180">
        <template #default="{ row }">{{ formatTime(row.started_at) }}</template>
      </el-table-column>
      <el-table-column label="Finished At" width="180">
        <template #default="{ row }">{{ formatTime(row.finished_at) }}</template>
      </el-table-column>
    </el-table>

    <!-- Pagination -->
    <div style="margin-top: 16px; display: flex; justify-content: flex-end">
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
  </div>
</template>
