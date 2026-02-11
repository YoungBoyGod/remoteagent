-- RemoteGPU PostgreSQL 初始化脚本

-- 创建扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- ============================================================
-- agents: 注册的 agent 设备信息
-- ============================================================
CREATE TABLE IF NOT EXISTS agents (
    agent_id        VARCHAR(255) PRIMARY KEY,
    tenant_id       VARCHAR(255) NOT NULL DEFAULT 'default',
    device_code     VARCHAR(255) NOT NULL,
    agent_version   VARCHAR(64),
    status          VARCHAR(32)  NOT NULL DEFAULT 'offline',
    hostname        VARCHAR(255),
    os              VARCHAR(64),
    arch            VARCHAR(64),
    ip              INET,
    labels          JSONB        NOT NULL DEFAULT '{}'::jsonb,
    capabilities    JSONB        NOT NULL DEFAULT '[]'::jsonb,
    heartbeat_interval INT       NOT NULL DEFAULT 30,
    poll_timeout    INT          NOT NULL DEFAULT 30,
    last_heartbeat_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents (tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_status    ON agents (status);

-- ============================================================
-- tasks: 任务主表
-- ============================================================
CREATE TABLE IF NOT EXISTS tasks (
    task_id     VARCHAR(255) PRIMARY KEY,
    tenant_id   VARCHAR(255) NOT NULL DEFAULT 'default',
    agent_id    VARCHAR(255) NOT NULL,
    task_type   VARCHAR(64)  NOT NULL DEFAULT 'command',
    payload     JSONB        NOT NULL DEFAULT '{}'::jsonb,
    status      VARCHAR(32)  NOT NULL DEFAULT 'pending',
    attempt     INT          NOT NULL DEFAULT 1,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_tasks_agent_id ON tasks (agent_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status   ON tasks (status);

-- ============================================================
-- task_events: 任务事件流水（幂等，event_id 去重）
-- ============================================================
CREATE TABLE IF NOT EXISTS task_events (
    event_id    VARCHAR(255) PRIMARY KEY,
    task_id     VARCHAR(255) NOT NULL,
    agent_id    VARCHAR(255) NOT NULL,
    event_type  VARCHAR(32)  NOT NULL,
    status      VARCHAR(32)  NOT NULL,
    body        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_task_events_task_id  ON task_events (task_id);
CREATE INDEX IF NOT EXISTS idx_task_events_agent_id ON task_events (agent_id);

-- ============================================================
-- task_results: 任务执行结果
-- ============================================================
CREATE TABLE IF NOT EXISTS task_results (
    task_id     VARCHAR(255) PRIMARY KEY,
    exit_code   INT          NOT NULL DEFAULT 0,
    stdout      TEXT         NOT NULL DEFAULT '',
    stderr      TEXT         NOT NULL DEFAULT '',
    truncated   BOOLEAN      NOT NULL DEFAULT false,
    started_at  TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- 授权
GRANT ALL PRIVILEGES ON DATABASE remotegpu TO remotegpu_user;
