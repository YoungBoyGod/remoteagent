<script setup lang="ts">
defineProps<{
  code: string
  lang?: string
}>()
</script>

<template>
  <pre
    style="
      background-color: #1e1e1e;
      color: #d4d4d4;
      padding: 12px 16px;
      border-radius: 4px;
      overflow-x: auto;
      font-family: 'Courier New', Courier, monospace;
      font-size: 13px;
      line-height: 1.5;
      white-space: pre-wrap;
      word-break: break-all;
    "
  ><code>{{ code }}</code></pre>
</template>
