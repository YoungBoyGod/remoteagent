-- ============================================================
-- RemoteAgent 数据库重建脚本
-- 警告: 此脚本会删除所有表和数据，仅用于开发/测试环境
-- ============================================================

BEGIN;

-- 按外键依赖顺序反向删除
DROP TABLE IF EXISTS remote_commands CASCADE;
DROP TABLE IF EXISTS support_messages CASCADE;
DROP TABLE IF EXISTS support_sessions CASCADE;
DROP TABLE IF EXISTS doc_feedback CASCADE;
DROP TABLE IF EXISTS doc_attachments CASCADE;
DROP TABLE IF EXISTS doc_versions CASCADE;
DROP TABLE IF EXISTS documents CASCADE;
DROP TABLE IF EXISTS doc_categories CASCADE;
DROP TABLE IF EXISTS distributions CASCADE;
DROP TABLE IF EXISTS operation_logs CASCADE;
DROP TABLE IF EXISTS customer_hosts CASCADE;
DROP TABLE IF EXISTS customers CASCADE;
DROP TABLE IF EXISTS hosts CASCADE;
DROP TABLE IF EXISTS control_commands CASCADE;
DROP TABLE IF EXISTS task_results CASCADE;
DROP TABLE IF EXISTS task_events CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;
DROP TABLE IF EXISTS agents CASCADE;

-- 删除触发器函数
DROP FUNCTION IF EXISTS update_updated_at_column() CASCADE;

COMMIT;

-- 重新创建所有表
\i 0000_complete_init.sql
