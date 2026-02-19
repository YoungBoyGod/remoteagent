<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Refresh, Search, Plus, Edit, Delete, CopyDocument, User, Link, Lock, Key, Connection as ConnectionIcon, EditPen } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { listHosts, createHost, updateHost, deleteHost } from '@/api/host'
import { listCustomers } from '@/api/customer'
import type { ManagedHost, HostCreateReq, HostUpdateReq, CustomerItem } from '@/api/types'
import { Platform } from '@element-plus/icons-vue'

dayjs.extend(relativeTime)

const loading = ref(false)
const hosts = ref<ManagedHost[]>([])
const total = ref(0)
const expandedRows = ref<string[]>([])

const filter = reactive({
  search: '',
  status: '',
  page: 1,
  page_size: 20,
})

const onlineCount = computed(() => hosts.value.filter((h) => h.status === 'online').length)
const offlineCount = computed(() => hosts.value.filter((h) => h.status === 'offline').length)
const unknownCount = computed(() => hosts.value.filter((h) => h.status === 'unknown').length)
const agentCount = computed(() => hosts.value.filter((h) => h.source === 'agent').length)
const manualCount = computed(() => hosts.value.filter((h) => h.source === 'manual').length)

function formatTime(ts: number | null | undefined): string {
  if (!ts || ts <= 0) return '-'
  return dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
}

function relativeTimeStr(ts: number | null | undefined): string {
  if (!ts || ts <= 0) return ''
  return dayjs.unix(ts).fromNow()
}

function copyText(text: string) {
  navigator.clipboard.writeText(text).then(
    () => ElMessage.success('已复制到剪贴板'),
    () => ElMessage.error('复制失败'),
  )
}

function sshCmd(row: ManagedHost): string {
  const port = row.port || 22
  const user = row.username || 'root'
  return port === 22 ? `ssh ${user}@${row.ip}` : `ssh -p ${port} ${user}@${row.ip}`
}

async function fetchHosts() {
  loading.value = true
  try {
    const data = await listHosts({
      page: filter.page,
      page_size: filter.page_size,
      status: filter.status || undefined,
      search: filter.search || undefined,
    })
    hosts.value = data.items ?? []
    total.value = data.total ?? 0
  } catch {
    hosts.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onSearch() {
  filter.page = 1
  fetchHosts()
}

function handlePageChange(page: number) {
  filter.page = page
  fetchHosts()
}

function handleSizeChange(size: number) {
  filter.page_size = size
  filter.page = 1
  fetchHosts()
}

function handleExpandChange(_row: ManagedHost, expanded: ManagedHost[]) {
  expandedRows.value = expanded.map((r) => r.host_id)
}

// ---- 添加/编辑对话框 ----
const dialogVisible = ref(false)
const dialogTitle = ref('添加主机')
const editingHostId = ref<string | null>(null)
const editingSource = ref<string>('manual')
const formSubmitting = ref(false)

const defaultForm = (): HostCreateReq => ({
  name: '',
  ip: '',
  hostname: '',
  port: 22,
  username: 'root',
  auth_type: 'password',
  password: '',
  ssh_key: '',
  vnc_addr: '',
  jupyter_addr: '',
  ext_ssh_addr: '',
  ext_vnc_addr: '',
  ext_jupyter_addr: '',
  assigned_to: '',
  description: '',
  tags: [],
})

const form = reactive<HostCreateReq>(defaultForm())
const tagInput = ref('')

const formRules = {
  name: [{ required: true, message: '请输入主机名称', trigger: 'blur' }],
  ip: [{ required: true, message: '请输入 IP 地址', trigger: 'blur' }],
  auth_type: [{ required: true, message: '请选择认证方式', trigger: 'change' }],
}

const formRef = ref()

function openAddDialog() {
  Object.assign(form, defaultForm())
  tagInput.value = ''
  editingHostId.value = null
  dialogTitle.value = '添加主机'
  dialogVisible.value = true
}

function openEditDialog(row: ManagedHost) {
  editingHostId.value = row.host_id
  editingSource.value = row.source ?? 'manual'
  dialogTitle.value = row.source === 'agent' ? '补充主机信息' : '编辑主机'
  Object.assign(form, {
    name: row.name,
    ip: row.ip,
    hostname: row.hostname,
    port: row.port || 22,
    username: row.username || 'root',
    auth_type: row.auth_type || 'password',
    password: '',
    ssh_key: '',
    vnc_addr: row.vnc_addr || '',
    jupyter_addr: row.jupyter_addr || '',
    ext_ssh_addr: row.ext_ssh_addr || '',
    ext_vnc_addr: row.ext_vnc_addr || '',
    ext_jupyter_addr: row.ext_jupyter_addr || '',
    assigned_to: row.assigned_to ?? '',
    description: row.description ?? '',
    tags: [...(row.tags ?? [])],
  })
  tagInput.value = (row.tags ?? []).join(', ')
  dialogVisible.value = true
}

async function submitForm() {
  try {
    await formRef.value?.validate()
  } catch {
    return
  }

  formSubmitting.value = true
  try {
    const tags = tagInput.value
      .split(/[,，]/)
      .map((t: string) => t.trim())
      .filter(Boolean)

    if (editingHostId.value) {
      const payload: HostUpdateReq = {
        name: form.name,
        ip: form.ip,
        hostname: form.hostname,
        port: form.port,
        username: form.username,
        auth_type: form.auth_type,
        vnc_addr: form.vnc_addr,
        jupyter_addr: form.jupyter_addr,
        ext_ssh_addr: form.ext_ssh_addr,
        ext_vnc_addr: form.ext_vnc_addr,
        ext_jupyter_addr: form.ext_jupyter_addr,
        assigned_to: form.assigned_to,
        description: form.description,
        tags,
      }
      if (form.auth_type === 'password' && form.password) {
        payload.password = form.password
      }
      if (form.auth_type === 'key' && form.ssh_key) {
        payload.ssh_key = form.ssh_key
      }
      await updateHost(editingHostId.value, payload)
      ElMessage.success('主机已更新')
    } else {
      const payload: HostCreateReq = { ...form, tags }
      await createHost(payload)
      ElMessage.success('主机已添加')
    }
    dialogVisible.value = false
    fetchHosts()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  } finally {
    formSubmitting.value = false
  }
}

async function handleDelete(row: ManagedHost) {
  try {
    await ElMessageBox.confirm(`确定删除主机「${row.name}」？此操作不可恢复。`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteHost(row.host_id)
    ElMessage.success('主机已删除')
    fetchHosts()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}

// ---- 关联客户对话框 ----
const assignDialogVisible = ref(false)
const assigningHostId = ref<string | null>(null)
const assigningHostName = ref('')
const selectedCustomerId = ref('')
const customers = ref<CustomerItem[]>([])
const customersLoading = ref(false)

async function fetchCustomers() {
  customersLoading.value = true
  try {
    const data = await listCustomers({ page: 1, page_size: 100, status: 'active' })
    customers.value = data.items ?? []
  } catch {
    customers.value = []
  } finally {
    customersLoading.value = false
  }
}

function handleAssignUser(row: ManagedHost) {
  assigningHostId.value = row.host_id
  assigningHostName.value = row.name
  selectedCustomerId.value = row.customer_id || ''
  assignDialogVisible.value = true
  fetchCustomers()
}

async function submitAssignCustomer() {
  if (!selectedCustomerId.value) {
    ElMessage.warning('请选择客户')
    return
  }
  const customer = customers.value.find(c => c.customer_id === selectedCustomerId.value)
  if (!customer) return
  try {
    await updateHost(assigningHostId.value!, { assigned_to: customer.name })
    ElMessage.success('已关联客户')
    assignDialogVisible.value = false
    fetchHosts()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '关联失败')
  }
}

async function handleUnbindUser(row: ManagedHost) {
  try {
    await ElMessageBox.confirm(`确定解绑主机「${row.name}」的关联用户「${row.assigned_to}」？`, '解绑确认', {
      confirmButtonText: '解绑',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await updateHost(row.host_id, { assigned_to: '' })
    ElMessage.success('已解绑用户')
    fetchHosts()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '解绑失败')
  }
}

onMounted(() => {
  fetchHosts()
})
</script>

<template>
  <div>
    <h2 class="page-title">
      <el-icon size="28"><Platform /></el-icon>
      主机管理
    </h2>

    <!-- 统计概览 -->
    <el-row :gutter="16" class="stats-row">
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-mini">
          <div class="stat-mini-value">{{ total }}</div>
          <div class="stat-mini-label">总数</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-mini online">
          <div class="stat-mini-value" style="color: #52c41a">{{ onlineCount }}</div>
          <div class="stat-mini-label">
            <span class="status-dot online" />在线
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-mini offline">
          <div class="stat-mini-value" style="color: #8c8c8c">{{ offlineCount }}</div>
          <div class="stat-mini-label">
            <span class="status-dot offline" />离线
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-mini">
          <div class="stat-mini-value" style="color: #faad14">{{ unknownCount }}</div>
          <div class="stat-mini-label">
            <span class="status-dot unknown" />未知
          </div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-mini">
          <div class="stat-mini-value" style="color: #4096ff">{{ agentCount }}</div>
          <div class="stat-mini-label">Agent 自动</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="8" :md="4">
        <div class="stat-mini">
          <div class="stat-mini-value" style="color: #722ed1">{{ manualCount }}</div>
          <div class="stat-mini-label">手动添加</div>
        </div>
      </el-col>
    </el-row>

    <!-- 工具栏 -->
    <div class="toolbar">
      <el-input
        v-model="filter.search"
        placeholder="搜索名称 / IP / 主机名"
        clearable
        :prefix-icon="Search"
        @clear="onSearch"
        @keyup.enter="onSearch"
        style="width: 260px"
      />
      <el-select v-model="filter.status" placeholder="状态筛选" clearable @change="onSearch" style="width: 140px">
        <el-option label="在线" value="online" />
        <el-option label="离线" value="offline" />
        <el-option label="未知" value="unknown" />
      </el-select>
      <div class="toolbar-spacer" />
      <el-button :icon="Refresh" @click="fetchHosts" :loading="loading">刷新</el-button>
      <el-button type="primary" :icon="Plus" @click="openAddDialog">添加主机</el-button>
    </div>

    <!-- 表格 -->
    <el-table
      :data="hosts"
      v-loading="loading"
      row-key="host_id"
      :expand-row-keys="expandedRows"
      @expand-change="handleExpandChange"
      border
      stripe
      style="width: 100%"
    >
      <el-table-column type="expand">
        <template #default="{ row }">
          <div class="expand-content">
            <!-- 基本信息 -->
            <el-descriptions title="基本信息" :column="3" border size="small" style="margin-bottom: 16px">
              <el-descriptions-item label="名称">{{ row.name || '-' }}</el-descriptions-item>
              <el-descriptions-item label="主机名">{{ row.hostname || '-' }}</el-descriptions-item>
              <el-descriptions-item label="IP 地址">{{ row.ip || '-' }}</el-descriptions-item>
              <el-descriptions-item label="端口">{{ row.port || 22 }}</el-descriptions-item>
              <el-descriptions-item label="用户名">{{ row.username || '-' }}</el-descriptions-item>
              <el-descriptions-item label="认证方式">
                <span class="auth-badge" :class="row.auth_type">
                  <el-icon size="12" v-if="row.auth_type === 'password'"><Lock /></el-icon>
                  <el-icon size="12" v-else><Key /></el-icon>
                  {{ row.auth_type === 'key' ? '密钥' : '密码' }}
                </span>
              </el-descriptions-item>
              <el-descriptions-item label="来源">
                <span class="source-badge" :class="row.source">
                  <el-icon size="12" v-if="row.source === 'agent'"><ConnectionIcon /></el-icon>
                  <el-icon size="12" v-else><EditPen /></el-icon>
                  {{ row.source === 'agent' ? 'Agent' : '手动' }}
                </span>
              </el-descriptions-item>
              <el-descriptions-item label="描述" :span="2">{{ row.description || '-' }}</el-descriptions-item>
              <el-descriptions-item label="标签" :span="3">
                <el-tag
                  v-for="tag in (row.tags || [])"
                  :key="tag"
                  size="small"
                  type="info"
                  effect="plain"
                  style="margin-right: 6px"
                >{{ tag }}</el-tag>
                <span v-if="!row.tags || row.tags.length === 0">-</span>
              </el-descriptions-item>
              <el-descriptions-item label="创建时间">{{ formatTime(row.created_at) }}</el-descriptions-item>
              <el-descriptions-item label="更新时间">{{ formatTime(row.updated_at) }}</el-descriptions-item>
            </el-descriptions>

            <!-- 连接方式 -->
            <el-descriptions title="内网连接" :column="2" border size="small" style="margin-bottom: 16px">
              <el-descriptions-item label="SSH">
                <div class="conn-row">
                  <code class="conn-value">{{ sshCmd(row) }}</code>
                  <el-button :icon="CopyDocument" link size="small" @click="copyText(sshCmd(row))" />
                </div>
              </el-descriptions-item>
              <el-descriptions-item label="VNC 地址">
                <div v-if="row.vnc_addr" class="conn-row">
                  <code class="conn-value">{{ row.vnc_addr }}</code>
                  <el-button :icon="CopyDocument" link size="small" @click="copyText(row.vnc_addr)" />
                </div>
                <span v-else>-</span>
              </el-descriptions-item>
              <el-descriptions-item label="Jupyter 地址">
                <div v-if="row.jupyter_addr" class="conn-row">
                  <code class="conn-value">{{ row.jupyter_addr }}</code>
                  <el-button :icon="CopyDocument" link size="small" @click="copyText(row.jupyter_addr)" />
                </div>
                <span v-else>-</span>
              </el-descriptions-item>
            </el-descriptions>

            <el-descriptions title="外网连接" :column="2" border size="small" style="margin-bottom: 16px">
              <el-descriptions-item label="SSH 访问地址">
                <div v-if="row.ext_ssh_addr" class="conn-row">
                  <code class="conn-value">{{ row.ext_ssh_addr }}</code>
                  <el-button :icon="CopyDocument" link size="small" @click="copyText(row.ext_ssh_addr)" />
                </div>
                <span v-else>-</span>
              </el-descriptions-item>
              <el-descriptions-item label="VNC 访问地址">
                <div v-if="row.ext_vnc_addr" class="conn-row">
                  <code class="conn-value">{{ row.ext_vnc_addr }}</code>
                  <el-button :icon="CopyDocument" link size="small" @click="copyText(row.ext_vnc_addr)" />
                </div>
                <span v-else>-</span>
              </el-descriptions-item>
              <el-descriptions-item label="Jupyter 访问地址">
                <div v-if="row.ext_jupyter_addr" class="conn-row">
                  <code class="conn-value">{{ row.ext_jupyter_addr }}</code>
                  <el-button :icon="CopyDocument" link size="small" @click="copyText(row.ext_jupyter_addr)" />
                </div>
                <span v-else>-</span>
              </el-descriptions-item>
            </el-descriptions>

            <!-- Agent 上报信息 -->
            <el-descriptions v-if="row.agent_id" title="Agent 上报信息" :column="3" border size="small">
              <el-descriptions-item label="Agent 状态">{{ row.agent_status || '-' }}</el-descriptions-item>
              <el-descriptions-item label="Agent 主机名">{{ row.agent_hostname || '-' }}</el-descriptions-item>
              <el-descriptions-item label="外网 IP">{{ row.external_ip || '-' }}</el-descriptions-item>
              <el-descriptions-item label="OS">{{ row.agent_os || '-' }}</el-descriptions-item>
              <el-descriptions-item label="架构">{{ row.agent_arch || '-' }}</el-descriptions-item>
              <el-descriptions-item label="Agent 版本">{{ row.agent_version || '-' }}</el-descriptions-item>
              <el-descriptions-item label="最后心跳" :span="3">
                {{ formatTime(row.last_heartbeat_at) }}
                <span v-if="row.last_heartbeat_at" style="color: var(--el-text-color-secondary); margin-left: 8px">
                  ({{ relativeTimeStr(row.last_heartbeat_at) }})
                </span>
              </el-descriptions-item>
            </el-descriptions>
          </div>
        </template>
      </el-table-column>

      <el-table-column label="名称 / 主机名" min-width="140" show-overflow-tooltip>
        <template #default="{ row }">
          <div class="host-info">
            <div class="host-name">{{ row.name || '-' }}</div>
            <div class="host-sub">{{ row.hostname || '-' }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="ip" label="IP 地址" min-width="130" />
      <el-table-column prop="port" label="端口" width="70" align="center" />
      <el-table-column prop="username" label="用户名" min-width="90" />
      <el-table-column label="认证方式" width="80" align="center">
        <template #default="{ row }">
          <span class="auth-badge" :class="row.auth_type">
            <el-icon size="12" v-if="row.auth_type === 'password'"><Lock /></el-icon>
            <el-icon size="12" v-else><Key /></el-icon>
            {{ row.auth_type === 'key' ? '密钥' : '密码' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="来源" width="80" align="center">
        <template #default="{ row }">
          <span class="source-badge" :class="row.source">
            {{ row.source === 'agent' ? 'Agent' : '手动' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <div class="status-cell">
            <span
              class="status-dot"
              :class="row.status === 'online' ? 'online' : row.status === 'offline' ? 'offline' : 'unknown'"
            />
            <span>{{ row.status === 'online' ? '在线' : row.status === 'offline' ? '离线' : '未知' }}</span>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="关联 Agent" min-width="120">
        <template #default="{ row }">
          <template v-if="row.agent_id">
            <span class="agent-link" :class="row.agent_status">
              <el-icon size="12"><Connection /></el-icon>
              {{ row.agent_id.slice(0, 10) }}
            </span>
          </template>
          <span v-else style="color: var(--el-text-color-secondary)">-</span>
        </template>
      </el-table-column>
      <el-table-column label="关联用户" min-width="110">
        <template #default="{ row }">
          <span v-if="row.assigned_to" class="user-badge">
            <el-icon size="12"><User /></el-icon>
            {{ row.assigned_to }}
          </span>
          <span v-else class="text-muted">未关联</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="220" align="center" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :icon="Edit" @click="openEditDialog(row)">编辑</el-button>
          <el-button v-if="!row.assigned_to" link type="success" :icon="User" @click="handleAssignUser(row)">关联</el-button>
          <el-button v-else link type="warning" :icon="Link" @click="handleUnbindUser(row)">解绑</el-button>
          <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div class="pagination-wrapper">
      <el-pagination
        v-model:current-page="filter.page"
        v-model:page-size="filter.page_size"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next, jumper"
        @current-change="handlePageChange"
        @size-change="handleSizeChange"
      />
    </div>

    <!-- 添加/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="760px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="100px">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="名称" prop="name">
              <el-input v-model="form.name" placeholder="主机名称" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="IP 地址" prop="ip">
              <el-input v-model="form.ip" placeholder="例如 192.168.1.100" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="主机名">
              <el-input v-model="form.hostname" placeholder="可选" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="SSH 端口">
              <el-input v-model.number="form.port" placeholder="22" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="用户名">
              <el-input v-model="form.username" placeholder="root" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="认证方式" prop="auth_type">
              <el-radio-group v-model="form.auth_type">
                <el-radio value="password">密码</el-radio>
                <el-radio value="key">密钥</el-radio>
              </el-radio-group>
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item v-if="form.auth_type === 'password'" label="密码">
          <el-input v-model="form.password" type="password" show-password :placeholder="editingHostId ? '留空则不修改' : '请输入密码'" />
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'key'" label="SSH 密钥">
          <el-input v-model="form.ssh_key" type="textarea" :rows="4" :placeholder="editingHostId ? '留空则不修改' : '粘贴私钥内容'" />
        </el-form-item>
        <el-divider content-position="left">访问地址</el-divider>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="VNC 地址">
              <el-input v-model="form.vnc_addr" placeholder="例如 192.168.1.100:5900" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Jupyter 地址">
              <el-input v-model="form.jupyter_addr" placeholder="例如 http://192.168.1.100:8888" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-divider content-position="left">外网访问</el-divider>
        <el-form-item label="SSH 访问地址">
          <el-input v-model="form.ext_ssh_addr" placeholder="例如 ssh -p 2222 root@example.com" />
        </el-form-item>
        <el-row :gutter="20">
          <el-col :span="12">
            <el-form-item label="VNC 访问地址">
              <el-input v-model="form.ext_vnc_addr" placeholder="例如 example.com:5900" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="Jupyter 访问地址">
              <el-input v-model="form.ext_jupyter_addr" placeholder="例如 http://example.com:8888" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="关联用户">
          <el-input v-model="form.assigned_to" placeholder="关联的用户名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选描述" />
        </el-form-item>
        <el-form-item label="标签">
          <el-input v-model="tagInput" placeholder="多个标签用逗号分隔" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="formSubmitting" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>

    <!-- 关联客户对话框 -->
    <el-dialog v-model="assignDialogVisible" title="关联客户" width="480px" destroy-on-close>
      <p style="margin-bottom: 16px; color: var(--el-text-color-secondary)">
        为主机「<strong>{{ assigningHostName }}</strong>」选择关联客户
      </p>
      <el-select
        v-model="selectedCustomerId"
        placeholder="请选择客户"
        filterable
        :loading="customersLoading"
        style="width: 100%"
      >
        <el-option
          v-for="c in customers"
          :key="c.customer_id"
          :label="`${c.name}${c.company ? ' - ' + c.company : ''}`"
          :value="c.customer_id"
        />
      </el-select>
      <template #footer>
        <el-button @click="assignDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="submitAssignCustomer">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
.stats-row {
  margin-bottom: 24px;
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
}

.stat-mini-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
}

.expand-content {
  padding: 20px 28px;
  background: #fafafa;
  border-radius: 8px;
  margin: 8px;
}

.host-info {
  line-height: 1.4;
}

.host-name {
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.host-sub {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 2px;
}

.status-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
}

.conn-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.conn-value {
  background: #f1f5f9;
  padding: 4px 10px;
  border-radius: 6px;
  font-family: 'JetBrains Mono', 'Consolas', monospace;
  font-size: 12px;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
  padding: 16px 0;
}

/* 认证方式徽章 - 无边框 */
.auth-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.auth-badge.password {
  color: #52c41a;
  background: #f6ffed;
}

.auth-badge.key {
  color: #faad14;
  background: #fffbe6;
}

/* 来源徽章 - 无边框 */
.source-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.source-badge.agent {
  color: #4096ff;
  background: #eaf6ff;
}

.source-badge.manual {
  color: #8c8c8c;
  background: #f5f5f5;
}

/* Agent 链接 - 无边框 */
.agent-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 500;
  font-family: 'JetBrains Mono', monospace;
}

.agent-link.online {
  color: #52c41a;
}

.agent-link:not(.online) {
  color: #8c8c8c;
}

/* 用户徽章 - 无边框 */
.user-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  color: #52c41a;
  background: #f6ffed;
}

/* 次要文字 */
.text-muted {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
}
</style>
