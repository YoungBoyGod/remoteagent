-- 安全分发模块迁移脚本
-- PostgreSQL 14+

BEGIN;

-- ============================================================
-- 1. 创建 distributions 分发记录表
-- ============================================================

CREATE TABLE IF NOT EXISTS distributions (
    id                  BIGSERIAL PRIMARY KEY,
    task_id             TEXT NOT NULL UNIQUE,
    file_name           TEXT NOT NULL,
    file_size           BIGINT NOT NULL,
    encrypted_file_path TEXT,
    sha256_original     TEXT NOT NULL,
    sha256_encrypted    TEXT,
    encryption_algo     TEXT NOT NULL DEFAULT 'AES-256',
    customer_name       TEXT NOT NULL,
    customer_email      TEXT NOT NULL,
    session_key_hash    TEXT,
    presigned_url       TEXT,
    url_expires_at      TIMESTAMPTZ,
    status              TEXT NOT NULL DEFAULT 'pending'
                            CHECK (status IN ('pending','encrypting','uploaded','sent','downloaded','expired')),
    download_ip         TEXT,
    download_at         TIMESTAMPTZ,
    release_notes       TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- 2. 索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_distributions_status     ON distributions(status);
CREATE INDEX IF NOT EXISTS idx_distributions_task_id    ON distributions(task_id);
CREATE INDEX IF NOT EXISTS idx_distributions_customer   ON distributions(customer_email);
CREATE INDEX IF NOT EXISTS idx_distributions_created_at ON distributions(created_at);

COMMIT;
