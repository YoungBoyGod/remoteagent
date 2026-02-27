# Distribution Center Tabs Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将分发中心页面改为上下结构，并把现有 card 区域统一为可切换的 Tab 界面，且“新建加密分发任务”改名为“新建加密任务”，使用现有 `EncryptionQueue` 作为“加密队列”Tab。

**Architecture:** 在 `frontend/src/pages/Distribution/index.vue` 中引入主级 `el-tabs` 作为页面分区容器。上半区放“新建加密任务”Tab，下半区放“加密队列/发布说明/分发记录”Tabs，并复用现有 3 个子组件。提交任务成功后自动切换到“加密队列”Tab，保持原有刷新逻辑。

**Tech Stack:** Vue 3 (`<script setup>`), Element Plus (`el-tabs`, `el-card`, `el-button`), TypeScript, Vite。

---

### Task 1: 主页面结构重排（上下布局 + Tabs 骨架）

**Files:**
- Modify: `frontend/src/pages/Distribution/index.vue`

**Step 1: Write the failing test**

通过最小行为断言定义预期（以构建+页面结构关键字为门槛）：

```bash
# 预期先失败：当前文件不存在新的 tab key（new-task / encrypt-queue）
grep -n "new-task\|encrypt-queue" frontend/src/pages/Distribution/index.vue
```

**Step 2: Run test to verify it fails**

Run:
```bash
grep -n "new-task\|encrypt-queue" frontend/src/pages/Distribution/index.vue
```
Expected: 未命中或仅部分命中，证明新结构尚未完整落地。

**Step 3: Write minimal implementation**

在 `index.vue` 中完成：
1. 新增两个 Tab 状态：
   - 顶部 `createTab`（默认 `new-task`）
   - 底部 `flowTab`（默认 `encrypt-queue`）
2. 页面模板改为“上下结构”：
   - 上：`el-tabs`（仅 1 个可见 pane：新建加密任务，便于后续扩展）
   - 下：`el-tabs`（加密队列 / 发布说明 / 分发记录）
3. 将原先右栏 card 直接替换为 Tab Pane 挂载组件：
   - `EncryptionQueue`
   - `ReleaseNotes`
   - `DistributionRecords`

**Step 4: Run test to verify it passes**

Run:
```bash
grep -n "new-task\|encrypt-queue" frontend/src/pages/Distribution/index.vue
```
Expected: 命中新增 tab key 与对应 pane。

**Step 5: Commit**

```bash
git add frontend/src/pages/Distribution/index.vue
git commit -m "refactor: convert distribution cards to tab-based vertical layout"
```

---

### Task 2: 文案与交互对齐（新建加密任务 + 提交后跳转队列）

**Files:**
- Modify: `frontend/src/pages/Distribution/index.vue`

**Step 1: Write the failing test**

```bash
# 预期旧文案存在，新文案不存在（或不完整）
grep -n "新建加密分发任务\|新建加密任务" frontend/src/pages/Distribution/index.vue
```

**Step 2: Run test to verify it fails**

Run:
```bash
grep -n "新建加密分发任务\|新建加密任务" frontend/src/pages/Distribution/index.vue
```
Expected: 仍含旧文案“新建加密分发任务”。

**Step 3: Write minimal implementation**

在 `index.vue`：
1. 文案替换：
   - `新建加密分发任务` → `新建加密任务`
2. `submitEncryption` 成功后增加：
   - `flowTab.value = 'encrypt-queue'`
   - 保留 `queueRef.value?.refresh()`
3. 不改业务 API 参数与提交流程。

**Step 4: Run test to verify it passes**

Run:
```bash
grep -n "新建加密分发任务" frontend/src/pages/Distribution/index.vue && exit 1 || true
grep -n "新建加密任务" frontend/src/pages/Distribution/index.vue
```
Expected: 旧文案无匹配，新文案存在；提交成功后跳转逻辑存在。

**Step 5: Commit**

```bash
git add frontend/src/pages/Distribution/index.vue
git commit -m "feat: rename create task section and focus encrypt queue after submit"
```

---

### Task 3: 视觉优化（统一 Tabs 风格与响应式）

**Files:**
- Modify: `frontend/src/pages/Distribution/index.vue`

**Step 1: Write the failing test**

```bash
# 预期尚无新的样式类
grep -n "distribution-tabs\|flow-tabs\|create-tabs" frontend/src/pages/Distribution/index.vue
```

**Step 2: Run test to verify it fails**

Run:
```bash
grep -n "distribution-tabs\|flow-tabs\|create-tabs" frontend/src/pages/Distribution/index.vue
```
Expected: 类名未完整出现。

**Step 3: Write minimal implementation**

在 `<style scoped>` 添加并应用样式：
1. 外层容器改纵向：统一间距、标题层级；
2. 顶部/底部 Tabs 风格一致（tab header、panel 背景、边距）；
3. 移除旧左右双栏样式（如 `page-body`, `right-panel`, 固定宽 `submit-panel`）；
4. 小屏下维持单列自适应。

**Step 4: Run test to verify it passes**

Run:
```bash
grep -n "distribution-tabs\|flow-tabs\|create-tabs" frontend/src/pages/Distribution/index.vue
```
Expected: 新样式类均存在，旧左右布局样式已不再用于主结构。

**Step 5: Commit**

```bash
git add frontend/src/pages/Distribution/index.vue
git commit -m "style: polish distribution center tab layout and spacing"
```

---

### Task 4: 集成验证（构建 + 页面可访问）

**Files:**
- Modify: `frontend/src/pages/Distribution/index.vue`（如需小修）

**Step 1: Write the failing test**

```bash
# 先进行一次完整构建作为回归门禁（如有类型/模板错误会失败）
npm --prefix frontend run build
```

**Step 2: Run test to verify it fails**

Run:
```bash
npm --prefix frontend run build
```
Expected: 若有模板/类型错误则 FAIL（必须先解决）。

**Step 3: Write minimal implementation**

修复构建暴露的最小问题（仅限本次 tab 重构相关）。

**Step 4: Run test to verify it passes**

Run:
```bash
npm --prefix frontend run build
```
Expected: PASS。

然后启动前端开发服务并验证页面：

```bash
nohup npm --prefix frontend run dev > frontend/logs/dev.log 2>&1 &
curl -fsS --max-time 3 http://127.0.0.1:7000/healthz
```

Expected: 健康检查返回 200，页面可访问。

**Step 5: Commit**

```bash
git add frontend/src/pages/Distribution/index.vue
git commit -m "test: verify distribution tabs layout build and runtime"
```

---

### Task 5: 最终整合提交（压缩为单功能提交）

**Files:**
- Modify: `frontend/src/pages/Distribution/index.vue`

**Step 1: Write the failing test**

```bash
git diff -- frontend/src/pages/Distribution/index.vue
```

**Step 2: Run test to verify it fails**

Run:
```bash
git diff -- frontend/src/pages/Distribution/index.vue
```
Expected: 存在未提交改动。

**Step 3: Write minimal implementation**

清理无关改动，确保只包含本次“分发中心 tabs 化 + 上下结构 + 文案调整”。

**Step 4: Run test to verify it passes**

Run:
```bash
npm --prefix frontend run build
git status --short
```
Expected: 构建通过，变更文件范围可控。

**Step 5: Commit**

```bash
git add frontend/src/pages/Distribution/index.vue
git commit -m "feat: redesign distribution center with vertical tabbed workflow"
```
