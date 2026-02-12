<script setup lang="ts">
import Sidebar from './Sidebar.vue'
</script>

<template>
  <el-container style="height: 100vh">
    <el-aside width="220px" class="app-sidebar">
      <Sidebar />
    </el-aside>
    <el-main class="app-main">
      <router-view />
    </el-main>
  </el-container>
</template>
