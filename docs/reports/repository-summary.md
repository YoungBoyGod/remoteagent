# RemoteAgent 代码仓库总结

## 📦 项目概览

**RemoteAgent** 是一个分布式远程命令执行与设备管理平台，采用 **Monorepo** 架构，包含三个核心组件：

- **Server** (Go/Gin) - 控制面 API 服务
- **Agent** (Go) - 设备端运行时
- **Frontend** (Vue 3 + Element Plus) - 管理面板

## 🏗️ 技术栈

| 组件 | 技术栈 |
|------|--------|
| **后端 Server** | Go 1.24, Gin, PostgreSQL 16, Redis, MeiliSearch, MinIO |
| **Agent** | Go 1.24, SQLite (本地存储) |
| **前端** | Vue 3, TypeScript, Vite, Element Plus, Tiptap (富文本) |
| **监控** | Prometheus + Grafana |
| **部署** | Docker Compose |

## 📁 目录结构

```
remoteagent/
├── src/
│   ├── server/          # 控制面 API
│   │   ├── cmd/server/main.go
│   │   ├── internal/
│   │   │   ├── app/         # 应用启动
│   │   │   ├── config/      # 配置加载
│   │   │   ├── router/      # 路由注册
│   │   │   ├── controller/  # HTTP 处理层 (19个文件)
│   │   │   ├── service/     # 业务逻辑层 (15个文件)
│   │   │   ├── store/       # 数据持久化层 (9个文件)
│   │   │   ├── model/       # 数据模型
│   │   │   └── api/         # 请求/响应类型
│   │   └── frontend/        # 内嵌前端静态资源
│   ├── agent/           # 设备端运行时
│   │   ├── cmd/agent/main.go
│   │   └── internal/
│   │       ├── config/      # 配置加载
│   │       ├── runtime/     # 核心运行时 (25个文件)
│   │       ├── logging/     # 日志初始化
│   │       └── observability/ # Prometheus 指标
│   └── frontend/        # Vue 3 管理面板
│       ├── src/
│       │   ├── api/         # API 客户端
│       │   ├── components/  # 通用组件
│       │   ├── layouts/     # 布局
│       │   ├── pages/       # 页面 (15+个页面)
│       │   ├── router/      # 路由配置
│       │   └── stores/      # Pinia 状态管理
│       └── package.json
├── infra/               # 基础设施 (PostgreSQL/Redis/MeiliSearch/MinIO/Prometheus/Grafana)
├── deploy/              # 生产部署配置
├── docs/                # 文档与 SQL 脚本
├── scripts/             # 开发脚本
└── Makefile             # 构建与开发命令
```

## 🔄 核心架构

### Server 分层架构

```
HTTP Request
    ↓
[Controller]  ← 鉴权、参数解析、响应封装
    ↓
[Service]     ← 业务逻辑、内存状态管理
    ↓
[Store]       ← PostgreSQL 持久化
    ↓
[Model]       ← 数据实体定义
```

### Agent 状态机

```
INIT → REGISTERING → RUNNING → DRAINING → STOPPED
                         ↓
                    AUTH_EXPIRED → REGISTERING (重注册)
```

## 🔑 核心功能

### 1. **Agent 生命周期管理**

- 注册 (`POST /api/v1/agent/register`)
- 心跳 (`POST /api/v1/agent/heartbeat`)
- 轮询 (`GET /api/v1/agent/poll`)
- 优雅退出

### 2. **任务调度系统**

- **Phase 1**: Long Poll 任务派发
- **Phase 2**: Redis 优先级队列 + 任务认领机制
- 支持批量创建、优先级调整、取消任务
- 任务状态：`pending` → `leased` → `running` → `success/failed/canceled`

### 3. **多租户与设备管理**

- 客户（Customer）管理
- 主机（Host）管理
- 设备（Device）绑定

### 4. **分发系统**

- 文件上传到 MinIO
- 加密队列处理
- 版本发布管理

### 5. **文档中心**

- Markdown 富文本编辑器 (Tiptap)
- 文档分类管理
- 全文搜索 (MeiliSearch)
- 版本差异对比

### 6. **监控与日志**

- Prometheus 指标采集
- Grafana 仪表板
- 操作日志审计

## 🗄️ 数据库设计

**核心表** (PostgreSQL 16):

- `agents` - Agent 实例信息
- `tasks` - 任务当前状态
- `task_events` - 状态变更历史（幂等）
- `task_results` - 执行结果（stdout/stderr）
- `customers` - 客户信息
- `hosts` - 主机管理
- `distributions` - 分发记录
- `documents` - 文档内容

## 🧪 测试覆盖

- **单元测试**: 24 个 `_test.go` 文件
- **集成测试**: `integration_test/` 目录
- **场景测试**: `scenario_test/`
- **边界测试**: `boundary_test/`
- **并发测试**: `concurrent_test/`
- **黑盒测试**: `blackbox_test/`

## 🚀 构建与部署

### 开发环境

```bash
make infra-up          # 启动基础设施
make dev               # 启动开发环境
```

### 生产构建

```bash
make release           # 交叉编译 linux/amd64 + arm64
make release-embed     # 包含前端的完整构建
```

### 生产部署

```bash
docker compose -f deploy/docker-compose.prod.yml up -d
```

## 📊 代码统计

- **Go 代码**: 114 个 `.go` 文件
- **TypeScript**: 14 个 `.ts` 文件
- **Vue 组件**: 31 个 `.vue` 文件
- **SQL 脚本**: 13 个迁移文件
- **测试文件**: 24 个测试文件
- **文档**: 12 个 `.md` 文件

## 🎯 核心特性

✅ **幂等设计** - `event_id` 唯一约束保证重复上报安全  
✅ **优雅关闭** - Agent 支持 DRAINING 状态等待任务完成  
✅ **Token GC** - 定期清理过期令牌  
✅ **并发控制** - Agent 端支持最大并发任务数限制  
✅ **抢占机制** - 高优先级任务可抢占低优先级任务  
✅ **本地持久化** - Agent 断线重连后恢复任务状态  
✅ **统一日志** - Graylog + 本地文件双输出  
✅ **Swagger 文档** - 自动生成 API 文档  

## 📝 关键文档

- `docs/architecture.md` - 架构设计
- `docs/api-overview.md` - API 接口概览
- `docs/deployment.md` - 部署手册
- `docs/local-dev.md` - 本地开发指南

---

**总结**: 这是一个设计良好、功能完整的分布式远程执行平台，代码结构清晰，测试覆盖全面，具备生产级的可靠性和可维护性。
