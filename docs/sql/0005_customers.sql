-- 客户管理模块迁移脚本
-- PostgreSQL 14+

BEGIN;

-- ============================================================
-- 1. 创建 customers 客户表
-- ============================================================

CREATE TABLE IF NOT EXISTS customers (
    customer_id   TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    email         TEXT,
    phone         TEXT,
    company       TEXT,
    description   TEXT,
    tags          JSONB DEFAULT '[]'::jsonb,
    status        TEXT NOT NULL DEFAULT 'active'
                      CHECK (status IN ('active', 'inactive')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 2. 创建 customer_hosts 客户-主机关联表
-- ============================================================

CREATE TABLE IF NOT EXISTS customer_hosts (
    id            SERIAL PRIMARY KEY,
    customer_id   TEXT NOT NULL REFERENCES customers(customer_id),
    host_id       TEXT NOT NULL REFERENCES hosts(host_id),
    assigned_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    note          TEXT,
    UNIQUE(customer_id, host_id)
);

-- ============================================================
-- 3. 创建 operation_logs 操作日志表
-- ============================================================

CREATE TABLE IF NOT EXISTS operation_logs (
    log_id        SERIAL PRIMARY KEY,
    resource_type TEXT NOT NULL,
    resource_id   TEXT NOT NULL,
    action        TEXT NOT NULL,
    operator      TEXT NOT NULL DEFAULT 'admin',
    detail        JSONB DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 4. hosts 表新增 customer_id 字段
-- ============================================================

ALTER TABLE hosts ADD COLUMN IF NOT EXISTS customer_id TEXT REFERENCES customers(customer_id);

-- ============================================================
-- 5. 索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_customers_status    ON customers(status);
CREATE INDEX IF NOT EXISTS idx_customers_company   ON customers(company);

CREATE INDEX IF NOT EXISTS idx_customer_hosts_customer ON customer_hosts(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_hosts_host     ON customer_hosts(host_id);

CREATE INDEX IF NOT EXISTS idx_operation_logs_resource ON operation_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_created  ON operation_logs(created_at);

CREATE INDEX IF NOT EXISTS idx_hosts_customer ON hosts(customer_id);

COMMIT;
