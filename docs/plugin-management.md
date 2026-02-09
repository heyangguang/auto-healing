# 插件管理 API 文档

## 概述

插件管理模块用于对接外部 ITSM（工单系统）和 CMDB（配置管理数据库）系统。

**设计理念：** Auto-Healing 只定义数据标准格式，不关心外部系统的实现细节。外部系统需要提供符合我们标准格式的 API 接口。

---

## 数据模型

### Plugin 插件

| 字段 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| `id` | UUID | 自动 | 插件唯一标识 |
| `name` | string | ✅ | 插件名称（唯一） |
| `type` | string | ✅ | 插件类型：`itsm` 或 `cmdb` |
| `description` | string | ❌ | 描述 |
| `version` | string | ❌ | 版本号，默认 `1.0.0` |
| `config` | object | ✅ | 连接配置（详见下方） |
| `field_mapping` | object | ❌ | 字段映射规则 |
| `sync_filter` | object | ❌ | 同步过滤器 |
| `sync_enabled` | boolean | ❌ | 是否开启自动同步，默认 `false` |
| `sync_interval_minutes` | integer | ❌ | 同步间隔（分钟），最小 1 |
| `status` | string | 自动 | 状态：`inactive`/`active`/`error` |
| `last_sync_at` | datetime | 自动 | 最后同步时间 |
| `next_sync_at` | datetime | 自动 | 下次同步时间 |
| `error_message` | string | 自动 | 错误信息 |

### Config 连接配置

| 字段 | 类型 | 必填 | 说明 |
|-----|------|------|------|
| `url` | string | ✅ | API 地址 |
| `auth_type` | string | ✅ | 认证方式：`basic`/`bearer`/`api_key` |
| `username` | string | 条件 | Basic 认证用户名 |
| `password` | string | 条件 | Basic 认证密码 |
| `token` | string | 条件 | Bearer 认证令牌 |
| `api_key` | string | 条件 | API Key |
| `api_key_header` | string | ❌ | API Key 的 Header 名，默认 `X-API-Key` |
| `since_param` | string | ❌ | 时间参数名（用于增量同步，详见下方） |
| `response_data_path` | string | ❌ | 响应数据路径（详见下方） |
| `extra_params` | object | ❌ | 额外查询参数（详见下方） |
| `close_incident_url` | string | ❌ | 关闭工单接口 URL（ITSM 专用） |
| `close_incident_method` | string | ❌ | 关闭工单方法，默认 `POST` |

---

### 增量同步机制

系统会自动记录每次同步的时间 (`last_sync_at`)，下次同步时只拉取该时间之后的数据。

**`since_param` - 时间参数名**

告诉系统在请求时使用什么参数名传递上次同步时间。

**示例配置：**
```json
{
  "url": "https://itsm.example.com/api/incidents",
  "since_param": "updated_after"
}
```

**实际请求：**
```
首次同步：GET https://itsm.example.com/api/incidents?updated_after=2026-01-06T00:00:00Z (默认24小时前)
再次同步：GET https://itsm.example.com/api/incidents?updated_after=2026-01-06T12:00:00Z (上次同步时间)
```

**不同 ITSM 系统的常见参数名：**
| 系统 | since_param 值 |
|------|---------------|
| ServiceNow | `sysparm_query=sys_updated_on>` |
| Jira | `updatedAfter` |
| 自定义系统 | `modified_since`、`since`、`updated_at` |

---

### `response_data_path` - 响应数据路径

告诉系统从响应 JSON 的哪个路径提取数据数组。

**示例1：ServiceNow 格式**
```json
{
  "result": [
    {"sys_id": "1", "short_description": "工单1"},
    {"sys_id": "2", "short_description": "工单2"}
  ]
}
```
配置：`"response_data_path": "result"`

**示例2：嵌套格式**
```json
{
  "code": 200,
  "data": {
    "items": [...]
  }
}
```
配置：`"response_data_path": "data.items"`

**示例3：直接返回数组**
```json
[{"id": "1"}, {"id": "2"}]
```
配置：留空或不填

---

### `extra_params` - 额外查询参数

如果外部 API 需要额外的固定查询参数，可以在此配置。

**示例：**
```json
{
  "url": "https://itsm.example.com/api/incidents",
  "extra_params": {
    "sysparm_limit": "100",
    "sysparm_display_value": "true",
    "assignment_group": "auto-healing"
  }
}
```

**实际请求：**
```
GET https://itsm.example.com/api/incidents?sysparm_limit=100&sysparm_display_value=true&assignment_group=auto-healing&updated_after=...
```

---

## API 接口

### 1. 创建插件

```http
POST /api/v1/plugins
```

**请求体：**
```json
{
  "name": "生产环境 ITSM",
  "type": "itsm",
  "description": "生产环境事件管理系统",
  "config": {
    "url": "https://itsm.example.com/api/incidents",
    "auth_type": "basic",
    "username": "api_user",
    "password": "secret123",
    "since_param": "updated_after",
    "response_data_path": "data.items",
    "close_incident_url": "https://itsm.example.com/api/incidents/{external_id}/close",
    "close_incident_method": "POST"
  },
  "field_mapping": {
    "incident_mapping": {
      "external_id": "ticket_id",
      "title": "subject",
      "description": "content",
      "severity": "urgency_level",
      "status": "ticket_status"
    }
  },
  "sync_filter": {
    "logic": "and",
    "rules": [
      {"field": "status", "operator": "not_equals", "value": "closed"},
      {"field": "severity", "operator": "in", "value": ["critical", "high"]}
    ]
  },
  "sync_enabled": true,
  "sync_interval_minutes": 5
}
```

**响应：** `201 Created`
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "生产环境 ITSM",
  "type": "itsm",
  "status": "inactive",
  ...
}
```

---

### 2. 获取插件列表

```http
GET /api/v1/plugins
```

**查询参数：**
| 参数 | 类型 | 说明 |
|-----|------|------|
| `page` | integer | 页码，默认 1 |
| `page_size` | integer | 每页数量，默认 20 |
| `type` | string | 过滤类型：`itsm`/`cmdb` |
| `status` | string | 过滤状态：`active`/`inactive`/`error` |

**响应：** `200 OK`
```json
{
  "data": [...],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 5
  }
}
```

---

### 3. 获取插件统计

```http
GET /api/v1/plugins/stats
```

返回插件的统计数据，用于仪表盘卡片展示。

**响应：** `200 OK`
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 8,
    "by_type": {
      "itsm": 5,
      "cmdb": 3
    },
    "by_status": {
      "active": 4,
      "inactive": 3,
      "error": 1
    },
    "sync_enabled": 6,
    "sync_disabled": 2,
    "active_count": 4,
    "inactive_count": 3,
    "error_count": 1
  }
}
```

**字段说明：**
| 字段 | 说明 |
|------|------|
| `total` | 插件总数 |
| `by_type` | 按类型分布 (itsm/cmdb) |
| `by_status` | 按状态分布 (active/inactive/error) |
| `sync_enabled` | 启用自动同步的数量 |
| `sync_disabled` | 未启用自动同步的数量 |
| `active_count` | 激活状态数量 |
| `inactive_count` | 未激活状态数量 |
| `error_count` | 错误状态数量 |

---

### 4. 获取插件详情

```http
GET /api/v1/plugins/{id}
```

---

### 4. 更新插件

```http
PUT /api/v1/plugins/{id}
```

**请求体：** （只需包含要更新的字段）
```json
{
  "description": "更新后的描述",
  "version": "1.1.0",
  "config": {
    "url": "https://new-url.example.com/api/incidents"
  },
  "field_mapping": {...},
  "sync_filter": {
    "logic": "and",
    "rules": [
      {"field": "priority", "operator": "in", "value": ["1", "2"]}
    ]
  },
  "sync_enabled": false,
  "sync_interval_minutes": 10
}
```

**可更新字段：**
| 字段 | 说明 |
|-----|------|
| `description` | 描述 |
| `version` | 版本号 |
| `config` | 连接配置 |
| `field_mapping` | 字段映射 |
| `sync_filter` | 同步过滤器 |
| `sync_enabled` | 是否开启自动同步 |
| `sync_interval_minutes` | 同步间隔 |

**不可修改：** `name`、`type`

---

### 5. 删除插件

```http
DELETE /api/v1/plugins/{id}
```

---

### 6. 测试连接

```http
POST /api/v1/plugins/{id}/test
```

只测试插件能否正常连接到外部系统，**不改变插件状态**。

**响应：** `200 OK`
```json
{
  "message": "连接测试成功"
}
```

---

### 7. 激活插件

```http
POST /api/v1/plugins/{id}/activate
```

测试连接，成功后激活插件。**只有激活状态的插件才会参与定时同步。**

**响应：** `200 OK`
```json
{
  "message": "插件已激活"
}
```

**失败响应：** `400 Bad Request`
```json
{
  "error": "激活失败: 连接测试失败: ..."
}
```

---

### 8. 停用插件

```http
POST /api/v1/plugins/{id}/deactivate
```

直接停用插件，不需要测试。停用后插件不再参与定时同步。

**响应：** `200 OK`
```json
{
  "message": "插件已停用"
}
```

---

### 9. 手动触发同步

```http
POST /api/v1/plugins/{id}/sync
```

手动触发一次数据同步，返回同步日志 ID。

**响应：** `200 OK`
```json
{
  "id": "...",
  "status": "running"
}
```

---

### 8. 获取同步日志

```http
GET /api/v1/plugins/{id}/logs
```

**查询参数：**
| 参数 | 类型 | 说明 |
|-----|------|------|
| `page` | integer | 页码 |
| `page_size` | integer | 每页数量 |

---

## 字段映射说明

当外部系统字段名和标准字段名不同时，使用字段映射进行转换。

### 工单 (Incident) 可映射字段

| 标准字段名 | 说明 |
|-----------|------|
| `external_id` | 外部工单 ID |
| `title` | 标题 |
| `description` | 描述 |
| `severity` | 严重程度 |
| `priority` | 优先级 |
| `status` | 状态 |
| `category` | 分类 |
| `affected_ci` | 受影响配置项 |
| `affected_service` | 受影响服务 |
| `assignee` | 处理人 |
| `reporter` | 报告人 |

### CMDB 可映射字段

| 标准字段名 | 说明 |
|-----------|------|
| `external_id` | 外部配置项 ID |
| `name` | 名称 |
| `type` | 类型 |
| `status` | 状态 |
| `ip_address` | IP 地址 |
| `hostname` | 主机名 |
| `os` | 操作系统 |
| `os_version` | 系统版本 |
| `cpu` | CPU 信息 |
| `memory` | 内存 |
| `disk` | 磁盘 |
| `location` | 位置 |
| `owner` | 负责人 |
| `environment` | 环境 |
| `manufacturer` | 厂商 |
| `model` | 型号 |
| `serial_number` | 序列号 |
| `department` | 所属部门 |

### 映射示例

```json
{
  "field_mapping": {
    "incident_mapping": {
      "external_id": "ticket_id",
      "title": "subject",
      "severity": "urgency_level"
    }
  }
}
```

表示：外部系统的 `ticket_id` 映射到我们的 `external_id`，`subject` 映射到 `title`。

---

## 同步过滤器说明

过滤器用于只同步符合条件的数据。

### 支持的操作符

| 操作符 | 说明 | 示例 |
|-------|------|------|
| `equals` | 等于 | `{"field": "status", "operator": "equals", "value": "open"}` |
| `not_equals` | 不等于 | `{"field": "status", "operator": "not_equals", "value": "closed"}` |
| `contains` | 包含 | `{"field": "title", "operator": "contains", "value": "网络"}` |
| `not_contains` | 不包含 | `{"field": "category", "operator": "not_contains", "value": "test"}` |
| `starts_with` | 以...开头 | `{"field": "category", "operator": "starts_with", "value": "Network"}` |
| `ends_with` | 以...结尾 | `{"field": "hostname", "operator": "ends_with", "value": ".prod"}` |
| `regex` | 正则匹配 | `{"field": "title", "operator": "regex", "value": "^服务器.*故障$"}` |
| `in` | 在列表中 | `{"field": "severity", "operator": "in", "value": ["critical", "high"]}` |
| `not_in` | 不在列表中 | `{"field": "category", "operator": "not_in", "value": ["test", "dev"]}` |

### 组合条件

使用 `logic` 和 `rules` 组合多个条件：

```json
{
  "sync_filter": {
    "logic": "and",
    "rules": [
      {"field": "status", "operator": "equals", "value": "open"},
      {"field": "severity", "operator": "in", "value": ["critical", "high"]}
    ]
  }
}
```

支持嵌套：

```json
{
  "sync_filter": {
    "logic": "and",
    "rules": [
      {"field": "status", "operator": "equals", "value": "open"},
      {
        "logic": "or",
        "rules": [
          {"field": "severity", "operator": "equals", "value": "critical"},
          {"field": "category", "operator": "equals", "value": "Network"}
        ]
      }
    ]
  }
}
```

---

## 工单 API

### 获取工单列表

```http
GET /api/v1/plugins/{plugin_id}/incidents
```

**查询参数：**
| 参数 | 类型 | 说明 |
|-----|------|------|
| `page` | integer | 页码 |
| `page_size` | integer | 每页数量 |
| `status` | string | 工单状态 |
| `severity` | string | 严重程度 |
| `source_plugin_name` | string | 来源插件名（用于查询已删除插件的工单） |

---

### 获取工单详情

```http
GET /api/v1/incidents/{id}
```

---

### 关闭工单

```http
POST /api/v1/incidents/{id}/close
```

**请求体：**
```json
{
  "resolution": "重启服务后问题已解决",
  "work_notes": "执行了自动修复流程",
  "close_code": "resolved",
  "close_status": "resolved"
}
```

**响应：**
```json
{
  "message": "工单已关闭",
  "local_status": "healed",
  "source_updated": true
}
```

`source_updated` 表示是否成功回写到源系统（如果配置了 `close_incident_url`）。

---

## 完整示例

### 创建 ITSM 插件

```bash
curl -X POST http://localhost:8080/api/v1/plugins \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "生产环境 ITSM",
    "type": "itsm",
    "config": {
      "url": "https://itsm.example.com/api/v1/incidents",
      "auth_type": "bearer",
      "token": "your-api-token",
      "since_param": "modified_since",
      "response_data_path": "result.data"
    },
    "sync_enabled": true,
    "sync_interval_minutes": 5
  }'
```

### 创建 CMDB 插件

```bash
curl -X POST http://localhost:8080/api/v1/plugins \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "生产环境 CMDB",
    "type": "cmdb",
    "config": {
      "url": "https://cmdb.example.com/api/v1/hosts",
      "auth_type": "api_key",
      "api_key": "your-api-key",
      "api_key_header": "X-CMDB-API-Key"
    },
    "field_mapping": {
      "cmdb_mapping": {
        "external_id": "asset_id",
        "name": "asset_name",
        "ip_address": "primary_ip"
      }
    },
    "sync_enabled": false
  }'
```
