# Scripts 脚本目录

## 目录结构

```
scripts/
├── dev/
│   ├── start.sh    # 启动开发环境（infra + server + frontend）
│   └── stop.sh     # 停止开发环境
├── server/
│   ├── start.sh    # 单独启动 server
│   └── stop.sh     # 单独停止 server
└── agent/
    ├── start.sh    # 单独启动 agent
    └── stop.sh     # 单独停止 agent
```

## 用法

```bash
# 启动完整开发环境
./scripts/dev/start.sh

# 停止开发环境
./scripts/dev/stop.sh
```

**访问地址：**
- Frontend: http://localhost:3000
- Server API: http://localhost:40001
- Grafana: http://localhost:23000

> 生产环境请使用 `deploy/` 目录下的部署脚本。
