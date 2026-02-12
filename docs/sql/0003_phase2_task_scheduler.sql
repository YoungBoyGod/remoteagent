-- Phase 2 任务调度系统迁移脚本
-- 基于 0001_init.sql 现有表结构进行增量迁移
-- PostgreSQL 14+

begin;

-- ============================================================
-- 1. tasks 表：新增调度相关字段
-- ============================================================

-- 幂等键，用于任务创建去重
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS idempotency_key text UNIQUE;

-- 执行模式：shared（共享并发）/ exclusive（独占）
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS exec_mode text NOT NULL DEFAULT 'shared'
    CHECK (exec_mode IN ('shared', 'exclusive'));

-- 优先级：1-100，默认 50，数值越大优先级越高
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS priority int NOT NULL DEFAULT 50
    CHECK (priority BETWEEN 1 AND 100);

-- 是否可被抢占
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS preemptible boolean NOT NULL DEFAULT false;

-- 最大重试次数
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS max_attempts int NOT NULL DEFAULT 3;

-- 抢占子状态
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS preempt_state text NOT NULL DEFAULT 'none'
    CHECK (preempt_state IN ('none', 'requested', 'acknowledged', 'terminating'));

-- 抢占请求时间
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS preempt_requested_at timestamptz;

-- 抢占截止时间
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS preempt_deadline timestamptz;

-- 抢占原因
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS preempt_reason text;

-- 下次重试时间
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS next_retry_at timestamptz;

-- 错误码
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS error_code text;

-- 错误信息
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS error_message text;

-- ============================================================
-- 2. tasks 表：更新 status 约束，加入 leased 和 timeout
-- ============================================================

-- 删除旧约束，重建新约束
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_task_status;
ALTER TABLE tasks ADD CONSTRAINT chk_task_status
    CHECK (status IN ('pending', 'leased', 'running', 'success', 'failed', 'timeout', 'canceled'));

-- ============================================================
-- 3. tasks 表：放宽 agent_id 约束
--    Phase 1 中 agent_id 是 NOT NULL 且有外键，
--    Phase 2 任务创建时 agent_id 为空（等待认领），需要改为可空
-- ============================================================

ALTER TABLE tasks ALTER COLUMN agent_id DROP NOT NULL;

-- ============================================================
-- 4. agents 表：新增并发控制字段
-- ============================================================

-- 共享任务最大并发槽位
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_concurrent int NOT NULL DEFAULT 4;

-- ============================================================
-- 5. 新增调度索引
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_tasks_sched
    ON tasks(status, exec_mode, priority DESC, created_at ASC);

-- ============================================================
-- 6. 补充 updated_at 字段（Phase 1 可能未包含）
-- ============================================================

ALTER TABLE tasks ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

commit;
