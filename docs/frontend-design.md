# RemoteAgent 前端设计文档

## 1. 项目概述

### 1.1 目标

为 RemoteAgent 任务执行平台提供 Web 管理界面，支持：
- 查看系统状态和 Agent 在线情况
- 管理和监控 Agent 设备
- 下发任务并实时查看执行结果
- 浏览任务历史记录
- 系统监控和健康检查

### 1.2 技术选型

| 技术 | 版本 | 用途 |
|------|------|------|
| Vue | 3.4+ | UI 框架（Composition API + script setup） |
| TypeScript | 5.x | 类型安全 |
| Vite | 5.x | 构建工具 |
| Element Plus | 2.x | UI 组件库 |
| Vue Router | 4.x | 路由管理 |
| Axios | 1.x | HTTP 客户端 |

### 1.3 部署方式

前端构建产物为静态文件（`dist/`），部署方式：
- 开发环境：Vite dev server，proxy 到 `http://localhost:40001`
- 生产环境：构建后的静态文件通过 Nginx 托管，反向代理 API 到 Server

### 1.4 认证方式

内部管理工具，使用固定 Admin Token（`X-Register-Token` header），通过环境变量 `VITE_ADMIN_TOKEN` 配置。

## 2. 页面规划

共 5 个主要页面，分配给 5 个前端开发者。

### 页面1 - Dashboard 总览页（路由: `/`）

**负责人**: 前端开发者 A

**功能描述**:
- 系统状态卡片：在线 Agent 数量、总任务数量、运行中任务数
- 健康检查状态：调用 `/healthz` 显示服务状态和时间戳
- 最近任务列表：展示最近 10 条任务（task_id、agent_id、status、时间）

**UI 布局**:
```
+------------------------------------------+
|  [Agent 在线数]  [任务总数]  [运行中任务]   |
+------------------------------------------+
|  服务状态: ok    时间戳: 2026-02-12 ...    |
+------------------------------------------+
|  最近任务                                  |
|  task_id | agent_id | status | 时间       |
|  ...     | ...      | ...    | ...        |
+------------------------------------------+
```

### 页面2 - Agent 管理页（路由: `/agents`）

**负责人**: 前端开发者 B

**功能描述**:
- Agent 列表表格：agent_id、device_code、hostname、OS、arch、IP、状态、最后心跳时间
- 在线/离线状态标识（绿色/灰色圆点）
- 判断逻辑：最后心跳时间距今超过 90 秒（3 倍心跳间隔）视为离线
- 点击行展开查看 Agent 详情（labels、capabilities、agent_version）
- 支持按状态筛选、按 device_code 搜索

**UI 布局**:
```
+--------------------------------------------------+
|  搜索: [device_code]   筛选: [全部|在线|离线]       |
+--------------------------------------------------+
|  agent_id | device_code | 状态 | IP | 最后心跳     |
|  agent-01 | device-01   | 在线 | .. | 10秒前       |
|  agent-02 | device-02   | 离线 | .. | 5分钟前      |
+--------------------------------------------------+
```

### 页面3 - 任务分发页（路由: `/dispatch`）

**负责人**: 前端开发者 C

**功能描述**:
- 选择目标 Agent：下拉选择单个 Agent 或选择"全部 Agent"广播
- 输入任务 ID（可自动生成 UUID）
- 输入命令（支持多行）
- 设置超时时间（默认 30 秒）
- 发送任务按钮
- 发送后自动轮询任务结果，实时展示 stdout/stderr
- 支持下发控制指令（refresh_token、shutdown、reload_config、cancel_task、cancel）

**UI 布局**:
```
+--------------------------------------------------+
|  目标 Agent: [下拉选择]   任务ID: [自动生成/手动]   |
|  命令: [多行输入框]                                 |
|  超时: [30] 秒          [发送任务]                  |
+--------------------------------------------------+
|  执行结果                                          |
|  状态: running -> finished                         |
|  Exit Code: 0                                     |
|  Stdout: ...                                      |
|  Stderr: ...                                      |
+--------------------------------------------------+
```

### 页面4 - 任务历史页（路由: `/tasks`）

**负责人**: 前端开发者 D

**功能描述**:
- 任务列表表格：task_id、agent_id、status、exit_code、started_at、finished_at
- 支持分页（每页 20 条）
- 支持按 agent_id 筛选
- 支持按 status 筛选（pending、running、finished、failed）
- 点击任务行展开详情：显示完整 stdout、stderr（代码块样式）
- 状态标签颜色：pending=蓝色、running=橙色、finished=绿色、failed=红色

**UI 布局**:
```
+----------------------------------------------------------+
|  筛选: Agent [下拉]  状态 [下拉]   [搜索 task_id]          |
+----------------------------------------------------------+
|  task_id | agent_id | status   | exit_code | 完成时间     |
|  task-01 | agent-01 | finished | 0         | 10:30:00    |
|  > stdout: ls -la output...                               |
|  > stderr: (empty)                                        |
|  task-02 | agent-02 | failed   | 1         | 10:31:00    |
+----------------------------------------------------------+
|  < 1 2 3 ... >                                            |
+----------------------------------------------------------+
```

### 页面5 - 系统监控页（路由: `/monitor`）

**负责人**: 前端开发者 E

**功能描述**:
- 嵌入 Grafana Dashboard（iframe，地址: `http://<host>:3002`）
- Agent 心跳状态面板：列出所有 Agent 的最后心跳时间和在线状态
- 系统健康检查：定时轮询 `/healthz`，展示服务可用性
- Prometheus 指标概览：从 `/metrics` 获取聚合指标（CPU、内存、磁盘）

**UI 布局**:
```
+----------------------------------------------------------+
|  [健康检查] 服务状态: ok   上次检查: 3秒前                  |
+----------------------------------------------------------+
|  Agent 心跳状态                                           |
|  agent_id | device_code | 状态 | CPU | 内存 | 最后心跳    |
|  agent-01 | device-01   | 在线 | 23% | 45%  | 5秒前      |
+----------------------------------------------------------+
|  Grafana Dashboard (iframe)                               |
|  [嵌入的 Grafana 面板]                                    |
+----------------------------------------------------------+
```

## 3. API 对接表

### 3.1 现有 API

| 端点 | 方法 | 认证 | 说明 | 使用页面 |
|------|------|------|------|----------|
| `/healthz` | GET | 无 | 健康检查 | Dashboard、监控页 |
| `/metrics` | GET | 无 | Prometheus 指标聚合 | 监控页 |
| `/api/v1/debug/state` | GET | AdminAuth | 内存状态统计（agent数、task数） | Dashboard |
| `/api/v1/debug/dispatch/task` | POST | AdminAuth | 下发任务 | 任务分发页 |
| `/api/v1/debug/dispatch/control` | POST | AdminAuth | 下发控制指令 | 任务分发页 |
| `/api/v1/debug/task/:task_id` | GET | AdminAuth | 查询单个任务结果 | 任务分发页、任务历史页 |

### 3.2 认证方式说明

所有 debug 路由使用 `AdminAuth` 中间件，前端请求时需携带：

```
Header: X-Register-Token: <admin_token>
```

`admin_token` 值从环境变量 `VITE_ADMIN_TOKEN` 读取，开发环境默认为 `dev-register-token`。

### 3.3 响应格式

所有 API 统一返回 `Envelope` 格式：

```json
{
  "code": 0,
  "message": "ok",
  "request_id": "req-xxxxxxxxxxxx",
  "data": { ... }
}
```

- `code = 0` 表示成功，非 0 表示失败
- 前端统一在 Axios 拦截器中处理错误

## 4. 后端需要补充的 API

现有 API 分析：
- `/api/v1/debug/state` 只返回 agent 数量和 task 数量的统计数字，不返回列表详情
- `/api/v1/debug/task/:task_id` 只能按单个 task_id 查询，无法列表查询
- 没有 Agent 列表 API

前端需要后端新增以下 2 个接口：

### 4.1 GET /api/v1/debug/agents - Agent 列表

**认证**: AdminAuth（`X-Register-Token`）

**查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| status | string | 否 | 筛选状态：online/offline，不传返回全部 |
| search | string | 否 | 按 device_code 模糊搜索 |

**响应 data**:

```json
[
  {
    "agent_id": "agent-01",
    "device_code": "device-01",
    "agent_version": "1.0.0",
    "status": "online",
    "hostname": "ubuntu-01",
    "os": "linux",
    "arch": "amd64",
    "ip": "172.20.0.5",
    "labels": {},
    "capabilities": [],
    "heartbeat_interval": 30,
    "last_heartbeat_at": "2026-02-12T10:30:00Z",
    "created_at": "2026-02-10T08:00:00Z"
  }
]
```

**数据来源**: 查询 `agents` 表，在线判断逻辑由后端处理（`last_heartbeat_at` 距今超过 `heartbeat_interval * 3` 秒则标记为 offline）。

### 4.2 GET /api/v1/debug/tasks - 任务列表

**认证**: AdminAuth（`X-Register-Token`）

**查询参数**:

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| page | int | 否 | 页码，默认 1 |
| page_size | int | 否 | 每页条数，默认 20，最大 100 |
| agent_id | string | 否 | 按 agent_id 筛选 |
| status | string | 否 | 按状态筛选：pending/running/finished/failed |

**响应 data**:

```json
{
  "total": 150,
  "page": 1,
  "page_size": 20,
  "items": [
    {
      "task_id": "task-001",
      "agent_id": "agent-01",
      "status": "finished",
      "exit_code": 0,
      "stdout": "...",
      "stderr": "",
      "truncated": false,
      "started_at": "2026-02-12T10:30:00Z",
      "finished_at": "2026-02-12T10:30:05Z",
      "created_at": "2026-02-12T10:29:58Z"
    }
  ]
}
```

**数据来源**: `tasks` 表 LEFT JOIN `task_results` 表，按 `created_at DESC` 排序。

## 5. 组件结构

### 5.1 公共组件

| 组件 | 文件 | 说明 |
|------|------|------|
| AppLayout | `src/layouts/AppLayout.vue` | 整体布局：左侧菜单 + 顶部 Header + 内容区 |
| Sidebar | `src/layouts/Sidebar.vue` | 左侧导航菜单（Ant Design Vue Menu） |
| Header | `src/layouts/Header.vue` | 顶部栏：Logo + 系统名称 |
| StatusTag | `src/components/StatusTag.vue` | 状态标签（在线/离线、任务状态），统一颜色映射 |
| CodeBlock | `src/components/CodeBlock.vue` | 代码块展示组件，用于 stdout/stderr 显示 |

### 5.2 各页面组件

**Dashboard 总览页** (`src/pages/Dashboard/`):
- `index.vue` - 页面主组件
- `StatCards.vue` - 统计卡片区域
- `HealthStatus.vue` - 健康检查状态
- `RecentTasks.vue` - 最近任务列表

**Agent 管理页** (`src/pages/Agents/`):
- `index.vue` - 页面主组件
- `AgentTable.vue` - Agent 列表表格
- `AgentDetail.vue` - Agent 详情展开面板

**任务分发页** (`src/pages/Dispatch/`):
- `index.vue` - 页面主组件
- `TaskForm.vue` - 任务表单（选择 Agent、输入命令）
- `ControlForm.vue` - 控制指令表单
- `TaskResult.vue` - 执行结果展示

**任务历史页** (`src/pages/Tasks/`):
- `index.vue` - 页面主组件
- `TaskTable.vue` - 任务列表表格
- `TaskDetail.vue` - 任务详情展开面板

**系统监控页** (`src/pages/Monitor/`):
- `index.vue` - 页面主组件
- `HeartbeatPanel.vue` - Agent 心跳状态面板
- `GrafanaEmbed.vue` - Grafana iframe 嵌入

## 6. 任务分配表

| 角色 | 负责人 | 任务内容 | 依赖 |
|------|--------|----------|------|
| 前端 A | 开发者 A | Dashboard 总览页 | 后端 API: `/debug/state`、`/debug/tasks`、`/healthz` |
| 前端 B | 开发者 B | Agent 管理页 | 后端新增 API: `/debug/agents` |
| 前端 C | 开发者 C | 任务分发页 | 后端 API: `/debug/dispatch/task`、`/debug/dispatch/control`、`/debug/task/:task_id`、`/debug/agents` |
| 前端 D | 开发者 D | 任务历史页 | 后端新增 API: `/debug/tasks` |
| 前端 E | 开发者 E | 系统监控页 | 后端 API: `/healthz`、`/metrics`、`/debug/agents`；Grafana 地址配置 |
| 后端 F | 开发者 F | 补充 2 个新 API（`/debug/agents`、`/debug/tasks`） | 无 |
| 测试 G | 测试工程师 | E2E 测试 + 各页面功能测试 | 全部页面和 API 完成后 |

### 6.1 开发顺序建议

1. **后端 F** 先完成 2 个新 API（`/debug/agents` 和 `/debug/tasks`）
2. **前端 A** 可先行开发（仅依赖现有 API），同时搭建项目脚手架和公共组件
3. **前端 B/C/D/E** 在后端 API 就绪后并行开发
4. **测试 G** 在各页面基本完成后介入

## 7. 项目初始化

### 7.1 目录结构

```
frontend/
├── index.html
├── package.json
├── tsconfig.json
├── vite.config.ts
├── env.d.ts                  # Vite 环境变量类型声明
├── .env.development          # VITE_ADMIN_TOKEN=dev-register-token
├── .env.production
├── public/
└── src/
    ├── main.ts               # 入口
    ├── App.vue               # 根组件
    ├── router/
    │   └── index.ts          # Vue Router 路由配置
    ├── api/
    │   ├── client.ts         # Axios 实例 + 拦截器
    │   └── types.ts          # API 请求/响应 TypeScript 类型
    ├── layouts/
    │   ├── AppLayout.vue     # 整体布局
    │   ├── Sidebar.vue       # 左侧导航
    │   └── Header.vue        # 顶部栏
    ├── components/
    │   ├── StatusTag.vue     # 状态标签
    │   └── CodeBlock.vue     # 代码块展示
    └── pages/
        ├── Dashboard/
        ├── Agents/
        ├── Dispatch/
        ├── Tasks/
        └── Monitor/
```

### 7.2 package.json 核心依赖

```json
{
  "name": "remoteagent-frontend",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "vue": "^3.4.0",
    "vue-router": "^4.4.0",
    "ant-design-vue": "^4.2.0",
    "@ant-design/icons-vue": "^7.0.0",
    "axios": "^1.7.0",
    "dayjs": "^1.11.0"
  },
  "devDependencies": {
    "@vitejs/plugin-vue": "^5.1.0",
    "vue-tsc": "^2.1.0",
    "typescript": "^5.6.0",
    "vite": "^5.4.0"
  }
}
```

### 7.3 路由配置

```ts
// src/router/index.ts
import { createRouter, createWebHistory } from 'vue-router';
import AppLayout from '../layouts/AppLayout.vue';

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: AppLayout,
      children: [
        { path: '', name: 'Dashboard', component: () => import('../pages/Dashboard/index.vue') },
        { path: 'agents', name: 'Agents', component: () => import('../pages/Agents/index.vue') },
        { path: 'dispatch', name: 'Dispatch', component: () => import('../pages/Dispatch/index.vue') },
        { path: 'tasks', name: 'Tasks', component: () => import('../pages/Tasks/index.vue') },
        { path: 'monitor', name: 'Monitor', component: () => import('../pages/Monitor/index.vue') },
      ],
    },
  ],
});

export default router;
```

### 7.4 API 客户端封装

```ts
// src/api/client.ts
import axios from 'axios';
import { message } from 'ant-design-vue';

const client = axios.create({
  baseURL: '',
  timeout: 30000,
});

// 请求拦截器：注入 Admin Token
client.interceptors.request.use((config) => {
  const token = import.meta.env.VITE_ADMIN_TOKEN || 'dev-register-token';
  config.headers['X-Register-Token'] = token;
  return config;
});

// 响应拦截器：统一错误处理
client.interceptors.response.use(
  (response) => {
    const { code, message: msg } = response.data;
    if (code !== 0) {
      message.error(msg || '请求失败');
      return Promise.reject(new Error(msg));
    }
    return response.data;
  },
  (error) => {
    message.error(error.message || '网络错误');
    return Promise.reject(error);
  }
);

export default client;
```

### 7.5 Vite 配置

```ts
// vite.config.ts
import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:40001',
        changeOrigin: true,
      },
      '/healthz': {
        target: 'http://localhost:40001',
        changeOrigin: true,
      },
      '/metrics': {
        target: 'http://localhost:40001',
        changeOrigin: true,
      },
    },
  },
});
```

### 7.6 TypeScript 类型定义

```ts
// src/api/types.ts

// 通用响应包装
export interface Envelope<T = any> {
  code: number;
  message: string;
  request_id: string;
  data: T;
}

// 健康检查
export interface HealthResp {
  service: string;
  status: string;
  timestamp: number;
}

// Agent 信息
export interface AgentInfo {
  agent_id: string;
  device_code: string;
  agent_version: string;
  status: 'online' | 'offline';
  hostname: string;
  os: string;
  arch: string;
  ip: string;
  labels: Record<string, string>;
  capabilities: string[];
  heartbeat_interval: number;
  last_heartbeat_at: string;
  created_at: string;
}

// 任务信息
export interface TaskInfo {
  task_id: string;
  agent_id: string;
  status: 'pending' | 'running' | 'finished' | 'failed';
  exit_code: number;
  stdout: string;
  stderr: string;
  truncated: boolean;
  started_at: string | null;
  finished_at: string | null;
  created_at: string;
}

// 任务列表分页响应
export interface TaskListResp {
  total: number;
  page: number;
  page_size: number;
  items: TaskInfo[];
}

// 任务下发请求
export interface DispatchTaskReq {
  agent_id: string;
  task_id: string;
  command: string;
  timeout: number;
}

// 控制指令请求
export interface DispatchControlReq {
  agent_id: string;
  action: 'refresh_token' | 'shutdown' | 'reload_config' | 'cancel_task' | 'cancel';
  payload?: Record<string, any>;
}

// 系统状态
export interface SystemState {
  agents: number;
  tasks: number;
}
```
