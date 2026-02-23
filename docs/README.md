# Docs 文档目录

本目录包含 RemoteAgent 项目的所有文档，包括设计文档、API 文档、测试文档等。

## 目录结构

```
docs/
├── README.md                           # 本文件（文档导航）
│
├── # ========== 架构与设计 ==========
├── architecture.md                     # 系统架构设计 - 项目总体架构说明
├── agent-server-design-phase1.md       # Phase 1 设计 - Agent/Server 基础架构设计
├── agent-server-design-phase2-task-scheduler.md  # Phase 2 任务调度设计
├── server-preempt-handler-store-design.md        # 抢占处理器设计
├── frontend-design.md                  # 前端设计 - Vue3 前端架构与页面规划
├── phase2-implementation-plan.md       # Phase 2 实施计划 - 分布式任务调度实施
├── plan.md                             # 定向分发+批量API实现计划
│
├── # ========== API 文档 ==========
├── api-overview.md                     # API 接口概览 - RESTful API 说明
├── openapi-server.yaml                 # Server OpenAPI 规范
├── openapi-agent-local.yaml            # Agent OpenAPI 规范
│
├── # ========== 部署与开发 ==========
├── deployment.md                       # 部署说明 - Docker Compose 部署指南
├── local-dev.md                        # 本地开发指南 - 无 Docker 开发环境搭建
│
├── # ========== 测试文档 ==========
├── testing.md                          # 测试指南 - Server 集成测试文档
├── test-plan.md                        # 测试计划 - 全面测试计划（331个用例）
├── test-report.md                      # 测试报告 - 测试结果报告（通过率100%）
├── acceptance-test-manual.md           # 验收测试手册 - 客户验收测试步骤
├── codex_集成测试用例.md               # Codex 集成测试用例
│
├── # ========== 功能模块设计 ==========
├── DocCenter-实施方案.md               # 文档中心实施方案
├── 文档中心.md                         # 文档中心开发任务分解
├── PLAN-secure-distribute.md           # 安全分发实施规划
├── secure-distribute-guide.md          # 安全分发部署与运维指南
├── design/
│   └── customer-management.md          # 客户管理功能设计
│
├── # ========== 数据库 ==========
├── sql/                                # 数据库脚本
│   ├── 0000_complete_init.sql          # 完整初始化脚本（包含所有表）
│   ├── 0001_init.sql                   # 初始迁移 - agents/tasks 基础表
│   ├── 0002_add_external_ip.sql        # 添加 external_ip 字段
│   ├── 0003_phase2_task_scheduler.sql  # Phase 2 任务调度表
│   ├── 0004_hosts.sql                  # Host 管理表
│   ├── 0005_customers.sql              # 客户管理表
│   ├── 0006_distributions.sql          # 安全分发表
│   ├── 0007_documents.sql              # 文档中心表
│   ├── 0008_target_agent.sql           # 目标 Agent 定向分发
│   ├── 0009_release_note_drafts.sql    # 发布说明草稿表
│   └── 0010_fix_task_status_constraint.sql  # 修复任务状态约束
│
└── # ========== 其他资源 ==========
    ├── frontend.png                    # 前端架构图
    ├── 分发中心.html                   # 分发中心原型页面
    └── 分发.html                       # 分发页面原型
```

## 文档分类详解

### 1. 架构与设计文档

| 文档 | 内容 | 适用读者 |
|------|------|----------|
| [architecture.md](architecture.md) | 项目总体架构、技术栈、目录结构 | 全体开发者 |
| [agent-server-design-phase1.md](agent-server-design-phase1.md) | Phase 1 核心设计：注册、心跳、轮询、执行闭环 | 后端开发者 |
| [agent-server-design-phase2-task-scheduler.md](agent-server-design-phase2-task-scheduler.md) | Phase 2 任务调度：优先级队列、租约、抢占、重试 | 后端开发者 |
| [server-preempt-handler-store-design.md](server-preempt-handler-store-design.md) | 抢占接口设计：preempt/preempt-ack 协议 | 后端开发者 |
| [frontend-design.md](frontend-design.md) | 前端架构、页面规划、组件设计 | 前端开发者 |
| [phase2-implementation-plan.md](phase2-implementation-plan.md) | Phase 2 编码规范、团队分工、实施步骤 | 项目负责人 |
| [plan.md](plan.md) | 定向分发+批量API实现计划 | 后端开发者 |

### 2. API 文档

| 文档 | 内容 | 使用方式 |
|------|------|----------|
| [api-overview.md](api-overview.md) | API 概览：认证机制、接口列表、响应格式 | 快速查阅 |
| [openapi-server.yaml](openapi-server.yaml) | Server OpenAPI 3.0 规范 | Swagger UI |
| [openapi-agent-local.yaml](openapi-agent-local.yaml) | Agent 本地 API 规范 | 调试 Agent |

**Swagger UI 访问**: http://localhost:40001/swagger/index.html

### 3. 部署与开发

| 文档 | 内容 | 场景 |
|------|------|------|
| [deployment.md](deployment.md) | Docker Compose 部署说明 | 生产部署 |
| [local-dev.md](local-dev.md) | 本地无 Docker 开发指南 | 本地开发 |

**注意**: 最新的部署文档请查看 [deploy/README.md](../deploy/README.md)

### 4. 测试文档

| 文档 | 内容 | 测试类型 |
|------|------|----------|
| [testing.md](testing.md) | Server 测试体系说明 | 单元/集成测试 |
| [test-plan.md](test-plan.md) | 全面测试计划（331个用例） | 测试规划 |
| [test-report.md](test-report.md) | 测试结果报告（通过率100%） | 测试总结 |
| [acceptance-test-manual.md](acceptance-test-manual.md) | 客户验收测试手册 | 验收测试 |
| [codex_集成测试用例.md](codex_集成测试用例.md) | Codex 风格集成测试用例 | 集成测试 |

### 5. 功能模块设计

| 文档 | 内容 | 模块 |
|------|------|------|
| [DocCenter-实施方案.md](DocCenter-实施方案.md) | 文档中心技术选型、架构设计 | 文档中心 |
| [文档中心.md](文档中心.md) | 文档中心开发任务分解、人员分工 | 文档中心 |
| [PLAN-secure-distribute.md](PLAN-secure-distribute.md) | 安全分发实施规划 | 安全分发 |
| [secure-distribute-guide.md](secure-distribute-guide.md) | GPG 密钥管理、部署运维指南 | 安全分发 |
| [design/customer-management.md](design/customer-management.md) | 客户管理功能设计 | 客户管理 |

### 6. 数据库脚本

| 脚本 | 说明 | 依赖 |
|------|------|------|
| [sql/0000_complete_init.sql](sql/0000_complete_init.sql) | **完整初始化脚本**（推荐） | 无 |
| [sql/0001_init.sql](sql/0001_init.sql) | 基础表：agents, tasks, task_events | 无 |
| [sql/0002_add_external_ip.sql](sql/0002_add_external_ip.sql) | 添加 external_ip 字段 | 0001 |
| [sql/0003_phase2_task_scheduler.sql](sql/0003_phase2_task_scheduler.sql) | Phase 2 任务调度表 | 0001 |
| [sql/0004_hosts.sql](sql/0004_hosts.sql) | Host 管理表 | 无 |
| [sql/0005_customers.sql](sql/0005_customers.sql) | 客户管理表 | 0004 |
| [sql/0006_distributions.sql](sql/0006_distributions.sql) | 安全分发表 | 无 |
| [sql/0007_documents.sql](sql/0007_documents.sql) | 文档中心表 | 无 |
| [sql/0007_release_note_drafts.sql](sql/0007_release_note_drafts.sql) | 发布说明草稿表 | 无 |
| [sql/0008_target_agent.sql](sql/0008_target_agent.sql) | 目标 Agent 定向分发 | 0003 |

**推荐使用**: `0000_complete_init.sql` 包含所有表结构，适合新环境初始化。

## 快速导航

### 按角色查找

| 角色 | 推荐阅读 |
|------|----------|
| **新成员入门** | [architecture.md](architecture.md) → [local-dev.md](local-dev.md) → [api-overview.md](api-overview.md) |
| **后端开发** | [agent-server-design-phase1.md](agent-server-design-phase1.md) → [agent-server-design-phase2-task-scheduler.md](agent-server-design-phase2-task-scheduler.md) |
| **前端开发** | [frontend-design.md](frontend-design.md) → [api-overview.md](api-overview.md) |
| **测试工程师** | [testing.md](testing.md) → [test-plan.md](test-plan.md) → [acceptance-test-manual.md](acceptance-test-manual.md) |
| **运维部署** | [deployment.md](deployment.md) → [secure-distribute-guide.md](secure-distribute-guide.md) |
| **项目经理** | [architecture.md](architecture.md) → [phase2-implementation-plan.md](phase2-implementation-plan.md) → [test-report.md](test-report.md) |

### 按主题查找

| 主题 | 相关文档 |
|------|----------|
| **系统架构** | [architecture.md](architecture.md) |
| **任务调度** | [agent-server-design-phase2-task-scheduler.md](agent-server-design-phase2-task-scheduler.md), [phase2-implementation-plan.md](phase2-implementation-plan.md) |
| **API 接口** | [api-overview.md](api-overview.md), [openapi-server.yaml](openapi-server.yaml) |
| **数据库** | [sql/0000_complete_init.sql](sql/0000_complete_init.sql) |
| **部署** | [deployment.md](deployment.md), [../deploy/README.md](../deploy/README.md) |
| **测试** | [testing.md](testing.md), [test-plan.md](test-plan.md), [test-report.md](test-report.md) |
| **文档中心** | [DocCenter-实施方案.md](DocCenter-实施方案.md), [文档中心.md](文档中心.md) |
| **安全分发** | [secure-distribute-guide.md](secure-distribute-guide.md), [PLAN-secure-distribute.md](PLAN-secure-distribute.md) |
| **客户管理** | [design/customer-management.md](design/customer-management.md) |

## 文档规范

1. **设计文档**: 使用 Markdown 格式，存放在根目录或 `design/` 目录
2. **API 文档**: 使用 OpenAPI 3.0 规范，YAML 格式
3. **数据库脚本**: 使用 `000X_description.sql` 命名格式，按顺序执行
4. **图片资源**: 放在 docs 根目录，使用相对路径引用
5. **原型页面**: HTML 文件，用于前端原型展示

## 更新日志

- **2024-02-14**: 整理文档目录结构，添加详细分类和导航
- **2024-02-13**: 添加文档中心相关文档
- **2024-02-12**: 添加 Phase 2 设计文档、客户管理设计
- **2024-02-11**: 添加测试计划、测试报告、验收测试手册
- **2024-02-10**: 初始文档结构，添加架构设计、API 文档
