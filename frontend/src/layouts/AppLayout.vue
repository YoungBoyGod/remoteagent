<script setup lang="ts">
import Sidebar from './Sidebar.vue'
</script>

<template>
  <el-container style="height: 100vh">
    <el-aside width="220px" style="background-color: #001529">
      <Sidebar />
    </el-aside>
    <el-main style="padding: 20px; background-color: #f5f7fa">
      <router-view />
    </el-main>
  </el-container>
</template>
