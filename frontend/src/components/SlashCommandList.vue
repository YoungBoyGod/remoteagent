<script setup lang="ts">
import { ref, watch } from 'vue'

export interface SlashCommandItem {
  title: string
  description: string
  icon: string
  command: (props: { editor: any; range: any }) => void
}

const props = defineProps<{
  items: SlashCommandItem[]
  command: (item: SlashCommandItem) => void
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
  <div class="slash-command-list" v-if="items.length">
    <div class="slash-header">插入内容</div>
    <button
      v-for="(item, index) in items"
      :key="item.title"
      class="slash-item"
      :class="{ selected: index === selectedIndex }"
      @click="selectItem(index)"
    >
      <span class="slash-icon">{{ item.icon }}</span>
      <div class="slash-text">
        <span class="slash-title">{{ item.title }}</span>
        <span class="slash-desc">{{ item.description }}</span>
      </div>
    </button>
  </div>
</template>

<style scoped>
.slash-command-list {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.12);
  padding: 6px;
  min-width: 260px;
  max-height: 320px;
  overflow-y: auto;
}

.slash-header {
  padding: 6px 12px;
  font-size: 11px;
  font-weight: 600;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.slash-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 8px 12px;
  border: none;
  background: transparent;
  border-radius: 8px;
  cursor: pointer;
  text-align: left;
  transition: all 0.15s;
}

.slash-item:hover,
.slash-item.selected {
  background: #f1f5f9;
}

.slash-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.slash-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.slash-title {
  font-size: 13px;
  font-weight: 500;
  color: #1e293b;
}

.slash-desc {
  font-size: 11px;
  color: #94a3b8;
}
</style>
