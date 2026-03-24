# 工作台 (Workbench) API 文档

> **Base URL**: `/api/v1/common/workbench`
>
> **认证方式**: Bearer Token（所有接口均需要 `Authorization: Bearer <token>` Header）
>
> **租户上下文**: 租户用户可显式传 `X-Tenant-ID`，未传时使用当前默认租户；平台用户仅在 Impersonation 场景下具备租户上下文。

---

## 1. 综合概览

### `GET /api/v1/common/workbench/overview`

获取工作台首页的核心统计数据，后端并发聚合 5 个 Section 返回。

**请求参数**: 无

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "system_health": {
      "status": "healthy",
      "version": "1.0.0",
      "uptime_seconds": 86400,
      "environment": "production",
      "api_latency_ms": 2,
      "db_latency_ms": 5
    },
    "healing_stats": {
      "today_success": 12,
      "today_failed": 1
    },
    "incident_stats": {
      "pending_count": 650,
      "last_7_days_total": 6
    },
    "host_stats": {
      "online_count": 92,
      "offline_count": 8
    },
    "resource_overview": {
      "flows": {
        "total": 6,
        "enabled": 6
      },
      "rules": {
        "total": 6,
        "enabled": 2
      },
      "hosts": {
        "total": 100,
        "offline": 8
      },
      "playbooks": {
        "total": 8,
        "needs_review": 0
      },
      "schedules": {
        "total": 11,
        "enabled": 4
      },
      "notification_templates": {
        "total": 14,
        "channels": 17
      },
      "secrets": {
        "total": 82,
        "types": "vault + file + webhook"
      },
      "users": {
        "total": 45,
        "admins": 9
      }
    }
  }
}
```

**字段说明**:

| 字段路径 | 类型 | 说明 |
|---|---|---|
| `system_health.status` | string | 系统状态：`healthy` / `degraded` / `down` |
| `system_health.version` | string | 系统版本号 |
| `system_health.uptime_seconds` | number | 系统运行时长（秒） |
| `system_health.environment` | string | 环境标识（production/staging/dev） |
| `system_health.api_latency_ms` | number | API 延迟（毫秒） |
| `system_health.db_latency_ms` | number | 数据库延迟（毫秒） |
| `healing_stats.today_success` | number | 今日自愈成功次数 |
| `healing_stats.today_failed` | number | 今日自愈失败次数 |
| `incident_stats.pending_count` | number | 待处理工单数 |
| `incident_stats.last_7_days_total` | number | 近 7 天工单总数 |
| `host_stats.online_count` | number | 在线主机数 |
| `host_stats.offline_count` | number | 离线主机数 |
| `resource_overview.flows` | object | 自愈流程：`total`（总数）、`enabled`（已启用） |
| `resource_overview.rules` | object | 自愈规则：`total`（总数）、`enabled`（已激活） |
| `resource_overview.hosts` | object | 主机：`total`（总数）、`offline`（离线数） |
| `resource_overview.playbooks` | object | Playbook：`total`（总数）、`needs_review`（草稿数） |
| `resource_overview.schedules` | object | 定时任务：`total`（总数）、`enabled`（已启用） |
| `resource_overview.notification_templates` | object | 通知模板：`total`（模板数）、`channels`（渠道数） |
| `resource_overview.secrets` | object | 密钥源：`total`（总数）、`types`（类型汇总文本） |
| `resource_overview.users` | object | 用户：`total`（总数）、`admins`（管理员数） |

---

## 2. 活动动态

### `GET /api/v1/common/workbench/activities`

获取最近的操作活动记录（来源：审计日志）。

**请求参数**:

| 参数 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `limit` | number | 否 | 10 | 返回条数 |

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "38b08d0b-5ddf-4756-a096-fd5b0875075e",
        "type": "system",
        "text": "用户登录系统",
        "created_at": "2026-02-20T22:26:29.282439+08:00"
      },
      {
        "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
        "type": "execution",
        "text": "执行执行任务：日志清理任务",
        "created_at": "2026-02-20T18:30:00+08:00"
      },
      {
        "id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
        "type": "flow",
        "text": "创建自愈流程：磁盘清理流程",
        "created_at": "2026-02-20T15:00:00+08:00"
      }
    ]
  }
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string (UUID) | 活动记录 ID |
| `type` | string | 活动类型：`system` / `execution` / `flow` / `rule` |
| `text` | string | 活动描述文本（中文） |
| `created_at` | string (ISO 8601) | 发生时间 |

---

## 3. 定时任务日历

### `GET /api/v1/common/workbench/schedule-calendar`

获取指定月份的定时任务日历数据，按日期分组。包含 cron 类型（解析 cron 表达式计算执行时间点）和 once 类型（一次性定时任务）。

**请求参数**:

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `year` | number | ✅ | 年份，如 `2026` |
| `month` | number | ✅ | 月份，`1-12` |

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "dates": {
      "2026-02-01": [
        {
          "name": "性能监控采集",
          "time": "00:30",
          "schedule_id": "7b1807b1-dc2f-4f1d-8868-672bddb72016"
        },
        {
          "name": "性能监控采集",
          "time": "01:30",
          "schedule_id": "7b1807b1-dc2f-4f1d-8868-672bddb72016"
        },
        {
          "name": "测试-Cron模式",
          "time": "10:00",
          "schedule_id": "134a995b-5b77-4c3c-a520-619e309aea63"
        },
        {
          "name": "工作日健康检查",
          "time": "09:30",
          "schedule_id": "afded79d-2421-40e8-81ca-aa45a2153c98"
        }
      ],
      "2026-02-02": [
        {
          "name": "性能监控采集",
          "time": "00:30",
          "schedule_id": "7b1807b1-dc2f-4f1d-8868-672bddb72016"
        }
      ]
    }
  }
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|---|---|---|
| `dates` | object | 日期 → 任务列表的映射，key 格式 `YYYY-MM-DD` |
| `dates[].name` | string | 定时任务名称 |
| `dates[].time` | string | 执行时间，格式 `HH:mm` |
| `dates[].schedule_id` | string (UUID) | 定时任务 ID |

> **注意**: 没有任务的日期不会出现在 `dates` 对象中。前端渲染日历时，对于不存在的日期 key，视为该日无任务。

---

## 4. 系统公告

### `GET /api/v1/common/workbench/announcements`

获取系统公告列表（来源：站内信中 `category = "announcement"` 的记录）。

**请求参数**:

| 参数 | 类型 | 必填 | 默认 | 说明 |
|---|---|---|---|---|
| `limit` | number | 否 | 5 | 返回条数 |

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "id": "c3d4e5f6-a7b8-9012-cdef-123456789012",
        "title": "系统升级通知",
        "content": "计划于本周六凌晨 2:00-4:00 进行系统升级维护...",
        "created_at": "2026-02-20T10:00:00+08:00"
      }
    ]
  }
}
```

**字段说明**:

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | string (UUID) | 公告 ID |
| `title` | string | 公告标题 |
| `content` | string | 公告内容（富文本 HTML） |
| `created_at` | string (ISO 8601) | 发布时间 |

> **如何创建公告**: 通过已有的站内信发送接口 `POST /api/v1/platform/site-messages`，`category` 传 `"announcement"` 即可。

---

## 5. 我的收藏

### `GET /api/v1/common/workbench/favorites`

获取当前用户的快捷收藏列表。

**请求参数**: 无

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "items": [
      {
        "key": "cmdb",
        "label": "资产管理",
        "icon": "DatabaseOutlined",
        "path": "/cmdb"
      },
      {
        "key": "rules",
        "label": "自愈规则",
        "icon": "ToolOutlined",
        "path": "/healing/rules"
      }
    ]
  }
}
```

> 首次请求（用户从未设置收藏）返回 `"items": []`

---

### `PUT /api/v1/common/workbench/favorites`

更新当前用户的快捷收藏列表（全量替换）。

**请求参数** (JSON Body):

```json
{
  "items": [
    {
      "key": "cmdb",
      "label": "资产管理",
      "icon": "DatabaseOutlined",
      "path": "/cmdb"
    },
    {
      "key": "rules",
      "label": "自愈规则",
      "icon": "ToolOutlined",
      "path": "/healing/rules"
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `items` | array | ✅ | 收藏项数组 |
| `items[].key` | string | ✅ | 唯一标识（如菜单 key） |
| `items[].label` | string | ✅ | 显示名称 |
| `items[].icon` | string | ✅ | Ant Design 图标名（如 `DatabaseOutlined`） |
| `items[].path` | string | ✅ | 前端路由路径 |

**响应示例**:

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "message": "success"
  }
}
```

---

## 错误响应格式

所有接口使用统一错误格式：

```json
{
  "code": 400,
  "message": "year 和 month 参数必填"
}
```

| HTTP 状态码 | 场景 |
|---|---|
| `400` | 参数校验失败 |
| `401` | 未认证 / Token 过期 |
| `500` | 服务内部错误 |
