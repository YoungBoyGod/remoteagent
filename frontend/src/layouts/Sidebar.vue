<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import {
  Monitor,
  Connection,
  Promotion,
  List,
  DataLine,
  Platform,
  Document,
  OfficeBuilding,
  Reading,
  Edit,
  FolderOpened,
  Management,
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()

const handleSelect = (index: string) => {
  router.push(index)
}
</script>

<template>
  <div class="sidebar-wrapper">
    <div class="logo">
      <el-icon size="28" class="logo-icon"><Connection /></el-icon>
      <span class="logo-text">RemoteAgent</span>
    </div>
    <el-menu
      :default-active="route.path"
      background-color="transparent"
      text-color="#94a3b8"
      active-text-color="#60a5fa"
      :router="false"
      @select="handleSelect"
      class="sidebar-menu"
    >
      <el-menu-item index="/">
        <el-icon><Monitor /></el-icon>
        <span>Dashboard</span>
      </el-menu-item>
      <el-menu-item index="/agents">
        <el-icon><Connection /></el-icon>
        <span>Agents</span>
      </el-menu-item>
      <el-menu-item index="/hosts">
        <el-icon><Platform /></el-icon>
        <span>主机管理</span>
      </el-menu-item>
      <el-menu-item index="/dispatch">
        <el-icon><Promotion /></el-icon>
        <span>任务分发</span>
      </el-menu-item>
      <el-menu-item index="/tasks">
        <el-icon><List /></el-icon>
        <span>任务管理</span>
      </el-menu-item>
      <el-menu-item index="/distribution">
        <el-icon><Promotion /></el-icon>
        <span>分发中心</span>
      </el-menu-item>
      <el-menu-item index="/monitor">
        <el-icon><DataLine /></el-icon>
        <span>监控中心</span>
      </el-menu-item>
      <el-menu-item index="/customers">
        <el-icon><OfficeBuilding /></el-icon>
        <span>客户管理</span>
      </el-menu-item>
      <el-menu-item index="/operation-logs">
        <el-icon><Document /></el-icon>
        <span>操作日志</span>
      </el-menu-item>
      <el-sub-menu index="/documents">
        <template #title>
          <el-icon><Reading /></el-icon>
          <span>文档中心</span>
        </template>
        <el-menu-item index="/documents">
          <el-icon><Reading /></el-icon>
          <span>文档浏览</span>
        </el-menu-item>
        <el-menu-item index="/documents/admin">
          <el-icon><Management /></el-icon>
          <span>文档管理</span>
        </el-menu-item>
        <el-menu-item index="/documents/editor">
          <el-icon><Edit /></el-icon>
          <span>新建文档</span>
        </el-menu-item>
        <el-menu-item index="/documents/categories">
          <el-icon><FolderOpened /></el-icon>
          <span>分类管理</span>
        </el-menu-item>
      </el-sub-menu>
    </el-menu>
    
    <div class="sidebar-footer">
      <div class="footer-line" />
      <div class="footer-text">v0.1.0</div>
    </div>
  </div>
</template>

<style scoped>
.sidebar-wrapper {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.logo {
  padding: 24px 20px;
  display: flex;
  align-items: center;
  gap: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  margin-bottom: 8px;
}

.logo-icon {
  color: #60a5fa;
  filter: drop-shadow(0 0 8px rgba(96, 165, 250, 0.3));
}

.logo-text {
  font-size: 18px;
  font-weight: 700;
  background: linear-gradient(135deg, #60a5fa 0%, #34d399 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  letter-spacing: 0.5px;
}

.sidebar-menu {
  flex: 1;
  border-right: none;
  padding: 0 12px;
}

.sidebar-menu :deep(.el-menu-item) {
  margin: 4px 0;
  border-radius: 10px;
  height: 46px;
  line-height: 46px;
  transition: all 0.25s ease;
  font-size: 14px;
}

.sidebar-menu :deep(.el-menu-item:hover) {
  background-color: rgba(255, 255, 255, 0.06);
  color: #e2e8f0;
}

.sidebar-menu :deep(.el-menu-item.is-active) {
  background: linear-gradient(135deg, rgba(96, 165, 250, 0.15) 0%, rgba(52, 211, 153, 0.08) 100%);
  color: #60a5fa;
  font-weight: 600;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
}

.sidebar-menu :deep(.el-menu-item .el-icon) {
  font-size: 18px;
  margin-right: 12px;
  transition: transform 0.2s ease;
}

.sidebar-menu :deep(.el-menu-item:hover .el-icon) {
  transform: scale(1.1);
}

/* Sub-menu 暗色主题 */
.sidebar-menu :deep(.el-sub-menu__title) {
  margin: 4px 0;
  border-radius: 10px;
  height: 46px;
  line-height: 46px;
  font-size: 14px;
  color: #94a3b8;
  transition: all 0.25s ease;
}

.sidebar-menu :deep(.el-sub-menu__title:hover) {
  background-color: rgba(255, 255, 255, 0.06);
  color: #e2e8f0;
}

.sidebar-menu :deep(.el-sub-menu__title .el-icon) {
  font-size: 18px;
  margin-right: 12px;
}

.sidebar-menu :deep(.el-sub-menu .el-menu) {
  background: transparent;
}

.sidebar-menu :deep(.el-sub-menu .el-menu .el-menu-item) {
  height: 40px;
  line-height: 40px;
  font-size: 13px;
  padding-left: 52px !important;
  margin: 2px 0;
}

.sidebar-menu :deep(.el-sub-menu__icon-arrow) {
  color: #64748b;
}

.sidebar-footer {
  padding: 16px 20px;
  text-align: center;
}

.footer-line {
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.1), transparent);
  margin-bottom: 12px;
}

.footer-text {
  font-size: 12px;
  color: #64748b;
  font-weight: 500;
}
</style>
