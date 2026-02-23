-- 修复 tasks 状态约束，加入 canceling 状态
ALTER TABLE tasks DROP CONSTRAINT IF EXISTS chk_task_status;
ALTER TABLE tasks ADD CONSTRAINT chk_task_status
    CHECK (status IN ('pending', 'leased', 'running', 'canceling', 'success', 'failed', 'timeout', 'canceled'));
