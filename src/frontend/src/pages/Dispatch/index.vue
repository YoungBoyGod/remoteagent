<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { Search, Promotion } from '@element-plus/icons-vue'
import client from '../../api/client'
import type { DebugAgentItem, Envelope, TaskBatchCreateResp, TaskCreateReq, TaskCreateResp } from '../../api/types'

type SidebarTag = {
  key: string
  label: string
  count: number
  agentIds: string[]
}

const loading = ref(false)
const submitting = ref(false)
const agents = ref<DebugAgentItem[]>([])
const searchText = ref('')
const selectedTagKeys = ref<string[]>([])
const selectedAgentIds = ref<string[]>([])
const createResults = ref<TaskCreateResp[]>([])

const form = reactive({
  task_type: 'shell',
  command: '',
  submitter: '',
  timeout: 30,
  exec_mode: 'shared' as 'shared' | 'exclusive',
  priority: 50,
  preemptible: false,
  max_attempts: 3,
})

function resolveCurrentUser(): string {
  const directKeys = ['current_user', 'username', 'user_name', 'operator', 'created_by']
  for (const key of directKeys) {
    const v = localStorage.getItem(key)
    if (v && v.trim()) return v.trim()
  }

  const jsonKeys = ['user', 'currentUser', 'profile']
  for (const key of jsonKeys) {
    const raw = localStorage.getItem(key)
    if (!raw) continue
    try {
      const obj = JSON.parse(raw) as Record<string, unknown>
      const candidates = [obj.username, obj.name, obj.account, obj.operator]
      for (const c of candidates) {
        if (typeof c === 'string' && c.trim()) return c.trim()
      }
    } catch {
      // ignore invalid JSON and continue fallback chain
    }
  }

  return 'admin'
}

const onlineAgents = computed(() => agents.value.filter((a) => a.status === 'online'))

const tagGroups = computed<SidebarTag[]>(() => {
  const map = new Map<string, { label: string; ids: Set<string> }>()

  for (const a of onlineAgents.value) {
    for (const [k, v] of Object.entries(a.labels || {})) {
      const key = `label:${k}=${v}`
      if (!map.has(key)) map.set(key, { label: `${k}=${v}`, ids: new Set<string>() })
      map.get(key)!.ids.add(a.agent_id)
    }

    for (const t of a.host_tags || []) {
      const key = `host:${t}`
      if (!map.has(key)) map.set(key, { label: t, ids: new Set<string>() })
      map.get(key)!.ids.add(a.agent_id)
    }
  }

  return Array.from(map.entries())
    .map(([key, value]) => ({
      key,
      label: value.label,
      count: value.ids.size,
      agentIds: Array.from(value.ids),
    }))
    .sort((a, b) => b.count - a.count || a.label.localeCompare(b.label))
})

const filteredAgents = computed(() => {
  const kw = searchText.value.trim().toLowerCase()

  let filtered = onlineAgents.value
  if (kw) {
    filtered = filtered.filter((a) => {
      const text = [
        a.agent_id,
        a.device_code,
        a.hostname,
        a.ip,
        ...(a.host_tags || []),
        ...Object.entries(a.labels || {}).map(([k, v]) => `${k}=${v}`),
      ]
        .join(' ')
        .toLowerCase()
      return text.includes(kw)
    })
  }

  if (selectedTagKeys.value.length === 0) return filtered

  const selectedIdSet = new Set<string>()
  for (const key of selectedTagKeys.value) {
    const g = tagGroups.value.find((item) => item.key === key)
    if (!g) continue
    for (const id of g.agentIds) selectedIdSet.add(id)
  }

  return filtered.filter((a) => selectedIdSet.has(a.agent_id))
})

const visibleAgentIds = computed(() => filteredAgents.value.map((a) => a.agent_id))

const allVisibleSelected = computed(() => {
  const visible = visibleAgentIds.value
  if (visible.length === 0) return false
  return visible.every((id) => selectedAgentIds.value.includes(id))
})

const selectedCount = computed(() => selectedAgentIds.value.length)

function isSelected(agentID: string): boolean {
  return selectedAgentIds.value.includes(agentID)
}

function toggleAgent(agentID: string, checked: boolean) {
  if (checked) {
    if (!selectedAgentIds.value.includes(agentID)) {
      selectedAgentIds.value = [...selectedAgentIds.value, agentID]
    }
    return
  }
  selectedAgentIds.value = selectedAgentIds.value.filter((id) => id !== agentID)
}

function toggleSelectVisible(checked: boolean) {
  if (checked) {
    const merged = new Set([...selectedAgentIds.value, ...visibleAgentIds.value])
    selectedAgentIds.value = Array.from(merged)
    return
  }
  const visible = new Set(visibleAgentIds.value)
  selectedAgentIds.value = selectedAgentIds.value.filter((id) => !visible.has(id))
}

function clearSelection() {
  selectedAgentIds.value = []
}

function toggleTag(tagKey: string, checked: boolean) {
  if (checked) {
    if (!selectedTagKeys.value.includes(tagKey)) {
      selectedTagKeys.value = [...selectedTagKeys.value, tagKey]
    }
    return
  }
  selectedTagKeys.value = selectedTagKeys.value.filter((k) => k !== tagKey)
}

async function fetchAgents() {
  loading.value = true
  try {
    const resp = await client.get<Envelope<DebugAgentItem[]>>('/api/v1/debug/agents')
    agents.value = resp.data.data ?? []

    const onlineSet = new Set(onlineAgents.value.map((a) => a.agent_id))
    selectedAgentIds.value = selectedAgentIds.value.filter((id) => onlineSet.has(id))
  } catch {
    agents.value = []
  } finally {
    loading.value = false
  }
}

async function submitTasks() {
  if (!form.command.trim()) {
    ElMessage.warning('请输入命令')
    return
  }
  if (selectedAgentIds.value.length === 0) {
    ElMessage.warning('请选择至少一个目标 Agent')
    return
  }

  submitting.value = true
  createResults.value = []
  try {
    const baseTask: Omit<TaskCreateReq, 'schedule'> = {
      task_type: form.task_type,
      payload: {
        command: form.command,
        env: form.submitter.trim() ? { RA_SUBMITTER: form.submitter.trim() } : undefined,
        timeout: form.timeout,
      },
      exec_mode: form.exec_mode,
      priority: form.priority,
      preemptible: form.preemptible,
      max_attempts: form.max_attempts,
    }

    let created: TaskCreateResp[] = []

    if (selectedAgentIds.value.length === 1) {
      const resp = await client.post<Envelope<TaskCreateResp>>('/api/v1/tasks', {
        ...baseTask,
        schedule: { target_agent_id: selectedAgentIds.value[0] },
      })
      created = [resp.data.data]
    } else {
      const tasks: TaskCreateReq[] = selectedAgentIds.value.map((agentID) => ({
        ...baseTask,
        schedule: { target_agent_id: agentID },
      }))
      const resp = await client.post<Envelope<TaskBatchCreateResp>>('/api/v1/tasks/batch', { tasks })
      created = resp.data.data.tasks
    }

    createResults.value = created
    ElMessage.success(`已创建 ${created.length} 个任务`)
  } catch {
    // interceptor handles error
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  if (!form.submitter.trim()) {
    form.submitter = resolveCurrentUser()
  }
  fetchAgents()
})
</script>

<template>
  <div class="dispatch-page">
    <h2 class="page-title">任务提交</h2>

    <div class="dispatch-layout">
      <el-card shadow="hover" class="left-panel">
        <template #header>
          <div class="panel-title">任务参数</div>
        </template>

        <el-form label-position="top">
          <el-form-item label="任务类型">
            <el-select v-model="form.task_type" style="width: 100%">
              <el-option label="shell" value="shell" />
              <el-option label="python" value="python" />
            </el-select>
          </el-form-item>

          <el-form-item label="命令">
            <el-input
              v-model="form.command"
              type="textarea"
              :rows="10"
              placeholder="例如: uname -a && uptime"
            />
          </el-form-item>

          <div class="grid-2">
            <el-form-item label="提交人">
              <el-input v-model="form.submitter" placeholder="例如: luoyi" />
            </el-form-item>
          </div>

          <div class="grid-2">
            <el-form-item label="超时(秒)">
              <el-input-number v-model="form.timeout" :min="1" :max="3600" style="width: 100%" />
            </el-form-item>
            <el-form-item label="优先级">
              <el-input-number v-model="form.priority" :min="1" :max="100" style="width: 100%" />
            </el-form-item>
          </div>

          <div class="grid-2">
            <el-form-item label="执行模式">
              <el-select v-model="form.exec_mode" style="width: 100%">
                <el-option label="shared" value="shared" />
                <el-option label="exclusive" value="exclusive" />
              </el-select>
            </el-form-item>
            <el-form-item label="重试次数">
              <el-input-number v-model="form.max_attempts" :min="1" :max="10" style="width: 100%" />
            </el-form-item>
          </div>

          <el-form-item>
            <el-checkbox v-model="form.preemptible">允许被抢占</el-checkbox>
          </el-form-item>

          <div class="submit-row">
            <el-button
              type="primary"
              :icon="Promotion"
              :loading="submitting"
              @click="submitTasks"
            >
              提交任务（{{ selectedCount }} 台）
            </el-button>
            <el-button @click="clearSelection">清空选择</el-button>
          </div>
        </el-form>
      </el-card>

      <el-card shadow="hover" class="middle-panel" v-loading="loading">
        <template #header>
          <div class="middle-header">
            <div class="panel-title">目标 Agent</div>
            <div class="header-actions">
              <el-button text @click="fetchAgents">刷新</el-button>
            </div>
          </div>
          <div class="toolbar-row">
            <el-input
              v-model="searchText"
              clearable
              placeholder="搜索 agent_id / device_code / hostname / ip / 标签"
            >
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
            <el-checkbox
              :model-value="allVisibleSelected"
              :disabled="visibleAgentIds.length === 0"
              @change="(v: boolean) => toggleSelectVisible(v)"
            >
              全选当前筛选结果（{{ visibleAgentIds.length }}）
            </el-checkbox>
          </div>
        </template>

        <el-table :data="filteredAgents" stripe height="640">
          <el-table-column label="选择" width="74" align="center">
            <template #default="{ row }">
              <el-checkbox
                :model-value="isSelected(row.agent_id)"
                @change="(v: boolean) => toggleAgent(row.agent_id, v)"
              />
            </template>
          </el-table-column>
          <el-table-column prop="agent_id" label="Agent ID" min-width="220" show-overflow-tooltip />
          <el-table-column prop="device_code" label="Device Code" min-width="140" show-overflow-tooltip />
          <el-table-column prop="hostname" label="Hostname" min-width="140" show-overflow-tooltip />
          <el-table-column prop="ip" label="IP" min-width="130" show-overflow-tooltip />
          <el-table-column label="标签" min-width="260">
            <template #default="{ row }">
              <div class="tag-list">
                <el-tag v-for="(v, k) in row.labels" :key="`l-${k}-${v}`" size="small" effect="plain">{{ k }}={{ v }}</el-tag>
                <el-tag v-for="t in (row.host_tags || [])" :key="`h-${t}`" size="small" type="success" effect="plain">{{ t }}</el-tag>
                <span v-if="Object.keys(row.labels || {}).length === 0 && (row.host_tags || []).length === 0" class="muted">-</span>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div class="result-box" v-if="createResults.length > 0">
          <div class="panel-title">创建结果</div>
          <div class="result-item" v-for="r in createResults" :key="r.task_id">
            <el-link type="primary" @click="$router.push(`/tasks/${r.task_id}`)">{{ r.task_id }}</el-link>
            <el-tag size="small" effect="plain">{{ r.status }}</el-tag>
          </div>
        </div>
      </el-card>

      <el-card shadow="hover" class="right-panel">
        <template #header>
          <div class="panel-title">设备标签筛选（OR）</div>
        </template>

        <div class="sidebar-stats">
          <span>在线 Agent: {{ onlineAgents.length }}</span>
          <span>已选标签: {{ selectedTagKeys.length }}</span>
        </div>

        <el-scrollbar height="650">
          <div class="tag-sidebar">
            <el-check-tag
              v-for="g in tagGroups"
              :key="g.key"
              :checked="selectedTagKeys.includes(g.key)"
              @change="(checked: boolean) => toggleTag(g.key, checked)"
            >
              {{ g.label }} ({{ g.count }})
            </el-check-tag>

            <div v-if="tagGroups.length === 0" class="muted">暂无可用标签</div>
          </div>
        </el-scrollbar>
      </el-card>
    </div>
  </div>
</template>

<style scoped>
.dispatch-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.page-title {
  margin: 0;
  font-size: 24px;
  font-weight: 700;
}

.dispatch-layout {
  display: grid;
  grid-template-columns: 340px 1fr 280px;
  gap: 16px;
  align-items: start;
}

.panel-title {
  font-weight: 600;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.submit-row {
  display: flex;
  gap: 8px;
}

.middle-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.toolbar-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 12px;
}

.tag-list {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.result-box {
  margin-top: 12px;
  border-top: 1px solid var(--el-border-color-light);
  padding-top: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.result-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 8px;
}

.sidebar-stats {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 8px;
}

.tag-sidebar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-content: flex-start;
}

.muted {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}

@media (max-width: 1280px) {
  .dispatch-layout {
    grid-template-columns: 1fr;
  }
}
</style>
