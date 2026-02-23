-- ============================================================
-- RemoteAgent 完整数据库初始化脚本
-- 包含所有模块: Agent管理、任务调度、主机管理、客户管理、
-- 安全分发、文档中心、客户支持平台
-- PostgreSQL 14+
-- ============================================================

BEGIN;

-- ============================================================
-- 1. agents 表 - Agent管理
-- ============================================================
CREATE TABLE IF NOT EXISTS agents (
    agent_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    device_code VARCHAR(128) NOT NULL UNIQUE,
    agent_version VARCHAR(32) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'unknown',
    hostname VARCHAR(128),
    os VARCHAR(32),
    arch VARCHAR(32),
    ip INET,
    external_ip INET,
    labels JSONB NOT NULL DEFAULT '{}'::JSONB,
    capabilities JSONB NOT NULL DEFAULT '[]'::JSONB,
    heartbeat_interval INT NOT NULL DEFAULT 30,
    poll_timeout INT NOT NULL DEFAULT 30,
    max_concurrent INT NOT NULL DEFAULT 4,
    last_heartbeat_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_status CHECK (status IN ('unknown', 'online', 'offline'))
);

CREATE INDEX IF NOT EXISTS idx_agents_tenant ON agents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_device_code ON agents(device_code);

-- ============================================================
-- 2. tasks 表 - 任务调度
-- ============================================================
CREATE TABLE IF NOT EXISTS tasks (
    task_id VARCHAR(64) PRIMARY KEY,
    tenant_id VARCHAR(64) NOT NULL DEFAULT 'default',
    agent_id VARCHAR(64) REFERENCES agents(agent_id),
    target_agent_id VARCHAR(64),
    idempotency_key TEXT UNIQUE,
    task_type VARCHAR(32) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    exec_mode TEXT NOT NULL DEFAULT 'shared' CHECK (exec_mode IN ('shared', 'exclusive')),
    priority INT NOT NULL DEFAULT 50 CHECK (priority BETWEEN 1 AND 100),
    preemptible BOOLEAN NOT NULL DEFAULT FALSE,
    max_attempts INT NOT NULL DEFAULT 3,
    attempt INT NOT NULL DEFAULT 1,
    preempt_state TEXT NOT NULL DEFAULT 'none' CHECK (preempt_state IN ('none', 'requested', 'acknowledged', 'terminating')),
    preempt_requested_at TIMESTAMPTZ,
    preempt_deadline TIMESTAMPTZ,
    preempt_reason TEXT,
    next_retry_at TIMESTAMPTZ,
    leased_until TIMESTAMPTZ,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    CONSTRAINT chk_task_status CHECK (status IN ('pending', 'leased', 'running', 'canceling', 'success', 'failed', 'timeout', 'canceled'))
);

CREATE INDEX IF NOT EXISTS idx_tasks_agent_status ON tasks(agent_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at);
CREATE INDEX IF NOT EXISTS idx_tasks_sched ON tasks(status, exec_mode, priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_tasks_target_agent ON tasks(target_agent_id, status) WHERE target_agent_id IS NOT NULL;

-- ============================================================
-- 3. task_events 表 - 任务事件
-- ============================================================
CREATE TABLE IF NOT EXISTS task_events (
    id BIGSERIAL PRIMARY KEY,
    event_id VARCHAR(64) NOT NULL UNIQUE,
    task_id VARCHAR(64) NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
    agent_id VARCHAR(64) NOT NULL,
    event_type VARCHAR(32) NOT NULL,
    status VARCHAR(16),
    body JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_events_task_created ON task_events(task_id, created_at);
CREATE INDEX IF NOT EXISTS idx_task_events_agent ON task_events(agent_id);

-- ============================================================
-- 4. task_results 表 - 任务结果
-- ============================================================
CREATE TABLE IF NOT EXISTS task_results (
    task_id VARCHAR(64) PRIMARY KEY REFERENCES tasks(task_id) ON DELETE CASCADE,
    exit_code INT,
    stdout TEXT,
    stderr TEXT,
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ============================================================
-- 5. control_commands 表 - 控制命令
-- ============================================================
CREATE TABLE IF NOT EXISTS control_commands (
    command_id VARCHAR(64) PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    action VARCHAR(32) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    delivered_at TIMESTAMPTZ,
    acked_at TIMESTAMPTZ,
    CONSTRAINT chk_control_action CHECK (action IN ('refresh_token', 'shutdown', 'reload_config', 'cancel_task', 'cancel')),
    CONSTRAINT chk_control_status CHECK (status IN ('pending', 'delivered', 'acked', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_control_agent_status ON control_commands(agent_id, status, created_at);

-- ============================================================
-- 6. hosts 表 - 主机管理
-- ============================================================
CREATE TABLE IF NOT EXISTS hosts (
    host_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL DEFAULT 'default',
    name TEXT NOT NULL,
    ip TEXT NOT NULL,
    hostname TEXT,
    port INT NOT NULL DEFAULT 22,
    username TEXT NOT NULL DEFAULT 'root',
    auth_type TEXT NOT NULL DEFAULT 'password' CHECK (auth_type IN ('password', 'key')),
    password TEXT,
    ssh_key TEXT,
    status TEXT NOT NULL DEFAULT 'unknown' CHECK (status IN ('online', 'offline', 'unknown', 'busy', 'maintenance')),
    source TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('agent', 'manual')),
    agent_id TEXT REFERENCES agents(agent_id) ON DELETE SET NULL,
    customer_id TEXT,
    description TEXT,
    vnc_addr TEXT NOT NULL DEFAULT '',
    jupyter_addr TEXT NOT NULL DEFAULT '',
    ext_ssh_addr TEXT NOT NULL DEFAULT '',
    ext_vnc_addr TEXT NOT NULL DEFAULT '',
    ext_jupyter_addr TEXT NOT NULL DEFAULT '',
    assigned_to TEXT,
    tags JSONB DEFAULT '[]'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_hosts_tenant ON hosts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_hosts_status ON hosts(status);
CREATE INDEX IF NOT EXISTS idx_hosts_agent ON hosts(agent_id);
CREATE INDEX IF NOT EXISTS idx_hosts_customer ON hosts(customer_id);

-- ============================================================
-- 7. customers 表 - 客户管理
-- ============================================================
CREATE TABLE IF NOT EXISTS customers (
    customer_id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    company TEXT,
    description TEXT,
    tags JSONB DEFAULT '[]'::JSONB,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_customers_status ON customers(status);
CREATE INDEX IF NOT EXISTS idx_customers_company ON customers(company);

-- 添加外键约束
ALTER TABLE hosts ADD CONSTRAINT fk_hosts_customer 
    FOREIGN KEY (customer_id) REFERENCES customers(customer_id) ON DELETE SET NULL;

-- ============================================================
-- 8. customer_hosts 表 - 客户-主机关联
-- ============================================================
CREATE TABLE IF NOT EXISTS customer_hosts (
    id SERIAL PRIMARY KEY,
    customer_id TEXT NOT NULL REFERENCES customers(customer_id) ON DELETE CASCADE,
    host_id TEXT NOT NULL REFERENCES hosts(host_id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    note TEXT,
    UNIQUE(customer_id, host_id)
);

CREATE INDEX IF NOT EXISTS idx_customer_hosts_customer ON customer_hosts(customer_id);
CREATE INDEX IF NOT EXISTS idx_customer_hosts_host ON customer_hosts(host_id);

-- ============================================================
-- 9. operation_logs 表 - 操作日志
-- ============================================================
CREATE TABLE IF NOT EXISTS operation_logs (
    log_id SERIAL PRIMARY KEY,
    resource_type TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    action TEXT NOT NULL,
    operator TEXT NOT NULL DEFAULT 'admin',
    detail JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_operation_logs_resource ON operation_logs(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_operation_logs_created ON operation_logs(created_at);

-- ============================================================
-- 10. distributions 表 - 安全分发
-- ============================================================
CREATE TABLE IF NOT EXISTS distributions (
    id BIGSERIAL PRIMARY KEY,
    task_id TEXT NOT NULL UNIQUE,
    file_name TEXT NOT NULL,
    file_size BIGINT NOT NULL,
    encrypted_file_path TEXT,
    sha256_original TEXT NOT NULL,
    sha256_encrypted TEXT,
    encryption_algo TEXT NOT NULL DEFAULT 'AES-256',
    customer_name TEXT NOT NULL,
    customer_email TEXT NOT NULL,
    session_key_hash TEXT,
    presigned_url TEXT,
    url_expires_at TIMESTAMPTZ,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'encrypting', 'uploaded', 'sent', 'downloaded', 'expired')),
    download_ip TEXT,
    download_at TIMESTAMPTZ,
    release_notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_distributions_status ON distributions(status);
CREATE INDEX IF NOT EXISTS idx_distributions_task_id ON distributions(task_id);
CREATE INDEX IF NOT EXISTS idx_distributions_customer ON distributions(customer_email);
CREATE INDEX IF NOT EXISTS idx_distributions_created_at ON distributions(created_at);

-- ============================================================
-- 11. doc_categories 表 - 文档分类
-- ============================================================
CREATE TABLE IF NOT EXISTS doc_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    slug VARCHAR(200) UNIQUE NOT NULL,
    icon VARCHAR(50),
    color VARCHAR(20),
    parent_id BIGINT REFERENCES doc_categories(id) ON DELETE SET NULL,
    sort_order INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_doc_categories_parent ON doc_categories(parent_id);
CREATE INDEX IF NOT EXISTS idx_doc_categories_slug ON doc_categories(slug);

-- ============================================================
-- 12. documents 表 - 文档
-- ============================================================
CREATE TABLE IF NOT EXISTS documents (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(255) UNIQUE NOT NULL,
    title VARCHAR(500) NOT NULL,
    category_id BIGINT REFERENCES doc_categories(id) ON DELETE SET NULL,
    content_key VARCHAR(500) NOT NULL DEFAULT '',
    format VARCHAR(20) NOT NULL DEFAULT 'markdown' CHECK (format IN ('markdown', 'html', 'pdf')),
    language VARCHAR(10) NOT NULL DEFAULT 'zh',
    author VARCHAR(100) DEFAULT 'admin',
    status VARCHAR(20) NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    sort_order INT DEFAULT 0,
    metadata JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_documents_slug ON documents(slug);
CREATE INDEX IF NOT EXISTS idx_documents_category ON documents(category_id);
CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_created ON documents(created_at);

-- ============================================================
-- 13. doc_versions 表 - 文档版本
-- ============================================================
CREATE TABLE IF NOT EXISTS doc_versions (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    version VARCHAR(50) NOT NULL,
    content_key VARCHAR(500) NOT NULL DEFAULT '',
    changelog TEXT,
    created_by VARCHAR(100),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(document_id, version)
);

CREATE INDEX IF NOT EXISTS idx_doc_versions_document ON doc_versions(document_id);

-- ============================================================
-- 14. doc_attachments 表 - 文档附件
-- ============================================================
CREATE TABLE IF NOT EXISTS doc_attachments (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    filename VARCHAR(500) NOT NULL,
    storage_key VARCHAR(500) NOT NULL,
    content_type VARCHAR(100) DEFAULT 'application/octet-stream',
    size_bytes BIGINT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_doc_attachments_document ON doc_attachments(document_id);

-- ============================================================
-- 15. doc_feedback 表 - 文档反馈
-- ============================================================
CREATE TABLE IF NOT EXISTS doc_feedback (
    id BIGSERIAL PRIMARY KEY,
    document_id BIGINT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    type VARCHAR(20) NOT NULL CHECK (type IN ('bug', 'suggestion', 'question', 'other')),
    description TEXT NOT NULL,
    email VARCHAR(200),
    status VARCHAR(20) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'resolved', 'rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_doc_feedback_document ON doc_feedback(document_id);
CREATE INDEX IF NOT EXISTS idx_doc_feedback_status ON doc_feedback(status);

-- ============================================================
-- 16. support_sessions 表 - 客户支持会话
-- ============================================================
CREATE TABLE IF NOT EXISTS support_sessions (
    session_id VARCHAR(64) PRIMARY KEY,
    host_id TEXT REFERENCES hosts(host_id) ON DELETE SET NULL,
    agent_id VARCHAR(64) REFERENCES agents(agent_id) ON DELETE SET NULL,
    customer_id TEXT REFERENCES customers(customer_id) ON DELETE SET NULL,
    customer_name TEXT NOT NULL,
    customer_email TEXT,
    issue_description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'waiting' CHECK (status IN ('waiting', 'active', 'paused', 'closed')),
    priority TEXT NOT NULL DEFAULT 'medium' CHECK (priority IN ('low', 'medium', 'high', 'urgent')),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    duration INT,
    notes TEXT,
    tags JSONB DEFAULT '[]'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_sessions_status ON support_sessions(status);
CREATE INDEX IF NOT EXISTS idx_support_sessions_priority ON support_sessions(priority);
CREATE INDEX IF NOT EXISTS idx_support_sessions_host ON support_sessions(host_id);
CREATE INDEX IF NOT EXISTS idx_support_sessions_agent ON support_sessions(agent_id);
CREATE INDEX IF NOT EXISTS idx_support_sessions_customer ON support_sessions(customer_id);
CREATE INDEX IF NOT EXISTS idx_support_sessions_started ON support_sessions(started_at);

-- ============================================================
-- 17. support_messages 表 - 支持消息
-- ============================================================
CREATE TABLE IF NOT EXISTS support_messages (
    message_id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) NOT NULL REFERENCES support_sessions(session_id) ON DELETE CASCADE,
    sender_type TEXT NOT NULL CHECK (sender_type IN ('agent', 'customer', 'system')),
    sender_id TEXT,
    sender_name TEXT NOT NULL,
    content TEXT NOT NULL,
    message_type TEXT NOT NULL DEFAULT 'text' CHECK (message_type IN ('text', 'file', 'command', 'screenshot')),
    metadata JSONB DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_support_messages_session ON support_messages(session_id);
CREATE INDEX IF NOT EXISTS idx_support_messages_created ON support_messages(created_at);

-- ============================================================
-- 18. remote_commands 表 - 远程命令执行记录
-- ============================================================
CREATE TABLE IF NOT EXISTS remote_commands (
    command_id VARCHAR(64) PRIMARY KEY,
    session_id VARCHAR(64) REFERENCES support_sessions(session_id) ON DELETE SET NULL,
    host_id TEXT REFERENCES hosts(host_id) ON DELETE SET NULL,
    task_id VARCHAR(64) REFERENCES tasks(task_id) ON DELETE SET NULL,
    command TEXT NOT NULL,
    output TEXT,
    exit_code INT,
    executed_by TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_remote_commands_session ON remote_commands(session_id);
CREATE INDEX IF NOT EXISTS idx_remote_commands_host ON remote_commands(host_id);
CREATE INDEX IF NOT EXISTS idx_remote_commands_status ON remote_commands(status);
CREATE INDEX IF NOT EXISTS idx_remote_commands_created ON remote_commands(created_at);

-- ============================================================
-- 创建更新时间触发器函数
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为需要自动更新 updated_at 的表创建触发器
CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_hosts_updated_at BEFORE UPDATE ON hosts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_customers_updated_at BEFORE UPDATE ON customers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_distributions_updated_at BEFORE UPDATE ON distributions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_documents_updated_at BEFORE UPDATE ON documents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_support_sessions_updated_at BEFORE UPDATE ON support_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;
