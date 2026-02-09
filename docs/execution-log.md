# 任务执行日志

## 概述

任务执行日志（Execution Log）记录了每条执行记录的详细输出，包括 Ansible 执行过程中的每个步骤、主机状态和错误信息。

### 数据层级关系

```
任务模板 (ExecutionTask)
    │
    └── 执行记录 (ExecutionRun)
            │ 1:N
            └── 执行日志 (ExecutionLog) ← 本文档
```

---

## 数据模型

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 日志 ID |
| `run_id` | UUID | 关联的执行记录 ID |
| `sequence` | int | 日志序号（顺序） |
| `log_level` | string | 日志级别 |
| `stage` | string | 执行阶段 |
| `message` | string | 日志内容 |
| `host` | string | 目标主机（如适用） |
| `task_name` | string | Ansible 任务名称 |
| `play_name` | string | Ansible Play 名称 |
| `details` | object | 详细信息（JSON） |
| `created_at` | datetime | 创建时间 |

---

## 日志级别 (log_level)

| 级别 | 说明 | 前端颜色建议 |
|------|------|-------------|
| `info` | 一般信息 | 白色 |
| `ok` | 任务成功 | 绿色 |
| `changed` | 状态变更 | 黄色 |
| `skipping` | 任务跳过 | 青色 |
| `failed` | 任务失败 | 红色 |
| `fatal` | 致命错误 | 红色加粗 |
| `unreachable` | 主机不可达 | 红色 |
| `debug` | 调试信息 | 灰色 |
| `warn` | 警告信息 | 橙色 |
| `error` | 错误信息 | 红色 |

---

## 执行阶段 (stage)

| 阶段 | 说明 |
|------|------|
| `prepare` | 准备阶段（环境检查、文件准备） |
| `execute` | 执行阶段（Ansible 实际执行） |
| `cleanup` | 清理阶段（临时文件清理） |

---

## API 接口

### 基础路径
```
/api/v1/execution-runs/{run_id}/logs
```

---

## 1. 获取执行日志列表

获取指定执行记录的所有日志。

```http
GET /api/v1/execution-runs/{id}/logs
```

**响应:**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "log-001",
      "run_id": "run-12345678",
      "sequence": 1,
      "log_level": "info",
      "stage": "prepare",
      "message": "开始准备执行环境",
      "host": "",
      "task_name": "",
      "play_name": "",
      "created_at": "2026-01-08T19:05:00Z"
    },
    {
      "id": "log-002",
      "run_id": "run-12345678",
      "sequence": 2,
      "log_level": "info",
      "stage": "execute",
      "message": "PLAY [基础环境检查] ***",
      "host": "",
      "task_name": "",
      "play_name": "基础环境检查",
      "created_at": "2026-01-08T19:05:01Z"
    },
    {
      "id": "log-003",
      "run_id": "run-12345678",
      "sequence": 3,
      "log_level": "ok",
      "stage": "execute",
      "message": "ok: [192.168.1.10]",
      "host": "192.168.1.10",
      "task_name": "检查磁盘空间",
      "play_name": "基础环境检查",
      "created_at": "2026-01-08T19:05:02Z"
    }
  ]
}
```

---

## 2. 实时日志流（SSE）

使用 Server-Sent Events (SSE) 获取实时日志推送。

```http
GET /api/v1/execution-runs/{id}/stream?token=YOUR_JWT_TOKEN
Accept: text/event-stream
```

> [!IMPORTANT]
> 由于浏览器 EventSource API 不支持自定义请求头，需要通过 URL query 参数 `token` 传递 JWT Token。

### SSE 事件类型

| 事件 | 说明 | 数据结构 |
|------|------|---------| 
| `log` | 日志行 | ExecutionLog 对象 |
| `done` | 执行完成 | status, exit_code, stats |
| `error` | 错误 | 错误信息 |

### 示例事件流

```
event: log
data: {"sequence": 1, "log_level": "info", "stage": "prepare", "message": "开始准备执行环境"}

event: log
data: {"sequence": 2, "log_level": "info", "stage": "execute", "message": "PLAY [Deploy] ***"}

event: log
data: {"sequence": 3, "log_level": "ok", "host": "192.168.1.10", "message": "ok: [192.168.1.10]"}

event: log
data: {"sequence": 4, "log_level": "changed", "host": "192.168.1.10", "message": "changed: [192.168.1.10]"}

event: done
data: {"status": "success", "exit_code": 0, "stats": {"ok": 10, "changed": 3, "failed": 0}}
```

---

## 前端集成示例

### 使用 EventSource 实时显示日志

```javascript
// 获取 JWT Token（假设存储在 localStorage）
const token = localStorage.getItem('token');

// 创建 SSE 连接，通过 URL 参数传递 token
const eventSource = new EventSource(`/api/v1/execution-runs/${runId}/stream?token=${token}`);

// 监听日志事件
eventSource.addEventListener('log', (event) => {
  const log = JSON.parse(event.data);
  appendLogLine(log);
});

// 监听完成事件
eventSource.addEventListener('done', (event) => {
  const result = JSON.parse(event.data);
  showExecutionResult(result);
  eventSource.close();
});

// 监听错误
eventSource.addEventListener('error', (event) => {
  console.error('SSE 连接错误:', event);
  eventSource.close();
});
```

### 日志渲染函数示例

```javascript
function appendLogLine(log) {
  const colorMap = {
    'ok': 'text-green-500',
    'changed': 'text-yellow-500',
    'failed': 'text-red-500',
    'fatal': 'text-red-600 font-bold',
    'unreachable': 'text-red-500',
    'skipping': 'text-cyan-500',
    'info': 'text-gray-300',
    'debug': 'text-gray-500',
    'warn': 'text-orange-500',
    'error': 'text-red-500'
  };
  
  const colorClass = colorMap[log.log_level] || 'text-gray-300';
  
  const line = document.createElement('div');
  line.className = colorClass;
  line.textContent = log.message;
  
  logContainer.appendChild(line);
  logContainer.scrollTop = logContainer.scrollHeight;
}
```

---

## 日志过滤和搜索

前端可以基于返回的日志数据进行客户端过滤：

| 过滤条件 | 字段 | 说明 |
|---------|------|------|
| 按主机 | `host` | 只显示特定主机的日志 |
| 按级别 | `log_level` | 过滤错误/警告等 |
| 按阶段 | `stage` | 只看 prepare/execute/cleanup |
| 搜索 | `message` | 关键字搜索 |

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /execution-runs/{id}/logs | execution:list |
| GET /execution-runs/{id}/stream | execution:list |

---

## 相关文档

- [任务模板管理](execution-task-template.md) - 创建和管理任务模板
- [任务调度管理](execution-schedule.md) - 定时执行任务
- [任务执行管理](execution-run.md) - 执行记录和状态
