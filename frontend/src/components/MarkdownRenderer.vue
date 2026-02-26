<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted, nextTick } from 'vue'
import MarkdownIt from 'markdown-it'
import anchor from 'markdown-it-anchor'
import Prism from 'prismjs'
import mermaid from 'mermaid'

// Prism 语言支持
import 'prismjs/components/prism-javascript'
import 'prismjs/components/prism-typescript'
import 'prismjs/components/prism-go'
import 'prismjs/components/prism-python'
import 'prismjs/components/prism-bash'
import 'prismjs/components/prism-sql'
import 'prismjs/components/prism-yaml'
import 'prismjs/components/prism-json'
import 'prismjs/components/prism-markup'
import 'prismjs/components/prism-css'

interface TocItem {
  id: string
  text: string
  level: number
}

const props = withDefaults(defineProps<{
  content: string
  theme?: 'dark' | 'light'
}>(), {
  theme: 'dark',
})

const emit = defineEmits<{
  'toc-generated': [items: TocItem[]]
}>()

const containerRef = ref<HTMLElement>()
const renderedHtml = ref('')

let mermaidCounter = 0

function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

function highlightCode(str: string, lang: string): string {
  // mermaid 特殊处理
  if (lang === 'mermaid') {
    const id = `mermaid-${mermaidCounter++}`
    return `<div class="mermaid-block" data-mermaid-id="${id}">${str}</div>`
  }

  const normalizedLang = lang === 'html' ? 'markup' : lang
  if (normalizedLang && Prism.languages[normalizedLang]) {
    const highlighted = Prism.highlight(str, Prism.languages[normalizedLang], normalizedLang)
    return `<div class="code-block-wrapper" data-lang="${lang}">` +
      `<div class="code-block-header">` +
        `<span class="code-lang">${lang}</span>` +
        `<button class="copy-btn" data-code="${encodeURIComponent(str)}">复制</button>` +
      `</div>` +
      `<pre class="code-block"><code class="language-${normalizedLang}">${highlighted}</code></pre>` +
    `</div>`
  }

  // 无语言标记的代码块
  return `<div class="code-block-wrapper">` +
    `<div class="code-block-header">` +
      `<span class="code-lang">code</span>` +
      `<button class="copy-btn" data-code="${encodeURIComponent(str)}">复制</button>` +
    `</div>` +
    `<pre class="code-block"><code>${escapeHtml(str)}</code></pre>` +
  `</div>`
}

// markdown-it 实例
const md = new MarkdownIt({
  html: true,
  linkify: true,
  typographer: true,
  highlight: highlightCode,
})

// 锚点插件
md.use(anchor, {
  permalink: false,
  slugify: (s: string) => s.trim().toLowerCase().replace(/\s+/g, '-').replace(/[^\w\u4e00-\u9fa5-]/g, ''),
})

// 提取 TOC
function extractToc(content: string): TocItem[] {
  const items: TocItem[] = []
  const tokens = md.parse(content, {})
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i]
    if (token.type === 'heading_open') {
      const level = parseInt(token.tag.slice(1))
      const inline = tokens[i + 1]
      if (inline?.type === 'inline' && inline.content) {
        const text = inline.content
        const id = text.trim().toLowerCase().replace(/\s+/g, '-').replace(/[^\w\u4e00-\u9fa5-]/g, '')
        items.push({ id, text, level })
      }
    }
  }
  return items
}

// 渲染 mermaid 图表
async function renderMermaid() {
  if (!containerRef.value) return
  const blocks = containerRef.value.querySelectorAll('.mermaid-block')
  if (blocks.length === 0) return

  mermaid.initialize({
    startOnLoad: false,
    theme: props.theme === 'dark' ? 'dark' : 'default',
    securityLevel: 'loose',
  })

  for (const block of blocks) {
    const id = block.getAttribute('data-mermaid-id') || `mermaid-${Date.now()}`
    const code = block.textContent || ''
    try {
      const { svg } = await mermaid.render(id, code)
      block.innerHTML = svg
      block.classList.add('mermaid-rendered')
    } catch {
      block.innerHTML = `<div class="mermaid-error">Mermaid 图表渲染失败</div>`
    }
  }
}

// 复制按钮事件委托
function handleClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (!target.classList.contains('copy-btn')) return

  const code = decodeURIComponent(target.getAttribute('data-code') || '')
  navigator.clipboard.writeText(code).then(() => {
    target.textContent = '已复制'
    target.classList.add('copied')
    setTimeout(() => {
      target.textContent = '复制'
      target.classList.remove('copied')
    }, 2000)
  })
}

// 渲染逻辑
function render() {
  mermaidCounter = 0
  renderedHtml.value = md.render(props.content || '')
  const toc = extractToc(props.content || '')
  emit('toc-generated', toc)

  nextTick(() => {
    renderMermaid()
  })
}

watch(() => props.content, render, { immediate: true })
watch(() => props.theme, () => {
  nextTick(() => renderMermaid())
})

onMounted(() => {
  containerRef.value?.addEventListener('click', handleClick)
})

onUnmounted(() => {
  containerRef.value?.removeEventListener('click', handleClick)
})
</script>

<template>
  <div
    ref="containerRef"
    class="md-renderer"
    :class="[`theme-${theme}`]"
    v-html="renderedHtml"
  />
</template>

<style scoped>
.md-renderer {
  font-size: 15px;
  line-height: 1.8;
  color: #cbd5e1;
  word-wrap: break-word;
}

/* 标题 */
.md-renderer :deep(h1) {
  font-size: 32px;
  font-weight: 700;
  color: #f8fafc;
  margin: 40px 0 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.md-renderer :deep(h2) {
  font-size: 24px;
  font-weight: 600;
  color: #e2e8f0;
  margin: 40px 0 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.md-renderer :deep(h3) {
  font-size: 18px;
  font-weight: 600;
  color: #e2e8f0;
  margin: 32px 0 12px;
}

.md-renderer :deep(h4) {
  font-size: 16px;
  font-weight: 600;
  color: #e2e8f0;
  margin: 24px 0 8px;
}

.md-renderer :deep(h5),
.md-renderer :deep(h6) {
  font-size: 14px;
  font-weight: 600;
  color: #e2e8f0;
  margin: 20px 0 8px;
}

/* 段落 */
.md-renderer :deep(p) {
  margin-bottom: 16px;
}

/* 列表 */
.md-renderer :deep(ul),
.md-renderer :deep(ol) {
  margin-bottom: 16px;
  padding-left: 24px;
}

.md-renderer :deep(li) {
  margin-bottom: 8px;
}

.md-renderer :deep(li > ul),
.md-renderer :deep(li > ol) {
  margin-top: 8px;
  margin-bottom: 0;
}

/* 链接 */
.md-renderer :deep(a) {
  color: #60a5fa;
  text-decoration: none;
  transition: color 0.2s;
}

.md-renderer :deep(a:hover) {
  text-decoration: underline;
  color: #93c5fd;
}

/* 行内代码 */
.md-renderer :deep(code) {
  background: rgba(30, 41, 59, 0.8);
  padding: 2px 6px;
  border-radius: 4px;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  color: #7dd3fc;
}

/* 代码块容器 */
.md-renderer :deep(.code-block-wrapper) {
  margin: 16px 0;
  border-radius: 8px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.md-renderer :deep(.code-block-header) {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
  background: rgba(15, 23, 42, 0.9);
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.md-renderer :deep(.code-lang) {
  font-size: 12px;
  color: #64748b;
  font-family: 'JetBrains Mono', monospace;
  text-transform: uppercase;
}

.md-renderer :deep(.copy-btn) {
  padding: 2px 10px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  color: #94a3b8;
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.md-renderer :deep(.copy-btn:hover) {
  background: rgba(255, 255, 255, 0.12);
  color: #e2e8f0;
}

.md-renderer :deep(.copy-btn.copied) {
  background: rgba(82, 196, 26, 0.15);
  border-color: rgba(82, 196, 26, 0.3);
  color: #52c41a;
}

.md-renderer :deep(.code-block) {
  background: rgba(15, 23, 42, 0.8);
  padding: 16px;
  margin: 0;
  overflow-x: auto;
}

.md-renderer :deep(.code-block code) {
  background: none !important;
  padding: 0 !important;
  color: #e2e8f0 !important;
  font-family: 'JetBrains Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 13px;
  line-height: 1.6;
  white-space: pre;
}

/* Prism 语法高亮 token 颜色 */
.md-renderer :deep(.token.comment),
.md-renderer :deep(.token.prolog),
.md-renderer :deep(.token.doctype),
.md-renderer :deep(.token.cdata) {
  color: #64748b;
}

.md-renderer :deep(.token.punctuation) {
  color: #94a3b8;
}

.md-renderer :deep(.token.property),
.md-renderer :deep(.token.tag),
.md-renderer :deep(.token.boolean),
.md-renderer :deep(.token.number),
.md-renderer :deep(.token.constant),
.md-renderer :deep(.token.symbol) {
  color: #f59e0b;
}

.md-renderer :deep(.token.selector),
.md-renderer :deep(.token.attr-name),
.md-renderer :deep(.token.string),
.md-renderer :deep(.token.char),
.md-renderer :deep(.token.builtin) {
  color: #34d399;
}

.md-renderer :deep(.token.operator),
.md-renderer :deep(.token.entity),
.md-renderer :deep(.token.url) {
  color: #7dd3fc;
}

.md-renderer :deep(.token.atrule),
.md-renderer :deep(.token.attr-value),
.md-renderer :deep(.token.keyword) {
  color: #c084fc;
}

.md-renderer :deep(.token.function),
.md-renderer :deep(.token.class-name) {
  color: #60a5fa;
}

.md-renderer :deep(.token.regex),
.md-renderer :deep(.token.important),
.md-renderer :deep(.token.variable) {
  color: #fb923c;
}

/* 引用块 */
.md-renderer :deep(blockquote) {
  margin: 16px 0;
  padding: 12px 20px;
  border-left: 4px solid #4096ff;
  background: rgba(64, 150, 255, 0.05);
  border-radius: 0 8px 8px 0;
  color: #94a3b8;
}

.md-renderer :deep(blockquote p:last-child) {
  margin-bottom: 0;
}

/* 表格 */
.md-renderer :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 20px 0;
  font-size: 14px;
}

.md-renderer :deep(th),
.md-renderer :deep(td) {
  border: 1px solid rgba(255, 255, 255, 0.1);
  padding: 12px 16px;
  text-align: left;
}

.md-renderer :deep(th) {
  background: rgba(30, 41, 59, 0.5);
  font-weight: 600;
  color: #e2e8f0;
}

.md-renderer :deep(td) {
  color: #cbd5e1;
}

.md-renderer :deep(tr:hover td) {
  background: rgba(255, 255, 255, 0.02);
}

/* 图片 */
.md-renderer :deep(img) {
  max-width: 100%;
  border-radius: 8px;
  margin: 16px 0;
}

/* 水平线 */
.md-renderer :deep(hr) {
  border: none;
  height: 1px;
  background: rgba(255, 255, 255, 0.1);
  margin: 32px 0;
}

/* 强调 */
.md-renderer :deep(strong) {
  color: #e2e8f0;
  font-weight: 600;
}

.md-renderer :deep(em) {
  color: #94a3b8;
  font-style: italic;
}

/* 删除线 */
.md-renderer :deep(del) {
  color: #64748b;
  text-decoration: line-through;
}

/* Mermaid */
.md-renderer :deep(.mermaid-block) {
  margin: 20px 0;
  padding: 20px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  text-align: center;
  overflow-x: auto;
}

.md-renderer :deep(.mermaid-rendered) {
  background: rgba(15, 23, 42, 0.4);
}

.md-renderer :deep(.mermaid-rendered svg) {
  max-width: 100%;
}

.md-renderer :deep(.mermaid-error) {
  color: #ff4d4f;
  font-size: 13px;
  padding: 12px;
}

/* 任务列表 */
.md-renderer :deep(input[type="checkbox"]) {
  margin-right: 8px;
  accent-color: #4096ff;
}
</style>
