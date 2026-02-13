-- hosts 主机管理表迁移脚本
-- PostgreSQL 14+

BEGIN;

-- ============================================================
-- 1. 创建 hosts 表
-- ============================================================

CREATE TABLE IF NOT EXISTS hosts (
    host_id       TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL DEFAULT 'default',
    name          TEXT NOT NULL,
    ip            TEXT NOT NULL,
    hostname      TEXT,
    port          INT NOT NULL DEFAULT 22,
    username      TEXT NOT NULL DEFAULT 'root',
    auth_type     TEXT NOT NULL DEFAULT 'password'
                      CHECK (auth_type IN ('password', 'key')),
    password      TEXT,
    ssh_key       TEXT,
    status        TEXT NOT NULL DEFAULT 'unknown'
                      CHECK (status IN ('online', 'offline', 'unknown')),
    source        TEXT NOT NULL DEFAULT 'manual'
                      CHECK (source IN ('agent', 'manual')),
    agent_id      TEXT REFERENCES agents(agent_id),
    description   TEXT,
    vnc_port      INT NOT NULL DEFAULT 0,
    jupyter_port  INT NOT NULL DEFAULT 0,
    external_domain TEXT NOT NULL DEFAULT '',
    tags          JSONB DEFAULT '[]'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 2. 索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_hosts_tenant ON hosts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_hosts_status ON hosts(status);
CREATE INDEX IF NOT EXISTS idx_hosts_agent  ON hosts(agent_id);

COMMIT;
