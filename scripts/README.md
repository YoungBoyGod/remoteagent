# Scripts 脚本目录

## 目录结构

```
scripts/
├── dev/
│   ├── start.sh    # 启动 Docker 开发环境（infra + server + frontend）
│   ├── start-sf.sh # 启动 server + frontend（含依赖）
│   ├── stop-sf.sh  # 停止 server + frontend
│   └── stop.sh     # 停止 Docker 开发环境
├── agent/
│   ├── start.sh        # 二进制启动 agent（dist/agent）
│   ├── stop.sh         # 停止二进制 agent
│   ├── start-local.sh  # 本地源码启动 agent（go run）
│   └── stop-local.sh   # 停止本地源码 agent
├── agent-docker/
│   └── start-5-agents.sh # 快速起 5 个 Docker Agent 实例
├── server/
│   ├── start.sh      # 二进制启动 server（dist/server）
│   ├── start-dev.sh  # 开发模式启动 server（air 热更新）
│   ├── stop.sh       # 停止 server
│   └── apply-db-migration.sh # 把 SQL 应用到当前 server 指向的 DB
└── ...
```

## 用法

```bash
# 启动完整开发环境
./scripts/dev/start.sh

# 启动 server + frontend 开发环境
./scripts/dev/start-sf.sh

# 停止 server + frontend 开发环境
./scripts/dev/stop-sf.sh

# 启动本地 agent（源码 go run，可选）
./scripts/agent/start-local.sh

# 启动 server 开发热更新
./scripts/server/start-dev.sh

# 执行数据库迁移（使用 src/server/.env 中的 DB 配置）
./scripts/server/apply-db-migration.sh docs/sql/0011_project_db_bootstrap.sql

# 停止本地 agent
./scripts/agent/stop-local.sh

# 停止开发环境
./scripts/dev/stop.sh
```

**访问地址：**
- Frontend: http://localhost:7000
- Server API: http://localhost:40001
- MinIO Console: http://localhost:29001 (rustfs/rustfs)

> 生产环境请使用 `deploy/` 目录下的部署脚本。
