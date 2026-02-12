<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router'
import {
  Monitor,
  Connection,
  Promotion,
  List,
  DataLine,
} from '@element-plus/icons-vue'

const route = useRoute()
const router = useRouter()

const handleSelect = (index: string) => {
  router.push(index)
}
</script>

<template>
  <div style="padding: 20px 0; text-align: center; color: #fff; font-size: 18px; font-weight: bold">
    RemoteAgent
  </div>
  <el-menu
    :default-active="route.path"
    background-color="#001529"
    text-color="#ffffffa6"
    active-text-color="#fff"
    :router="false"
    @select="handleSelect"
  >
    <el-menu-item index="/">
      <el-icon><Monitor /></el-icon>
      <span>Dashboard</span>
    </el-menu-item>
    <el-menu-item index="/agents">
      <el-icon><Connection /></el-icon>
      <span>Agents</span>
    </el-menu-item>
    <el-menu-item index="/dispatch">
      <el-icon><Promotion /></el-icon>
      <span>Dispatch</span>
    </el-menu-item>
    <el-menu-item index="/tasks">
      <el-icon><List /></el-icon>
      <span>Tasks</span>
    </el-menu-item>
    <el-menu-item index="/monitor">
      <el-icon><DataLine /></el-icon>
      <span>Monitor</span>
    </el-menu-item>
  </el-menu>
</template>
