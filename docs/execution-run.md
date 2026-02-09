# 任务执行管理

## 概述

任务执行（Execution Run）是任务模板的实际运行实例。每次执行任务都会创建一条执行记录，记录执行状态、输出日志和结果。

### 数据层级关系

```
任务模板 (ExecutionTask)
    │ 1:N
    ├── 调度任务 (ExecutionSchedule) ← 需删除按钮
    │
    └── 执行记录 (ExecutionRun) ← 日志类数据，不需删除按钮
            │ 1:N
            └── 执行日志 (ExecutionLog)
```

> [!NOTE]
> **执行记录是日志类数据**，会随任务模板一起级联删除，不需要单独的删除按钮。

---

## 执行生命周期

```
pending（待执行）
    │
    ▼
running（执行中）
    │
    ├──────────────────────┬───────────────────┐
    ▼                      ▼                   ▼
success（成功）       failed（失败）      cancelled（取消）
                                               │
                                               ▼
                                         timeout（超时）
```

---

## 执行记录字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 执行记录 ID |
| `task_id` | UUID | 关联的任务模板 ID |
| `status` | string | 状态 |
| `exit_code` | int | Ansible 退出码 |
| `stats` | object | 执行统计（ok/changed/failed/skipped） |
| `triggered_by` | string | 触发者 |
| `started_at` | datetime | 开始时间 |
| `completed_at` | datetime | 完成时间 |
| `created_at` | datetime | 创建时间 |

---

## 触发来源类型

| triggered_by | 说明 |
|-------------|------|
| `manual` | 手动执行（默认值，前端不传时自动设置） |
| `scheduler:cron` | Cron 周期调度触发 |
| `scheduler:once` | 一次性调度触发 |
| `healing:rule-xxx` | 自愈工作流触发（格式：`healing:规则ID`） |

---

## API 接口

### 基础路径
```
/api/v1/execution-runs
```

---

## 获取执行记录列表

```http
GET /api/v1/execution-runs
```

### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | integer | 页码，默认 1 |
| `page_size` | integer | 每页条数，默认 20 |
| `search` | string | 全局搜索（匹配 ID、triggered_by、任务名） |
| `status` | string | 状态筛选：pending/running/success/failed/cancelled/timeout |
| `task_id` | UUID | 按任务模板筛选 |
| `triggered_by` | string | 触发来源筛选：manual / scheduler:cron / scheduler:once / healing:xxx |
| `started_after` | ISO8601 | 开始时间范围（started_at >= 值） |
| `started_before` | ISO8601 | 结束时间范围（started_at <= 值） |

### 示例

```bash
# 全局搜索（匹配 ID、触发者、任务名）
GET /api/v1/execution-runs?search=81a8

# 筛选失败的执行记录
GET /api/v1/execution-runs?status=failed

# 筛选手动触发的记录
GET /api/v1/execution-runs?triggered_by=manual

# 时间范围筛选
GET /api/v1/execution-runs?started_after=2026-01-10T00:00:00+08:00&started_before=2026-01-10T23:59:59+08:00

# 组合筛选
GET /api/v1/execution-runs?status=failed&triggered_by=scheduler
```

---

## 执行任务

从任务模板创建一条执行记录并开始执行。

```http
POST /api/v1/execution-tasks/{id}/execute
```

**请求体:**
```json
{
  "triggered_by": "admin",
  "secrets_source_ids": ["sec-001"],
  "extra_vars": {"cleanup_threshold": 90},
  "target_hosts": "192.168.1.10,192.168.1.20",
  "skip_notification": false
}
```

| 参数 | 类型 | 说明 |
|------|------|------|
| `triggered_by` | string | 触发者 |
| `secrets_source_ids` | array | 密钥源 ID 列表 |
| `extra_vars` | object | 运行时覆盖变量（与任务模板合并） |
| `target_hosts` | string | 覆盖目标主机（完全替换任务模板配置） |
| `skip_notification` | boolean | 跳过本次通知（全局） |

**响应:**
```json
{
  "code": 0,
  "data": {
    "id": "run-xxx",
    "task_id": "task-xxx",
    "status": "pending"
  }
}
```

---

## 获取任务的执行历史

获取指定任务模板的所有执行记录列表。

```http
GET /api/v1/execution-tasks/{id}/runs?page=1&page_size=20
```

**响应:**
```json
{
  "code": 0,
  "data": [
    {
      "id": "run-001",
      "task_id": "task-xxx",
      "status": "success",
      "triggered_by": "admin",
      "started_at": "2026-01-08T19:05:00Z",
      "completed_at": "2026-01-08T19:07:30Z"
    }
  ],
  "total": 15
}
```

---

## 1. 获取执行记录详情

```http
GET /api/v1/execution-runs/{id}
```

**响应:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "run-12345678",
    "task_id": "a1b2c3d4-5678-90ab-cdef-123456789012",
    "status": "success",
    "exit_code": 0,
    "stats": {
      "ok": 10,
      "changed": 3,
      "failed": 0,
      "skipped": 2,
      "unreachable": 0
    },
    "triggered_by": "admin",
    "started_at": "2026-01-08T19:05:00Z",
    "completed_at": "2026-01-08T19:07:30Z",
    "created_at": "2026-01-08T19:05:00Z",
    "task": {
      "id": "a1b2c3d4...",
      "name": "生产环境日志清理"
    }
  }
}
```

---

## 2. 获取执行日志

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
      "level": "info",
      "message": "PLAY [基础环境检查] ***",
      "host": "",
      "task_name": "",
      "created_at": "2026-01-08T19:05:01Z"
    },
    {
      "id": "log-002",
      "run_id": "run-12345678",
      "sequence": 2,
      "level": "info",
      "message": "TASK [记录开始时间] ***",
      "host": "192.168.1.10",
      "task_name": "记录开始时间",
      "created_at": "2026-01-08T19:05:02Z"
    }
  ]
}
```

---

## 3. 实时日志流（SSE）

```http
GET /api/v1/execution-runs/{id}/stream
Accept: text/event-stream
```

**SSE 事件:**

| 事件类型 | 说明 |
|---------|------|
| `log` | 日志行 |
| `done` | 执行完成 |
| `error` | 错误 |

**示例:**
```
event: log
data: {"sequence": 1, "level": "info", "message": "PLAY [Deploy] ***"}

event: log
data: {"sequence": 2, "level": "ok", "message": "ok: [192.168.1.10]", "host": "192.168.1.10"}

event: done
data: {"status": "success", "exit_code": 0, "stats": {"ok": 10, "changed": 3}}
```

---

## 4. 取消执行

```http
POST /api/v1/execution-runs/{id}/cancel
```

**响应:**
```json
{
  "code": 0,
  "message": "执行已取消"
}
```

---

## 执行状态说明

| 状态 | 说明 |
|------|------|
| `pending` | 等待执行 |
| `running` | 执行中 |
| `success` | 执行成功（exit_code = 0） |
| `failed` | 执行失败（exit_code ≠ 0） |
| `cancelled` | 用户取消 |
| `timeout` | 执行超时 |

---

## Ansible 退出码

| 退出码 | 说明 |
|--------|------|
| 0 | 成功 |
| 1 | 主机错误或一般错误 |
| 2 | 解析错误、无效参数 |
| 3 | 主机不可达 |
| 4 | 执行器内部错误 |

---

## 执行统计（stats）

```json
{
  "ok": 10,        // 成功执行的任务数
  "changed": 3,    // 状态变更的任务数
  "failed": 0,     // 失败的任务数
  "skipped": 2,    // 跳过的任务数
  "unreachable": 0 // 不可达主机数
}
```

---

## 相关文档

- [任务模板管理](execution-task-template.md) - 创建和管理任务模板
- [任务调度管理](execution-schedule.md) - 定时执行任务
- [任务执行日志](execution-log.md) - 日志详情、实时流、前端集成
