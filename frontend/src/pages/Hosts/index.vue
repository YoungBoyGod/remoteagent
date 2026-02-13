<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { Refresh, Search, Plus, Edit, Delete, CopyDocument, User, Link } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { listHosts, createHost, updateHost, deleteHost } from '@/api/host'
import { listCustomers } from '@/api/customer'
import type { ManagedHost, HostCreateReq, HostUpdateReq, CustomerItem } from '@/api/types'

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
    () => ElMessage.success('已复制'),
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
    <h2 class="page-title">主机管理</h2>

    <!-- 统计概览 -->
    <el-row :gutter="12" style="margin-bottom: 16px">
      <el-col :span="4">
        <el-statistic title="总数" :value="total" />
      </el-col>
      <el-col :span="4">
        <el-statistic title="在线">
          <template #default>
            <span style="color: #22c55e; font-weight: 600; font-size: 24px">{{ onlineCount }}</span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="4">
        <el-statistic title="离线">
          <template #default>
            <span style="color: #64748b; font-weight: 600; font-size: 24px">{{ offlineCount }}</span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="4">
        <el-statistic title="未知">
          <template #default>
            <span style="color: #f59e0b; font-weight: 600; font-size: 24px">{{ unknownCount }}</span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="4">
        <el-statistic title="Agent 自动">
          <template #default>
            <span style="color: #3b82f6; font-weight: 600; font-size: 24px">{{ agentCount }}</span>
          </template>
        </el-statistic>
      </el-col>
      <el-col :span="4">
        <el-statistic title="手动添加">
          <template #default>
            <span style="color: #8b5cf6; font-weight: 600; font-size: 24px">{{ manualCount }}</span>
          </template>
        </el-statistic>
      </el-col>
    </el-row>

    <!-- Toolbar -->
    <el-row :gutter="12" style="margin-bottom: 16px" align="middle">
      <el-col :span="6">
        <el-input
          v-model="filter.search"
          placeholder="搜索名称 / IP / 主机名"
          clearable
          :prefix-icon="Search"
          @clear="onSearch"
          @keyup.enter="onSearch"
        />
      </el-col>
      <el-col :span="4">
        <el-select v-model="filter.status" placeholder="状态筛选" clearable @change="onSearch">
          <el-option label="在线" value="online" />
          <el-option label="离线" value="offline" />
          <el-option label="未知" value="unknown" />
        </el-select>
      </el-col>
      <el-col :span="3">
        <el-button :icon="Refresh" @click="fetchHosts" :loading="loading">刷新</el-button>
      </el-col>
      <el-col :span="3">
        <el-button type="primary" :icon="Plus" @click="openAddDialog">添加主机</el-button>
      </el-col>
    </el-row>

    <!-- Table -->
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
          <div style="padding: 16px 24px">
            <!-- 基本信息 -->
            <el-descriptions title="基本信息" :column="3" border size="small" style="margin-bottom: 16px">
              <el-descriptions-item label="名称">{{ row.name || '-' }}</el-descriptions-item>
              <el-descriptions-item label="主机名">{{ row.hostname || '-' }}</el-descriptions-item>
              <el-descriptions-item label="IP 地址">{{ row.ip || '-' }}</el-descriptions-item>
              <el-descriptions-item label="端口">{{ row.port || 22 }}</el-descriptions-item>
              <el-descriptions-item label="用户名">{{ row.username || '-' }}</el-descriptions-item>
              <el-descriptions-item label="认证方式">
                <el-tag size="small" :type="row.auth_type === 'password' ? 'success' : 'warning'" effect="plain">
                  {{ row.auth_type === 'key' ? '密钥' : '密码' }}
                </el-tag>
              </el-descriptions-item>
              <el-descriptions-item label="来源">
                <el-tag size="small" :type="row.source === 'agent' ? 'primary' : 'info'" effect="plain">
                  {{ row.source === 'agent' ? 'Agent 上报' : '手动添加' }}
                </el-tag>
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
              <el-descriptions-item label="密码">
                <div class="conn-row">
                  <code class="conn-value">{{ row.password || '-' }}</code>
                  <el-button v-if="row.password" :icon="CopyDocument" link size="small" @click="copyText(row.password)" />
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
              <el-descriptions-item label="密码">
                <div class="conn-row">
                  <code class="conn-value">{{ row.password || '-' }}</code>
                  <el-button v-if="row.password" :icon="CopyDocument" link size="small" @click="copyText(row.password)" />
                </div>
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

      <el-table-column label="名称 / 主机名" width="120" show-overflow-tooltip>
        <template #default="{ row }">
          <div style="line-height: 1.4">
            <div style="font-weight: 500">{{ row.name || '-' }}</div>
            <div style="font-size: 12px; color: var(--el-text-color-secondary)">{{ row.hostname || '-' }}</div>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="ip" label="IP 地址" width="140" />
      <el-table-column prop="port" label="端口" width="70" align="center" />
      <el-table-column prop="username" label="用户名" width="100" />
      <el-table-column label="认证方式" width="90" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="row.auth_type === 'password' ? 'success' : 'warning'" effect="plain">
            {{ row.auth_type === 'key' ? '密钥' : '密码' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="来源" width="100" align="center">
        <template #default="{ row }">
          <el-tag size="small" :type="row.source === 'agent' ? 'primary' : 'info'" effect="plain">
            {{ row.source === 'agent' ? 'Agent 上报' : '手动添加' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <span>
            <span
              class="status-dot"
              :class="row.status === 'online' ? 'online' : row.status === 'offline' ? 'offline' : 'unknown'"
            />
            {{ row.status === 'online' ? '在线' : row.status === 'offline' ? '离线' : '未知' }}
          </span>
        </template>
      </el-table-column>
      <el-table-column label="关联 Agent" width="120">
        <template #default="{ row }">
          <template v-if="row.agent_id">
            <el-tag size="small" :type="row.agent_status === 'online' ? 'success' : 'info'" effect="plain">
              {{ row.agent_id.slice(0, 8) }}
            </el-tag>
          </template>
          <span v-else style="color: var(--el-text-color-secondary)">-</span>
        </template>
      </el-table-column>
      <el-table-column label="关联用户" width="110">
        <template #default="{ row }">
          <template v-if="row.assigned_to">
            <el-tag size="small" type="success">{{ row.assigned_to }}</el-tag>
          </template>
          <span v-else style="color: var(--el-text-color-placeholder)">未关联</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="240" align="center" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :icon="Edit" @click="openEditDialog(row)">编辑</el-button>
          <el-button v-if="!row.assigned_to" link type="success" :icon="User" @click="handleAssignUser(row)">关联用户</el-button>
          <el-button v-else link type="warning" :icon="Link" @click="handleUnbindUser(row)">解绑用户</el-button>
          <el-button link type="danger" :icon="Delete" @click="handleDelete(row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <!-- 分页 -->
    <div style="display: flex; justify-content: flex-end; margin-top: 16px">
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
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="720px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="90px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="主机名称" />
        </el-form-item>
        <el-form-item label="IP 地址" prop="ip">
          <el-input v-model="form.ip" placeholder="例如 192.168.1.100" />
        </el-form-item>
        <el-form-item label="主机名">
          <el-input v-model="form.hostname" placeholder="可选" />
        </el-form-item>
        <el-row :gutter="16">
          <el-col :span="12">
            <el-form-item label="SSH 端口">
              <el-input v-model.number="form.port" placeholder="22" />
            </el-form-item>
          </el-col>
          <el-col :span="12">
            <el-form-item label="用户名">
              <el-input v-model="form.username" placeholder="root" />
            </el-form-item>
          </el-col>
        </el-row>
        <el-form-item label="认证方式" prop="auth_type">
          <el-radio-group v-model="form.auth_type">
            <el-radio value="password">密码</el-radio>
            <el-radio value="key">密钥</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'password'" label="密码">
          <el-input v-model="form.password" type="password" show-password :placeholder="editingHostId ? '留空则不修改' : '请输入密码'" />
        </el-form-item>
        <el-form-item v-if="form.auth_type === 'key'" label="SSH 密钥">
          <el-input v-model="form.ssh_key" type="textarea" :rows="4" :placeholder="editingHostId ? '留空则不修改' : '粘贴私钥内容'" />
        </el-form-item>
        <el-row :gutter="16">
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
        <el-row :gutter="16">
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
    <el-dialog v-model="assignDialogVisible" title="关联客户" width="420px" destroy-on-close>
      <p style="margin-bottom: 12px; color: var(--el-text-color-secondary)">
        为主机「{{ assigningHostName }}」选择关联客户
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
.status-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-right: 6px;
  vertical-align: middle;
}
.status-dot.online {
  background-color: #22c55e;
  box-shadow: 0 0 4px #22c55e80;
}
.status-dot.offline {
  background-color: #64748b;
}
.status-dot.unknown {
  background-color: #f59e0b;
  box-shadow: 0 0 4px #f59e0b80;
}
</style>

<style>
.conn-section {
  margin-bottom: 12px;
}
.conn-section:last-child {
  margin-bottom: 0;
}
.conn-title {
  font-weight: 600;
  font-size: 13px;
  margin-bottom: 6px;
  color: var(--el-text-color-primary);
}
.conn-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 12px;
}
.conn-label {
  color: var(--el-text-color-secondary);
  min-width: 32px;
  flex-shrink: 0;
}
.conn-value {
  background: var(--el-fill-color-light);
  padding: 2px 8px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
