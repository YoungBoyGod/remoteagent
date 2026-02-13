<script setup lang="ts">
import Sidebar from './Sidebar.vue'
</script>

<template>
  <el-container class="app-container">
    <el-aside width="240px" class="app-sidebar">
      <Sidebar />
    </el-aside>
    <el-main class="app-main">
      <router-view v-slot="{ Component }">
        <transition name="fade-transform" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </el-main>
  </el-container>
</template>

<style scoped>
.app-container {
  height: 100vh;
  overflow: hidden;
}

.app-sidebar {
  background: linear-gradient(180deg, #0f172a 0%, #1e293b 100%);
  border-right: 1px solid rgba(255, 255, 255, 0.05);
  box-shadow: 4px 0 24px rgba(0, 0, 0, 0.08);
  z-index: 100;
}

.app-main {
  padding: 24px 28px;
  background-color: #f5f7fa;
  overflow-y: auto;
  min-height: 100vh;
}

/* 页面切换动画 */
.fade-transform-enter-active,
.fade-transform-leave-active {
  transition: all 0.3s ease;
}

.fade-transform-enter-from {
  opacity: 0;
  transform: translateX(-10px);
}

.fade-transform-leave-to {
  opacity: 0;
  transform: translateX(10px);
}
</style>
