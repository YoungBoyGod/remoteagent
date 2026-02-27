# Infra 基础设施服务

本目录包含 RemoteAgent 所需的所有基础设施服务，使用 Docker Compose 管理。

## 目录结构

```
infra/
├── docker-compose.yml              # 主基础设施配置
├── docker-compose.test.yml         # 测试环境配置
├── .env.example                    # 环境变量模板
├── README.md                       # 本文件
├── data/                           # 数据持久化目录
│   ├── postgres/                   # PostgreSQL 数据
│   ├── redis/                      # Redis 数据
│   ├── mongo/                      # MongoDB 数据
│   ├── elasticsearch/              # Elasticsearch 数据
│   ├── graylog/                    # Graylog 数据
│   ├── rustfs/                     # RustFS 对象存储
│   ├── prometheus/                 # Prometheus 数据
│   └── grafana/                    # Grafana 配置
└── monitoring/                     # 监控配置
    ├── prometheus/
    │   └── prometheus.yml          # Prometheus 配置
    └── grafana/
        └── provisioning/           # Grafana 自动配置
```

## 包含的服务

| 服务 | 端口 | 说明 |
|------|------|------|
| PostgreSQL | 25432 | 主数据库 |
| Redis | 26379 | 缓存和消息队列 |
| MinIO API | 29000 | S3 兼容对象存储 |
| MinIO Console | 29001 | MinIO 管理界面 |
| MeiliSearch | 27700 | 全文搜索引擎 |
| Prometheus | 29090 | 监控和告警 |
| Grafana | 23000 | 可视化监控面板 |

## 快速开始

### 1. 初始化配置

```bash
cd infra
cp .env.example .env
# 编辑 .env 文件，修改密码和端口配置
```

### 2. 创建数据目录

```bash
mkdir -p data/{postgres,redis,mongo,elasticsearch,graylog,rustfs,prometheus,grafana}
```

### 3. 启动服务

```bash
# 使用主配置启动
docker-compose up -d

# 或使用特定配置
docker-compose -f docker-compose.infra.yml up -d
```

### 4. 查看状态

```bash
docker-compose ps
docker-compose logs -f [service-name]
```

### 5. 停止服务

```bash
docker-compose down
# 删除数据卷（谨慎使用）
docker-compose down -v
```

## 服务访问

- **PostgreSQL**: `localhost:25432`
- **Redis**: `localhost:26379`
- **Graylog**: http://localhost:29000 (admin/admin)
- **MinIO Console**: http://localhost:29001 (rustfs/rustfs)
- **Prometheus**: http://localhost:29090
- **Grafana**: http://localhost:23000 (admin/admin)

## 配置文件说明

### docker-compose.yml
主配置文件，包含完整的 Infra 服务栈：
- PostgreSQL + Redis
- MongoDB + Elasticsearch + Graylog
- RustFS (S3 存储)
- Prometheus + Grafana + Exporters

### docker-compose.infra.yml
仅包含基础设施服务，适用于独立部署 Infra 的场景。

### docker-compose.agents.yml
用于部署 Agent 集群，配合 Server 使用。

### docker-compose.allinone.yml
一体化部署，包含 Infra + Server + Frontend，适用于单机演示。

### docker-compose.test.yml
测试环境配置，使用轻量级服务。

## 注意事项

1. **端口冲突**: 如果端口被占用，修改 `.env` 文件中的端口配置
2. **内存要求**: Graylog + Elasticsearch 至少需要 4GB 内存
3. **数据备份**: 定期备份 `data/` 目录
4. **生产环境**: 务必修改所有默认密码

## 故障排查

```bash
# 查看日志
docker-compose logs [service-name]

# 检查端口占用
netstat -tlnp | grep [port]

# 修复权限
sudo chown -R 1000:1000 data/elasticsearch
sudo chown -R 999:999 data/postgres
```
