# Server 集成测试文档

## 概述

本文档描述 server 端的测试体系，包括单元测试和集成测试的覆盖范围、运行方式和测试用例说明。

## 测试目录结构

```
server/
  internal/
    controller/
      controller_test.go    # controller 层单元测试 (26 个)
    service/
      service_test.go        # service 层单元测试 (4 个)
      export_test.go         # service 层测试辅助导出
    store/
      store_test.go          # store 层单元测试 (9 个)
    integration_test/
      helpers_test.go        # 集成测试公共工具
      lifecycle_test.go      # 完整生命周期集成测试
      auth_test.go           # 认证相关集成测试
      error_test.go          # 异常场景集成测试
      concurrent_test.go     # 并发场景集成测试
```

## 运行方式

```bash
# 运行全部测试
cd server && go test ./... -count=1

# 运行单元测试
go test ./internal/controller/ -v
go test ./internal/service/ -v
go test ./internal/store/ -v

# 运行集成测试
go test ./internal/integration_test/ -v
```

## 测试技术栈

| 组件 | 用途 |
|------|------|
| `testing` | Go 标准测试框架 |
| `net/http/httptest` | HTTP 请求模拟 |
| `gin.TestMode` | Gin 测试模式，抑制日志输出 |
| `go-sqlmock` | 数据库 mock，模拟 SQL 操作 |

## 单元测试用例

### Controller 层 (26 个)

**Health**
| 用例 | 说明 |
|------|------|
| TestHealthHandler_ReturnsOK | 健康检查返回 200 |

**AdminAuth 中间件**
| 用例 | 说明 |
|------|------|
| TestAdminAuth_MissingToken | 缺少 X-Register-Token 返回 401 |
| TestAdminAuth_WrongToken | 错误的 X-Register-Token 返回 401 |

**BearerAuth 中间件**
| 用例 | 说明 |
|------|------|
| TestBearerAuth_MissingHeader | 缺少 Authorization 头返回 401 |
| TestBearerAuth_InvalidFormat | 非 Bearer 格式返回 401 |
| TestBearerAuth_UnknownToken | 未知 token 返回 401 |

**Register**
| 用例 | 说明 |
|------|------|
| TestRegisterHandler_Success | 正常注册返回 token |
| TestRegisterHandler_MissingFields | 缺少必填字段返回 400 |
| TestRegisterHandler_InvalidJSON | 无效 JSON 返回 400 |

**Heartbeat**
| 用例 | 说明 |
|------|------|
| TestHeartbeatHandler_Success | 正常心跳返回 200 |
| TestHeartbeatHandler_AgentIDMismatch | agent_id 与 token 不匹配返回 400 |

**TaskStatus**
| 用例 | 说明 |
|------|------|
| TestTaskStatusHandler_Success | 正常上报任务状态返回 200 |
| TestTaskStatusHandler_MissingFields | 缺少必填字段返回 400 |
| TestTaskStatusHandler_AgentIDMismatch | agent_id 不匹配返回 400 |
| TestTaskStatusHandler_InvalidStatus | 无效状态值返回 400 |

**TaskReport**
| 用例 | 说明 |
|------|------|
| TestTaskReportHandler_Success | 正常上报任务结果返回 200 |
| TestTaskReportHandler_MissingFields | 缺少必填字段返回 400 |
| TestTaskReportHandler_InvalidStatus | 无效 report 状态返回 400 |

**Poll**
| 用例 | 说明 |
|------|------|
| TestPollHandler_AgentIDMismatch | agent_id 不匹配返回 400 |
| TestPollHandler_ReturnsEnqueuedTask | 返回已入队的任务 |

**Debug**
| 用例 | 说明 |
|------|------|
| TestDebugDispatchTaskHandler_Success | 正常下发任务返回 200 |
| TestDebugDispatchTaskHandler_MissingFields | 缺少必填字段返回 400 |
| TestDebugDispatchControlHandler_Success | 正常下发控制指令返回 200 |
| TestDebugDispatchControlHandler_InvalidAction | 无效 action 返回 400 |
| TestDebugDispatchControlHandler_MissingFields | 缺少必填字段返回 400 |
| TestDebugStateHandler_Success | 查看内存状态返回 200 |

### Service 层 (4 个)

| 用例 | 说明 |
|------|------|
| TestProcessTaskStatus_OrderAndStateUpdate | 任务状态上报：DB 写入顺序和内存更新 |
| TestProcessTaskStatus_IdempotentEventSkipsMemoryUpdate | 重复事件幂等：跳过内存更新 |
| TestProcessTaskReport_OrderAndStateUpdate | 任务报告上报：DB 写入顺序和内存更新 |
| TestProcessTaskReport_DuplicateEventSkipsMemoryUpdate | 重复报告幂等：跳过内存更新 |

### Store 层 (9 个)

| 用例 | 说明 |
|------|------|
| TestUpsertAgent_Success | 正常 upsert agent 记录 |
| TestUpsertAgent_DefaultTenant | 空 tenant_id 默认为 "default" |
| TestUpdateHeartbeat_Success | 更新心跳时间 |
| TestInsertTaskEvent_Inserted | 插入新事件返回 true |
| TestInsertTaskEvent_Duplicate | 重复事件返回 false（幂等） |
| TestUpsertTaskStatus_Running | running 状态设置 started_at |
| TestUpsertTaskStatus_Finished | 非 running 状态设置 finished_at |
| TestUpsertTaskStatus_DefaultAttempt | attempt=0 默认为 1 |
| TestUpsertTaskReport_Success | 正常写入 tasks + task_results |

## 集成测试用例 (7 个)

### 完整生命周期 (lifecycle_test.go)

| 用例 | 说明 |
|------|------|
| TestFullLifecycle_RegisterHeartbeatTaskFlow | 注册->心跳->任务下发->轮询->状态上报->结果上报->状态查看 |

### 认证场景 (auth_test.go)

| 用例 | 说明 |
|------|------|
| TestReRegister_SameAgent | 同一 agent 重复注册获得新 token，新旧 token 均有效 |

### 异常场景 (error_test.go)

| 用例 | 说明 |
|------|------|
| TestUnauthorized_NoToken | 所有需认证端点无 token 返回 401 |
| TestAgentIDMismatch_CrossAgent | 用 agent-A 的 token 操作 agent-B 返回 400 |
| TestInvalidTaskStatus_Rejected | 无效任务状态值被拒绝 |
| TestInvalidTaskReportStatus_Rejected | 无效 report 状态值被拒绝 |

### 并发场景 (concurrent_test.go)

| 用例 | 说明 |
|------|------|
| TestConcurrent_MultipleAgentsRegister | 5 个 agent 并发注册，验证线程安全 |

## 测试统计

| 层 | 测试数 | 文件 |
|----|--------|------|
| Controller | 26 | controller_test.go |
| Service | 4 | service_test.go |
| Store | 9 | store_test.go |
| 集成测试 | 7 | integration_test/*.go |
| **总计** | **46** | |
