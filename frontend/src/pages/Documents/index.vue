<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import {
  Search,
  ArrowLeft,
  ArrowRight,
  Download,
  Clock,
  User,
  Check,
  Edit,
  Share,
  Collection,
  Setting,
  Lock,
  CircleCheck,
  CircleClose,
  View,
  Document as DocumentIcon,
  Service,
  List as ListIcon,
  Reading,
  Upload,
  MoreFilled,
  FolderOpened,
  ArrowRight as ArrowRightIcon,
  HomeFilled,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import type { DocumentCategory, DocumentVersion } from './types'

// ==================== 状态管理 ====================
const searchQuery = ref('')
const searchVisible = ref(false)
const currentVersion = ref('v2.4.1')
const feedbackVisible = ref(false)
const diffVisible = ref(false)
const activeDocId = ref('quickstart')
const activeTab = ref('list')
const currentCategory = ref('all')
const tocItems = ref<{ id: string; text: string; level: number }[]>([])
const activeTocId = ref('')
const expandedCategories = ref<string[]>(['product', 'version', 'technical', 'ops'])

// ==================== 文档数据 ====================
const versions: DocumentVersion[] = [
  { version: 'v2.4.1', date: '2024-01-15', isCurrent: true, isStable: true },
  { version: 'v2.4.0', date: '2024-01-01', isCurrent: false },
  { version: 'v2.3.5', date: '2023-12-15', isCurrent: false },
]

const categories: DocumentCategory[] = [
  {
    id: 'product',
    name: '产品文档',
    icon: 'Reading',
    color: '#4096ff',
    items: [
      { id: 'whitepaper', title: '产品白皮书', level: 0 },
      { id: 'architecture', title: '架构设计', level: 0 },
      { id: 'features', title: '功能清单', level: 0 },
      { id: 'quickstart', title: '快速开始', level: 0, isActive: true },
    ],
  },
  {
    id: 'version',
    name: '版本文档',
    icon: 'Collection',
    color: '#52c41a',
    badge: 'v2.4.1',
    items: [
      { id: 'release-notes', title: '发布说明', level: 0 },
      { id: 'upgrade-guide', title: '升级指南', level: 0 },
      { id: 'compatibility', title: '兼容性说明', level: 0 },
      { id: 'changelog', title: '变更日志', level: 0, badge: '47 commits' },
    ],
  },
  {
    id: 'technical',
    name: '技术文档',
    icon: 'Setting',
    color: '#722ed1',
    items: [
      { id: 'api-reference', title: 'API 参考', level: 0 },
      { id: 'sdk', title: 'SDK 集成', level: 0 },
      { id: 'webhook', title: 'Webhook 配置', level: 0 },
      { id: 'error-codes', title: '错误代码', level: 0 },
    ],
  },
  {
    id: 'ops',
    name: '运维手册',
    icon: 'Clock',
    color: '#fa8c16',
    items: [
      { id: 'deployment', title: '部署指南', level: 0 },
      { id: 'monitoring', title: '监控配置', level: 0 },
      { id: 'backup', title: '备份恢复', level: 0 },
      { id: 'troubleshooting', title: '故障排查', level: 0 },
    ],
  },
]

// 模拟文档列表数据
const documentList = ref([
  { id: 1, title: '产品白皮书', category: 'product', categoryName: '产品文档', version: 'v2.4.1', updatedAt: '2024-01-15', author: '张三', views: 1234 },
  { id: 2, title: '快速开始指南', category: 'product', categoryName: '产品文档', version: 'v2.4.1', updatedAt: '2024-01-15', author: '李四', views: 2345 },
  { id: 3, title: 'API 参考文档', category: 'technical', categoryName: '技术文档', version: 'v2.4.1', updatedAt: '2024-01-14', author: '王五', views: 3456 },
  { id: 4, title: '部署指南', category: 'ops', categoryName: '运维手册', version: 'v2.4.0', updatedAt: '2024-01-01', author: '赵六', views: 890 },
  { id: 5, title: '升级指南', category: 'version', categoryName: '版本文档', version: 'v2.4.1', updatedAt: '2024-01-15', author: '张三', views: 567 },
  { id: 6, title: '架构设计', category: 'product', categoryName: '产品文档', version: 'v2.4.0', updatedAt: '2024-01-10', author: '李四', views: 1567 },
  { id: 7, title: 'SDK 集成指南', category: 'technical', categoryName: '技术文档', version: 'v2.4.1', updatedAt: '2024-01-12', author: '王五', views: 2341 },
  { id: 8, title: '监控配置手册', category: 'ops', categoryName: '运维手册', version: 'v2.4.0', updatedAt: '2024-01-05', author: '赵六', views: 789 },
])

// ==================== 计算属性 ====================
const filteredDocuments = computed(() => {
  let result = documentList.value
  if (currentCategory.value !== 'all') {
    result = result.filter(doc => doc.category === currentCategory.value)
  }
  if (searchQuery.value) {
    const query = searchQuery.value.toLowerCase()
    result = result.filter(doc => doc.title.toLowerCase().includes(query))
  }
  return result
})

// ==================== 方法 ====================
function toggleCategory(catId: string) {
  const idx = expandedCategories.value.indexOf(catId)
  if (idx > -1) {
    expandedCategories.value.splice(idx, 1)
  } else {
    expandedCategories.value.push(catId)
  }
}

function isCategoryExpanded(catId: string) {
  return expandedCategories.value.includes(catId)
}

function selectDoc(docId: string) {
  activeDocId.value = docId
  activeTab.value = 'detail'
  nextTick(() => {
    generateTOC()
  })
}

function generateTOC() {
  tocItems.value = [
    { id: 'overview', text: '概述', level: 0 },
    { id: 'requirements', text: '系统要求', level: 0 },
    { id: 'installation', text: '安装步骤', level: 0 },
    { id: 'download', text: '下载安装包', level: 1 },
    { id: 'dependencies', text: '安装依赖', level: 1 },
    { id: 'deploy', text: '部署服务', level: 1 },
    { id: 'verification', text: '验证安装', level: 0 },
    { id: 'troubleshooting', text: '故障排查', level: 0 },
  ]
}

function scrollToSection(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    activeTocId.value = id
  }
}

function goBack() {
  activeTab.value = 'list'
}

function showVersionDiff() {
  diffVisible.value = true
}

function downloadPDF() {
  ElMessage.success('PDF 导出成功')
}

function filterByCategory(categoryId: string) {
  currentCategory.value = categoryId
}

// 键盘快捷键
onMounted(() => {
  document.addEventListener('keydown', (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
      e.preventDefault()
      searchVisible.value = true
    }
  })
})
</script>

<template>
  <div>
    <!-- 文档列表页 -->
    <template v-if="activeTab === 'list'">
      <h2 class="page-title">
        <router-link to="/" class="back-home-link" title="返回主页">
          <el-icon size="28"><HomeFilled /></el-icon>
        </router-link>
        <span class="title-divider">|</span>
        <el-icon size="28"><Reading /></el-icon>
        文档中心
      </h2>

      <el-row :gutter="20">
        <!-- 左侧分类导航 -->
        <el-col :span="5">
          <el-card shadow="hover" class="category-nav-card">
            <template #header>
              <div class="nav-header">
                <el-icon><FolderOpened /></el-icon>
                <span>文档分类</span>
              </div>
            </template>
            
            <div class="category-list">
              <div 
                class="category-item" 
                :class="{ active: currentCategory === 'all' }"
                @click="filterByCategory('all')"
              >
                <el-icon><DocumentIcon /></el-icon>
                <span>全部文档</span>
                <el-tag size="small" type="info" effect="plain">{{ documentList.length }}</el-tag>
              </div>
              
              <div v-for="cat in categories" :key="cat.id" class="category-section">
                <div 
                  class="category-title" 
                  :class="{ expanded: isCategoryExpanded(cat.id) }"
                  @click="toggleCategory(cat.id)"
                >
                  <div class="category-title-left">
                    <el-icon :style="{ color: cat.color }">
                      <component :is="cat.icon" />
                    </el-icon>
                    <span>{{ cat.name }}</span>
                    <el-tag v-if="cat.badge" type="success" size="small" effect="dark">{{ cat.badge }}</el-tag>
                  </div>
                  <el-icon class="category-arrow"><ArrowRightIcon /></el-icon>
                </div>
                
                <div v-show="isCategoryExpanded(cat.id)" class="category-docs">
                  <div
                    v-for="item in cat.items"
                    :key="item.id"
                    class="doc-item"
                    :class="{ active: activeDocId === item.id }"
                    @click="selectDoc(item.id)"
                  >
                    <el-icon size="14"><DocumentIcon /></el-icon>
                    <span class="doc-item-text">{{ item.title }}</span>
                    <el-tag v-if="item.badge" type="primary" size="small" effect="plain">{{ item.badge }}</el-tag>
                  </div>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <!-- 右侧内容区 -->
        <el-col :span="19">
          <!-- 统计概览 -->
          <el-row :gutter="16" class="stats-row">
            <el-col :xs="12" :sm="6">
              <div class="stat-mini">
                <div class="stat-mini-value">{{ documentList.length }}</div>
                <div class="stat-mini-label">总文档数</div>
              </div>
            </el-col>
            <el-col :xs="12" :sm="6">
              <div class="stat-mini">
                <div class="stat-mini-value" style="color: #4096ff">{{ categories[0].items.length }}</div>
                <div class="stat-mini-label">产品文档</div>
              </div>
            </el-col>
            <el-col :xs="12" :sm="6">
              <div class="stat-mini">
                <div class="stat-mini-value" style="color: #52c41a">{{ categories[2].items.length }}</div>
                <div class="stat-mini-label">技术文档</div>
              </div>
            </el-col>
            <el-col :xs="12" :sm="6">
              <div class="stat-mini">
                <div class="stat-mini-value" style="color: #722ed1">24</div>
                <div class="stat-mini-label">本周更新</div>
              </div>
            </el-col>
          </el-row>

          <!-- 工具栏 -->
          <div class="toolbar">
            <el-input
              v-model="searchQuery"
              placeholder="搜索文档标题、内容..."
              clearable
              :prefix-icon="Search"
              style="width: 300px"
            />
            <el-select v-model="currentVersion" placeholder="版本筛选" style="width: 140px">
              <el-option v-for="v in versions" :key="v.version" :label="v.version" :value="v.version" />
            </el-select>
            <div class="toolbar-spacer" />
            <el-button :icon="Upload" type="primary">上传文档</el-button>
          </div>

          <!-- 文档表格 -->
          <el-table :data="filteredDocuments" stripe border style="width: 100%">
            <el-table-column prop="title" label="文档名称" min-width="250" show-overflow-tooltip>
              <template #default="{ row }">
                <el-link type="primary" :underline="false" @click="selectDoc(row.id)">
                  <el-icon style="margin-right: 8px; color: var(--el-color-primary)"><DocumentIcon /></el-icon>
                  {{ row.title }}
                </el-link>
              </template>
            </el-table-column>
            <el-table-column prop="categoryName" label="分类" width="120" align="center">
              <template #default="{ row }">
                <span class="category-badge">{{ row.categoryName }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="version" label="版本" width="90" align="center">
              <template #default="{ row }">
                <span class="version-badge">{{ row.version }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="author" label="作者" width="100" align="center" />
            <el-table-column prop="updatedAt" label="更新时间" width="140" align="center">
              <template #default="{ row }">
                <el-icon style="margin-right: 4px; color: var(--el-text-color-secondary); font-size: 12px"><Clock /></el-icon>
                {{ row.updatedAt }}
              </template>
            </el-table-column>
            <el-table-column prop="views" label="阅读量" width="100" align="center">
              <template #default="{ row }">
                <el-icon style="margin-right: 4px; color: var(--el-text-color-secondary); font-size: 12px"><View /></el-icon>
                {{ row.views }}
              </template>
            </el-table-column>
            <el-table-column label="操作" width="140" align="center" fixed="right">
              <template #default="{ row }">
                <el-button link type="primary" @click="selectDoc(row.id)">查看</el-button>
                <el-dropdown trigger="click">
                  <el-button link :icon="MoreFilled" />
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item :icon="Download">下载</el-dropdown-item>
                      <el-dropdown-item :icon="Share">分享</el-dropdown-item>
                      <el-dropdown-item :icon="Edit" divided>编辑</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
              </template>
            </el-table-column>
          </el-table>

          <!-- 分页 -->
          <div class="pagination-wrapper">
            <el-pagination
              :total="filteredDocuments.length"
              :page-sizes="[10, 20, 50]"
              layout="total, sizes, prev, pager, next, jumper"
            />
          </div>
        </el-col>
      </el-row>
    </template>

    <!-- 文档详情页 -->
    <template v-else>
      <!-- 返回按钮 -->
      <div style="margin-bottom: 16px">
        <el-button text @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回列表
        </el-button>
      </div>

      <el-row :gutter="20">
        <!-- 左侧文档导航 -->
        <el-col :span="5">
          <el-card shadow="hover" class="doc-nav-card">
            <template #header>
              <div class="nav-header">
                <el-icon><FolderOpened /></el-icon>
                <span>文档导航</span>
              </div>
            </template>
            
            <div class="doc-nav">
              <div v-for="cat in categories" :key="cat.id" class="nav-section">
                <div 
                  class="nav-section-title" 
                  :class="{ expanded: isCategoryExpanded(cat.id) }"
                  @click="toggleCategory(cat.id)"
                >
                  <div class="nav-section-left">
                    <el-icon :style="{ color: cat.color }">
                      <component :is="cat.icon" />
                    </el-icon>
                    <span>{{ cat.name }}</span>
                  </div>
                  <el-icon class="nav-arrow"><ArrowRightIcon /></el-icon>
                </div>
                
                <div v-show="isCategoryExpanded(cat.id)" class="nav-items">
                  <a
                    v-for="item in cat.items"
                    :key="item.id"
                    class="nav-item"
                    :class="{ active: activeDocId === item.id }"
                    @click="selectDoc(item.id)"
                  >
                    {{ item.title }}
                  </a>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>

        <!-- 中间内容区 -->
        <el-col :span="14">
          <el-card shadow="hover">
            <!-- 文档头部 -->
            <div class="doc-detail-header">
              <div class="doc-meta">
                <span class="doc-badge success">
                  <el-icon><Check /></el-icon>
                  最新版本
                </span>
                <span class="doc-meta-item">
                  <el-icon><Clock /></el-icon>
                  最后更新: 2024-01-15
                </span>
                <span class="doc-meta-item">
                  <el-icon><User /></el-icon>
                  张三
                </span>
                <div style="flex: 1" />
                <el-button size="small" @click="showVersionDiff">
                  <el-icon><Collection /></el-icon>
                  版本对比
                </el-button>
                <el-button size="small" @click="downloadPDF">
                  <el-icon><Download /></el-icon>
                  导出 PDF
                </el-button>
              </div>
              <h1 class="doc-title">快速开始</h1>
              <p class="doc-desc">在 30 分钟内完成产品部署，从安装到首个 API 调用。</p>
            </div>

            <el-divider />

            <!-- 文档正文 -->
            <div class="doc-body">
              <h2 id="overview">概述</h2>
              <p>本文档指导您完成产品的快速部署。适用于以下硬件版本：</p>
              <ul>
                <li><strong>HW-v3.2</strong> (推荐) - 完整功能支持</li>
                <li><strong>HW-v3.1</strong> - 完全兼容，建议固件 v3.1.5+</li>
                <li><strong>HW-v3.0</strong> - 有限兼容，AI 功能不可用</li>
              </ul>

              <el-alert
                title="硬件兼容性检查"
                type="warning"
                :closable="false"
                style="margin: 20px 0"
              >
                <p>部署前请确认您的硬件版本。不兼容的硬件将导致服务无法启动。</p>
                <el-button type="warning" size="small" style="margin-top: 8px">
                  检查我的硬件版本
                </el-button>
              </el-alert>

              <h2 id="requirements">系统要求</h2>
              <el-table :data="[
                { component: 'CPU', min: '8 核 x86_64', recommend: '16 核', optimize: '支持 AVX-512 加速' },
                { component: '内存', min: '16 GB', recommend: '32 GB', optimize: '支持大页内存' },
                { component: '存储', min: '100 GB SSD', recommend: '500 GB NVMe', optimize: '支持 I/O 隔离' },
                { component: '网络', min: '千兆以太网', recommend: '万兆以太网', optimize: '支持 RDMA' },
              ]" border size="small" style="margin: 16px 0">
                <el-table-column prop="component" label="组件" width="100" />
                <el-table-column prop="min" label="最低要求" />
                <el-table-column prop="recommend" label="推荐配置" />
                <el-table-column prop="optimize" label="HW-v3.2 优化" />
              </el-table>

              <h2 id="installation">安装步骤</h2>
              
              <h3 id="download">1. 下载安装包</h3>
              <p>从 Release Portal 下载对应版本的加密安装包：</p>
              <pre class="code-block"><code># 下载后解密（需要 GPG 密钥)
gpg --decrypt release-v2.4.1.zip.gpg > release-v2.4.1.zip

# 验证完整性
sha256sum -c release-v2.4.1.zip.sha256</code></pre>

              <h3 id="dependencies">2. 安装依赖</h3>
              <pre class="code-block"><code># CentOS/RHEL
sudo yum install -y docker-ce docker-compose-plugin

# Ubuntu
sudo apt-get update
sudo apt-get install -y docker-ce docker-compose-plugin</code></pre>

              <h3 id="deploy">3. 部署服务</h3>
              <pre class="code-block"><code># 解压安装包
unzip release-v2.4.1.zip -d /opt/product
cd /opt/product

# 根据硬件版本选择配置
cp config/hardware-v3.2.yml docker-compose.override.yml

# 启动服务
docker compose up -d</code></pre>

              <h2 id="verification">验证安装</h2>
              <p>服务启动后，访问健康检查端点：</p>
              <pre class="code-block"><code>curl http://localhost:8080/health

# 预期响应
{
  "status": "healthy",
  "version": "2.4.1",
  "hardware": "HW-v3.2"
}</code></pre>

              <h2 id="troubleshooting">故障排查</h2>
              <el-collapse>
                <el-collapse-item title="服务无法启动，日志显示 Illegal instruction">
                  <p><strong>可能原因：</strong>CPU 不支持 AVX-512（HW-v2.5 或更早）</p>
                  <p><strong>解决方案：</strong>升级硬件或使用 v2.3.x 版本</p>
                </el-collapse-item>
                <el-collapse-item title="数据库连接超时">
                  <p><strong>可能原因：</strong>内存不足或磁盘 I/O 瓶颈</p>
                  <p><strong>解决方案：</strong>增加内存至 32GB，使用 NVMe 存储</p>
                </el-collapse-item>
                <el-collapse-item title="AI 功能返回 503">
                  <p><strong>可能原因：</strong>硬件版本过低（HW-v3.0）</p>
                  <p><strong>解决方案：</strong>升级至 HW-v3.1+ 或禁用 AI 模块</p>
                </el-collapse-item>
              </el-collapse>
            </div>

            <el-divider />

            <!-- 文档底部 -->
            <div class="doc-footer-actions">
              <span class="footer-label">这篇文档对您有帮助吗？</span>
              <el-button @click="ElMessage.success('感谢您的认可！')">
                <el-icon><CircleCheck /></el-icon>
                有帮助
              </el-button>
              <el-button @click="feedbackVisible = true">
                <el-icon><CircleClose /></el-icon>
                需要改进
              </el-button>
              <span class="view-count">
                <el-icon><View /></el-icon>
                1,234 次阅读
              </span>
            </div>
          </el-card>
        </el-col>

        <!-- 右侧目录 -->
        <el-col :span="5">
          <div class="toc-sidebar">
            <el-card shadow="hover">
              <template #header>
                <div class="toc-header">
                  <el-icon><ListIcon /></el-icon>
                  <span>目录</span>
                </div>
              </template>
              <div class="toc-nav">
                <a
                  v-for="item in tocItems"
                  :key="item.id"
                  class="toc-item"
                  :class="{ 
                    active: activeTocId === item.id,
                    'toc-level-1': item.level === 1 
                  }"
                  @click="scrollToSection(item.id)"
                >
                  {{ item.text }}
                </a>
              </div>
            </el-card>

            <el-card shadow="hover" style="margin-top: 16px">
              <template #header>
                <div class="toc-header">
                  <el-icon><FolderOpened /></el-icon>
                  <span>相关文档</span>
                </div>
              </template>
              <div class="related-list">
                <a href="#" class="related-item" @click.prevent>
                  <el-icon><DocumentIcon /></el-icon>
                  <span>部署指南</span>
                </a>
                <a href="#" class="related-item" @click.prevent>
                  <el-icon><Setting /></el-icon>
                  <span>配置参考</span>
                </a>
                <a href="#" class="related-item" @click.prevent>
                  <el-icon><Lock /></el-icon>
                  <span>安全加固</span>
                </a>
              </div>
            </el-card>

            <el-card shadow="hover" style="margin-top: 16px; background: var(--el-color-primary-light-9)">
              <div class="support-box">
                <div class="support-title">遇到问题？</div>
                <p class="support-desc">技术支持团队随时为您服务</p>
                <el-button type="primary" size="small" style="width: 100%">
                  <el-icon><Service /></el-icon>
                  联系支持
                </el-button>
              </div>
            </el-card>
          </div>
        </el-col>
      </el-row>
    </template>

    <!-- 搜索对话框 -->
    <el-dialog
      v-model="searchVisible"
      title="搜索文档"
      width="600px"
      destroy-on-close
    >
      <el-input
        v-model="searchQuery"
        placeholder="搜索文档标题、内容..."
        clearable
        :prefix-icon="Search"
        style="margin-bottom: 20px"
      />
      <div class="search-results">
        <div class="search-section">
          <div class="search-section-title">最近搜索</div>
          <el-space wrap>
            <el-tag
              v-for="tag in ['快速开始', 'API 文档', '部署指南']"
              :key="tag"
              style="cursor: pointer"
              @click="searchQuery = tag"
            >
              {{ tag }}
            </el-tag>
          </el-space>
        </div>
      </div>
    </el-dialog>

    <!-- 版本对比对话框 -->
    <el-dialog
      v-model="diffVisible"
      title="版本对比"
      width="800px"
      destroy-on-close
    >
      <div class="diff-header">
        <div class="diff-version old">
          <div class="diff-label">旧版本</div>
          <div class="diff-name">v2.4.0</div>
          <div class="diff-date">2024-01-01</div>
        </div>
        <el-icon class="diff-arrow"><ArrowRight /></el-icon>
        <div class="diff-version new">
          <div class="diff-label">新版本</div>
          <div class="diff-name">v2.4.1</div>
          <div class="diff-date">2024-01-15</div>
        </div>
      </div>

      <div class="diff-list">
        <div class="diff-item added">
          <el-tag type="success" size="small" effect="dark">新增</el-tag>
          <span class="diff-title">HW-v3.2 优化说明</span>
        </div>
        <div class="diff-item added">
          <el-tag type="success" size="small" effect="dark">新增</el-tag>
          <span class="diff-title">故障排查章节</span>
        </div>
        <div class="diff-item modified">
          <el-tag type="warning" size="small" effect="dark">修改</el-tag>
          <span class="diff-title">硬件兼容性警告</span>
        </div>
      </div>
    </el-dialog>

    <!-- 反馈对话框 -->
    <el-dialog
      v-model="feedbackVisible"
      title="文档反馈"
      width="500px"
      destroy-on-close
    >
      <el-form label-position="top">
        <el-form-item label="反馈类型">
          <el-radio-group>
            <el-radio-button label="content">内容错误</el-radio-button>
            <el-radio-button label="missing">缺少信息</el-radio-button>
            <el-radio-button label="other">其他建议</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="详细描述">
          <el-input type="textarea" :rows="4" placeholder="请描述您遇到的问题或建议..." />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="feedbackVisible = false">取消</el-button>
        <el-button type="primary" @click="feedbackVisible = false; ElMessage.success('感谢您的反馈！')">
          提交反馈
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<style scoped>
/* 页面头部 */
.title-divider {
  color: #d9d9d9;
  font-weight: 300;
  margin: 0 8px;
  font-size: 24px;
  line-height: 1;
}

.back-home-link {
  display: inline-flex;
  align-items: center;
  color: #64748b;
  text-decoration: none;
  transition: color 0.2s;
}

.back-home-link:hover {
  color: var(--el-color-primary);
}

/* 左侧分类导航 */
.category-nav-card :deep(.el-card__header) {
  padding: 12px 16px;
}

.nav-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.category-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.category-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
  color: var(--el-text-color-regular);
}

.category-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}

.category-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}

.category-item .el-tag {
  margin-left: auto;
}

.category-section {
  margin-top: 4px;
}

.category-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}

.category-title:hover {
  background: var(--el-fill-color-light);
}

.category-title-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.category-arrow {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  transition: transform 0.2s;
}

.category-title.expanded .category-arrow {
  transform: rotate(90deg);
}

.category-docs {
  padding-left: 12px;
}

.doc-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px 8px 24px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  font-size: 13px;
  color: var(--el-text-color-regular);
}

.doc-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}

.doc-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}

.doc-item-text {
  flex: 1;
}

/* 统计卡片 */
.stats-row {
  margin-bottom: 20px;
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
  color: var(--el-text-color-primary);
}

.stat-mini-label {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 表格样式 */
.category-badge {
  display: inline-block;
  padding: 2px 10px;
  background: var(--el-fill-color-light);
  border-radius: 4px;
  font-size: 12px;
  color: var(--el-text-color-regular);
}

.version-badge {
  display: inline-block;
  padding: 2px 8px;
  background: var(--el-color-primary-light-9);
  border-radius: 4px;
  font-size: 12px;
  color: var(--el-color-primary);
  font-weight: 500;
}

/* 文档详情页左侧导航 */
.doc-nav-card :deep(.el-card__header) {
  padding: 12px 16px;
}

.doc-nav {
  max-height: calc(100vh - 300px);
  overflow-y: auto;
}

.nav-section {
  margin-bottom: 4px;
}

.nav-section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 0;
  font-size: 14px;
  font-weight: 500;
  color: var(--el-text-color-primary);
  cursor: pointer;
  transition: all 0.2s;
  border-radius: 6px;
}

.nav-section-title:hover {
  background: var(--el-fill-color-light);
}

.nav-section-left {
  display: flex;
  align-items: center;
  gap: 10px;
}

.nav-arrow {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  transition: transform 0.2s;
}

.nav-section-title.expanded .nav-arrow {
  transform: rotate(90deg);
}

.nav-items {
  padding-left: 24px;
}

.nav-item {
  display: block;
  padding: 8px 12px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  cursor: pointer;
  border-radius: 6px;
  transition: all 0.2s;
  text-decoration: none;
}

.nav-item:hover {
  background: var(--el-fill-color-light);
  color: var(--el-color-primary);
}

.nav-item.active {
  background: var(--el-color-primary-light-9);
  color: var(--el-color-primary);
  font-weight: 500;
}

/* 文档内容 */
.doc-detail-header {
  margin-bottom: 20px;
}

.doc-meta {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.doc-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 500;
}

.doc-badge.success {
  background: var(--el-color-success-light-9);
  color: var(--el-color-success);
  border: 1px solid var(--el-color-success-light-5);
}

.doc-meta-item {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.doc-title {
  font-size: 32px;
  font-weight: 700;
  color: var(--el-text-color-primary);
  margin-bottom: 12px;
}

.doc-desc {
  font-size: 16px;
  color: var(--el-text-color-secondary);
}

.doc-body {
  font-size: 14px;
  line-height: 1.8;
  color: var(--el-text-color-regular);
}

.doc-body :deep(h2) {
  font-size: 22px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-top: 32px;
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--el-border-color-light);
}

.doc-body :deep(h3) {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin-top: 24px;
  margin-bottom: 12px;
}

.doc-body :deep(p) {
  margin-bottom: 16px;
}

.doc-body :deep(ul),
.doc-body :deep(ol) {
  margin-bottom: 16px;
  padding-left: 24px;
}

.doc-body :deep(li) {
  margin-bottom: 8px;
}

.doc-body :deep(a) {
  color: var(--el-color-primary);
  text-decoration: none;
}

.doc-body :deep(a:hover) {
  text-decoration: underline;
}

.code-block {
  background: #1e1e1e;
  border-radius: 8px;
  padding: 16px;
  margin: 16px 0;
  overflow-x: auto;
}

.code-block code {
  color: #d4d4d4;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre;
}

/* 文档底部 */
.doc-footer-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.footer-label {
  font-size: 14px;
  color: var(--el-text-color-secondary);
}

.view-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin-left: auto;
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

/* 右侧目录 */
.toc-sidebar {
  position: sticky;
  top: 20px;
}

.toc-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}

.toc-nav {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toc-item {
  display: block;
  padding: 6px 0;
  color: var(--el-text-color-regular);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  text-decoration: none;
  border-radius: 4px;
}

.toc-item:hover {
  color: var(--el-color-primary);
  background: var(--el-fill-color-light);
  padding-left: 8px;
}

.toc-item.active {
  color: var(--el-color-primary);
  font-weight: 500;
  background: var(--el-color-primary-light-9);
  padding-left: 8px;
}

.toc-level-1 {
  padding-left: 12px;
}

/* 相关文档 */
.related-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.related-item {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--el-text-color-regular);
  font-size: 13px;
  cursor: pointer;
  transition: color 0.2s;
  padding: 4px 0;
}

.related-item:hover {
  color: var(--el-color-primary);
}

/* 支持框 */
.support-box {
  text-align: center;
}

.support-title {
  font-size: 14px;
  font-weight: 500;
  color: var(--el-color-primary);
  margin-bottom: 8px;
}

.support-desc {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 12px;
}

/* 版本对比 */
.diff-header {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 24px;
  padding: 16px;
  background: var(--el-fill-color-light);
  border-radius: 12px;
}

.diff-version {
  flex: 1;
  text-align: center;
}

.diff-version.old {
  opacity: 0.7;
}

.diff-label {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 4px;
}

.diff-name {
  font-size: 18px;
  font-weight: 600;
  color: var(--el-text-color-primary);
}

.diff-date {
  font-size: 13px;
  color: var(--el-text-color-secondary);
}

.diff-arrow {
  font-size: 20px;
  color: var(--el-text-color-secondary);
}

.diff-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.diff-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
}

.diff-title {
  flex: 1;
  font-size: 14px;
  color: var(--el-text-color-primary);
}

/* 分页 */
.pagination-wrapper {
  display: flex;
  justify-content: flex-end;
  margin-top: 20px;
  padding: 16px 0;
}

/* 搜索 */
.search-section {
  margin-bottom: 16px;
}

.search-section-title {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-bottom: 12px;
}
</style>
