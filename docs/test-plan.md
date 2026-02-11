# RemoteAgent 全面测试计划

## 1. 项目概述

RemoteAgent 是一个远程命令执行管理系统，采用 Client-Server 架构：
- **Server**：控制面，基于 Gin 框架，PostgreSQL 存储，提供 REST API
- **Agent**：设备端，状态机驱动，本地 SQLite 存储，执行远程命令

## 2. 测试范围

### 2.1 单元测试（Unit Test）
对每个模块的函数/方法进行独立测试，使用 mock 隔离外部依赖。

### 2.2 白盒测试（White-box Test）
基于代码内部逻辑，覆盖分支、边界条件、错误路径。

### 2.3 黑盒测试（Black-box Test）
基于 API 规范，不关心内部实现，验证输入输出正确性。

### 2.4 场景测试（Scenario Test）
模拟真实业务流程的端到端测试。

## 3. 测试团队分工

| 角色 | 编号 | 负责模块 | 测试类型 |
|------|------|----------|----------|
| 测试经理 | TM-01 | 全局协调、报告汇总 | 管理 |
| 测试员 | T-01 | Server Controller 层 | 单元测试 |
| 测试员 | T-02 | Server Service 层 | 单元测试 |
| 测试员 | T-03 | Server Store 层 | 单元测试 |
| 测试员 | T-04 | Agent Runtime 核心 | 单元测试 |
| 测试员 | T-05 | Agent Config/Logging/Metrics | 单元测试 |
| 测试员 | T-06 | Server API 黑盒测试 | 黑盒测试 |
| 测试员 | T-07 | 认证与安全测试 | 白盒+黑盒 |
| 测试员 | T-08 | 并发与性能测试 | 白盒测试 |
| 测试员 | T-09 | 端到端场景测试 | 场景测试 |
| 测试员 | T-10 | 边界条件与异常测试 | 白盒测试 |

## 4. 详细测试用例

---

### 4.1 T-01: Server Controller 层单元测试

#### TC-0101: Register 注册接口
- 正常注册：提供完整 agent_id, device_code, labels, capabilities
- 缺少必填字段：agent_id 为空
- 重复注册：同一 agent_id 再次注册
- 无效 JSON body
- 缺少 X-Register-Token header

#### TC-0102: Heartbeat 心跳接口
- 正常心跳：提供 agent_id, metrics, running_tasks
- 无效 Bearer Token
- agent_id 为空
- metrics 字段缺失

#### TC-0103: Poll 轮询接口
- 正常轮询：有待分发任务
- 正常轮询：无任务（超时返回空）
- 缺少 agent_id query 参数
- 无效 token

#### TC-0104: TaskStatus 任务状态上报
- 正常状态上报：running/success/failed/canceled
- 无效状态值
- 重复 event_id（幂等性）
- 缺少必填字段

#### TC-0105: TaskReport 任务结果上报
- 正常结果上报：exit_code, stdout, stderr
- stdout/stderr 超长截断
- 重复上报（幂等性）

#### TC-0106: Debug 调试接口
- dispatch task 正常分发
- dispatch control 正常分发
- state 查询
- 无 admin token 访问

#### TC-0107: Health 健康检查
- GET /healthz 返回 200

---

### 4.2 T-02: Server Service 层单元测试

#### TC-0201: Agent 管理
- RegisterAgent：新注册、重复注册、token 生成
- ValidateToken：有效 token、过期 token、不存在的 token
- Heartbeat：更新心跳时间、更新 running_tasks

#### TC-0202: Task 管理
- UpdateTaskStatus：状态流转 pending→running→success
- UpdateTaskStatus：无效状态流转（如 success→running）
- SaveTaskReport：正常保存、重复保存
- 幂等性：相同 event_id 不重复处理

#### TC-0203: Poll 机制
- EnqueueMessage：正常入队
- WaitForMessage：有消息立即返回
- WaitForMessage：无消息等待超时
- 多 agent 独立队列

#### TC-0204: Dispatch 分发
- DispatchTask：agent 在线
- DispatchTask：agent 不存在
- DispatchControl：正常分发控制命令

---

### 4.3 T-03: Server Store 层单元测试

#### TC-0301: Agent Store
- UpsertAgent：新增 agent 记录
- UpsertAgent：更新已有 agent
- UpdateHeartbeat：更新心跳时间和 running_tasks
- 数据库连接失败处理
- SQL 超时处理（5秒超时）

#### TC-0302: Task Store
- SaveTaskStatus：正常写入 task + task_event
- SaveTaskStatus：重复 event_id 幂等处理
- SaveTaskReport：正常写入 task_result
- SaveTaskReport：更新 task 的 finished_at
- 事务回滚测试

---

### 4.4 T-04: Agent Runtime 核心单元测试

#### TC-0401: 状态机
- INIT → REGISTERING 转换
- REGISTERING → RUNNING 转换
- RUNNING → AUTH_EXPIRED 转换
- RUNNING → DRAINING 转换
- DRAINING → STOPPED 转换
- 无效状态转换拒绝

#### TC-0402: 注册流程
- 正常注册成功
- 注册失败重试（指数退避）
- 注册 token 保存到本地

#### TC-0403: 心跳循环
- 正常心跳发送
- 心跳失败处理
- 心跳间隔配置

#### TC-0404: 轮询循环
- 收到任务消息处理
- 收到控制命令处理
- 轮询超时重试
- 认证过期处理

#### TC-0405: 命令执行
- 正常命令执行（sh -c）
- 命令超时取消
- 命令执行失败（非零退出码）
- 进程组管理
- stdout/stderr 捕获

#### TC-0406: 任务上报
- 状态上报成功
- 结果上报成功
- 上报失败重试
- pending_reports 持久化

#### TC-0407: Transport 层
- HTTP 请求封装
- Bearer Token 注入
- 错误响应解析
- 网络超时处理

---

### 4.5 T-05: Agent Config/Logging/Metrics 单元测试

#### TC-0501: 配置加载
- 加载 base.yaml
- 环境覆盖（dev.yaml）
- 环境变量覆盖
- 配置文件不存在处理
- 无效 YAML 格式处理
- 热重载（SIGHUP）

#### TC-0502: 日志系统
- 文件日志写入
- Graylog GELF 发送
- 日志级别过滤
- 日志轮转

#### TC-0503: 可观测性
- Prometheus metrics 注册
- metrics 端点暴露
- 自定义 metrics 收集

---

### 4.6 T-06: Server API 黑盒测试

#### TC-0601: 完整注册流程
- POST /api/v1/agent/register 正常响应格式验证
- 响应包含 token 字段
- 响应 code=0, message="ok"

#### TC-0602: 心跳 API
- POST /api/v1/agent/heartbeat 正常响应
- 无 Authorization header 返回 401

#### TC-0603: 轮询 API
- GET /api/v1/agent/poll?agent_id=xxx 正常响应
- 超时返回空数据

#### TC-0604: 任务状态 API
- POST /api/v1/agent/task/status 正常响应
- 无效 body 返回 400

#### TC-0605: 任务结果 API
- POST /api/v1/agent/task/report 正常响应

#### TC-0606: Debug API
- POST /api/v1/debug/dispatch/task
- POST /api/v1/debug/dispatch/control
- GET /api/v1/debug/state

#### TC-0607: 响应格式一致性
- 所有接口返回统一 envelope: {code, message, request_id, data}
- request_id 格式验证（req-xxxxxxxxxxxx）

---

### 4.7 T-07: 认证与安全测试

#### TC-0701: AdminAuth 中间件
- 正确 X-Register-Token 通过
- 错误 X-Register-Token 拒绝（401）
- 缺少 X-Register-Token 拒绝
- 空 X-Register-Token 拒绝

#### TC-0702: BearerAuth 中间件
- 正确 Bearer Token 通过
- 过期 Bearer Token 拒绝（401）
- 格式错误的 Authorization header
- 缺少 Authorization header
- 伪造 token

#### TC-0703: Token 生命周期
- Token 生成唯一性
- Token 过期时间正确（JWT_TTL_SECONDS）
- Token 过期后重新注册获取新 token

#### TC-0704: 安全边界
- SQL 注入尝试（agent_id 字段）
- XSS 注入尝试（labels 字段）
- 超大 payload 处理
- 非法 Content-Type

---

### 4.8 T-08: 并发与性能测试

#### TC-0801: 多 Agent 并发注册
- 100 个 agent 同时注册
- 验证所有注册成功且 token 唯一

#### TC-0802: 多 Agent 并发心跳
- 50 个 agent 同时发送心跳
- 验证心跳时间正确更新

#### TC-0803: 并发轮询
- 多个 agent 同时 long-poll
- 任务分发到正确 agent

#### TC-0804: 并发任务上报
- 同一任务多次并发状态上报
- 验证幂等性（event_id 去重）

#### TC-0805: 竞态条件
- 注册和心跳并发
- 任务分发和 agent 下线并发
- Token 过期和请求并发

---

### 4.9 T-09: 端到端场景测试

#### TC-0901: 完整生命周期
1. Agent 注册 → 获取 token
2. 发送心跳 → 确认在线
3. Admin 分发任务 → Agent 轮询获取
4. Agent 上报 running 状态
5. Agent 上报执行结果（exit_code=0）
6. 验证 Server 状态一致

#### TC-0902: Agent 重新注册
1. Agent 注册获取 token
2. Token 过期
3. Agent 重新注册获取新 token
4. 使用新 token 继续工作

#### TC-0903: 任务失败场景
1. 分发一个会失败的命令
2. Agent 执行失败
3. 上报 failed 状态和非零 exit_code
4. 验证 stderr 正确记录

#### TC-0904: 优雅关闭
1. Agent 正在执行任务
2. 发送 shutdown 控制命令
3. Agent 进入 DRAINING 状态
4. 等待任务完成后停止
5. 验证任务结果已上报

#### TC-0905: 任务取消
1. 分发长时间运行的任务
2. 发送 cancel_task 控制命令
3. Agent 取消任务执行
4. 上报 canceled 状态

#### TC-0906: 网络中断恢复
1. Agent 正常运行
2. 模拟网络中断（心跳失败）
3. 网络恢复
4. Agent 自动恢复心跳和轮询

#### TC-0907: 多 Agent 协作
1. 注册 3 个 agent
2. 分别分发不同任务
3. 各 agent 独立执行并上报
4. 验证任务结果互不干扰

---

### 4.10 T-10: 边界条件与异常测试

#### TC-1001: 输入边界
- agent_id 最大长度
- labels 数组最大数量
- capabilities 数组最大数量
- stdout/stderr 最大长度（截断测试）
- command 为空字符串
- timeout 为 0 或负数

#### TC-1002: 状态异常
- 对不存在的 agent 发送心跳
- 对不存在的 task 上报状态
- 对已完成的 task 再次上报
- 对未注册的 agent 分发任务

#### TC-1003: 数据库异常
- 数据库连接断开时的请求处理
- 数据库超时（>5秒）
- 数据库事务冲突

#### TC-1004: 配置异常
- 无效端口号
- 无效数据库连接串
- JWT TTL 为 0
- Poll timeout 为 0

#### TC-1005: 资源限制
- 大量 pending tasks 堆积
- 内存中 agent 数量上限
- 长时间运行的 long-poll 连接数

---

## 5. 测试环境

- Go 1.22+ (agent) / Go 1.23+ (server)
- PostgreSQL 16（使用 go-sqlmock 模拟）
- Docker Compose（集成测试）
- 测试框架：Go testing + httptest + go-sqlmock

## 6. 测试通过标准

- 所有单元测试通过
- 代码覆盖率 ≥ 80%
- 所有 API 黑盒测试通过
- 所有场景测试通过
- 无 P0/P1 级别 bug
- 并发测试无竞态条件

## 7. 缺陷等级定义

| 等级 | 定义 | 示例 |
|------|------|------|
| P0-致命 | 系统崩溃或数据丢失 | panic、数据库写入丢失 |
| P1-严重 | 核心功能不可用 | 注册失败、任务无法分发 |
| P2-一般 | 功能异常但有 workaround | 错误信息不准确 |
| P3-轻微 | 体验问题 | 日志格式不统一 |
