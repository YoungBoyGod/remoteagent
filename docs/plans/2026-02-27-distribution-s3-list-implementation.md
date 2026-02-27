# Distribution S3 List Integration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 在分发中心“新建加密任务”的 S3 模式中，接入后端真实 S3 列表接口，支持按 prefix 浏览并选择对象，以 `s3_key` 提交加密任务。

**Architecture:** 后端在现有 `/api/v1/distributions` 路由组下新增 `GET /s3-objects`，由服务层统一做 prefix 白名单校验并调用 `storage.ListObjects`，再在服务层完成分页 token（基于 key）切片。前端替换静态 `s3Files` 为 API 数据源，保留本地上传路径，提交时对 S3 模式携带 `source_type=s3` 与 `s3_key`。整个实现遵循 @superpowers:test-driven-development 与 @superpowers:verification-before-completion。

**Tech Stack:** Go + Gin + sqlmock/integration tests、Vue 3 `<script setup>` + Element Plus、TypeScript、Axios。

---

### Task 1: 后端 S3 列表 API 契约与路由

**Files:**
- Modify: `server/internal/api/distribution_types.go`
- Modify: `server/internal/controller/distribution.go`
- Modify: `server/internal/router/router.go`
- Modify: `server/internal/controller/distribution_test.go`

**Step 1: Write the failing test**

在 `server/internal/controller/distribution_test.go` 新增：
```go
func TestListDistributionS3Objects(t *testing.T) {
  // 组装 fake service 返回 2 条 S3 对象
  // 请求 GET /api/v1/distributions/s3-objects?prefix=releases/2026/&page_size=50
  // 断言 200 + envelope.data.items 长度为 2
}

func TestListDistributionS3ObjectsInvalidPrefix(t *testing.T) {
  // 请求 prefix=../../etc
  // 断言 400
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./server/internal/controller -tags=integration -run "TestListDistributionS3Objects" -v
```
Expected: FAIL（缺少 handler/路由或响应字段）。

**Step 3: Write minimal implementation**

1. 在 `distribution_types.go` 增加请求/响应类型：
```go
type DistributionS3ListRequest struct {
  Prefix            string `form:"prefix"`
  PageSize          int    `form:"page_size"`
  ContinuationToken string `form:"continuation_token"`
}

type DistributionS3ObjectItem struct {
  Key          string `json:"key"`
  Size         int64  `json:"size"`
  LastModified int64  `json:"last_modified"`
}

type DistributionS3ListResponse struct {
  Items     []DistributionS3ObjectItem `json:"items"`
  NextToken string                     `json:"next_token,omitempty"`
  HasMore   bool                       `json:"has_more"`
}
```
2. 在 `distribution.go` 增加 `ListDistributionS3ObjectsHandler`：
   - `ShouldBindQuery`
   - 调 `svc.ListDistributionS3Objects(req)`
   - 错误返回 400/500，成功 `OK(c, resp)`。
3. 在 `router.go` 注册：
```go
dist.GET("/s3-objects", controller.ListDistributionS3ObjectsHandler(svc))
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./server/internal/controller -tags=integration -run "TestListDistributionS3Objects" -v
```
Expected: PASS。

**Step 5: Commit**

```bash
git add server/internal/api/distribution_types.go server/internal/controller/distribution.go server/internal/router/router.go server/internal/controller/distribution_test.go
git commit -m "feat: add distribution S3 object list endpoint"
```

---

### Task 2: 服务层实现 S3 列表、prefix 校验与分页 token

**Files:**
- Modify: `server/internal/service/distribution.go`
- (Optional tiny helper in same file) `server/internal/service/distribution.go`
- Modify: `server/internal/service/service.go`（仅当需注入配置时）
- Create: `server/internal/service/distribution_s3_test.go`

**Step 1: Write the failing test**

新增 `distribution_s3_test.go`：
```go
func TestListDistributionS3ObjectsPrefixWhitelist(t *testing.T) {
  // prefix = "../../etc" -> expect error
}

func TestListDistributionS3ObjectsPagination(t *testing.T) {
  // fake storage 返回 key: a,b,c
  // page_size=2 token="" -> 返回 a,b has_more=true next_token="b"
  // page_size=2 token="b" -> 返回 c has_more=false
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./server/internal/service -run "TestListDistributionS3Objects" -v
```
Expected: FAIL（方法未实现/行为不符）。

**Step 3: Write minimal implementation**

在 `distribution.go` 增加：
```go
func (s *Service) ListDistributionS3Objects(req api.DistributionS3ListRequest) (*api.DistributionS3ListResponse, error) {
  // 1) 校验 prefix：仅允许 "releases/" 开头
  // 2) 规范化 page_size：默认 50，最大 200
  // 3) 调 s.sto.ListObjects(ctx, req.Prefix)
  // 4) 过滤目录占位对象（以 / 结尾）
  // 5) 基于 continuation_token(上一页最后一个 key)切片
  // 6) 转换 last_modified 为 unix 秒
}
```

错误分层：
- prefix 不合法 -> `fmt.Errorf("invalid prefix")`
- storage 未配置 -> `fmt.Errorf("s3 storage not configured")`
- S3 调用失败 -> 原样包装 `fmt.Errorf("list s3 objects: %w", err)`

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./server/internal/service -run "TestListDistributionS3Objects" -v
```
Expected: PASS。

**Step 5: Commit**

```bash
git add server/internal/service/distribution.go server/internal/service/distribution_s3_test.go
git commit -m "feat: implement S3 object listing with prefix guard and token paging"
```

---

### Task 3: 扩展创建分发请求支持 source_type/s3_key

**Files:**
- Modify: `server/internal/api/distribution_types.go`
- Modify: `server/internal/service/distribution.go`
- Modify: `server/internal/controller/distribution_test.go`

**Step 1: Write the failing test**

在 `distribution_test.go` 增加：
```go
func TestCreateDistributionWithS3Source(t *testing.T) {
  // body: {source_type:"s3", s3_key:"releases/2026/a.zip", file_name:"releases/2026/a.zip"}
  // 断言 200
}

func TestCreateDistributionWithS3SourceMissingKey(t *testing.T) {
  // source_type=s3 且 s3_key 为空 -> 400
}
```

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./server/internal/controller -tags=integration -run "TestCreateDistributionWithS3Source" -v
```
Expected: FAIL。

**Step 3: Write minimal implementation**

1. `DistributionCreateRequest` 增加字段：
```go
SourceType string `json:"source_type"` // s3|local
S3Key      string `json:"s3_key"`
```
2. 在 `CreateDistribution` 中：
   - 当 `source_type == "s3"` 时校验 `s3_key` 非空且前缀合法；
   - 将 `file_name` 使用 `s3_key`（或保持两者一致），保证后续任务脚本拿到真实 S3 key。
3. 保持 local 现有逻辑不变（YAGNI）。

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./server/internal/controller -tags=integration -run "TestCreateDistributionWithS3Source" -v
```
Expected: PASS。

**Step 5: Commit**

```bash
git add server/internal/api/distribution_types.go server/internal/service/distribution.go server/internal/controller/distribution_test.go
git commit -m "feat: support s3 source fields for distribution creation"
```

---

### Task 4: 前端 API 与类型接入 S3 列表

**Files:**
- Modify: `frontend/src/api/types.ts`
- Modify: `frontend/src/api/distribution.ts`

**Step 1: Write the failing test**

用类型检查作为门禁（新增类型前应失败）：
```bash
npm --prefix frontend run build
```
Expected: 在引用新接口前会因缺少类型/方法失败（或下一步先加调用再触发失败）。

**Step 2: Run test to verify it fails**

Run:
```bash
npm --prefix frontend run build
```
Expected: FAIL（当 `index.vue` 已开始引用 listS3Objects 但 API 尚未定义）。

**Step 3: Write minimal implementation**

1. 在 `types.ts` 新增：
```ts
export interface DistributionS3ObjectItem {
  key: string
  size: number
  last_modified: number
}

export interface DistributionS3ListResp {
  items: DistributionS3ObjectItem[]
  next_token?: string
  has_more: boolean
}
```
2. 在 `distribution.ts` 新增：
```ts
export async function listDistributionS3Objects(params: {
  prefix?: string
  page_size?: number
  continuation_token?: string
}) {
  const resp = await client.get<Envelope<DistributionS3ListResp>>('/api/v1/distributions/s3-objects', { params })
  return resp.data.data
}
```
3. 扩展 `DistributionCreateReq`：
```ts
source_type?: 's3' | 'local'
s3_key?: string
```

**Step 4: Run test to verify it passes**

Run:
```bash
npm --prefix frontend run build
```
Expected: PASS（API 层类型通过）。

**Step 5: Commit**

```bash
git add frontend/src/api/types.ts frontend/src/api/distribution.ts
git commit -m "feat: add frontend API/types for distribution S3 object listing"
```

---

### Task 5: 分发页面接入真实 S3 列表与提交 s3_key

**Files:**
- Modify: `frontend/src/pages/Distribution/index.vue`

**Step 1: Write the failing test**

先写行为断言（关键字检查 + 构建门禁）：
```bash
grep -n "listDistributionS3Objects\|source_type\|s3_key" frontend/src/pages/Distribution/index.vue
npm --prefix frontend run build
```
Expected: grep 未命中、或构建因缺引用失败。

**Step 2: Run test to verify it fails**

Run:
```bash
grep -n "listDistributionS3Objects\|source_type\|s3_key" frontend/src/pages/Distribution/index.vue
```
Expected: FAIL（旧实现仍是静态数组）。

**Step 3: Write minimal implementation**

在 `index.vue`：
1. 移除静态 `s3Files`，改为 `s3FilesLoading/s3Files/s3NextToken/s3HasMore` 状态。
2. “浏览”按钮触发 `listDistributionS3Objects({ prefix: s3Path.value, page_size: 50 })`。
3. 文件展示使用后端返回 `key,size,last_modified`。
4. 选中逻辑改为按 `key` 选择。
5. 提交时分支：
```ts
source_type: sourceMode.value,
s3_key: sourceMode.value === 's3' ? selectedFile.value.name : undefined,
```
6. S3 模式不做本地 SHA-256 计算；local 模式保持原逻辑。

**Step 4: Run test to verify it passes**

Run:
```bash
npm --prefix frontend run build
```
Expected: PASS。

**Step 5: Commit**

```bash
git add frontend/src/pages/Distribution/index.vue
git commit -m "feat: wire distribution new-task page to backend S3 object listing"
```

---

### Task 6: 端到端验证与回归

**Files:**
- Modify if needed: `frontend/src/pages/Distribution/index.vue`
- Modify if needed: `server/internal/controller/distribution.go`
- Modify if needed: `server/internal/service/distribution.go`

**Step 1: Write the failing test**

统一回归命令（先跑一次，预期可能失败暴露遗漏）：
```bash
go test ./server/internal/service -run "TestListDistributionS3Objects" -v
go test ./server/internal/controller -tags=integration -run "TestListDistributionS3Objects|TestCreateDistributionWithS3Source" -v
npm --prefix frontend run build
```

**Step 2: Run test to verify it fails**

Run同上。Expected: 任一失败即继续最小修复。

**Step 3: Write minimal implementation**

仅修复上述命令暴露的问题（不扩展额外特性）。

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./server/internal/service -run "TestListDistributionS3Objects" -v
go test ./server/internal/controller -tags=integration -run "TestListDistributionS3Objects|TestCreateDistributionWithS3Source" -v
npm --prefix frontend run build
```
Expected: 全部 PASS。

**Step 5: Commit**

```bash
git add server/internal/api/distribution_types.go server/internal/controller/distribution.go server/internal/controller/distribution_test.go server/internal/service/distribution.go server/internal/service/distribution_s3_test.go server/internal/router/router.go frontend/src/api/types.ts frontend/src/api/distribution.ts frontend/src/pages/Distribution/index.vue
git commit -m "feat: integrate S3 object browser into distribution task creation"
```
