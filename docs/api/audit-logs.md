# 审计日志 API 接口文档

> 基路径：`/api/v1/audit-logs`  
> 认证方式：`Authorization: Bearer <token>`  
> 权限要求：`audit:list`（查询类）、`audit:export`（导出）

---

## 统一响应格式

所有接口遵循统一响应结构：

```json
{
  "code": 0,          // 0=成功，非0=失败
  "message": "success",
  "data": {},          // 数据（对象或数组）
  "total": 100,        // 仅分页接口返回
  "page": 1,           // 仅分页接口返回
  "page_size": 20      // 仅分页接口返回
}
```

---

## 1. 审计日志列表

```
GET /api/v1/audit-logs
```

### 请求参数（Query）

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `page` | int | 否 | `1` | 页码 |
| `page_size` | int | 否 | `20` | 每页条数 |
| `action` | string | 否 | — | 过滤操作类型：`create` / `update` / `delete` / `execute` / `activate` / `deactivate` / `assign_role` / `reset_password` 等 |
| `resource_type` | string | 否 | — | 过滤资源类型：`users` / `roles` / `plugins` / `playbooks` / `executions` / `cmdb` / `secrets` / `notifications` / `healing` 等 |
| `username` | string | 否 | — | 精确匹配用户名 |
| `user_id` | uuid | 否 | — | 精确匹配用户 ID |
| `status` | string | 否 | — | 过滤状态：`success` / `failed` |
| `risk_level` | string | 否 | — | 过滤风险等级：`high` / `normal` |
| `search` | string | 否 | — | 模糊搜索（匹配 username / resource_name / request_path） |
| `created_after` | string | 否 | — | 开始时间（RFC3339 格式，如 `2026-02-01T00:00:00+08:00`）|
| `created_before` | string | 否 | — | 结束时间（RFC3339 格式）|
| `sort_by` | string | 否 | `created_at` | 排序字段：`created_at` / `action` / `resource_type` / `username` / `status` |
| `sort_order` | string | 否 | `desc` | 排序方向：`asc` / `desc` |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "d4cfb1e7-4a03-4b9a-b033-36e34429d529",
      "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
      "username": "admin",
      "ip_address": "::1",
      "user_agent": "curl/7.61.1",
      "action": "update",
      "resource_type": "auth",
      "resource_id": null,
      "resource_name": "",
      "request_method": "PUT",
      "request_path": "/api/v1/auth/profile",
      "response_status": 200,
      "status": "success",
      "error_message": "",
      "risk_level": "normal",
      "risk_reason": "",
      "created_at": "2026-02-13T03:46:15.932198+08:00",
      "user": {
        "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
        "username": "admin",
        "email": "admin@example.com",
        "display_name": "管理员",
        "status": "active",
        "last_login_at": "2026-02-13T03:47:38.000654+08:00",
        "last_login_ip": "::1",
        "created_at": "2026-01-03T18:04:04.745722+08:00",
        "updated_at": "2026-02-13T03:47:38.001036+08:00"
      }
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

### 列表项字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 审计日志 ID |
| `user_id` | uuid \| null | 操作用户 ID |
| `username` | string | 操作用户名 |
| `ip_address` | string | 客户端 IP |
| `user_agent` | string | 客户端 User-Agent |
| `action` | string | 操作类型（见下方枚举值） |
| `resource_type` | string | 资源类型 |
| `resource_id` | uuid \| null | 资源 ID |
| `resource_name` | string | 资源名称 |
| `request_method` | string | HTTP 方法：`POST` / `PUT` / `DELETE` / `PATCH` |
| `request_path` | string | 请求路径 |
| `response_status` | int | HTTP 响应状态码 |
| `status` | string | 操作结果：`success` / `failed` |
| `error_message` | string | 错误信息（失败时） |
| `risk_level` | string | **风险等级：`high` / `normal`** |
| `risk_reason` | string | **风险原因（如 "删除操作"、"用户管理操作"）** |
| `created_at` | string | 操作时间（RFC3339） |
| `user` | object \| null | 关联的用户对象 |

---

## 2. 审计日志详情

```
GET /api/v1/audit-logs/:id
```

### 响应示例

比列表项多出以下字段：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "d4cfb1e7-4a03-4b9a-b033-36e34429d529",
    "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "username": "admin",
    "ip_address": "::1",
    "user_agent": "curl/7.61.1",
    "action": "update",
    "resource_type": "auth",
    "resource_id": null,
    "resource_name": "",
    "request_method": "PUT",
    "request_path": "/api/v1/auth/profile",
    "request_body": {
      "display_name": "管理员"
    },
    "response_status": 200,
    "changes": null,
    "status": "success",
    "error_message": "",
    "risk_level": "normal",
    "risk_reason": "",
    "created_at": "2026-02-13T03:46:15.932198+08:00",
    "user": { "...同上..." }
  }
}
```

### 详情额外字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `request_body` | object \| null | 请求体内容（JSON，最大 10KB） |
| `changes` | object \| null | 变更详情（预留字段） |

---

## 3. 统计概览

```
GET /api/v1/audit-logs/stats
```

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_count": 1,
    "success_count": 1,
    "failed_count": 0,
    "high_risk_count": 0,
    "action_stats": [
      { "action": "update", "count": 1 },
      { "action": "create", "count": 0 }
    ],
    "today_count": 1,
    "week_count": 1
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `total_count` | int | 全部记录总数 |
| `success_count` | int | 成功操作数 |
| `failed_count` | int | 失败操作数 |
| `high_risk_count` | int | 高危操作数 |
| `action_stats` | array | 按操作类型分组统计（降序） |
| `action_stats[].action` | string | 操作类型 |
| `action_stats[].count` | int | 对应操作次数 |
| `today_count` | int | 今日操作数 |
| `week_count` | int | 本周操作数 |

---

## 4. 用户操作排行榜

```
GET /api/v1/audit-logs/user-ranking
```

### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `limit` | int | 否 | `10` | 返回条数（最大 100） |
| `days` | int | 否 | `7` | 统计最近天数（0=全部） |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "rankings": [
      {
        "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
        "username": "admin",
        "count": 42
      },
      {
        "user_id": "...",
        "username": "operator",
        "count": 18
      }
    ],
    "limit": 10,
    "days": 7
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `rankings` | array | 排行列表（按 count 降序） |
| `rankings[].user_id` | string | 用户 ID |
| `rankings[].username` | string | 用户名 |
| `rankings[].count` | int | 操作次数 |
| `limit` | int | 返回条数限制 |
| `days` | int | 统计天数范围 |

---

## 5. 操作分组统计

```
GET /api/v1/audit-logs/action-grouping
```

### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `action` | string | 否 | — | 过滤特定操作（如 `delete`），不传则返回全部操作的分组 |
| `days` | int | 否 | `30` | 统计最近天数（0=全部） |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "action": "delete",
        "resource_type": "plugins",
        "username": "admin",
        "count": 5
      },
      {
        "action": "delete",
        "resource_type": "users",
        "username": "operator",
        "count": 2
      }
    ],
    "action": "delete",
    "days": 30
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `items` | array \| null | 分组列表（按 count 降序），无数据时为 `null` |
| `items[].action` | string | 操作类型 |
| `items[].resource_type` | string | 资源类型 |
| `items[].username` | string | 用户名 |
| `items[].count` | int | 操作次数 |
| `action` | string | 过滤的操作类型 |
| `days` | int | 统计天数范围 |

---

## 6. 资源类型统计

```
GET /api/v1/audit-logs/resource-stats
```

### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `days` | int | 否 | `30` | 统计最近天数（0=全部） |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      { "resource_type": "auth", "count": 15 },
      { "resource_type": "users", "count": 8 },
      { "resource_type": "plugins", "count": 3 }
    ],
    "days": 30
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `items` | array | 资源类型列表（按 count 降序） |
| `items[].resource_type` | string | 资源类型名 |
| `items[].count` | int | 操作次数 |
| `days` | int | 统计天数范围 |

---

## 7. 操作趋势

```
GET /api/v1/audit-logs/trend
```

### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `days` | int | 否 | `30` | 统计最近天数 |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      { "date": "2026-02-11", "count": 5 },
      { "date": "2026-02-12", "count": 12 },
      { "date": "2026-02-13", "count": 1 }
    ],
    "days": 7
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `items` | array | 每日数据（按日期升序） |
| `items[].date` | string | 日期（`YYYY-MM-DD`） |
| `items[].count` | int | 当日操作数 |
| `days` | int | 统计天数范围 |

> **注意**：无操作的日期不会出现在 items 中，前端需自行补 0。

---

## 8. 高危操作日志

```
GET /api/v1/audit-logs/high-risk
```

### 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `page` | int | 否 | `1` | 页码 |
| `page_size` | int | 否 | `20` | 每页条数 |

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "...",
      "username": "admin",
      "action": "delete",
      "resource_type": "users",
      "resource_name": "testuser",
      "status": "success",
      "ip_address": "192.168.1.100",
      "risk_reason": "删除操作",
      "created_at": "2026-02-13T10:30:00+08:00",
      "user": { "...同上..." }
    }
  ],
  "total": 0,
  "page": 1,
  "page_size": 5
}
```

### 高危判定规则

| 类别 | 触发条件 | risk_reason |
|------|----------|-------------|
| 高危操作 | `action = delete` | 删除操作 |
| 高危操作 | `action = reset_password` | 重置密码 |
| 高危操作 | `action = assign_role` | 角色分配变更 |
| 高危操作 | `action = deactivate` | 停用操作 |
| 高危资源 | `resource_type = user` | 用户管理操作 |
| 高危资源 | `resource_type = role` | 角色管理操作 |

---

## 9. 导出 CSV

```
GET /api/v1/audit-logs/export
```

权限要求：`audit:export`

### 请求参数

支持与**列表接口**相同的过滤参数（`action`, `resource_type`, `username`, `status`, `risk_level`, `search`, `created_after`, `created_before`），最多导出 10000 条。

### 响应

- Content-Type: `text/csv; charset=utf-8`
- Content-Disposition: `attachment; filename=audit_logs_20260213_150405.csv`
- 含 UTF-8 BOM（兼容 Excel 打开中文）

### CSV 列

| 列名 | 说明 |
|------|------|
| 时间 | `YYYY-MM-DD HH:mm:ss` |
| 用户 | 用户名 |
| 操作 | action 值 |
| 资源类型 | resource_type 值 |
| 资源名称 | resource_name 值 |
| 请求方法 | GET/POST/PUT/DELETE |
| 请求路径 | 如 `/api/v1/users/xxx` |
| 状态 | 成功 / 失败 |
| 风险等级 | 正常 / 高危 |
| IP 地址 | 客户端 IP |
| 错误信息 | 失败时的错误描述 |

---

## 附录：枚举值参考

### action 操作类型

| 值 | 说明 | 来源 |
|----|------|------|
| `create` | 创建 | POST 请求 |
| `update` | 更新 | PUT 请求 |
| `patch` | 部分更新 | PATCH 请求 |
| `delete` | 删除 | DELETE 请求 |
| `activate` | 激活 | POST .../activate |
| `deactivate` | 停用 | POST .../deactivate |
| `enable` | 启用 | POST .../enable |
| `disable` | 禁用 | POST .../disable |
| `execute` | 执行 | POST .../execute |
| `test` | 测试 | POST .../test |
| `sync` | 同步 | POST .../sync |
| `approve` | 审批通过 | POST .../approve |
| `reject` | 审批拒绝 | POST .../reject |
| `cancel` | 取消 | POST .../cancel |
| `retry` | 重试 | POST .../retry |
| `reset_password` | 重置密码 | POST .../reset-password |
| `assign_role` | 分配角色 | PUT .../roles |
| `assign_permission` | 分配权限 | PUT .../permissions |
| `trigger` | 触发 | POST .../trigger |
| `dry_run` | 试运行 | POST .../dry-run |
| `scan` | 扫描 | POST .../scan |
| `send` | 发送 | POST .../send |
| `preview` | 预览 | POST .../preview |
| `maintenance` | 维护 | POST .../maintenance |
| `resume` | 恢复 | POST .../resume |

### resource_type 资源类型

由 URL 路径第一段自动推断，常见值：

| 值 | 说明 |
|----|------|
| `auth` | 认证相关 |
| `users` | 用户管理 |
| `roles` | 角色管理 |
| `permissions` | 权限管理 |
| `plugins` | 插件管理 |
| `cmdb` | 资产管理 |
| `secrets` | 密钥管理 |
| `git-repos` | Git 仓库 |
| `playbooks` | Playbook |
| `executions` | 执行管理 |
| `schedules` | 计划任务 |
| `notifications` | 通知管理 |
| `healing` | 自愈规则 |
| `incidents` | 工单管理 |
| `dashboard` | 仪表盘 |

### status 状态

| 值 | 说明 |
|----|------|
| `success` | 操作成功（HTTP < 400） |
| `failed` | 操作失败（HTTP >= 400） |

### risk_level 风险等级

| 值 | 说明 |
|----|------|
| `high` | 高危操作 |
| `normal` | 普通操作 |
