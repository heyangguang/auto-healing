# 定时任务调度

## 概述

定时任务调度模块用于配置任务模板的定时执行计划。支持两种调度模式：

- **Cron 模式**：使用 Cron 表达式进行循环执行
- **Once 模式**：在指定时间点执行一次

---

## 模块关系

```
任务模板 (ExecutionTask)
    │ 1:N
    ▼
定时调度 (ExecutionSchedule)
    │ 触发时创建
    ▼
执行记录 (ExecutionRun)
```

一个任务模板可以关联多个定时调度，支持不同时间策略的执行计划。

---

## 调度模式

### Cron 模式（循环调度）

使用 Cron 表达式按周期自动执行。适用于定期任务，如每日备份、每周巡检等。

### Once 模式（单次调度）

在指定时间点执行一次。适用于临时任务，如紧急补丁部署、定时发布等。

---

## 字段说明

### 基础字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 调度 ID |
| `name` | string | 调度名称 |
| `task_id` | UUID | 关联的任务模板 ID |
| `schedule_type` | string | 调度类型：`cron` 或 `once` |
| `schedule_expr` | string | Cron 表达式（仅 cron 模式） |
| `scheduled_at` | datetime | 执行时间点（仅 once 模式） |
| `status` | string | 调度状态（见下表） |
| `next_run_at` | datetime | 下次执行时间（自动计算） |
| `last_run_at` | datetime | 上次执行时间 |
| `enabled` | boolean | 是否启用 |
| `description` | string | 描述 |

### 状态说明

| 状态 | 英文 | 适用模式 | 说明 |
|------|------|----------|------|
| 运行中 | `running` | cron | 循环调度启用中，按计划执行 |
| 待执行 | `pending` | once | 单次调度启用中，等待执行 |
| 已完成 | `completed` | once | 单次调度已执行完毕 |
| 已禁用 | `disabled` | 所有 | 开关关闭 |

### 执行参数覆盖（可选）

> [!TIP]
> 这些字段与手动执行的参数一致，允许每个调度配置不同的执行参数。

| 字段 | 类型 | 说明 |
|------|------|------|
| `target_hosts_override` | string | 覆盖目标主机（逗号分隔） |
| `extra_vars_override` | object | 覆盖变量 |
| `secrets_source_ids` | array | 覆盖密钥源 ID 列表 |
| `skip_notification` | boolean | 跳过通知 |

---

## Cron 表达式格式

```
分钟 小时 日期 月份 星期
 │    │    │    │    │
 │    │    │    │    └── 星期 (0-6, 0=周日)
 │    │    │    └── 月份 (1-12)
 │    │    └── 日期 (1-31)
 │    └── 小时 (0-23)
 └── 分钟 (0-59)
```

**示例:**
- `0 2 * * *` - 每天凌晨 2 点
- `0 */6 * * *` - 每 6 小时
- `30 9 * * 1-5` - 工作日 9:30
- `0 0 1 * *` - 每月 1 号 0 点

---

## API 接口

### 基础路径
```
/api/v1/execution-schedules
```

---

## 1. 创建定时调度

```http
POST /api/v1/execution-schedules
```

### 请求体（Cron 模式）
```json
{
  "name": "每日凌晨备份",
  "task_id": "uuid-task-xxx",
  "schedule_type": "cron",
  "schedule_expr": "0 2 * * *",
  "description": "每天凌晨 2 点执行数据备份"
}
```

### 请求体（Once 模式）
```json
{
  "name": "紧急补丁部署",
  "task_id": "uuid-task-xxx",
  "schedule_type": "once",
  "scheduled_at": "2026-01-10T15:30:00+08:00",
  "description": "在指定时间执行一次"
}
```

### 请求体（带执行参数覆盖）
```json
{
  "name": "生产环境日志清理",
  "task_id": "uuid-task-xxx",
  "schedule_type": "cron",
  "schedule_expr": "0 3 * * *",
  "target_hosts_override": "192.168.1.10,192.168.1.11",
  "extra_vars_override": {
    "retention_days": 7
  },
  "secrets_source_ids": ["uuid-secret-1"],
  "skip_notification": false
}
```

### 响应
```json
{
  "code": 0,
  "data": {
    "id": "uuid-schedule-xxx",
    "name": "每日凌晨备份",
    "task_id": "uuid-task-xxx",
    "schedule_type": "cron",
    "schedule_expr": "0 2 * * *",
    "status": "running",
    "next_run_at": "2026-01-11T02:00:00+08:00",
    "enabled": true
  }
}
```

---

## 2. 获取调度列表

```http
GET /api/v1/execution-schedules
```

### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | integer | 页码，默认 1 |
| `page_size` | integer | 每页条数，默认 20 |
| `search` | string | 模糊搜索（匹配名称或描述） |
| `task_id` | UUID | 按任务模板筛选 |
| `enabled` | boolean | 按启用状态筛选 |
| `schedule_type` | string | 按调度类型筛选：`cron`/`once` |
| `status` | string | 按状态筛选：`running`/`pending`/`completed`/`disabled` |

### 示例

```bash
# 筛选循环调度
GET /api/v1/execution-schedules?schedule_type=cron

# 筛选待执行的单次调度
GET /api/v1/execution-schedules?schedule_type=once&status=pending

# 组合筛选
GET /api/v1/execution-schedules?search=生产&enabled=true
```

---

## 3. 获取调度详情

```http
GET /api/v1/execution-schedules/{id}
```

---

## 4. 更新调度

```http
PUT /api/v1/execution-schedules/{id}
```

### 请求体
```json
{
  "name": "更新后的名称",
  "scheduled_at": "2026-01-15T10:00:00+08:00",
  "description": "修改执行时间"
}
```

> [!NOTE]
> Once 模式可以更新 `scheduled_at` 来重新设置执行时间。

---

## 5. 删除调度

```http
DELETE /api/v1/execution-schedules/{id}
```

> [!IMPORTANT]
> **保护性删除**：调度任务必须先禁用才能删除。

---

## 6. 启用调度

```http
POST /api/v1/execution-schedules/{id}/enable
```

**行为说明：**
- **Cron 模式**：重新计算 `next_run_at`
- **Once 模式**：检查 `scheduled_at` 是否是未来时间
  - 如果已过期，返回错误：`执行时间已过期，请重新设置执行时间`
  - 如果是未来时间，启用成功
  - 如果之前已完成，会重置状态为待执行

---

## 7. 禁用调度

```http
POST /api/v1/execution-schedules/{id}/disable
```

禁用后状态变为 `disabled`，不再自动执行。

---

## 触发执行

调度器每 30 秒检查一次，当 `next_run_at <= 当前时间` 且 `enabled = true` 时：

1. 调用关联的任务模板执行，传递调度中配置的覆盖参数
2. 执行记录的 `triggered_by` 为 `scheduler`
3. **Cron 模式**：更新 `next_run_at` 为下一个周期，保持 `running` 状态
4. **Once 模式**：自动禁用，状态变为 `completed`

---

## 状态转换

### Cron 模式
```
创建(启用) → running
禁用 → disabled
启用 → running（重新计算 next_run_at）
执行完成 → running（更新 next_run_at）
```

### Once 模式
```
创建(启用) → pending
禁用 → disabled
启用 → pending（校验 scheduled_at 必须是未来时间）
执行完成 → completed（自动禁用）
已完成后启用 → pending（重置，需先更新 scheduled_at）
```

---

## 错误场景

| 场景 | 错误提示 |
|------|----------|
| Cron 模式缺少 schedule_expr | `循环调度必须提供 Cron 表达式` |
| Once 模式缺少 scheduled_at | `单次调度必须提供执行时间` |
| Once 创建时 scheduled_at 是过去时间 | `执行时间不能是过去时间` |
| Once 启用时 scheduled_at 已过期 | `执行时间已过期，请重新设置执行时间` |
| 删除未禁用的调度 | `无法删除：调度任务正在启用中，请先禁用再删除` |

---

## 使用场景

### 场景 1：同一任务，不同环境不同调度

```
任务模板: 日志清理
├── 调度1: 生产环境 (cron, 凌晨2点, target_hosts_override=prod)
├── 调度2: 测试环境 (cron, 凌晨3点, target_hosts_override=test)
└── 调度3: 开发环境 (cron, 凌晨4点, skip_notification=true)
```

### 场景 2：紧急临时任务

```
任务模板: 补丁部署
└── 调度: 紧急发布 (once, 2026-01-10T15:30:00+08:00)
```
