# 定时调度管理 API 文档

**路径前缀**: `/api/v1/execution-schedules`  
**权限**: 已登录用户（租户内数据隔离）

> 定时调度（Schedule）为执行任务模板配置 Cron 表达式，实现自动化定时执行。

---

## 1. 获取调度列表

**GET** `/api/v1/execution-schedules`

**权限**: `task:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称） |
| `name` | string | ❌ | 按名称精确筛选 |
| `task_id` | uuid | ❌ | 按任务模板筛选 |
| `enabled` | bool | ❌ | 是否启用：`true` / `false` |
| `schedule_type` | string | ❌ | 调度类型：`cron` / `interval` / `once` |
| `status` | string | ❌ | 状态：`active` / `paused` / `completed` |
| `skip_notification` | bool | ❌ | 是否跳过通知 |
| `has_overrides` | bool | ❌ | 是否有变量覆盖 |
| `created_from` | string | ❌ | 创建时间起始（RFC3339） |
| `created_to` | string | ❌ | 创建时间结束（RFC3339） |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` / `next_run_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
      {
        "id": "uuid",
        "name": "每日磁盘清理",
        "description": "每天凌晨 2 点执行磁盘清理",
        "task_id": "uuid",
        "task": {"id": "uuid", "name": "磁盘清理任务"},
        "schedule_type": "cron",
        "cron_expression": "0 2 * * *",
        "timezone": "Asia/Shanghai",
        "enabled": true,
        "status": "active",
        "override_vars": {"max_age_days": "7"},
        "override_targets": "",
        "skip_notification": false,
        "last_run_at": "2026-02-18T02:00:00Z",
        "last_run_status": "success",
        "next_run_at": "2026-02-19T02:00:00Z",
        "run_count": 30,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-02-18T10:00:00Z"
      }
  ],
  "total": 10,
  "page": 1,
  "page_size": 20
}
```

---

## 2. 创建调度

**POST** `/api/v1/execution-schedules`

**权限**: `task:create`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 调度名称 |
| `description` | string | ❌ | 描述 |
| `task_id` | uuid | ✅ | 关联的任务模板 ID |
| `schedule_type` | string | ✅ | 调度类型：`cron` / `once` |
| `schedule_expr` | string | ❌ | Cron 表达式（`schedule_type=cron` 时必填） |
| `scheduled_at` | string | ❌ | 执行时间（`schedule_type=once` 时必填，RFC3339） |
| `enabled` | bool | ❌ | 是否启用，默认 true |
| `max_failures` | int | ❌ | 最大连续失败次数，默认 5 |
| `extra_vars_override` | object | ❌ | 覆盖额外变量（键值对） |
| `target_hosts_override` | string | ❌ | 覆盖目标主机 |
| `secrets_source_ids` | []uuid | ❌ | 覆盖密钥源 ID 列表 |
| `skip_notification` | bool | ❌ | 是否跳过通知 |

---

## 3. 获取调度统计

**GET** `/api/v1/execution-schedules/stats`

**权限**: `task:list`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 10,
    "enabled": 8,
    "disabled": 2,
    "by_type": {"cron": 7, "interval": 2, "once": 1}
  }
}
```

---

## 4. 获取调度时间线（可视化）

**GET** `/api/v1/execution-schedules/timeline`

**权限**: `task:list`

返回轻量级调度时间线数据，用于日历/时间轴可视化展示。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `date` | string | ❌ | 日期（`YYYY-MM-DD` 格式），默认为今天 |
| `enabled` | bool | ❌ | 是否启用：`true` / `false` |
| `schedule_type` | string | ❌ | 调度类型：`cron` / `interval` / `once` |

---

## 5. 获取调度详情

**GET** `/api/v1/execution-schedules/:id`

**权限**: `task:detail`

---

## 6. 更新调度

**PUT** `/api/v1/execution-schedules/:id`

**权限**: `task:update`

### 请求体（所有字段可选）

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 名称 |
| `description` | string | 描述 |
| `schedule_type` | string | 调度类型：`cron` / `once` |
| `schedule_expr` | string | Cron 表达式（`schedule_type=cron` 时） |
| `scheduled_at` | string | 执行时间（`schedule_type=once` 时，RFC3339） |
| `max_failures` | int | 最大连续失败次数 |
| `extra_vars_override` | object | 覆盖额外变量（键值对） |
| `target_hosts_override` | string | 覆盖目标主机 |
| `secrets_source_ids` | []uuid | 覆盖密钥源 ID 列表 |
| `skip_notification` | bool | 是否跳过通知 |

---

## 7. 删除调度

**DELETE** `/api/v1/execution-schedules/:id`

**权限**: `task:delete`

---

## 8. 启用调度

**POST** `/api/v1/execution-schedules/:id/enable`

**权限**: `task:update`

---

## 9. 禁用调度

**POST** `/api/v1/execution-schedules/:id/disable`

**权限**: `task:update`

---

## Cron 表达式说明

格式：`分 时 日 月 周`

| 示例 | 说明 |
|------|------|
| `0 2 * * *` | 每天凌晨 2:00 |
| `0 */6 * * *` | 每 6 小时 |
| `0 9 * * 1-5` | 工作日上午 9:00 |
| `0 0 1 * *` | 每月 1 日 0:00 |
| `*/5 * * * *` | 每 5 分钟 |
