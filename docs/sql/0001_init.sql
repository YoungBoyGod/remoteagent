-- luoyi-server phase1 schema
-- PostgreSQL 14+

begin;

create table if not exists agents (
  agent_id varchar(64) primary key,
  tenant_id varchar(64) not null,
  device_code varchar(128) not null unique,
  agent_version varchar(32) not null,
  status varchar(16) not null default 'unknown',
  hostname varchar(128),
  os varchar(32),
  arch varchar(32),
  ip inet,
  labels jsonb not null default '{}'::jsonb,
  capabilities jsonb not null default '[]'::jsonb,
  heartbeat_interval int not null default 30,
  poll_timeout int not null default 30,
  last_heartbeat_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint chk_agent_status check (status in ('unknown','online','offline'))
);

create table if not exists tasks (
  task_id varchar(64) primary key,
  tenant_id varchar(64) not null,
  agent_id varchar(64) not null references agents(agent_id),
  task_type varchar(32) not null,
  payload jsonb not null,
  status varchar(16) not null,
  attempt int not null default 1,
  leased_until timestamptz,
  created_at timestamptz not null default now(),
  started_at timestamptz,
  finished_at timestamptz,
  constraint chk_task_status check (status in ('pending','running','success','failed','canceled'))
);
create index if not exists idx_tasks_agent_status on tasks(agent_id, status);
create index if not exists idx_tasks_created_at on tasks(created_at);

create table if not exists task_events (
  id bigserial primary key,
  event_id varchar(64) not null unique,
  task_id varchar(64) not null references tasks(task_id),
  agent_id varchar(64) not null,
  event_type varchar(32) not null,
  status varchar(16),
  body jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);
create index if not exists idx_task_events_task_created on task_events(task_id, created_at);

create table if not exists task_results (
  task_id varchar(64) primary key references tasks(task_id),
  exit_code int,
  stdout text,
  stderr text,
  truncated boolean not null default false,
  started_at timestamptz,
  finished_at timestamptz,
  created_at timestamptz not null default now()
);

create table if not exists control_commands (
  command_id varchar(64) primary key,
  agent_id varchar(64) not null references agents(agent_id),
  action varchar(32) not null,
  payload jsonb not null default '{}'::jsonb,
  status varchar(16) not null default 'pending',
  created_at timestamptz not null default now(),
  delivered_at timestamptz,
  acked_at timestamptz,
  constraint chk_control_action check (action in ('refresh_token','shutdown','reload_config')),
  constraint chk_control_status check (status in ('pending','delivered','acked','expired'))
);
create index if not exists idx_control_agent_status on control_commands(agent_id, status, created_at);

commit;
