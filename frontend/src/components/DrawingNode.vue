<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, watch } from 'vue'
import { nodeViewProps } from '@tiptap/vue-3'

const props = defineProps(nodeViewProps)

const canvasRef = ref<HTMLCanvasElement | null>(null)
const isDrawing = ref(false)
const currentTool = ref<'pen' | 'eraser' | 'line' | 'rect' | 'ellipse'>('pen')
const strokeColor = ref('#1e293b')
const strokeWidth = ref(2)
const canvasHeight = ref(360)

let ctx: CanvasRenderingContext2D | null = null
let lastX = 0
let lastY = 0
let startX = 0
let startY = 0
let snapshot: ImageData | null = null

const colors = ['#1e293b', '#ef4444', '#3b82f6', '#22c55e', '#f59e0b', '#8b5cf6', '#ec4899', '#06b6d4']
const widths = [1, 2, 4, 6]

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return
  canvas.width = canvas.offsetWidth
  canvas.height = canvasHeight.value
  ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.lineCap = 'round'
  ctx.lineJoin = 'round'

  // restore saved data
  const saved = props.node.attrs.data
  if (saved) {
    const img = new Image()
    img.onload = () => ctx!.drawImage(img, 0, 0)
    img.src = saved
  }
})

function getPos(e: MouseEvent) {
  const rect = canvasRef.value!.getBoundingClientRect()
  return { x: e.clientX - rect.left, y: e.clientY - rect.top }
}

function startDraw(e: MouseEvent) {
  if (!ctx) return
  isDrawing.value = true
  const { x, y } = getPos(e)
  lastX = x
  lastY = y
  startX = x
  startY = y
  if (currentTool.value === 'line' || currentTool.value === 'rect' || currentTool.value === 'ellipse') {
    snapshot = ctx.getImageData(0, 0, canvasRef.value!.width, canvasRef.value!.height)
  }
}

function draw(e: MouseEvent) {
  if (!isDrawing.value || !ctx) return
  const { x, y } = getPos(e)

  ctx.strokeStyle = currentTool.value === 'eraser' ? '#ffffff' : strokeColor.value
  ctx.lineWidth = currentTool.value === 'eraser' ? strokeWidth.value * 4 : strokeWidth.value

  if (currentTool.value === 'pen' || currentTool.value === 'eraser') {
    ctx.beginPath()
    ctx.moveTo(lastX, lastY)
    ctx.lineTo(x, y)
    ctx.stroke()
    lastX = x
    lastY = y
  } else if (snapshot) {
    ctx.putImageData(snapshot, 0, 0)
    ctx.beginPath()
    if (currentTool.value === 'line') {
      ctx.moveTo(startX, startY)
      ctx.lineTo(x, y)
      ctx.stroke()
    } else if (currentTool.value === 'rect') {
      ctx.strokeRect(startX, startY, x - startX, y - startY)
    } else if (currentTool.value === 'ellipse') {
      const cx = (startX + x) / 2
      const cy = (startY + y) / 2
      const rx = Math.abs(x - startX) / 2
      const ry = Math.abs(y - startY) / 2
      ctx.ellipse(cx, cy, rx, ry, 0, 0, Math.PI * 2)
      ctx.stroke()
    }
  }
}

function endDraw() {
  if (!isDrawing.value) return
  isDrawing.value = false
  snapshot = null
  saveData()
}

function saveData() {
  if (!canvasRef.value) return
  const data = canvasRef.value.toDataURL('image/png')
  props.updateAttributes({ data })
}

function clearCanvas() {
  if (!ctx || !canvasRef.value) return
  ctx.clearRect(0, 0, canvasRef.value.width, canvasRef.value.height)
  saveData()
}

function deleteNode() {
  props.deleteNode()
}
</script>

<template>
  <node-view-wrapper class="drawing-wrapper" contenteditable="false">
    <div class="drawing-toolbar">
      <div class="tool-group">
        <button
          v-for="tool in (['pen', 'eraser', 'line', 'rect', 'ellipse'] as const)"
          :key="tool"
          :class="{ active: currentTool === tool }"
          @click="currentTool = tool"
          :title="{ pen: '画笔', eraser: '橡皮擦', line: '直线', rect: '矩形', ellipse: '椭圆' }[tool]"
        >
          {{ { pen: '✏️', eraser: '🧹', line: '╱', rect: '▭', ellipse: '◯' }[tool] }}
        </button>
      </div>
      <div class="tool-divider" />
      <div class="tool-group colors">
        <button
          v-for="c in colors"
          :key="c"
          class="color-btn"
          :class="{ active: strokeColor === c }"
          :style="{ background: c }"
          @click="strokeColor = c"
        />
      </div>
      <div class="tool-divider" />
      <div class="tool-group">
        <button
          v-for="w in widths"
          :key="w"
          :class="{ active: strokeWidth === w }"
          @click="strokeWidth = w"
        >
          <span class="width-dot" :style="{ width: w * 2 + 'px', height: w * 2 + 'px' }" />
        </button>
      </div>
      <div class="tool-divider" />
      <button class="tool-action" @click="clearCanvas" title="清空">🗑️</button>
      <button class="tool-action danger" @click="deleteNode" title="删除画板">✕</button>
    </div>
    <canvas
      ref="canvasRef"
      class="drawing-canvas"
      :height="canvasHeight"
      @mousedown="startDraw"
      @mousemove="draw"
      @mouseup="endDraw"
      @mouseleave="endDraw"
    />
  </node-view-wrapper>
</template>

<style scoped>
.drawing-wrapper {
  margin: 16px 0;
  border: 1px solid #e2e8f0;
  border-radius: 10px;
  overflow: hidden;
  background: #fff;
}

.drawing-toolbar {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  background: #f8fafc;
  border-bottom: 1px solid #e2e8f0;
  flex-wrap: wrap;
}

.tool-group {
  display: flex;
  align-items: center;
  gap: 2px;
}

.tool-divider {
  width: 1px;
  height: 20px;
  background: #e2e8f0;
  margin: 0 4px;
}

.drawing-toolbar button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  border: none;
  background: transparent;
  border-radius: 6px;
  cursor: pointer;
  font-size: 14px;
  transition: all 0.15s;
}

.drawing-toolbar button:hover {
  background: #e2e8f0;
}

.drawing-toolbar button.active {
  background: #e0e7ff;
  color: #4f46e5;
}

.color-btn {
  width: 20px !important;
  height: 20px !important;
  border-radius: 50% !important;
  border: 2px solid transparent !important;
}

.color-btn.active {
  border-color: #1e293b !important;
  box-shadow: 0 0 0 2px #fff, 0 0 0 4px #1e293b;
}

.width-dot {
  display: block;
  border-radius: 50%;
  background: #475569;
}

.tool-action {
  font-size: 13px !important;
}

.tool-action.danger:hover {
  background: #fee2e2;
  color: #ef4444;
}

.drawing-canvas {
  display: block;
  width: 100%;
  cursor: crosshair;
  background: #fff;
}
</style>
