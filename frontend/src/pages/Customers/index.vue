<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { Search, Plus, Edit, Delete, Refresh, Connection } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import dayjs from 'dayjs'
import { listCustomers, createCustomer, updateCustomer, deleteCustomer, listCustomerHosts, assignHost, unassignHost } from '@/api/customer'
import client from '@/api/client'
import type { CustomerItem, CustomerCreateReq, CustomerUpdateReq, CustomerHostItem, Envelope, HostListResp, ManagedHost } from '@/api/types'

const loading = ref(false)
const customers = ref<CustomerItem[]>([])
const total = ref(0)

const filter = reactive({
  search: '',
  status: '',
  page: 1,
  page_size: 20,
})

function formatTime(ts: number | null | undefined): string {
  if (!ts || ts <= 0) return '-'
  return dayjs.unix(ts).format('YYYY-MM-DD HH:mm:ss')
}

async function fetchCustomers() {
  loading.value = true
  try {
    const data = await listCustomers({
      page: filter.page,
      page_size: filter.page_size,
      status: filter.status || undefined,
      search: filter.search || undefined,
    })
    customers.value = data.items ?? []
    total.value = data.total ?? 0
  } catch {
    customers.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

function onSearch() {
  filter.page = 1
  fetchCustomers()
}

function handlePageChange(page: number) {
  filter.page = page
  fetchCustomers()
}

function handleSizeChange(size: number) {
  filter.page_size = size
  filter.page = 1
  fetchCustomers()
}

// ---- 新建/编辑对话框 ----
const dialogVisible = ref(false)
const dialogTitle = ref('新建客户')
const editingId = ref<string | null>(null)
const formSubmitting = ref(false)
const formRef = ref()

const defaultForm = (): CustomerCreateReq => ({
  name: '',
  email: '',
  phone: '',
  company: '',
  description: '',
  tags: [],
})

const form = reactive<CustomerCreateReq>(defaultForm())

const formRules = {
  name: [{ required: true, message: '请输入客户名称', trigger: 'blur' }],
}

function openAddDialog() {
  Object.assign(form, defaultForm())
  editingId.value = null
  dialogTitle.value = '新建客户'
  dialogVisible.value = true
}

function openEditDialog(row: CustomerItem) {
  editingId.value = row.customer_id
  dialogTitle.value = '编辑客户'
  Object.assign(form, {
    name: row.name,
    email: row.email || '',
    phone: row.phone || '',
    company: row.company || '',
    description: row.description || '',
    tags: [...(row.tags ?? [])],
  })
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
    if (editingId.value) {
      const payload: CustomerUpdateReq = {
        name: form.name,
        email: form.email,
        phone: form.phone,
        company: form.company,
        description: form.description,
        tags: form.tags,
      }
      await updateCustomer(editingId.value, payload)
      ElMessage.success('客户已更新')
    } else {
      await createCustomer(form)
      ElMessage.success('客户已创建')
    }
    dialogVisible.value = false
    fetchCustomers()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '操作失败')
  } finally {
    formSubmitting.value = false
  }
}

async function handleDelete(row: CustomerItem) {
  try {
    await ElMessageBox.confirm(`确定删除客户「${row.name}」？此操作不可恢复。`, '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning',
    })
  } catch {
    return
  }
  try {
    await deleteCustomer(row.customer_id)
    ElMessage.success('客户已删除')
    fetchCustomers()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '删除失败')
  }
}

// ---- 主机分配对话框 ----
const hostDialogVisible = ref(false)
const hostDialogCustomer = ref<CustomerItem | null>(null)
const assignedHosts = ref<CustomerHostItem[]>([])
const allHosts = ref<ManagedHost[]>([])
const hostLoading = ref(false)
const assignNote = ref('')
const selectedHostId = ref('')

async function handleAssignHost(row: CustomerItem) {
  hostDialogCustomer.value = row
  hostDialogVisible.value = true
  hostLoading.value = true
  try {
    const [assigned, hostsResp] = await Promise.all([
      listCustomerHosts(row.customer_id),
      client.get<Envelope<HostListResp>>('/api/v1/hosts', { params: { page: 1, page_size: 1000 } }),
    ])
    assignedHosts.value = assigned?.items ?? []
    allHosts.value = hostsResp.data.data?.items ?? []
  } catch {
    assignedHosts.value = []
    allHosts.value = []
  } finally {
    hostLoading.value = false
  }
}

// 返回未分配给当前客户的主机（含已被其他客户占用的，用于 disabled 展示）
function unassignedToCurrentHosts(): ManagedHost[] {
  const assignedIds = new Set(assignedHosts.value.map((h) => h.host_id))
  return allHosts.value.filter((h) => !assignedIds.has(h.host_id))
}

// 判断主机是否已被其他客户占用
function isHostTaken(h: ManagedHost): boolean {
  return !!h.customer_id && h.customer_id !== hostDialogCustomer.value?.customer_id
}

async function doAssignHost() {
  if (!selectedHostId.value || !hostDialogCustomer.value) return
  try {
    await assignHost(hostDialogCustomer.value.customer_id, {
      host_id: selectedHostId.value,
      note: assignNote.value || undefined,
    })
    ElMessage.success('主机已分配')
    selectedHostId.value = ''
    assignNote.value = ''
    await refreshHostDialog()
    fetchCustomers()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '分配失败')
  }
}

async function doUnassignHost(hostId: string) {
  if (!hostDialogCustomer.value) return
  try {
    await ElMessageBox.confirm('确定回收该主机？', '回收确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await unassignHost(hostDialogCustomer.value.customer_id, hostId)
    ElMessage.success('主机已回收')
    await refreshHostDialog()
    fetchCustomers()
  } catch (e: any) {
    ElMessage.error(e?.response?.data?.message || '回收失败')
  }
}

async function refreshHostDialog() {
  if (!hostDialogCustomer.value) return
  hostLoading.value = true
  try {
    const [assigned, hostsResp] = await Promise.all([
      listCustomerHosts(hostDialogCustomer.value.customer_id),
      client.get<Envelope<HostListResp>>('/api/v1/hosts', { params: { page: 1, page_size: 1000 } }),
    ])
    assignedHosts.value = assigned?.items ?? []
    allHosts.value = hostsResp.data.data?.items ?? []
  } catch { /* ignore */ } finally {
    hostLoading.value = false
  }
}

onMounted(() => {
  fetchCustomers()
})
</script>

<template>
  <div>
    <h2 class="page-title">客户管理</h2>

    <!-- Toolbar -->
    <el-row :gutter="12" style="margin-bottom: 16px" align="middle">
      <el-col :span="7">
        <el-input
          v-model="filter.search"
          placeholder="搜索客户名称、公司、手机号"
          clearable
          :prefix-icon="Search"
          @clear="onSearch"
          @keyup.enter="onSearch"
        />
      </el-col>
      <el-col :span="4">
        <el-select v-model="filter.status" placeholder="状态筛选" clearable @change="onSearch">
          <el-option label="活跃" value="active" />
          <el-option label="停用" value="inactive" />
        </el-select>
      </el-col>
      <el-col :span="3">
        <el-button :icon="Refresh" @click="fetchCustomers" :loading="loading">刷新</el-button>
      </el-col>
      <el-col :span="3">
        <el-button type="primary" :icon="Plus" @click="openAddDialog">新建客户</el-button>
      </el-col>
    </el-row>

    <!-- Table -->
    <el-table :data="customers" v-loading="loading" row-key="customer_id" border stripe style="width: 100%">
      <el-table-column prop="name" label="客户名称" width="140" show-overflow-tooltip />
      <el-table-column prop="company" label="公司" width="160" show-overflow-tooltip>
        <template #default="{ row }">{{ row.company || '-' }}</template>
      </el-table-column>
      <el-table-column prop="email" label="邮箱" width="180" show-overflow-tooltip>
        <template #default="{ row }">{{ row.email || '-' }}</template>
      </el-table-column>
      <el-table-column prop="phone" label="电话" width="140">
        <template #default="{ row }">{{ row.phone || '-' }}</template>
      </el-table-column>
      <el-table-column prop="host_count" label="已分配主机" width="110" align="center" />
      <el-table-column label="状态" width="90" align="center">
        <template #default="{ row }">
          <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small" effect="plain">
            {{ row.status === 'active' ? '活跃' : '停用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="创建时间" width="170">
        <template #default="{ row }">{{ formatTime(row.created_at) }}</template>
      </el-table-column>
      <el-table-column label="操作" min-width="200" align="center" fixed="right">
        <template #default="{ row }">
          <el-button link type="primary" :icon="Edit" @click="openEditDialog(row)">编辑</el-button>
          <el-button link type="primary" :icon="Connection" @click="handleAssignHost(row)">分配主机</el-button>
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

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="560px" destroy-on-close>
      <el-form ref="formRef" :model="form" :rules="formRules" label-width="80px">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="客户名称" />
        </el-form-item>
        <el-form-item label="公司">
          <el-input v-model="form.company" placeholder="公司名称（可选）" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="form.email" placeholder="邮箱地址（可选）" />
        </el-form-item>
        <el-form-item label="电话">
          <el-input v-model="form.phone" placeholder="联系电话（可选）" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.description" type="textarea" :rows="3" placeholder="备注信息（可选）" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="formSubmitting" @click="submitForm">确定</el-button>
      </template>
    </el-dialog>

    <!-- 主机分配对话框 -->
    <el-dialog
      v-model="hostDialogVisible"
      :title="`主机分配 — ${hostDialogCustomer?.name ?? ''}`"
      width="860px"
      destroy-on-close
    >
      <div v-loading="hostLoading">
        <!-- 已分配主机 -->
        <h4 style="margin: 0 0 12px">已分配主机 ({{ assignedHosts.length }})</h4>
        <el-table :data="assignedHosts" border size="small" style="margin-bottom: 20px" v-if="assignedHosts.length > 0">
          <el-table-column prop="host_name" label="主机名" min-width="120" show-overflow-tooltip />
          <el-table-column prop="ip" label="IP" width="140" />
          <el-table-column prop="hostname" label="Hostname" width="150" show-overflow-tooltip>
            <template #default="{ row }">{{ row.hostname || '-' }}</template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="90" align="center">
            <template #default="{ row }">
              <el-tag :type="row.status === 'online' ? 'success' : 'info'" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="分配时间" width="170">
            <template #default="{ row }">{{ formatTime(row.assigned_at) }}</template>
          </el-table-column>
          <el-table-column prop="note" label="备注" min-width="100" show-overflow-tooltip>
            <template #default="{ row }">{{ row.note || '-' }}</template>
          </el-table-column>
          <el-table-column label="操作" width="80" align="center">
            <template #default="{ row }">
              <el-button link type="danger" size="small" @click="doUnassignHost(row.host_id)">回收</el-button>
            </template>
          </el-table-column>
        </el-table>
        <el-empty v-else description="暂无已分配主机" :image-size="48" style="margin-bottom: 16px" />

        <!-- 分配新主机 -->
        <el-divider content-position="left">分配新主机</el-divider>
        <el-row :gutter="12" align="middle">
          <el-col :span="10">
            <el-select v-model="selectedHostId" placeholder="选择可用主机" filterable clearable style="width: 100%">
              <el-option
                v-for="h in unassignedToCurrentHosts()"
                :key="h.host_id"
                :label="`${h.name} (${h.ip})`"
                :value="h.host_id"
                :disabled="isHostTaken(h)"
              >
                <span>{{ h.name }} ({{ h.ip }})</span>
                <span v-if="isHostTaken(h)" style="float: right; color: #f56c6c; font-size: 12px">已分配</span>
              </el-option>
            </el-select>
          </el-col>
          <el-col :span="10">
            <el-input v-model="assignNote" placeholder="分配备注（可选）" />
          </el-col>
          <el-col :span="4">
            <el-button type="primary" :disabled="!selectedHostId" @click="doAssignHost">分配</el-button>
          </el-col>
        </el-row>
      </div>
      <template #footer>
        <el-button @click="hostDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>
