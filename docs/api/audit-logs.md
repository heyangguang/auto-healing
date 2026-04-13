# 租户审计日志 API 文档

**Base URL**: `/api/v1/audit-logs`  
**权限**: `audit:list`（导出接口除外，导出需要 `audit:export`）

> 租户审计日志仅记录当前租户上下文内的操作行为；认证入口产生的全局安全事件（如登录、登出、邀请注册）统一记录到平台审计日志。

---

## 1. 获取审计日志列表

**GET** `/api/v1/audit-logs`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（用户名、资源名称、请求路径） |
| `category` | string | ❌ | 操作分类 |
| `action` | string | ❌ | 操作类型（见下方枚举） |
| `resource_type` | string | ❌ | 资源类型（见下方枚举） |
| `username` | string | ❌ | 按用户名筛选 |
| `user_id` | uuid | ❌ | 按用户 ID 筛选 |
| `status` | string | ❌ | 操作状态：`success` / `failed` |
| `risk_level` | string | ❌ | 风险等级：`normal` / `high` |
| `exclude_action` | string | ❌ | 排除的操作类型（逗号分隔，如 `login,logout`） |
| `exclude_resource_type` | string | ❌ | 排除的资源类型（逗号分隔） |
| `created_after` | string | ❌ | 开始时间（RFC3339 格式） |
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
        "username": "zhangsan",
        "ip_address": "192.168.1.100",
        "user_agent": "Mozilla/5.0...",
        "category": "resource_operation",
        "action": "create",
        "resource_type": "plugin",
        "resource_id": "uuid",
        "resource_name": "Zabbix 监控插件",
        "request_method": "POST",
        "request_path": "/api/v1/plugins",
        "request_body": "{...}",
        "response_status": 201,
        "changes": {
          "before": null,
          "after": {"name": "Zabbix 监控插件", "type": "itsm"}
        },
        "status": "success",
        "error_message": "",
        "risk_level": "normal",
        "risk_reason": "",
        "created_at": "2026-02-18T10:00:00Z",
        "user": {
          "id": "uuid",
          "username": "zhangsan",
          "display_name": "张三"
        }
      }
  ],
  "total": 1000,
  "page": 1,
  "page_size": 20
}
```

---

## 2. 获取审计日志详情

**GET** `/api/v1/audit-logs/:id`

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 日志 ID |

---

## 3. 获取审计统计概览

**GET** `/api/v1/audit-logs/stats`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 10000,
    "today": 250,
    "success": 9800,
    "failed": 200,
    "high_risk": 30
  }
}
```

---

## 4. 获取用户操作排行

**GET** `/api/v1/audit-logs/user-ranking`

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
        "username": "zhangsan",
        "operation_count": 500
      }
    ],
    "limit": 10,
    "days": 7
  }
}
```

---

## 5. 按操作类型分组统计

**GET** `/api/v1/audit-logs/action-grouping`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `action` | string | ❌ | 过滤特定操作类型 |
| `days` | int | ❌ | 统计天数，默认 30 |

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "action": "create",
        "resource_type": "plugin",
        "count": 150
      }
    ],
    "action": "",
    "days": 30
  }
}
```

---

## 6. 获取资源类型统计

**GET** `/api/v1/audit-logs/resource-stats`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `days` | int | ❌ | 统计天数，默认 30 |

---

## 7. 获取操作趋势

**GET** `/api/v1/audit-logs/trend`

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
        "total": 250,
        "success": 240,
        "failed": 10
      }
    ],
    "days": 30
  }
}
```

---

## 8. 获取高危操作日志

**GET** `/api/v1/audit-logs/high-risk`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |

---

## 9. 导出审计日志（CSV）

**GET** `/api/v1/audit-logs/export`

**权限**: `audit:export`

支持与列表接口相同的过滤参数（除 `page` / `page_size` 外），最多导出 10000 条。

**响应**: 直接返回 CSV 文件下载（`Content-Type: text/csv`）。

---

## 操作类型（action）枚举

| 值 | 说明 |
|----|------|
| `login` | 登录 |
| `logout` | 登出 |
| `create` | 创建 |
| `update` | 更新 |
| `delete` | 删除 |
| `trigger` | 触发 |
| `approve` | 审批通过 |
| `reject` | 审批拒绝 |
| `execute` | 执行 |
| `cancel` | 取消 |
| `enable` | 启用 |
| `disable` | 禁用 |
| `sync` | 同步 |
| `export` | 导出 |

## 资源类型（resource_type）枚举

| 值 | 说明 |
|----|------|
| `user` | 用户 |
| `role` | 角色 |
| `plugin` | 插件 |
| `cmdb` | 配置项 |
| `secrets_source` | 密钥源 |
| `git_repo` | Git 仓库 |
| `playbook` | Playbook |
| `execution_task` | 执行任务 |
| `execution_run` | 执行记录 |
| `execution_schedule` | 定时调度 |
| `healing_flow` | 自愈流程 |
| `healing_rule` | 自愈规则 |
| `healing_instance` | 自愈实例 |
| `notification_channel` | 通知渠道 |
| `notification_template` | 通知模板 |
| `site_message` | 站内信 |
