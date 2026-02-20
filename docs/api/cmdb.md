# CMDB 配置管理 API 文档

**路径前缀**: `/api/v1/cmdb`  
**权限**: 已登录用户（租户内数据隔离）

---

## 1. 获取配置项列表

**GET** `/api/v1/cmdb`

**权限**: `plugin:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `plugin_id` | uuid | ❌ | 按插件 ID 筛选 |
| `type` | string | ❌ | 配置项类型（如 `server`、`network`、`database`） |
| `status` | string | ❌ | 状态：`active` / `inactive` / `maintenance` |
| `environment` | string | ❌ | 环境：`prod` / `staging` / `dev` |
| `source_plugin_name` | string | ❌ | 按插件名称模糊筛选 |
| `has_plugin` | bool | ❌ | 是否关联插件：`true` / `false` |
| `sort_by` | string | ❌ | 排序字段 |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

## 2. 获取 CMDB 统计

**GET** `/api/v1/cmdb/stats`

**权限**: `plugin:list`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 200,
    "active": 180,
    "inactive": 15,
    "maintenance": 5,
    "by_plugin": [
      {"plugin_id": "uuid", "plugin_name": "Zabbix", "count": 150}
    ]
  }
}
```

---

## 3. 批量测试连接

**POST** `/api/v1/cmdb/batch-test-connection`

**权限**: `plugin:sync`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `cmdb_ids` | []uuid | ✅ | 配置项 ID 列表（最多 50 个） |
| `secrets_source_id` | uuid | ✅ | 凭据来源 ID（用于 SSH 认证） |

---

## 4. 批量进入维护模式

**POST** `/api/v1/cmdb/batch/maintenance`

**权限**: `plugin:update`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uuid | ✅ | 配置项 ID 列表 |
| `reason` | string | ✅ | 维护原因 |
| `end_at` | string | ❌ | 维护结束时间（RFC3339 格式），不传表示无限期维护 |

---

## 5. 批量退出维护模式

**POST** `/api/v1/cmdb/batch/resume`

**权限**: `plugin:update`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uuid | ✅ | 配置项 ID 列表 |

---

## 6. 获取所有配置项 ID 列表（轻量接口）

**GET** `/api/v1/cmdb/ids`

**权限**: `plugin:list`

返回所有配置项的 ID 列表，用于"全选"功能，不包含分页。

### 查询参数

支持与列表接口相同的过滤参数：`type`、`status`、`environment`、`source_plugin_name`、`plugin_id`、`has_plugin`。

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {"id": "uuid1", "name": "prod-web-01"},
      {"id": "uuid2", "name": "prod-db-01"}
    ],
    "total": 150
  }
}
```

---

## 7. 获取配置项详情

**GET** `/api/v1/cmdb/:id`

**权限**: `plugin:list`

### 响应

```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "plugin_id": "uuid",
    "plugin": {"id": "uuid", "name": "Zabbix 监控"},
    "name": "prod-web-01",
    "ip": "192.168.1.100",
    "hostname": "prod-web-01.example.com",
    "os_type": "linux",
    "os_version": "CentOS 7.9",
    "cpu_cores": 8,
    "memory_gb": 16,
    "status": "active",
    "ssh_port": 22,
    "tags": {"env": "prod", "role": "web"},
    "extra_data": {},
    "last_sync_at": "2026-02-18T09:00:00Z",
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-02-18T10:00:00Z"
  }
}
```

---

## 8. 测试单个配置项连接

**POST** `/api/v1/cmdb/:id/test-connection`

**权限**: `plugin:sync`

测试 SSH 连接是否可达。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `secrets_source_id` | uuid | ✅ | 密鑰源 ID（用于 SSH 认证） |

### 响应

```json
{
  "code": 0,
  "data": {
    "cmdb_id": "uuid",
    "host": "192.168.1.100",
    "success": true,
    "latency_ms": 25,
    "message": "连接成功",
    "auth_type": "ssh_key"
  }
}
```

---

## 9. 进入维护模式

**POST** `/api/v1/cmdb/:id/maintenance`

**权限**: `plugin:update`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `reason` | string | ✅ | 维护原因（必填） |
| `end_at` | string | ❌ | 维护结束时间（RFC3339 格式），不传表示无限期维护 |

---

## 10. 退出维护模式

**POST** `/api/v1/cmdb/:id/resume`

**权限**: `plugin:update`

---

## 11. 获取维护日志

**GET** `/api/v1/cmdb/:id/maintenance-logs`

**权限**: `plugin:list`

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "cmdb_id": "uuid",
      "action": "enter",
      "reason": "系统升级",
      "operator": "admin",
      "created_at": "2026-02-18T10:00:00Z"
    }
  ]
}
```

---

## 配置项状态说明

| 状态 | 说明 |
|------|------|
| `active` | 正常运行 |
| `inactive` | 已下线 |
| `maintenance` | 维护模式（不参与自愈执行） |
