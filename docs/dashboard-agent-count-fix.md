# Dashboard 在线 Agent 数量显示为 0 排查记录

**日期：** 2026-02-21
**问题：** Dashboard 页面"在线 Agent"卡片始终显示 0，其他数据（任务总数、受管主机、客户数量）均正常。

---

## 一、现象

| 卡片 | 期望值 | 实际显示 |
|------|--------|----------|
| 在线 Agent | 8 / 9 | **0** |
| 任务总数 | 252 | 252 ✓ |
| 受管主机 | 9 | 9 ✓ |
| 客户数量 | 1 | 1 ✓ |

---

## 二、排查过程

### 1. 确认后端 API 数据正确

```bash
curl -s http://localhost:40001/api/v1/dashboard/summary \
  -H "X-Register-Token: dev-register-token"
```

返回：`agent_total: 9, agent_online: 8` — **后端数据正确**。

### 2. 用 Playwright 抓取前端实际网络请求

```python
page.on('response', lambda resp: capture(resp) if 'dashboard/summary' in resp.url else None)
```

前端收到的响应同样是 `agent_online: 8` — **数据传输正确**。

### 3. 读取页面 DOM 元素

```python
page.query_selector_all('.stat-agents span')  # 只返回一个 span，值为 '0'
```

只有一个 `<span>` 被渲染，说明自定义 slot 内容完全没有生效。

### 4. 查看实际 HTML 结构

```python
page.evaluate("document.querySelector('.stat-agents').innerHTML")
```

输出：
```html
<div class="el-statistic__content">
  <!--v-if-->
  <span class="el-statistic__number">0</span>
  <!--v-if-->
</div>
```

**关键发现：** 渲染的是 `el-statistic` 组件自带的 `el-statistic__number`（默认值 0），自定义 slot 内容完全没有插入。

---

## 三、根本原因

模板使用了 `<template #default>` slot：

```html
<!-- 错误写法 -->
<el-statistic title="在线 Agent">
  <template #default>
    <div>
      <span>{{ agentOnlineCount }}</span>
      <span>/ {{ agentTotal }}</span>
    </div>
  </template>
</el-statistic>
```

**Element Plus `el-statistic` 组件没有 `#default` slot。**
其支持的 slot 为：`prefix`、`suffix`、`title`、`formatter`。
传入不存在的 slot 名称时，Vue 静默忽略，组件回退到默认行为——显示 `:value` prop 的值，而 `:value` 未传入，默认为 0。

---

## 四、修复方案

改用 `:value` prop 传入数值，`#suffix` slot 显示总数：

```html
<!-- 正确写法 -->
<el-statistic
  title="在线 Agent"
  :value="agentOnlineCount"
  value-style="color:#4096ff;font-size:32px;font-weight:700"
>
  <template #suffix>
    <span style="font-size:16px;color:var(--el-text-color-secondary)">
      / {{ agentTotal }}
    </span>
  </template>
</el-statistic>
```

---

## 五、验证

Playwright 验证结果：

```
在线 Agent
8 / 9
```

---

## 六、经验总结

- **使用不存在的 slot 名称时，Vue 不会报错**，会静默忽略，容易造成"数据正确但不显示"的假象。
- 排查此类问题的有效手段：直接检查 DOM 的 `innerHTML`，对比组件文档确认 slot 名称。
- `el-statistic` 自定义值显示应使用 `:value` prop，复杂格式用 `#formatter` slot。
