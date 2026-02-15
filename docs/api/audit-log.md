# 审计日志 API 文档

> 基础路径: `http://localhost:8080/api/v1`
> 认证方式: `Authorization: Bearer <access_token>`
> 所有接口需要 `audit:list` 权限（导出需要 `audit:export` 权限）

---

## 目录

- [审计日志列表](#1-获取审计日志列表)
- [审计日志详情](#2-获取审计日志详情)
- [统计概览](#3-获取审计统计概览)
- [用户操作排行](#4-获取用户操作排行)
- [操作类型分组](#5-按操作类型分组统计)
- [资源类型统计](#6-获取资源类型统计)
- [操作趋势](#7-获取操作趋势)
- [高危操作日志](#8-获取高危操作日志)
- [导出 CSV](#9-导出审计日志)
- [附录：字段枚举值](#附录字段枚举值)

---

## 通用响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 列表响应（分页）

```json
{
  "code": 0,
  "message": "success",
  "data": [ ... ],
  "total": 115,
  "page": 1,
  "page_size": 20
}
```

### 错误响应

```json
{
  "code": -1,
  "message": "错误描述"
}
```

---

## 1. 获取审计日志列表

> 权限: `audit:list`

```
GET /audit-logs
```

**查询参数:**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页数量 |
| search | string | - | 模糊搜索（用户名/资源名/请求路径等） |
| action | string | - | 按操作类型筛选，见[操作类型枚举](#操作类型-action) |
| resource_type | string | - | 按资源类型筛选，见[资源类型枚举](#资源类型-resource_type) |
| username | string | - | 按用户名筛选 |
| user_id | uuid | - | 按用户 ID 筛选 |
| status | string | - | 按状态筛选：`success` / `failed` |
| risk_level | string | - | 按风险等级筛选：`high` / `normal` |
| created_after | string | - | 创建时间起始（RFC3339 格式，如 `2026-02-01T00:00:00+08:00`） |
| created_before | string | - | 创建时间结束（RFC3339 格式） |
| sort_by | string | created_at | 排序字段 |
| sort_order | string | desc | 排序方向：`asc` / `desc` |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "2652867e-b4f5-4700-9cfa-5783aff61425",
      "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
      "username": "admin",
      "ip_address": "192.168.31.66",
      "user_agent": "Mozilla/5.0 ...",
      "action": "patch",
      "resource_type": "user",
      "resource_id": null,
      "resource_name": "",
      "request_method": "PATCH",
      "request_path": "/api/v1/user/preferences",
      "response_status": 200,
      "status": "success",
      "error_message": "",
      "risk_level": "high",
      "risk_reason": "用户管理操作",
      "created_at": "2026-02-14T03:56:07.957921+08:00",
      "user": {
        "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
        "username": "admin",
        "email": "admin@example.com",
        "display_name": "管理员",
        "status": "active",
        "last_login_at": "2026-02-14T04:05:18.780775+08:00",
        "last_login_ip": "::1",
        "created_at": "2026-01-03T18:04:04.745722+08:00",
        "updated_at": "2026-02-14T04:05:18.781093+08:00"
      }
    }
  ],
  "total": 115,
  "page": 1,
  "page_size": 20
}
```

**列表响应字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | 日志 ID |
| user_id | uuid | 操作用户 ID |
| username | string | 操作用户名 |
| ip_address | string | 客户端 IP 地址 |
| user_agent | string | 客户端 User-Agent |
| action | string | 操作类型 |
| resource_type | string | 资源类型 |
| resource_id | uuid / null | 资源 ID |
| resource_name | string | 资源名称 |
| request_method | string | HTTP 方法 (GET/POST/PUT/PATCH/DELETE) |
| request_path | string | 请求路径 |
| response_status | int | HTTP 响应状态码 |
| status | string | 操作结果：`success` / `failed` |
| error_message | string | 错误信息（失败时有值） |
| risk_level | string | 风险等级：`high` / `normal` |
| risk_reason | string | 风险原因（高危时有值） |
| created_at | string | 创建时间（ISO 8601） |
| user | object | 操作用户详情对象 |

**示例:**

```bash
# 基本列表
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs?page=1&page_size=20"

# 按操作类型筛选
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs?action=delete"

# 按用户 + 时间范围筛选
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs?username=admin&created_after=2026-02-13T00:00:00%2B08:00&created_before=2026-02-14T00:00:00%2B08:00"

# 高危操作 + 失败状态
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs?risk_level=high&status=failed"

# 按资源类型筛选 + 排序
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs?resource_type=users&sort_by=created_at&sort_order=asc"
```

---

## 2. 获取审计日志详情

> 权限: `audit:list`
> 详情比列表多返回 `request_body` 和 `changes` 字段

```
GET /audit-logs/:id
```

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | uuid | 审计日志 ID |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "2652867e-b4f5-4700-9cfa-5783aff61425",
    "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "username": "admin",
    "ip_address": "192.168.31.66",
    "user_agent": "Mozilla/5.0 ...",
    "action": "patch",
    "resource_type": "user",
    "resource_id": null,
    "resource_name": "",
    "request_method": "PATCH",
    "request_path": "/api/v1/user/preferences",
    "request_body": {
      "preferences": {
        "user_list_column_widths": {
          "email": 193,
          "roles": 187,
          "username": 298
        }
      }
    },
    "response_status": 200,
    "changes": null,
    "status": "success",
    "error_message": "",
    "risk_level": "high",
    "risk_reason": "用户管理操作",
    "created_at": "2026-02-14T03:56:07.957921+08:00",
    "user": {
      "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
      "username": "admin",
      "email": "admin@example.com",
      "display_name": "管理员",
      "status": "active",
      "last_login_at": "...",
      "last_login_ip": "::1",
      "created_at": "...",
      "updated_at": "..."
    }
  }
}
```

**详情独有字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| request_body | object / null | 请求体内容（JSON 对象） |
| changes | object / null | 变更内容（如有记录差异） |

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/audit-logs/2652867e-b4f5-4700-9cfa-5783aff61425
```

---

## 3. 获取审计统计概览

> 权限: `audit:list`

```
GET /audit-logs/stats
```

**无查询参数**

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total_count": 115,
    "success_count": 109,
    "failed_count": 6,
    "high_risk_count": 62,
    "today_count": 57,
    "week_count": 115,
    "action_stats": [
      { "action": "patch", "count": 59 },
      { "action": "update", "count": 10 },
      { "action": "batch_create", "count": 9 },
      { "action": "reset_scan", "count": 9 },
      { "action": "create", "count": 7 },
      { "action": "resume", "count": 7 },
      { "action": "maintenance", "count": 7 },
      { "action": "assign_role", "count": 2 },
      { "action": "sync", "count": 2 },
      { "action": "test", "count": 1 },
      { "action": "activate", "count": 1 },
      { "action": "delete", "count": 1 }
    ]
  }
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| total_count | int | 审计日志总数 |
| success_count | int | 成功操作数 |
| failed_count | int | 失败操作数 |
| high_risk_count | int | 高危操作数 |
| today_count | int | 今日操作数 |
| week_count | int | 本周操作数 |
| action_stats | array | 按操作类型统计数组 |
| action_stats[].action | string | 操作类型 |
| action_stats[].count | int | 该操作次数 |

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/audit-logs/stats
```

---

## 4. 获取用户操作排行

> 权限: `audit:list`

```
GET /audit-logs/user-ranking
```

**查询参数:**

| 参数 | 类型 | 默认值 | 范围 | 说明 |
|------|------|--------|------|------|
| limit | int | 10 | 1-100 | 返回排行数量 |
| days | int | 7 | - | 统计天数 |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "rankings": [
      {
        "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
        "username": "admin",
        "count": 115
      }
    ],
    "limit": 10,
    "days": 7
  }
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| rankings | array | 排行列表 |
| rankings[].user_id | uuid | 用户 ID |
| rankings[].username | string | 用户名 |
| rankings[].count | int | 操作次数 |
| limit | int | 返回的排行数量 |
| days | int | 统计的天数 |

**示例:**

```bash
# 最近 7 天 Top 10
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/user-ranking?limit=10&days=7"

# 最近 30 天 Top 5
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/user-ranking?limit=5&days=30"
```

---

## 5. 按操作类型分组统计

> 权限: `audit:list`

```
GET /audit-logs/action-grouping
```

**查询参数:**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| action | string | - | 按操作类型筛选（为空则返回所有操作类型） |
| days | int | 30 | 统计天数 |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "action": "patch",
        "resource_type": "user",
        "username": "admin",
        "count": 59
      },
      {
        "action": "reset_scan",
        "resource_type": "incidents",
        "username": "admin",
        "count": 9
      },
      {
        "action": "update",
        "resource_type": "users",
        "username": "admin",
        "count": 9
      },
      {
        "action": "batch_create",
        "resource_type": "cmdb",
        "username": "admin",
        "count": 9
      }
    ],
    "action": "",
    "days": 30
  }
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| items | array | 分组统计列表 |
| items[].action | string | 操作类型 |
| items[].resource_type | string | 资源类型 |
| items[].username | string | 用户名 |
| items[].count | int | 操作次数 |
| action | string | 筛选的操作类型（空表示全部） |
| days | int | 统计天数 |

**示例:**

```bash
# 所有操作分组（最近 30 天）
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/action-grouping?days=30"

# 仅查看删除操作分组
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/action-grouping?action=delete&days=30"
```

---

## 6. 获取资源类型统计

> 权限: `audit:list`

```
GET /audit-logs/resource-stats
```

**查询参数:**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| days | int | 30 | 统计天数 |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      { "resource_type": "user", "count": 59 },
      { "resource_type": "cmdb", "count": 29 },
      { "resource_type": "users", "count": 13 },
      { "resource_type": "incidents", "count": 9 },
      { "resource_type": "plugins", "count": 4 },
      { "resource_type": "auth", "count": 1 }
    ],
    "days": 30
  }
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| items | array | 资源类型统计列表 |
| items[].resource_type | string | 资源类型 |
| items[].count | int | 操作次数 |
| days | int | 统计天数 |

**示例:**

```bash
# 最近 7 天
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/resource-stats?days=7"

# 最近 30 天
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/resource-stats?days=30"
```

---

## 7. 获取操作趋势

> 权限: `audit:list`

```
GET /audit-logs/trend
```

**查询参数:**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| days | int | 30 | 统计天数（最小值 1） |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      { "date": "2026-02-12", "count": 0 },
      { "date": "2026-02-13", "count": 58 },
      { "date": "2026-02-14", "count": 57 }
    ],
    "days": 3
  }
}
```

**字段说明:**

| 字段 | 类型 | 说明 |
|------|------|------|
| items | array | 每日趋势数据 |
| items[].date | string | 日期 (YYYY-MM-DD) |
| items[].count | int | 当日操作次数 |
| days | int | 统计天数 |

**示例:**

```bash
# 最近 7 天趋势
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/trend?days=7"

# 最近 30 天趋势
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/trend?days=30"
```

---

## 8. 获取高危操作日志

> 权限: `audit:list`
> 仅返回风险等级为 `high` 的操作日志

```
GET /audit-logs/high-risk
```

**查询参数:**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页数量 |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "2652867e-b4f5-4700-9cfa-5783aff61425",
      "username": "admin",
      "action": "patch",
      "resource_type": "user",
      "resource_name": "",
      "status": "success",
      "ip_address": "192.168.31.66",
      "risk_reason": "用户管理操作",
      "created_at": "2026-02-14T03:56:07.957921+08:00",
      "user": {
        "id": "81186ca0-...",
        "username": "admin",
        "email": "admin@example.com",
        "display_name": "管理员",
        "status": "active",
        "last_login_at": "...",
        "last_login_ip": "::1",
        "created_at": "...",
        "updated_at": "..."
      }
    }
  ],
  "total": 62,
  "page": 1,
  "page_size": 20
}
```

**高危列表字段说明（精简版，少于列表）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uuid | 日志 ID |
| username | string | 操作用户名 |
| action | string | 操作类型 |
| resource_type | string | 资源类型 |
| resource_name | string | 资源名称 |
| status | string | 操作结果 |
| ip_address | string | IP 地址 |
| risk_reason | string | 风险原因 |
| created_at | string | 创建时间 |
| user | object | 用户详情 |

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/audit-logs/high-risk?page=1&page_size=20"
```

---

## 9. 导出审计日志

> 权限: `audit:export`
> 返回 CSV 文件（最多导出 10000 条）

```
GET /audit-logs/export
```

**查询参数:** 同[获取审计日志列表](#1-获取审计日志列表)的筛选参数（分页参数无效，固定最多 10000 条）

| 参数 | 类型 | 说明 |
|------|------|------|
| search | string | 模糊搜索 |
| action | string | 按操作类型筛选 |
| resource_type | string | 按资源类型筛选 |
| username | string | 按用户名筛选 |
| status | string | 按状态筛选 |
| risk_level | string | 按风险等级筛选 |
| created_after | string | 创建时间起始（RFC3339） |
| created_before | string | 创建时间结束（RFC3339） |

**响应:**

- **Content-Type**: `text/csv; charset=utf-8`
- **Content-Disposition**: `attachment; filename=audit_logs_20260214_040000.csv`
- 文件带 UTF-8 BOM（Excel 兼容）

**CSV 列头:**

```
时间, 用户, 操作, 资源类型, 资源名称, 请求方法, 请求路径, 状态, 风险等级, IP 地址, 错误信息
```

**示例:**

```bash
# 导出所有日志
curl -H "Authorization: Bearer $TOKEN" \
  -o audit_logs.csv \
  "http://localhost:8080/api/v1/audit-logs/export"

# 导出指定条件的日志
curl -H "Authorization: Bearer $TOKEN" \
  -o audit_logs_delete.csv \
  "http://localhost:8080/api/v1/audit-logs/export?action=delete&status=success"

# 导出指定时间范围
curl -H "Authorization: Bearer $TOKEN" \
  -o audit_logs_range.csv \
  "http://localhost:8080/api/v1/audit-logs/export?created_after=2026-02-01T00:00:00%2B08:00&created_before=2026-02-14T00:00:00%2B08:00"
```

---

## 附录：字段枚举值

### 操作类型 (action)

| 值 | 说明 |
|------|------|
| create | 创建 |
| update | 更新 |
| patch | 部分更新 |
| delete | 删除 |
| batch_create | 批量创建 |
| sync | 同步 |
| test | 测试 |
| activate | 激活 |
| resume | 恢复 |
| maintenance | 维护 |
| reset_scan | 重置扫描 |
| assign_role | 分配角色 |

### 资源类型 (resource_type)

| 值 | 说明 |
|------|------|
| user | 用户（偏好等） |
| users | 用户管理 |
| auth | 认证 |
| cmdb | 资产管理 |
| incidents | 事件管理 |
| plugins | 插件管理 |

### 操作状态 (status)

| 值 | 说明 |
|------|------|
| success | 成功 |
| failed | 失败 |

### 风险等级 (risk_level)

| 值 | 说明 |
|------|------|
| high | 高危 |
| normal | 正常 |

---

## user 嵌套对象结构

所有接口中的 `user` 字段均为以下结构：

```json
{
  "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
  "username": "admin",
  "email": "admin@example.com",
  "display_name": "管理员",
  "status": "active",
  "last_login_at": "2026-02-14T04:05:18.780775+08:00",
  "last_login_ip": "::1",
  "created_at": "2026-01-03T18:04:04.745722+08:00",
  "updated_at": "2026-02-14T04:05:18.781093+08:00"
}
```

---

## 快速获取 Token 脚本

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' \
  | python3 -c "import json,sys;print(json.load(sys.stdin)['access_token'])")

# 后续所有请求使用
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/audit-logs
```

---

## 接口与权限速查表

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | /audit-logs | audit:list | 审计日志列表 |
| GET | /audit-logs/stats | audit:list | 统计概览 |
| GET | /audit-logs/user-ranking | audit:list | 用户操作排行 |
| GET | /audit-logs/action-grouping | audit:list | 操作类型分组 |
| GET | /audit-logs/resource-stats | audit:list | 资源类型统计 |
| GET | /audit-logs/trend | audit:list | 操作趋势 |
| GET | /audit-logs/high-risk | audit:list | 高危操作日志 |
| GET | /audit-logs/export | audit:export | 导出 CSV |
| GET | /audit-logs/:id | audit:list | 审计日志详情 |
