<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  items: { id: string; label: string }[]
  command: (item: { id: string; label: string }) => void
}>()

const selectedIndex = ref(0)

watch(() => props.items, () => {
  selectedIndex.value = 0
})

function onKeyDown(event: KeyboardEvent) {
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    selectedIndex.value = (selectedIndex.value + props.items.length - 1) % props.items.length
    return true
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    selectedIndex.value = (selectedIndex.value + 1) % props.items.length
    return true
  }
  if (event.key === 'Enter') {
    event.preventDefault()
    selectItem(selectedIndex.value)
    return true
  }
  return false
}

function selectItem(index: number) {
  const item = props.items[index]
  if (item) props.command(item)
}

defineExpose({ onKeyDown })
</script>

<template>
  <div class="mention-list" v-if="items.length">
    <button
      v-for="(item, index) in items"
      :key="item.id"
      class="mention-item"
      :class="{ selected: index === selectedIndex }"
      @click="selectItem(index)"
    >
      <span class="mention-avatar">{{ item.label[0] }}</span>
      <span>{{ item.label }}</span>
    </button>
  </div>
</template>

<style scoped>
.mention-list {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.1);
  padding: 4px;
  min-width: 180px;
  max-height: 240px;
  overflow-y: auto;
}

.mention-item {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  font-size: 13px;
  color: #334155;
  transition: all 0.15s;
}

.mention-item:hover,
.mention-item.selected {
  background: #f1f5f9;
  color: #4f46e5;
}

.mention-avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #e0e7ff;
  color: #4f46e5;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}
</style>
