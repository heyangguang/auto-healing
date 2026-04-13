# 平台审计日志 API 文档

**Base URL**: `/api/v1/platform/audit-logs`  
**权限**: `platform:audit:list`（且需要平台管理员）

> 平台审计日志记录平台级别的操作与全局认证安全事件（如登录、登出、邀请注册），与租户级审计日志分开存储。

---

## 1. 获取平台审计日志列表

**GET** `/api/v1/platform/audit-logs`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（用户名、资源名称、请求路径） |
| `category` | string | ❌ | 操作分类：`login` / `user_management` / `tenant_management` / `platform_settings` |
| `action` | string | ❌ | 操作类型：`create` / `update` / `delete` / `login` / `logout` 等 |
| `resource_type` | string | ❌ | 资源类型：`user` / `tenant` / `role` / `platform_settings` 等 |
| `username` | string | ❌ | 按用户名筛选 |
| `user_id` | uuid | ❌ | 按用户 ID 筛选 |
| `status` | string | ❌ | 操作状态：`success` / `failed` |
| `created_after` | string | ❌ | 开始时间（RFC3339 格式，如 `2026-01-01T00:00:00Z`） |
| `created_before` | string | ❌ | 结束时间（RFC3339 格式） |
| `sort_by` | string | ❌ | 排序字段：`created_at`（默认） |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc`（默认 `desc`） |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
      {
        "id": "uuid",
        "user_id": "uuid",
        "username": "admin",
        "ip_address": "192.168.1.1",
        "user_agent": "Mozilla/5.0...",
        "category": "user_management",
        "action": "create",
        "resource_type": "user",
        "resource_id": "uuid",
        "resource_name": "zhangsan",
        "request_method": "POST",
        "request_path": "/api/v1/platform/users",
        "request_body": "{\"username\":\"zhangsan\",...}",
        "response_status": 201,
        "changes": {
          "before": null,
          "after": {"username": "zhangsan", "email": "zhangsan@example.com"}
        },
        "status": "success",
        "error_message": "",
        "risk_level": "normal",
        "risk_reason": "",
        "created_at": "2026-02-18T10:00:00Z"
      }
  ],
  "total": 500,
  "page": 1,
  "page_size": 20
}
```

### 响应字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 日志 ID |
| `user_id` | uuid | 操作用户 ID |
| `username` | string | 操作用户名 |
| `ip_address` | string | 客户端 IP |
| `user_agent` | string | 浏览器 UA |
| `category` | string | 操作分类 |
| `action` | string | 操作类型 |
| `resource_type` | string | 资源类型 |
| `resource_id` | string | 资源 ID |
| `resource_name` | string | 资源名称 |
| `request_method` | string | HTTP 方法 |
| `request_path` | string | 请求路径 |
| `request_body` | string | 请求体（JSON 字符串） |
| `response_status` | int | HTTP 响应状态码 |
| `changes` | object | 变更内容（before/after） |
| `status` | string | 操作状态：`success` / `failed` |
| `error_message` | string | 错误信息（失败时） |
| `risk_level` | string | 风险等级：`normal` / `high` |
| `risk_reason` | string | 高危原因说明 |
| `created_at` | string | 操作时间 |

---

## 2. 获取平台审计日志详情

**GET** `/api/v1/platform/audit-logs/:id`

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 日志 ID |

### 响应

返回单条日志详情（字段同列表）。

---

## 3. 获取平台审计统计

**GET** `/api/v1/platform/audit-logs/stats`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 5000,
    "today": 120,
    "success": 4800,
    "failed": 200,
    "high_risk": 15,
    "by_category": {
      "login": 1000,
      "user_management": 2000,
      "tenant_management": 500,
      "platform_settings": 300
    }
  }
}
```

---

## 4. 获取平台审计趋势

**GET** `/api/v1/platform/audit-logs/trend`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `days` | int | ❌ | 统计天数，默认 30 |

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "date": "2026-02-18",
        "total": 120,
        "success": 115,
        "failed": 5
      }
    ],
    "days": 30
  }
}
```

---

## 5. 获取平台用户操作排行

**GET** `/api/v1/platform/audit-logs/user-ranking`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `limit` | int | ❌ | 返回数量，默认 10，最大 100 |
| `days` | int | ❌ | 统计天数，默认 7 |

### 响应

```json
{
  "code": 0,
  "data": {
    "rankings": [
      {
        "user_id": "uuid",
        "username": "admin",
        "operation_count": 350
      }
    ],
    "limit": 10,
    "days": 7
  }
}
```

---

## 6. 获取平台高危操作日志

**GET** `/api/v1/platform/audit-logs/high-risk`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |

### 响应

返回高危操作日志列表（字段同列表，但只包含 `risk_level = "high"` 的记录）。
