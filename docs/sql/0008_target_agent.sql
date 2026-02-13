-- 0008: 任务定向分发 — 添加 target_agent_id
ALTER TABLE tasks ADD COLUMN target_agent_id VARCHAR(64);

-- 部分索引：只覆盖有定向需求的任务
CREATE INDEX idx_tasks_target_agent ON tasks(target_agent_id, status)
  WHERE target_agent_id IS NOT NULL;
