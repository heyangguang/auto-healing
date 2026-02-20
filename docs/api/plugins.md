# 插件管理 API 文档

**Base URL**: `/api/v1/plugins`  
**权限**: 已登录用户（租户内数据隔离）

---

## 1. 获取插件列表

**GET** `/api/v1/plugins`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称、描述） |
| `type` | string | ❌ | 插件类型：`monitoring` / `cmdb` / `ticketing` / `custom` 等 |
| `status` | string | ❌ | 状态：`active` / `inactive` / `error` |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "name": "Zabbix 监控",
        "description": "Zabbix 监控系统集成插件",
        "type": "monitoring",
        "status": "active",
        "is_enabled": true,
        "config": {
          "url": "http://zabbix.example.com",
          "username": "admin"
        },
        "last_sync_at": "2026-02-18T09:00:00Z",
        "sync_interval": 300,
        "cmdb_count": 150,
        "incident_count": 25,
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-02-18T10:00:00Z"
      }
    ],
    "total": 5,
    "page": 1,
    "page_size": 20
  }
}
```

---

## 2. 创建插件

**POST** `/api/v1/plugins`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 插件名称 |
| `description` | string | ❌ | 描述 |
| `type` | string | ✅ | 插件类型 |
| `version` | string | ❌ | 版本号，默认 `1.0.0` |
| `config` | object | ✅ | 插件配置（根据类型不同而不同） |
| `field_mapping` | object | ❌ | 字段映射配置（JSON 对象） |
| `sync_filter` | object | ❌ | 同步过滤配置（JSON 对象） |
| `sync_enabled` | bool | ❌ | 是否启用自动同步，默认 false |
| `sync_interval_minutes` | int | ❌ | 同步间隔（分钟） |
| `max_failures` | int | ❌ | 最大连续失败次数，默认 5 |

```json
{
  "name": "Zabbix 监控",
  "description": "生产环境 Zabbix 监控",
  "type": "monitoring",
  "config": {
    "url": "http://zabbix.example.com",
    "username": "admin",
    "password": "secret",
    "api_token": "xxxxx"
  },
  "sync_enabled": true,
  "sync_interval_minutes": 5
}
```

---

## 3. 获取插件详情

**GET** `/api/v1/plugins/:id`

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 插件 ID |

---

## 4. 更新插件

**PUT** `/api/v1/plugins/:id`

### 请求体（所有字段可选）

| 字段 | 类型 | 说明 |
|------|------|------|
| `description` | string | 描述 |
| `version` | string | 版本号 |
| `config` | object | 插件配置 |
| `field_mapping` | object | 字段映射配置 |
| `sync_filter` | object | 同步过滤配置 |
| `sync_enabled` | bool | 是否启用自动同步 |
| `sync_interval_minutes` | int | 同步间隔（分钟） |
| `max_failures` | int | 最大连续失败次数 |

---

## 5. 删除插件

**DELETE** `/api/v1/plugins/:id`

---

## 6. 测试插件连接

**POST** `/api/v1/plugins/:id/test`

**权限**: `plugin:test`

仅测试连接，不改变插件状态。

### 响应

```json
{
  "code": 0,
  "message": "连接测试成功"
}
```

---

## 7. 激活插件

**POST** `/api/v1/plugins/:id/activate`

**权限**: `plugin:update`

测试连接成功后，将插件状态设为 `active`。

### 响应

```json
{
  "code": 0,
  "message": "插件已激活"
}
```

---

## 8. 停用插件

**POST** `/api/v1/plugins/:id/deactivate`

**权限**: `plugin:update`

---

## 9. 手动触发同步

**POST** `/api/v1/plugins/:id/sync`

触发插件立即同步数据（CMDB 配置项、工单等）。

### 响应

```json
{
  "code": 0,
  "message": "同步已触发"
}
```

---

## 10. 获取插件同步日志

**GET** `/api/v1/plugins/:id/logs`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "plugin_id": "uuid",
        "status": "success",
        "message": "同步成功，新增 5 条，更新 10 条",
        "synced_count": 15,
        "error_count": 0,
        "started_at": "2026-02-18T09:00:00Z",
        "completed_at": "2026-02-18T09:00:05Z",
        "duration_ms": 5000
      }
    ],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

---

## 11. 获取插件统计

**GET** `/api/v1/plugins/stats`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 5,
    "active": 4,
    "inactive": 1,
    "error": 0,
    "by_type": {
      "monitoring": 2,
      "cmdb": 1,
      "ticketing": 1,
      "custom": 1
    }
  }
}
```

---

## 工单（Incident）管理

插件会自动从外部系统同步工单数据。工单接口为**独立路由组**，路径前缀为 `/api/v1/incidents`，详见 [incidents.md](./incidents.md)。

### 12. 获取工单列表

**GET** `/api/v1/incidents`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `plugin_id` | uuid | ❌ | 按插件筛选 |
| `status` | string | ❌ | 工单状态：`open` / `resolved` / `closed` 等 |
| `severity` | string | ❌ | 严重程度：`critical` / `high` / `medium` / `low` |
| `healing_status` | string | ❌ | 自愈状态：`pending` / `triggered` / `success` / `failed` / `skipped` |
| `search` | string | ❌ | 模糊搜索（标题、外部 ID、受影响 CI） |

#### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "plugin_id": "uuid",
        "external_id": "INC-12345",
        "title": "服务器 CPU 使用率过高",
        "description": "生产服务器 CPU 持续超过 90%",
        "severity": "high",
        "priority": "P1",
        "status": "open",
        "category": "performance",
        "affected_ci": "prod-server-01",
        "affected_service": "payment-service",
        "assignee": "zhangsan",
        "reporter": "zabbix-system",
        "healing_status": "triggered",
        "matched_rule_id": "uuid",
        "healing_flow_instance_id": "uuid",
        "raw_data": {},
        "created_at": "2026-02-18T08:00:00Z",
        "updated_at": "2026-02-18T10:00:00Z"
      }
    ],
    "total": 50,
    "page": 1,
    "page_size": 20
  }
}
```

### 13. 获取工单详情

**GET** `/api/v1/incidents/:id`

### 工单字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 工单 ID |
| `plugin_id` | uuid | 来源插件 ID |
| `external_id` | string | 外部系统工单 ID |
| `title` | string | 标题 |
| `description` | string | 描述 |
| `severity` | string | 严重程度：`critical` / `high` / `medium` / `low` |
| `priority` | string | 优先级：`P1` / `P2` / `P3` / `P4` |
| `status` | string | 工单状态 |
| `category` | string | 分类 |
| `affected_ci` | string | 受影响的配置项 |
| `affected_service` | string | 受影响的服务 |
| `assignee` | string | 负责人 |
| `reporter` | string | 上报人 |
| `healing_status` | string | 自愈状态：`pending` / `triggered` / `success` / `failed` / `skipped` |
| `matched_rule_id` | uuid | 匹配的自愈规则 ID |
| `healing_flow_instance_id` | uuid | 触发的自愈流程实例 ID |
| `raw_data` | object | 原始数据（来自外部系统） |
