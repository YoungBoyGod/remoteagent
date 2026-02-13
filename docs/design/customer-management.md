# 客户管理功能 — 设计文档

> 状态：已完成 ✅
> 创建时间：2026-02-12

## 一、功能概述

在现有 remoteagent 系统中新增「客户管理」模块：

1. 客户信息 CRUD（姓名、联系方式、公司、备注等）
2. 主机分配（将 hosts 表中的主机分配给客户，支持一客户多主机）
3. 操作日志（记录客户创建、主机分配/回收等关键操作的审计轨迹）
4. 前端页面（客户列表、主机分配管理、操作日志查看）

## 二、数据库设计

迁移文件：`docs/sql/0005_customers.sql`

### 2.1 customers 客户表

| 字段 | 类型 | 说明 |
|------|------|------|
| customer_id | TEXT PK | 客户ID，格式 cust-xxxx |
| name | TEXT NOT NULL | 客户名称 |
| email | TEXT | 邮箱 |
| phone | TEXT | 电话 |
| company | TEXT | 公司 |
| description | TEXT | 备注 |
| tags | JSONB | 标签数组 |
| status | TEXT | active / inactive |
| created_at | TIMESTAMPTZ | 创建时间 |
| updated_at | TIMESTAMPTZ | 更新时间 |

### 2.2 customer_hosts 客户-主机关联表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | SERIAL PK | 自增ID |
| customer_id | TEXT FK | 客户ID |
| host_id | TEXT FK | 主机ID |
| assigned_at | TIMESTAMPTZ | 分配时间 |
| note | TEXT | 分配备注 |
| UNIQUE(customer_id, host_id) | | 唯一约束 |

### 2.3 operation_logs 操作日志表

| 字段 | 类型 | 说明 |
|------|------|------|
| log_id | SERIAL PK | 自增ID |
| resource_type | TEXT | customer / host_assign |
| resource_id | TEXT | 资源ID |
| action | TEXT | create/update/delete/assign_host/unassign_host |
| operator | TEXT | 操作人 |
| detail | JSONB | 操作详情 |
| created_at | TIMESTAMPTZ | 操作时间 |

### 2.4 hosts 表变更

新增 `customer_id TEXT REFERENCES customers(customer_id)` 冗余字段。

**实现状态：** `[ ]` 待完成

---

## 三、后端 API

### 3.1 客户 CRUD

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/customers | 创建客户 |
| GET | /api/v1/customers | 客户列表（分页/搜索/状态筛选） |
| GET | /api/v1/customers/:id | 客户详情（含已分配主机） |
| PUT | /api/v1/customers/:id | 更新客户 |
| DELETE | /api/v1/customers/:id | 删除客户 |

### 3.2 主机分配

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/customers/:id/hosts | 分配主机 |
| DELETE | /api/v1/customers/:id/hosts/:host_id | 回收主机 |
| GET | /api/v1/customers/:id/hosts | 客户已分配主机列表 |

### 3.3 操作日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/operation-logs | 操作日志列表 |

**实现状态：** `[ ]` 待完成

---

## 四、后端代码结构

```
server/internal/
├── api/customer_types.go          # 请求/响应类型
├── controller/customer.go         # 客户 handler
├── controller/operation_log.go    # 日志 handler
├── service/customer.go            # 客户业务逻辑
├── service/operation_log.go       # 日志业务逻辑
├── store/customer.go              # 客户 DB 操作
├── store/operation_log.go         # 日志 DB 操作
└── router/router.go               # 路由注册（修改）
```

**实现状态：** `[ ]` 待完成

---

## 五、前端设计

### 5.1 新增页面

- `pages/Customers/index.vue` — 客户列表页
- `pages/OperationLogs/index.vue` — 操作日志页

### 5.2 路由

```
/customers        → 客户管理
/operation-logs   → 操作日志
```

### 5.3 客户列表页功能

- 搜索栏：按名称/公司/手机号搜索，状态筛选
- 表格列：客户名称、公司、联系方式、已分配主机数、状态、创建时间、操作
- 新建/编辑客户弹窗
- 分配主机弹窗：展示可用主机列表，支持勾选分配
- 已分配主机展开行或弹窗查看

### 5.4 操作日志页功能

- 按资源类型、时间范围筛选
- 表格展示操作记录

### 5.5 侧边栏

新增「客户管理」和「操作日志」菜单项。

**实现状态：** `[ ]` 待完成

---

## 六、团队分工

| 角色 | 成员 | 负责模块 | 状态 |
|------|------|----------|------|
| 架构师 | team-lead | 协调、API 评审、集成验证 | 已完成 |
| 后端工程师 A | backend-a | DB 迁移 + store 层 + service + controller + router | 已完成 |
| 后端工程师 D | backend-d | API 类型定义 | 已完成 |
| 前端工程师 A | frontend-a | 客户列表页 + API 封装 | 已完成 |
| 前端工程师 D | frontend-d | TS 类型/路由/侧边栏 + 操作日志页 + 主机分配组件 | 已完成 |
| OPS 工程师 | team-lead | 构建验证 | 已完成 |

## 七、开发顺序

1. DB 迁移脚本 + API 类型定义（backend-a, backend-d 并行）
2. store 层实现（backend-a，依赖步骤1）
3. service 层实现（backend-b，依赖步骤2）
4. controller + router（backend-c, backend-d，依赖步骤3）
5. 前端 TS 类型 + 路由/侧边栏（frontend-d，与后端并行）
6. 前端页面开发（frontend-a, frontend-b, frontend-c，依赖步骤5）
7. 集成验证（architect + ops）

---

## 八、完成记录

> 各成员完成后在此回填实现细节。

### 8.1 数据库迁移
- 文件：docs/sql/0005_customers.sql
- 执行状态：已创建

### 8.2 后端 API 类型
- 文件：server/internal/api/customer_types.go
- 类型数量：11（CustomerCreateRequest, CustomerUpdateRequest, CustomerItem, CustomerListRequest, CustomerListResponse, CustomerHostAssignRequest, CustomerHostItem, CustomerHostListResponse, OperationLogItem, OperationLogListRequest, OperationLogListResponse）

### 8.3 Store 层
- 文件：server/internal/store/customer.go, server/internal/store/operation_log.go
- 函数列表：genCustomerID, InsertCustomer, UpdateCustomer, DeleteCustomer, GetCustomer, ListCustomers, AssignHost, UnassignHost, ListCustomerHosts, InsertOperationLog, ListOperationLogs

### 8.4 Service 层
- 文件：server/internal/service/customer.go, server/internal/service/operation_log.go
- 函数列表：CreateCustomer, UpdateCustomer, DeleteCustomer, GetCustomer, ListCustomers, AssignHost, UnassignHost, ListCustomerHosts, ListOperationLogs, recordOpLog

### 8.5 Controller 层
- 文件：server/internal/controller/customer.go, server/internal/controller/operation_log.go
- Handler 列表：CreateCustomerHandler, ListCustomersHandler, GetCustomerHandler, UpdateCustomerHandler, DeleteCustomerHandler, AssignHostHandler, UnassignHostHandler, ListCustomerHostsHandler, ListOperationLogsHandler

### 8.6 Router 注册
- 新增路由数：10（customers CRUD 5 + 主机分配 3 + operation-logs 1 + 1 路由组）

### 8.7 前端 TS 类型
- 文件：frontend/src/api/types.ts
- 接口数量：8（CustomerItem, CustomerCreateReq, CustomerUpdateReq, CustomerListResp, CustomerHostAssignReq, CustomerHostItem, CustomerHostListResp, OperationLogItem, OperationLogListResp）

### 8.8 前端路由 + 侧边栏
- 新增路由：/customers (Customers), /operation-logs (OperationLogs)
- 侧边栏菜单：客户管理（User icon）、操作日志（Document icon）

### 8.9 客户列表页
- 文件：frontend/src/pages/Customers/index.vue, frontend/src/api/customer.ts
- 功能点：搜索（名称/公司/手机号）、状态筛选（active/inactive）、分页、新建/编辑客户弹窗（name必填校验）、删除确认、分配主机按钮（占位，待 frontend-b 实现）
- API 封装：listCustomers, getCustomer, createCustomer, updateCustomer, deleteCustomer, listCustomerHosts, assignHost, unassignHost

### 8.10 主机分配组件
- 文件：frontend/src/pages/Customers/index.vue（内嵌实现）
- 功能点：已分配主机列表（含回收按钮+二次确认）、可用主机下拉选择（filterable）、分配备注输入、分配/回收后自动刷新列表和host_count

### 8.11 操作日志页
- 文件：frontend/src/pages/OperationLogs/index.vue
- 状态：已完成
- 功能点：
  - 筛选栏：资源类型 el-select（customer/host_assign）、操作类型 el-select（create/update/delete/assign_host/unassign_host），clearable 支持重置
  - 数据表格：日志ID、资源类型（el-tag 按类型着色 primary/success）、资源ID、操作类型（el-tag 按动作着色 success/warning/danger/info）、操作人、操作详情（el-popover 点击展开格式化 JSON）、操作时间（dayjs 格式化）
  - 分页：el-pagination，支持 page/page_size 切换，layout 含 total/sizes/prev/pager/next/jumper
  - API：GET /api/v1/operation-logs?resource_type=&action=&page=&page_size=，通过 axios client 自动附加 X-Register-Token header
  - 路由：/operation-logs（router 已注册），侧边栏已添加入口（Document 图标）

### 8.12 构建验证
- 后端编译：`go build ./...` 通过，无错误
- 前端编译：`vue-tsc --noEmit` 通过，无 TypeScript 错误
- 新文件检查：全部存在（0005_customers.sql, customer_types.go, store/customer.go, service/customer.go, controller/customer.go, controller/operation_log.go, api/customer.ts, Customers/index.vue, OperationLogs/index.vue）
- Router 注册：/api/v1/customers（CRUD + 主机分配子路由）、/api/v1/operation-logs 已注册
- 前端路由：/customers、/operation-logs 已注册，侧边栏已添加菜单项
