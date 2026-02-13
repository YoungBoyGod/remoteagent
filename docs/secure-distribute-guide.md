# SecureRelease 安全分发系统 — 部署与运维指南

> 本文档面向运维工程师与安全管理员，涵盖 GPG 密钥管理、系统部署、安全审计及日常运维全流程。

---

## 1. GPG 密钥准备

### 1.1 企业密钥对生成（RSA-4096）

生成用于文件签名与加密的企业主密钥：

```bash
# 交互式生成（推荐，可设置密码保护）
gpg --full-generate-key
```

交互过程中选择：

| 步骤 | 选项 |
|------|------|
| 密钥类型 | `(1) RSA and RSA` |
| 密钥长度 | `4096` |
| 有效期 | `2y`（建议 2 年，到期前轮换） |
| 真实姓名 | 企业名称，如 `LuoYi SecureRelease` |
| 电子邮件 | 企业安全邮箱，如 `security@example.com` |
| 密码短语 | 设置强密码（>= 16 字符，含大小写+数字+特殊字符） |

验证生成结果：

```bash
# 列出密钥
gpg --list-keys --keyid-format long

# 查看密钥详情（确认算法为 RSA-4096）
gpg --list-keys --with-colons security@example.com
```

导出与备份：

```bash
# 导出公钥（可分发给客户）
gpg --armor --export security@example.com > enterprise-public.asc

# 导出私钥（离线安全存储）
gpg --armor --export-secret-keys security@example.com > enterprise-private.asc

# 设置私钥文件权限
chmod 600 enterprise-private.asc

# 生成吊销证书（密钥泄露时使用）
gpg --gen-revoke security@example.com > enterprise-revoke.asc
chmod 600 enterprise-revoke.asc
```

备份清单：

| 文件 | 用途 | 存储位置 |
|------|------|----------|
| `enterprise-public.asc` | 公钥分发 | 版本控制 / 内部文档 |
| `enterprise-private.asc` | 签名与解密 | 离线加密存储（如 USB 硬件密钥） |
| `enterprise-revoke.asc` | 紧急吊销 | 与私钥分开的离线存储 |

### 1.2 客户公钥导入

```bash
# 导入客户公钥
gpg --import customer-public.asc

# 查看导入结果
gpg --list-keys customer@example.com

# 设置信任级别（交互式）
gpg --edit-key customer@example.com
```

在 `gpg>` 提示符下执行：

```
gpg> trust
```

信任级别选择：

| 级别 | 含义 | 建议场景 |
|------|------|----------|
| 3 - I trust marginally | 边际信任 | 首次合作客户 |
| 4 - I trust fully | 完全信任 | 已验证身份的长期客户 |
| 5 - I trust ultimately | 终极信任 | 仅用于自有密钥 |

建议对已通过线下身份验证的客户设置为 `4 (fully)`，然后输入 `quit` 退出。

验证客户密钥指纹（通过安全渠道确认）：

```bash
gpg --fingerprint customer@example.com
```

### 1.3 密钥轮换策略

| 项目 | 建议 |
|------|------|
| 轮换周期 | 每 12-24 个月，或发生安全事件时立即轮换 |
| 会话密钥 | 每次分发自动生成，无需手动轮换 |
| 企业主密钥 | 到期前 30 天启动轮换流程 |

轮换步骤：

1. 生成新密钥对（按 1.1 步骤）
2. 用旧密钥签署新公钥（建立信任链）：
   ```bash
   gpg --default-key old-key@example.com --sign-key new-key@example.com
   ```
3. 向所有客户分发新公钥
4. 设置过渡期（建议 30 天），期间新旧密钥并行
5. 过渡期结束后吊销旧密钥：
   ```bash
   gpg --import enterprise-revoke.asc
   ```
6. 更新服务器配置中的密钥标识
7. 归档旧密钥（保留用于解密历史文件）

---

## 2. 系统依赖安装

### 2.1 必需依赖

| 依赖 | 最低版本 | 用途 |
|------|----------|------|
| `gpg` (GnuPG) | >= 2.2 | AES-256 对称加密、密钥管理 |
| `aws-cli` | v2 | S3 上传与预签名 URL 生成 |
| `pv` (pipe viewer) | 任意 | 大文件传输进度显示 |
| `sha256sum` / `shasum` | 系统自带 | 文件完整性校验 |
| `jq` | >= 1.6 | JSON 输出解析 |
| `bc` | 系统自带 | 文件大小计算 |

### 2.2 安装命令

#### Ubuntu / Debian

```bash
sudo apt update
sudo apt install -y gnupg2 pv jq bc coreutils

# AWS CLI v2
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install
rm -rf aws awscliv2.zip

# 验证
gpg --version | head -1
aws --version
pv --version | head -1
```

#### CentOS / RHEL

```bash
sudo yum install -y gnupg2 pv jq bc coreutils

# AWS CLI v2
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip
sudo ./aws/install
rm -rf aws awscliv2.zip
```

#### macOS (Homebrew)

```bash
brew install gnupg pv jq coreutils

# AWS CLI v2
brew install awscli

# macOS 使用 shasum 替代 sha256sum，脚本已兼容
# 如需 sha256sum 命令：
# ln -s /usr/local/bin/gsha256sum /usr/local/bin/sha256sum
```

---

## 3. 配置说明

### 3.1 S3 Bucket 配置

```bash
# 创建专用 bucket（建议独立于其他业务）
aws s3 mb s3://secure-releases --region ap-east-1

# 启用版本控制（防误删）
aws s3api put-bucket-versioning \
  --bucket secure-releases \
  --versioning-configuration Status=Enabled

# 启用服务端加密（SSE-S3）
aws s3api put-bucket-encryption \
  --bucket secure-releases \
  --server-side-encryption-configuration '{
    "Rules": [{"ApplyServerSideEncryptionByDefault": {"SSEAlgorithm": "AES256"}}]
  }'

# 禁止公开访问
aws s3api put-public-access-block \
  --bucket secure-releases \
  --public-access-block-configuration '{
    "BlockPublicAcls": true,
    "IgnorePublicAcls": true,
    "BlockPublicPolicy": true,
    "RestrictPublicBuckets": true
  }'

# 设置生命周期策略（90 天后自动清理加密文件）
aws s3api put-bucket-lifecycle-configuration \
  --bucket secure-releases \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "cleanup-encrypted-files",
      "Prefix": "encrypted/",
      "Status": "Enabled",
      "Expiration": {"Days": 90}
    }]
  }'
```

### 3.2 邮件服务配置（可选，用于分发通知）

如需邮件通知客户下载链接，配置 SMTP：

| 变量 | 示例值 | 说明 |
|------|--------|------|
| `SMTP_HOST` | `smtp.example.com` | SMTP 服务器地址 |
| `SMTP_PORT` | `587` | SMTP 端口（TLS） |
| `SMTP_USER` | `noreply@example.com` | 发件人账号 |
| `SMTP_PASSWORD` | `***` | 发件人密码 |
| `SMTP_FROM` | `SecureRelease <noreply@example.com>` | 发件人显示名 |

### 3.3 密钥路径配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `GPG_HOME` | `~/.gnupg` | GPG 密钥环目录 |
| `SESSION_KEY_DIR` | `/var/lib/secure-release/keys` | 会话密钥临时存储目录 |
| `SIGNER_KEY` | `security@example.com` | 签名密钥标识 |

### 3.4 环境变量清单

以下为 `secure-distribute.sh` 和 `s3-upload.sh` 使用的完整环境变量：

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `S3_BUCKET` | `secure-releases` | S3 存储桶名称 |
| `S3_PREFIX` | `encrypted/` | S3 对象键前缀 |
| `S3_REGION` | `ap-east-1` | AWS 区域 |
| `PRESIGN_EXPIRES` | `86400` | 预签名 URL 有效期（秒），默认 24 小时 |
| `SCP_TARGET` | (空) | SCP 备选目标，如 `user@host:/data/` |
| `SCP_URL_BASE` | (空) | SCP 下载基础 URL |
| `AWS_ACCESS_KEY_ID` | (空) | AWS 访问密钥 |
| `AWS_SECRET_ACCESS_KEY` | (空) | AWS 密钥 |
| `AWS_DEFAULT_REGION` | (空) | AWS 默认区域 |

服务端（Go Server）相关环境变量参见 [deployment.md](deployment.md)。

---

## 4. 部署步骤

### 4.1 克隆代码

```bash
git clone <repo-url> remoteagent
cd remoteagent
```

### 4.2 安装依赖

```bash
# 系统依赖（参见第 2 节）
# ...

# 前端依赖
cd frontend && npm ci && cd ..

# Go 依赖（server 和 agent 自动管理）
cd server && go mod download && cd ..
```

### 4.3 配置环境变量

```bash
# 复制环境变量模板
cp .env.example .env

# 编辑配置（至少修改以下项）
# - DB_PASSWORD: 数据库密码
# - REGISTER_TOKEN: Agent 注册令牌
# - AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY: AWS 凭证
```

### 4.4 初始化数据库

```bash
# 使用 Docker Compose 自动初始化
docker compose up -d postgres
# 等待 PostgreSQL 就绪后，初始化脚本自动执行

# 或手动执行
psql -h 127.0.0.1 -p 25432 -U remotegpu_user -d remotegpu \
  -f docs/sql/0001_init.sql

# Phase 2 调度能力（如需要）
psql -h 127.0.0.1 -p 25432 -U remotegpu_user -d remotegpu \
  -f docs/sql/0003_task_preempt_fields.sql
```

### 4.5 启动服务

```bash
# 方式一：Docker Compose（推荐生产环境）
docker compose up -d

# 方式二：本地开发
make dev

# 方式三：单体部署（前端内嵌到 server 二进制）
make server-embed
./dist/server
```

### 4.6 设置脚本权限

```bash
chmod +x scripts/secure-distribute.sh
chmod +x scripts/s3-upload.sh
```

### 4.7 验证部署

```bash
# Server 健康检查
curl -s http://127.0.0.1:40001/healthz | jq

# 加密脚本 dry-run 测试
./scripts/secure-distribute.sh \
  --action encrypt \
  --input /tmp/test-file.txt \
  --json \
  --dry-run

# S3 上传 dry-run 测试
./scripts/s3-upload.sh \
  --file /tmp/test-file.txt \
  --dry-run

# 前端访问
# http://127.0.0.1:80 (Docker) 或 http://127.0.0.1:5173 (开发模式)
```

---

## 5. 安全审计清单

部署完成后，逐项检查以下安全要求：

### 5.1 加密与密钥安全

- [ ] GPG 版本 >= 2.2
  ```bash
  gpg --version | head -1
  # 预期输出: gpg (GnuPG) 2.2.x 或更高
  ```

- [ ] 加密算法为 AES-256（gpg --list-packets 验证）
  ```bash
  # 对任意已加密文件检查
  gpg --list-packets encrypted-file.gpg 2>&1 | grep -i "algo"
  # 预期输出包含: cipher 9 (AES256)
  ```

- [ ] 密钥文件权限 600
  ```bash
  stat -c '%a %n' ~/.gnupg/private-keys-v1.d/*
  stat -c '%a %n' /var/lib/secure-release/keys/*.key
  # 所有文件权限应为 600
  ```

- [ ] 会话密钥随机性（entropy 检查）
  ```bash
  # 检查系统熵池
  cat /proc/sys/kernel/random/entropy_avail
  # 应 >= 256（推荐 >= 1000）

  # 如果熵不足，安装 haveged
  sudo apt install haveged
  sudo systemctl enable --now haveged
  ```

- [ ] GPG 密钥环目录权限正确
  ```bash
  stat -c '%a' ~/.gnupg
  # 应为 700
  ```

### 5.2 传输安全

- [ ] 传输使用 TLS 1.2+
  ```bash
  # 检查 server 是否配置 TLS（生产环境必须通过反向代理启用）
  # Nginx 示例检查:
  nginx -T 2>/dev/null | grep ssl_protocols
  # 预期: ssl_protocols TLSv1.2 TLSv1.3;
  ```

- [ ] S3 bucket 非公开
  ```bash
  aws s3api get-public-access-block --bucket secure-releases
  # 所有 4 项应为 true
  ```

- [ ] 预签名 URL 有效期 <= 24h
  ```bash
  # 检查 s3-upload.sh 默认配置
  grep "EXPIRES=" scripts/s3-upload.sh
  # 默认值: 86400 (24小时)，不应超过此值
  ```

### 5.3 数据安全

- [ ] 日志中无明文密钥
  ```bash
  # 搜索日志中是否泄露密钥内容
  grep -rn "session.key\|passphrase\|password" /var/log/secure-release/ || echo "OK: 无明文密钥"
  ```

- [ ] 数据库中不存储明文会话密钥
  ```bash
  # 检查 distribution 表结构，session_key 字段应存储 hash 而非明文
  psql -c "\d distributions" | grep session_key
  # 字段名应为 session_key_hash
  ```

- [ ] 临时文件加密后清理
  ```bash
  # secure-distribute.sh 使用 trap 自动清理，验证:
  grep "trap cleanup" scripts/secure-distribute.sh
  # 应存在 trap cleanup EXIT INT TERM
  ```

### 5.4 访问控制

- [ ] Server API 使用 Bearer Token 认证
  ```bash
  # 未认证请求应返回 401
  curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:40001/api/v1/agent/heartbeat
  # 预期: 401
  ```

- [ ] 管理接口使用 AdminAuth 保护
  ```bash
  curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:40001/api/v1/debug/stats
  # 预期: 401
  ```

---

## 6. 日常运维

### 6.1 分发操作流程

#### 命令行分发

```bash
# 第一步：加密文件
./scripts/secure-distribute.sh \
  --action encrypt \
  --input /path/to/release-v1.2.3.tar.gz \
  --output /tmp/release-v1.2.3.tar.gz.gpg \
  --password-file /tmp/release-v1.2.3.session.key \
  --recipient customer@example.com \
  --json

# 第二步：上传到 S3 并生成预签名 URL
./scripts/s3-upload.sh \
  --file /tmp/release-v1.2.3.tar.gz.gpg \
  --bucket secure-releases \
  --prefix "encrypted/" \
  --expires 86400

# 第三步：验证加密文件完整性
./scripts/secure-distribute.sh \
  --action verify \
  --input /path/to/release-v1.2.3.tar.gz \
  --output /tmp/release-v1.2.3.tar.gz.gpg

# 第四步：将预签名 URL 和会话密钥通过安全渠道发送给客户
# 注意：URL 和密钥必须通过不同渠道发送（如 URL 走邮件，密钥走即时通讯）
```

#### 通过 Web 界面分发

1. 访问前端页面，进入「安全分发」模块
2. Stage 1: 选择要分发的 Release 包
3. Stage 2: 系统自动执行加密与 SHA-256 校验
4. Stage 3: 自动上传至 S3 并生成预签名下载链接
5. Stage 4: 配置客户信息并确认分发
6. 分发完成后可在「分发记录」中查看状态

#### 通过 API 分发

```bash
# 创建分发任务
curl -X POST http://127.0.0.1:40001/api/v1/distribute \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "file_path": "/path/to/release.tar.gz",
    "customer_name": "Acme Corp",
    "customer_email": "admin@acme.com"
  }'

# 查询分发记录
curl http://127.0.0.1:40001/api/v1/distributions?page=1&page_size=20 \
  -H "Authorization: Bearer <admin-token>"
```

### 6.2 故障排查

#### GPG 错误码对照

| 错误码 | 含义 | 排查方法 |
|--------|------|----------|
| `gpg: decryption failed: Bad session key` | 会话密钥不匹配 | 确认使用正确的 `--password-file` |
| `gpg: decryption failed: No secret key` | 缺少私钥 | 检查 `gpg --list-secret-keys` |
| `gpg: can't open 'file'` | 文件不存在或无权限 | 检查文件路径和权限 |
| `gpg: problem with the agent` | gpg-agent 异常 | 执行 `gpgconf --kill gpg-agent` 后重试 |
| `gpg: public key not found` | 收件人公钥未导入 | 执行 `gpg --import customer-public.asc` |
| `gpg: signing failed: Unusable secret key` | 签名密钥过期或被吊销 | 检查密钥有效期 `gpg --list-keys` |

#### S3 上传失败排查

| 症状 | 可能原因 | 解决方法 |
|------|----------|----------|
| `AccessDenied` | IAM 权限不足 | 检查 IAM 策略，确保有 `s3:PutObject` 和 `s3:GetObject` 权限 |
| `NoSuchBucket` | Bucket 不存在 | 确认 bucket 名称和区域正确 |
| `RequestTimeout` | 网络超时 | 检查网络连通性；脚本内置 3 次指数退避重试 |
| `SlowDown` | 请求频率过高 | 降低并发，等待后重试 |
| 上传中断 | 大文件网络不稳定 | 考虑使用 `aws s3 cp` 的分段上传（自动启用） |

```bash
# 诊断 AWS 连通性
aws sts get-caller-identity
aws s3 ls s3://secure-releases/encrypted/ --summarize

# 检查 bucket 策略
aws s3api get-bucket-policy --bucket secure-releases
```

#### 预签名链接过期处理

```bash
# 重新生成预签名 URL（无需重新上传）
aws s3 presign s3://secure-releases/encrypted/release-v1.2.3.tar.gz.gpg \
  --expires-in 86400

# 或通过 s3-upload.sh 重新上传（会覆盖同名文件）
./scripts/s3-upload.sh \
  --file /tmp/release-v1.2.3.tar.gz.gpg \
  --expires 86400
```

#### 常见系统问题

| 症状 | 排查命令 | 解决方法 |
|------|----------|----------|
| 熵不足导致密钥生成慢 | `cat /proc/sys/kernel/random/entropy_avail` | 安装 `haveged` 或 `rng-tools` |
| 磁盘空间不足 | `df -h /tmp /var/lib/secure-release` | 清理过期加密文件和临时文件 |
| GPG agent 占用内存 | `ps aux \| grep gpg-agent` | `gpgconf --kill gpg-agent` |
| 脚本权限错误 | `ls -la scripts/*.sh` | `chmod +x scripts/*.sh` |

### 6.3 监控建议

#### 关键指标

| 指标 | 监控方式 | 告警阈值 |
|------|----------|----------|
| 分发成功率 | API 返回状态统计 | < 95% 触发告警 |
| 加密耗时 | 脚本执行时间 | > 300s（大文件除外） |
| S3 上传耗时 | s3-upload.sh 日志 | > 600s |
| 预签名 URL 即将过期 | 定时检查 `url_expires_at` | 过期前 2 小时告警 |
| 磁盘使用率 | `df` / Prometheus node_exporter | > 80% |
| 系统熵池 | `/proc/sys/kernel/random/entropy_avail` | < 256 |

#### Prometheus 集成

项目已配置 Prometheus + Grafana（见 `docker-compose.yml`）：

- Prometheus: `http://127.0.0.1:9090`
- Grafana: `http://127.0.0.1:3002` (默认账号 admin/admin)

建议添加的自定义指标：

```
# 分发任务计数
secure_release_distributions_total{status="success|failed"}

# 加密操作耗时
secure_release_encrypt_duration_seconds

# S3 上传耗时
secure_release_s3_upload_duration_seconds

# 活跃预签名 URL 数量
secure_release_active_presigned_urls
```

#### 日志审计

```bash
# 查看最近的分发操作日志
journalctl -u secure-release --since "1 hour ago" | grep -E "encrypt|upload|distribute"

# 检查是否有异常访问
grep "401\|403\|500" /var/log/nginx/access.log | tail -20
```

#### 定期维护任务

| 任务 | 频率 | 命令 |
|------|------|------|
| 清理过期会话密钥 | 每日 | `find /var/lib/secure-release/keys -mtime +7 -delete` |
| 清理本地临时加密文件 | 每日 | `find /tmp -name "*.gpg" -mtime +3 -delete` |
| 检查 GPG 密钥有效期 | 每月 | `gpg --list-keys --with-colons \| grep "^pub" \| cut -d: -f7` |
| 验证 S3 bucket 安全配置 | 每月 | 重新执行第 5 节审计清单 |
| 数据库备份 | 每日 | `pg_dump -h 127.0.0.1 -p 25432 -U remotegpu_user remotegpu > backup.sql` |
