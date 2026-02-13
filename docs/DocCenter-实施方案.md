# DocCenter 企业级文档中心 - 实施方案

## 1. 项目现状

| 模块 | 状态 | 说明 |
|------|------|------|
| 前端骨架 | 已完成 | 三栏布局、搜索弹窗、版本切换、diff 对比 UI 已就绪，全部使用 mock 数据 |
| 前端路由 | 已完成 | `/documents` 和 `/documents/:docId` 已配置 |
| 后端结构 | 已完成 | Go + Gin，model/store/service/controller/router 分层架构 |
| 存储 | 未开始 | 需接入 S3/RustFS |
| 文档 API | 未开始 | 需要全套 CRUD + 搜索 + 版本管理 |

## 2. 技术选型

| 层级 | 技术 | 说明 |
|------|------|------|
| 前端框架 | Vue 3 + TypeScript + Vite | 已有 |
| UI 组件 | Element Plus + Tailwind CSS | 已有 |
| Markdown 渲染 | markdown-it + Prism.js + Mermaid.js | 新增 |
| Markdown 编辑 | Vditor 或 MdEditor-v3 | 新增 |
| 后端框架 | Go + Gin | 已有 |
| 数据库 | PostgreSQL | 已有 |
| 对象存储 | S3 兼容（RustFS/MinIO） | 新增 |
| 全文搜索 | Meilisearch | 新增，轻量级，比 ES 更适合文档场景 |
| PDF 生成 | wkhtmltopdf 或 chromedp | 新增 |

## 3. 系统架构

```
┌─────────────────────────────────────────────────────┐
│                    前端 (Vue 3)                       │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐ │
│  │ 文档阅读  │ │ 文档编辑  │ │ 搜索组件  │ │ 管理页面 │ │
│  │  Reader   │ │  Editor  │ │  Search  │ │  Admin  │ │
│  └──────────┘ └──────────┘ └──────────┘ └─────────┘ │
└────────────────────┬────────────────────────────────┘
                     │ REST API
┌────────────────────▼────────────────────────────────┐
│                   后端 (Go + Gin)                     │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌─────────┐ │
│  │ Doc API  │ │ Search   │ │ Storage  │ │ Version │ │
│  │ CRUD     │ │ Service  │ │ Service  │ │ Service │ │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └────┬────┘ │
└───────┼────────────┼────────────┼─────────────┼─────┘
        │            │            │             │
   ┌────▼────┐  ┌────▼─────┐ ┌───▼──────┐ ┌───▼───┐
   │PostgreSQL│  │Meilisearch│ │S3/RustFS │ │  Git  │
   │ 元数据   │  │ 全文索引  │ │ 文件存储  │ │ 可选  │
   └─────────┘  └──────────┘ └──────────┘ └───────┘
```

## 4. 数据库设计

### documents（文档表）
```sql
CREATE TABLE documents (
    id            BIGSERIAL PRIMARY KEY,
    slug          VARCHAR(255) UNIQUE NOT NULL,  -- URL 友好标识
    title         VARCHAR(500) NOT NULL,
    category_id   BIGINT REFERENCES doc_categories(id),
    content_key   VARCHAR(500) NOT NULL,         -- S3 对象 key
    format        VARCHAR(20) DEFAULT 'markdown',
    language      VARCHAR(10) DEFAULT 'zh',
    author        VARCHAR(100),
    status        VARCHAR(20) DEFAULT 'draft',   -- draft/published/archived
    sort_order    INT DEFAULT 0,
    metadata      JSONB DEFAULT '{}',            -- 扩展字段
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    updated_at    TIMESTAMPTZ DEFAULT NOW()
);
```

### doc_categories（分类表）
```sql
CREATE TABLE doc_categories (
    id          BIGSERIAL PRIMARY KEY,
    name        VARCHAR(200) NOT NULL,
    slug        VARCHAR(200) UNIQUE NOT NULL,
    icon        VARCHAR(50),
    color       VARCHAR(20),
    parent_id   BIGINT REFERENCES doc_categories(id),
    sort_order  INT DEFAULT 0,
    created_at  TIMESTAMPTZ DEFAULT NOW()
);
```

### doc_versions（版本表）
```sql
CREATE TABLE doc_versions (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT REFERENCES documents(id) ON DELETE CASCADE,
    version       VARCHAR(50) NOT NULL,          -- v2.4.1
    content_key   VARCHAR(500) NOT NULL,         -- S3 对象 key（该版本快照）
    changelog     TEXT,
    created_by    VARCHAR(100),
    created_at    TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(document_id, version)
);
```

### doc_attachments（附件表）
```sql
CREATE TABLE doc_attachments (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT REFERENCES documents(id) ON DELETE CASCADE,
    filename      VARCHAR(500) NOT NULL,
    storage_key   VARCHAR(500) NOT NULL,         -- S3 对象 key
    content_type  VARCHAR(100),
    size_bytes    BIGINT,
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
```

### doc_feedback（反馈表）
```sql
CREATE TABLE doc_feedback (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT REFERENCES documents(id),
    type          VARCHAR(20) NOT NULL,          -- content/missing/other
    description   TEXT NOT NULL,
    email         VARCHAR(200),
    status        VARCHAR(20) DEFAULT 'pending', -- pending/resolved/rejected
    created_at    TIMESTAMPTZ DEFAULT NOW()
);
```

## 5. API 设计

### 文档 CRUD
```
GET    /api/v1/docs                    # 文档列表（分页、分类筛选）
GET    /api/v1/docs/:slug              # 文档详情（含 Markdown 内容）
POST   /api/v1/docs                    # 创建文档
PUT    /api/v1/docs/:slug              # 更新文档
DELETE /api/v1/docs/:slug              # 删除文档
```

### 分类管理
```
GET    /api/v1/docs/categories         # 分类树
POST   /api/v1/docs/categories         # 创建分类
PUT    /api/v1/docs/categories/:id     # 更新分类
DELETE /api/v1/docs/categories/:id     # 删除分类
```

### 版本管理
```
GET    /api/v1/docs/:slug/versions     # 版本列表
GET    /api/v1/docs/:slug/versions/:v  # 指定版本内容
POST   /api/v1/docs/:slug/versions     # 创建版本快照
GET    /api/v1/docs/:slug/diff?from=v1&to=v2  # 版本对比
```

### 搜索
```
GET    /api/v1/docs/search?q=keyword&category=xxx&lang=zh  # 全文搜索
GET    /api/v1/docs/search/suggest?q=keyword               # 搜索建议
```

### 附件/文件
```
POST   /api/v1/docs/:slug/attachments  # 上传附件（图片等）
GET    /api/v1/docs/attachments/:id    # 获取附件（预签名 URL）
DELETE /api/v1/docs/attachments/:id    # 删除附件
```

### 反馈
```
POST   /api/v1/docs/:slug/feedback     # 提交反馈
GET    /api/v1/docs/feedback            # 反馈列表（管理端）
PUT    /api/v1/docs/feedback/:id        # 处理反馈
```

### PDF 导出
```
GET    /api/v1/docs/:slug/export/pdf   # 导出 PDF
```

## 6. S3/RustFS 存储结构

```
docs-bucket/
├── documents/
│   ├── {slug}/
│   │   ├── latest.md              # 最新内容
│   │   └── versions/
│   │       ├── v2.4.0.md
│   │       └── v2.4.1.md
│   └── ...
├── attachments/
│   ├── {doc_id}/
│   │   ├── image1.png
│   │   └── diagram.svg
│   └── ...
└── exports/
    └── {slug}-{version}.pdf       # 缓存的 PDF
```

## 7. 10 人团队分工

### 后端团队（5人）

| 编号 | 角色 | 职责 | 产出 |
|------|------|------|------|
| BE-1 | 存储基础 | S3/RustFS 客户端封装、上传/下载/预签名 URL、存储配置 | `internal/storage/` 包 |
| BE-2 | 数据模型 | 数据库迁移脚本、Model 定义、Store 层 CRUD | `internal/model/doc*.go` + `internal/store/doc*.go` |
| BE-3 | 文档 API | Controller + Service 层、文档 CRUD + 分类 + 版本 + 附件 API | `internal/controller/doc*.go` + `internal/service/doc*.go` |
| BE-4 | 搜索服务 | Meilisearch 集成、索引构建、搜索 API、搜索建议 | `internal/search/` 包 |
| BE-5 | 辅助服务 | PDF 导出、版本 diff 生成、反馈管理、docker-compose 更新 | `internal/service/export.go` + `internal/service/diff.go` |

### 前端团队（5人）

| 编号 | 角色 | 职责 | 产出 |
|------|------|------|------|
| FE-1 | Markdown 渲染 | markdown-it 集成、代码高亮、Mermaid 图表、代码复制按钮、锚点导航 | `components/MarkdownRenderer.vue` |
| FE-2 | API 对接 | Pinia Store、API 服务层、替换所有 mock 数据为真实接口 | `stores/doc.ts` + `api/doc.ts` |
| FE-3 | 搜索功能 | 搜索弹窗对接真实 API、搜索结果高亮、搜索建议、搜索历史 | 改造 `Documents/index.vue` 搜索部分 |
| FE-4 | 文档编辑器 | 管理端文档编辑页面、Markdown 编辑器、附件上传、分类管理 | `pages/Documents/editor.vue` + `pages/Documents/admin.vue` |
| FE-5 | 增强功能 | 版本 diff 对接真实数据、PDF 导出、反馈提交、阅读统计、面包屑导航 | 改造现有组件 |

## 8. 依赖关系

```
Phase 1 - 基础层（可并行）
═══════════════════════════════════════
BE-1 (S3 存储)  ──┐
BE-2 (数据模型)  ──┤── 无依赖，立即开始
FE-1 (MD 渲染)  ──┤
FE-4 (编辑器)   ──┘

Phase 2 - 核心层（依赖 Phase 1）
═══════════════════════════════════════
BE-3 (文档 API)  ←── 依赖 BE-1 + BE-2
BE-4 (搜索服务)  ←── 依赖 BE-2
FE-2 (API 对接)  ←── 依赖 BE-3

Phase 3 - 集成层（依赖 Phase 2）
═══════════════════════════════════════
BE-5 (辅助服务)  ←── 依赖 BE-3
FE-3 (搜索功能)  ←── 依赖 BE-4 + FE-2
FE-5 (增强功能)  ←── 依赖 FE-2
```

## 9. 关键技术决策

1. **文档内容存 S3，元数据存 PostgreSQL** — 大文本不进数据库，利用 S3 的版本管理能力
2. **Meilisearch 而非 Elasticsearch** — 更轻量，API 更简洁，文档搜索场景足够
3. **版本快照存 S3** — 每次发布版本时将当前内容复制一份到 `versions/` 目录
4. **PDF 用 chromedp 生成** — 比 wkhtmltopdf 渲染效果更好，支持现代 CSS
5. **Markdown 渲染前端完成** — 后端只存原始 Markdown，前端用 markdown-it 渲染，减少后端压力
6. **附件通过预签名 URL 直传 S3** — 大文件不经过后端，减少带宽压力

## 10. docker-compose 新增服务

```yaml
# 新增到现有 docker-compose.yml
services:
  meilisearch:
    image: getmeili/meilisearch:v1.6
    ports:
      - "7700:7700"
    environment:
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:-dev-master-key}
    volumes:
      - meili_data:/meili_data

  rustfs:  # 或 minio
    image: minio/minio:latest  # 开发环境用 MinIO，生产用 RustFS
    ports:
      - "9000:9000"
      - "9001:9001"
    environment:
      MINIO_ROOT_USER: ${S3_ACCESS_KEY:-minioadmin}
      MINIO_ROOT_PASSWORD: ${S3_SECRET_KEY:-minioadmin}
    command: server /data --console-address ":9001"
    volumes:
      - s3_data:/data

volumes:
  meili_data:
  s3_data:
```

## 11. 验收标准

- [ ] 文档列表页加载真实数据，分类导航可折叠展开
- [ ] 文档详情页渲染 Markdown，支持代码高亮 + Mermaid 图表
- [ ] Ctrl+K 搜索返回真实结果，支持高亮和建议
- [ ] 版本切换加载对应版本内容
- [ ] 版本 diff 显示真实变更
- [ ] 管理端可创建/编辑/删除文档
- [ ] 附件上传到 S3 并在文档中正确显示
- [ ] PDF 导出可用
- [ ] 反馈提交和管理可用
- [ ] 暗色主题完整适配
