-- 文档中心模块迁移脚本
-- PostgreSQL 14+

BEGIN;

-- ============================================================
-- 1. doc_categories 文档分类表
-- ============================================================

CREATE TABLE IF NOT EXISTS doc_categories (
    id            BIGSERIAL PRIMARY KEY,
    name          VARCHAR(200) NOT NULL,
    slug          VARCHAR(200) UNIQUE NOT NULL,
    icon          VARCHAR(50),
    color         VARCHAR(20),
    parent_id     BIGINT REFERENCES doc_categories(id) ON DELETE SET NULL,
    sort_order    INT DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 2. documents 文档表
-- ============================================================

CREATE TABLE IF NOT EXISTS documents (
    id            BIGSERIAL PRIMARY KEY,
    slug          VARCHAR(255) UNIQUE NOT NULL,
    title         VARCHAR(500) NOT NULL,
    category_id   BIGINT REFERENCES doc_categories(id) ON DELETE SET NULL,
    content_key   VARCHAR(500) NOT NULL DEFAULT '',
    format        VARCHAR(20) NOT NULL DEFAULT 'markdown'
                      CHECK (format IN ('markdown', 'html', 'pdf')),
    language      VARCHAR(10) NOT NULL DEFAULT 'zh',
    author        VARCHAR(100) DEFAULT 'admin',
    status        VARCHAR(20) NOT NULL DEFAULT 'draft'
                      CHECK (status IN ('draft', 'published', 'archived')),
    sort_order    INT DEFAULT 0,
    metadata      JSONB DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 3. doc_versions 文档版本表
-- ============================================================

CREATE TABLE IF NOT EXISTS doc_versions (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version       VARCHAR(50) NOT NULL,
    content_key   VARCHAR(500) NOT NULL DEFAULT '',
    changelog     TEXT,
    created_by    VARCHAR(100),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(document_id, version)
);

-- ============================================================
-- 4. doc_attachments 文档附件表
-- ============================================================

CREATE TABLE IF NOT EXISTS doc_attachments (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    filename      VARCHAR(500) NOT NULL,
    storage_key   VARCHAR(500) NOT NULL,
    content_type  VARCHAR(100) DEFAULT 'application/octet-stream',
    size_bytes    BIGINT DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 5. doc_feedback 文档反馈表
-- ============================================================

CREATE TABLE IF NOT EXISTS doc_feedback (
    id            BIGSERIAL PRIMARY KEY,
    document_id   BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    type          VARCHAR(20) NOT NULL
                      CHECK (type IN ('bug', 'suggestion', 'question', 'other')),
    description   TEXT NOT NULL,
    email         VARCHAR(200),
    status        VARCHAR(20) NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'resolved', 'rejected')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 6. 索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_doc_categories_parent    ON doc_categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_doc_categories_slug      ON doc_categories(slug);

CREATE INDEX IF NOT EXISTS idx_documents_slug           ON documents(slug);
CREATE INDEX IF NOT EXISTS idx_documents_category       ON documents(category_id);
CREATE INDEX IF NOT EXISTS idx_documents_status         ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_created        ON documents(created_at);

CREATE INDEX IF NOT EXISTS idx_doc_versions_document    ON doc_versions(document_id);
CREATE INDEX IF NOT EXISTS idx_doc_attachments_document ON doc_attachments(document_id);
CREATE INDEX IF NOT EXISTS idx_doc_feedback_document    ON doc_feedback(document_id);
CREATE INDEX IF NOT EXISTS idx_doc_feedback_status      ON doc_feedback(status);

COMMIT;
