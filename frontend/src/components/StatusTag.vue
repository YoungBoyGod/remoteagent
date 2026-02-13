<script setup lang="ts">
withDefaults(defineProps<{
  status: string
}>(), {
  status: '',
})

const colorMap: Record<string, 'success' | 'warning' | 'info' | 'danger' | 'primary'> = {
  // 在线状态
  online: 'success',
  offline: 'info',
  
  // 任务状态
  running: 'primary',
  leased: 'primary',
  pending: 'warning',
  
  // 完成状态
  finished: 'success',
  done: 'success',
  success: 'success',
  
  // 失败状态
  failed: 'danger',
  timeout: 'warning',
  canceled: 'info',
  cancelled: 'info',
  canceling: 'warning',
  
  // 健康状态
  ok: 'success',
  healthy: 'success',
  error: 'danger',
  critical: 'danger',
  
  // 其他
  active: 'success',
  inactive: 'info',
  waiting: 'warning',
  busy: 'warning',
  unknown: 'info',
}

const textMap: Record<string, string> = {
  online: '在线',
  offline: '离线',
  running: '运行中',
  leased: '已分配',
  pending: '等待中',
  done: '已完成',
  success: '成功',
  failed: '失败',
  timeout: '超时',
  canceled: '已取消',
  cancelled: '已取消',
  canceling: '取消中',
  ok: '正常',
  healthy: '健康',
  error: '错误',
  critical: '严重',
  active: '活跃',
  inactive: '停用',
  waiting: '等待中',
  busy: '忙碌',
  unknown: '未知',
}

function getDisplayText(status: string): string {
  return textMap[status] || status
}
</script>

<template>
  <span class="status-badge" :class="colorMap[status] ?? 'info'">
    {{ getDisplayText(status) }}
  </span>
</template>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.status-badge.success {
  color: #52c41a;
  background: #f6ffed;
}

.status-badge.warning {
  color: #faad14;
  background: #fffbe6;
}

.status-badge.danger {
  color: #ff4d4f;
  background: #fff2f0;
}

.status-badge.info {
  color: #8c8c8c;
  background: #f5f5f5;
}

.status-badge.primary {
  color: #4096ff;
  background: #eaf6ff;
}
</style>
