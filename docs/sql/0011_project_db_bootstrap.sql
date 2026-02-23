-- ============================================================
-- Project-specific PostgreSQL bootstrap (non-remotegpu)
-- Usage:
--   psql -h 127.0.0.1 -p 25432 -U postgres -v db_user='your_user' -v db_password='your_password' -v db_name='your_db' -f docs/sql/0011_project_db_bootstrap.sql
-- ============================================================

\set ON_ERROR_STOP on

DO $$
DECLARE
    v_db_user text := :'db_user';
    v_db_password text := :'db_password';
    v_db_name text := :'db_name';
BEGIN
    IF coalesce(trim(v_db_user), '') = '' THEN
        RAISE EXCEPTION 'db_user is required';
    END IF;
    IF coalesce(trim(v_db_password), '') = '' THEN
        RAISE EXCEPTION 'db_password is required';
    END IF;
    IF coalesce(trim(v_db_name), '') = '' THEN
        RAISE EXCEPTION 'db_name is required';
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = v_db_user) THEN
        EXECUTE format('CREATE ROLE %I LOGIN PASSWORD %L', v_db_user, v_db_password);
    ELSE
        EXECUTE format('ALTER ROLE %I WITH LOGIN PASSWORD %L', v_db_user, v_db_password);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = v_db_name) THEN
        EXECUTE format('CREATE DATABASE %I OWNER %I', v_db_name, v_db_user);
    END IF;

    EXECUTE format('GRANT CONNECT ON DATABASE %I TO %I', v_db_name, v_db_user);
END
$$;

\connect :db_name

GRANT USAGE, CREATE ON SCHEMA public TO :db_user;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO :db_user;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO :db_user;
GRANT ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public TO :db_user;

ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON TABLES TO :db_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON SEQUENCES TO :db_user;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL PRIVILEGES ON FUNCTIONS TO :db_user;
