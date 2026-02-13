# 分发中心页面重构方案

## 问题

当前页面是阻塞式 4 阶段工作流：用户点击"开始加密处理"后，页面卡在第 2 步轮询等待加密完成，无法提交新任务。加密是异步操作，页面不应该阻塞。

## 目标

- 提交后页面立即重置，允许连续提交多个文件
- 页面下方展示"加密队列"，实时显示所有任务的状态
- 队列中已完成（uploaded）的任务可以展开操作（复制链接、配置分发）

## 改动范围

### 1. 前端 index.vue — 重构页面流程

**改前**：4 阶段线性流程（文件选择 → 加密等待 → 上传存储 → 客户分发），一次只能处理一个任务

**改后**：上半部分是"提交区"（文件选择 + 客户信息 + 提交按钮），下半部分是"加密队列"

具体改动：
- 去掉 steps 进度条和 currentStep 状态机
- 把客户信息表单（name、email、releaseNotes）移到第 1 步，和文件选择放在一起
- 点击"开始加密处理"后：调用 API 创建任务 → 弹出成功提示 → 重置表单 → 刷新下方队列
- 去掉 stage 2/3/4 的卡片，全部由队列中的行内操作替代

### 2. 新建 EncryptionQueue.vue 组件

替代当前的 DistributionRecords.vue（保留 Records 作为历史记录），新增一个专门展示"进行中"任务的队列组件：

- 自动轮询 `GET /api/v1/distributions?status=pending,encrypting,uploaded`（多状态查询）
- 每条任务显示：文件名、客户、状态、进度动画、操作按钮
- 状态为 `uploaded` 的任务显示"配置分发"按钮，点击弹出 dialog 填写/确认分发信息
- 状态为 `pending`/`encrypting` 的任务显示加密进度动画
- 状态为 `sent` 的任务自动移出队列（进入下方历史记录）

### 3. 后端 — 支持多状态查询

当前 `ListDistributions` 的 `status` 参数只支持单个状态值。需要支持逗号分隔的多状态查询：
- `GET /api/v1/distributions?status=pending,encrypting,uploaded`
- 修改 `store/distribution.go` 的 `ListDistributions`，将 `status = $N` 改为 `status IN ($N...)`

### 4. 后端 — 任务指定 agent 标签

当前 `CreateDistribution` 创建的任务没有指定 `Schedule.TargetLabels`。如果要让专门的 agent 处理分发任务，需要：
- 在 `TaskCreateRequest` 中设置 `Schedule.TargetLabels: {"role": "distributor"}`
- 这是配置层面的改动，不影响代码逻辑，只需在 service/distribution.go 中加上 label

## 文件清单

| 文件 | 改动 |
|------|------|
| `frontend/src/pages/Distribution/index.vue` | 重构：去掉 steps，上半提交区 + 下半队列 |
| `frontend/src/pages/Distribution/components/EncryptionQueue.vue` | 新建：加密队列组件 |
| `frontend/src/pages/Distribution/components/DistributionRecords.vue` | 小改：只展示 sent/downloaded/expired 状态 |
| `frontend/src/api/distribution.ts` | 不变 |
| `server/internal/store/distribution.go` | 改 ListDistributions 支持多状态查询 |
| `server/internal/service/distribution.go` | 加 TargetLabels 到任务创建 |
