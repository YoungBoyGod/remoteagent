-- 安全分发模块迁移脚本：增加计划分发时间
-- PostgreSQL 14+

BEGIN;

ALTER TABLE distributions
    ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_distributions_scheduled_at
    ON distributions (scheduled_at)
    WHERE scheduled_at IS NOT NULL;

COMMIT;
