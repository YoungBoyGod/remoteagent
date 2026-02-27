# Distribution S3 Browse Integration Design

**Date:** 2026-02-27
**Scope:** `frontend/src/pages/Distribution/index.vue`, `frontend/src/api/*`, `server/internal/controller/distribution.go`, `server/internal/service/*`, `server/internal/api/distribution_types.go`

---

## 1. Goal

在“新建加密任务”的 S3 模式下，改为真实对接后端 S3 列表接口，支持按前缀浏览对象并选择文件创建加密任务。
约束已确认：
- 固定 bucket（服务端配置）
- 前端输入 prefix 浏览
- 提交时仅传 `s3_key`

---

## 2. Chosen Approach

采用 **方案 A（后端代理 S3 List）**：
- 新增后端只读 API，前端通过该 API 获取 S3 对象列表。
- 不在前端暴露 S3 凭证，不做前端直连 S3。
- 保持现有 Distribution 创建流程，增量扩展 source 类型与 `s3_key` 字段。

---

## 3. Architecture

### 3.1 Backend

新增接口：`GET /api/v1/distributions/s3-objects`

**Request Query**
- `prefix` (optional): S3 前缀（如 `releases/2026/`）
- `page_size` (optional): 默认 50，最大 200
- `continuation_token` (optional): 翻页 token

**Response**
- `items`: `[{ key, size, last_modified }]`
- `next_token`: string
- `has_more`: bool

**Security Boundary**
- 复用现有 AdminAuth。
- bucket 固定为服务端 `S3_BUCKET`，不允许前端传 bucket。
- 服务端校验 prefix（白名单规则，例如仅允许 `releases/`）。
- 响应仅返回对象元数据，不返回下载 URL、access key 或 endpoint 细节。

### 3.2 Frontend

在 `Distribution/index.vue` 的 S3 区域：
- 保留 prefix 输入框与“浏览”按钮。
- 点击“浏览”调用后端 `s3-objects` 接口。
- 用返回结果替换当前静态 `s3Files` 列表。
- 列表项展示：`basename(key)`、完整 key（次级）、size、last_modified。
- 选中文件后提交创建任务，payload 携带 `source_type: "s3"` 与 `s3_key`。

### 3.3 Create Distribution Payload

为 `POST /api/v1/distributions` 扩展字段：
- `source_type: "s3" | "local"`（建议新增，便于后端分支处理）
- `s3_key: string`（当 `source_type=s3` 必填）

保留现有本地上传路径，不破坏已上线流程。

---

## 4. Data Flow

1. 用户选择“S3 存储”并输入 prefix。
2. 前端调用 `GET /api/v1/distributions/s3-objects?prefix=...`。
3. 后端按固定 bucket + prefix list objects 并返回分页结果。
4. 用户在列表选择一个对象。
5. 前端提交 `POST /api/v1/distributions`，携带 `source_type=s3` + `s3_key`。
6. 后端创建 distribution 并派发后续加密流程（延续现有任务编排逻辑）。

---

## 5. Error Handling & UX Rules

1. **List 失败**
   - 前端提示：`S3 文件列表加载失败`
   - 显示空态 + 重试入口
   - 保留 prefix 输入值

2. **prefix 非法**
   - 前端做轻校验（长度、空白）
   - 后端做强校验（白名单规则）
   - 返回 400 + 明确错误信息

3. **列表为空**
   - 展示空态文案：当前前缀无文件
   - 禁止提交按钮

4. **创建任务前对象消失**
   - 后端可做 `HeadObject` 二次校验
   - 对象不存在返回业务错误，前端提示重新浏览

---

## 6. Testing Strategy

### 6.1 Backend
- Controller 参数校验测试：`prefix/page_size/continuation_token`
- Service list 成功/异常测试
- prefix 白名单拦截测试

### 6.2 Frontend
- 浏览成功渲染列表
- 浏览失败错误提示
- 未选择文件时禁用提交
- 选择后提交 payload 包含 `s3_key`

### 6.3 Regression
- 本地上传流程保持可用
- 分发队列、发布说明、分发记录页面行为不回归

---

## 7. Incremental Rollout Order

1. 后端先实现并开放 `s3-objects` 接口（curl 可验）。
2. 前端接入浏览接口，替换静态 S3 列表。
3. 扩展 create distribution payload（`source_type/s3_key`）并联调。
4. 完成后执行构建与核心回归测试。

---

## 8. Non-Goals

- 不实现前端直连 S3（STS/CORS）
- 不做 S3 对象本地缓存同步任务
- 不新增多 bucket 切换功能

---

## 9. Decision Summary

本设计满足当前业务目标与安全边界：
- 用户可在页面直接浏览 S3 文件并选择创建任务。
- 不暴露 S3 凭证，符合现有后端统一鉴权模型。
- 通过最小增量改造落地，保留本地上传能力并降低回归风险。
