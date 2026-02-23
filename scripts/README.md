# Scripts 脚本目录

## 目录结构

```
scripts/
├── dev/
│   ├── start.sh    # 启动开发环境（server + frontend + agent）
│   ├── start-sf.sh # 启动开发环境（server + frontend）
│   ├── stop-sf.sh  # 停止开发环境（server + frontend）
│   └── stop.sh     # 停止开发环境
├── agent/
│   ├── start.sh        # 二进制启动 agent（dist/agent）
│   ├── stop.sh         # 停止二进制 agent
│   ├── start-local.sh  # 本地源码启动 agent（go run）
│   └── stop-local.sh   # 停止本地源码 agent
├── server/
│   ├── start.sh      # 二进制启动 server（dist/server）
│   ├── start-dev.sh  # 开发模式启动 server（air 热更新）
│   └── stop.sh       # 停止 server
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

# 启动本地 agent（源码 go run）
./scripts/agent/start-local.sh

# 启动 server 开发热更新
./scripts/server/start-dev.sh

# 停止本地 agent
./scripts/agent/stop-local.sh

# 停止开发环境
./scripts/dev/stop.sh
```

**访问地址：**
- Frontend: http://localhost:3000
- Server API: http://localhost:40001
- Grafana: http://localhost:23000

> 生产环境请使用 `deploy/` 目录下的部署脚本。
