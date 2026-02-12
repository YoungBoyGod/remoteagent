<script setup lang="ts">
withDefaults(defineProps<{
  status: string
}>(), {
  status: '',
})

const colorMap: Record<string, 'success' | 'warning' | 'info' | 'danger'> = {
  online: 'success',
  offline: 'info',
  running: 'info',
  finished: 'success',
  done: 'success',
  success: 'success',
  failed: 'danger',
  timeout: 'warning',
  pending: 'warning',
  leased: 'info',
  canceled: 'info',
  cancelled: 'info',
  canceling: 'warning',
  ok: 'success',
}
</script>

<template>
  <el-tag :type="colorMap[status] ?? 'info'" size="small">
    {{ status }}
  </el-tag>
</template>
