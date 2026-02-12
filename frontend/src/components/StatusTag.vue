<script setup lang="ts">
withDefaults(defineProps<{
  status: string
}>(), {
  status: '',
})

const colorMap: Record<string, string> = {
  online: 'success',
  offline: 'info',
  running: '',
  finished: 'success',
  done: 'success',
  failed: 'danger',
  timeout: 'warning',
  pending: 'warning',
  cancelled: 'info',
  ok: 'success',
}
</script>

<template>
  <el-tag :type="(colorMap[status] as any) || 'info'" size="small">
    {{ status }}
  </el-tag>
</template>
