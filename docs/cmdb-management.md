# CMDB 管理

CMDB（Configuration Management Database）配置管理数据库模块。

---

## 数据模型

```
CMDBItem
├── id                    # UUID 主键
├── plugin_id            # 关联插件 ID
├── name                 # 配置项名称
├── type                 # 类型 (server, database, application, network)
├── status               # 状态 (active, offline, maintenance)
├── maintenance_reason   # 维护原因
├── maintenance_start_at # 维护开始时间
├── maintenance_end_at   # 维护结束时间（到期自动恢复）
├── ip_address           # IP 地址
├── hostname             # 主机名
├── os / os_version      # 操作系统
├── cpu / memory / disk  # 硬件信息
├── environment          # 环境 (production, staging, development)
└── raw_data             # 原始数据 (JSON)
```

### 状态说明

| 状态 | 来源 | 说明 |
|------|------|------|
| `active` | CMDB 同步/手动恢复 | 正常运行 |
| `offline` | CMDB 同步 | 已下线 |
| `maintenance` | 平台手动设置 | 维护中（到期自动恢复） |

---

## 维护模式

当需要临时暂停主机的自愈操作时，可将其设置为维护模式。

**维护模式的主机在自愈流程中会被跳过**，不会执行任何脚本。

### 进入维护模式

```http
POST /api/v1/cmdb/{id}/maintenance
```

**请求体：**
```json
{
  "reason": "系统升级",
  "end_at": "2026-01-08T18:00:00+08:00"
}
```

### 退出维护模式

```http
POST /api/v1/cmdb/{id}/resume
```

### 获取维护日志

```http
GET /api/v1/cmdb/{id}/maintenance-logs?page=1&page_size=20
```

### 批量进入维护

```http
POST /api/v1/cmdb/batch/maintenance
```

**请求体：**
```json
{
  "ids": ["uuid1", "uuid2", ...],
  "reason": "批量维护"
}
```

### 批量启用

```http
POST /api/v1/cmdb/batch-enable
```

**请求体：**
```json
{"ids": ["uuid1", "uuid2", ...]}
```

**响应：** `{"total": 10, "success": 9, "failed": 1}`

---

## 接口详情

### 1. 获取 CMDB 统计

```http
GET /api/v1/cmdb/stats
```

---

### 2. 获取 CMDB 列表

```http
GET /api/v1/cmdb
```

| 参数 | 说明 |
|------|------|
| `plugin_id` | 按插件筛选 |
| `has_plugin` | true=有插件, false=插件已删除 |
| `type`, `status`, `environment` | 筛选 |

---

### 3. 获取 CMDB 详情

```http
GET /api/v1/cmdb/{id}
```

---

### 4. 测试单个配置项 SSH 连接

```http
POST /api/v1/cmdb/{id}/test-connection
```

**请求体：**

```json
{
  "secrets_source_id": "uuid-of-secrets-source"
}
```

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "cmdb_id": "uuid",
    "host": "192.168.31.103",
    "success": true,
    "message": "连接成功",
    "auth_type": "ssh_key",
    "latency_ms": 150
  }
}
```

---

### 5. 批量测试 SSH 连接

```http
POST /api/v1/cmdb/batch-test-connection
```

**请求体：**

```json
{
  "cmdb_ids": ["uuid1", "uuid2", "uuid3"],
  "secrets_source_id": "uuid-of-secrets-source"
}
```

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 3,
    "success": 2,
    "failed": 1,
    "results": [
      {
        "cmdb_id": "uuid1",
        "host": "192.168.31.103",
        "success": true,
        "message": "连接成功",
        "auth_type": "ssh_key",
        "latency_ms": 120
      },
      {
        "cmdb_id": "uuid2",
        "host": "192.168.31.100",
        "success": true,
        "message": "连接成功",
        "auth_type": "password",
        "latency_ms": 180
      },
      {
        "cmdb_id": "uuid3",
        "host": "192.168.31.200",
        "success": false,
        "message": "连接失败: connection refused"
      }
    ]
  }
}
```

---

## 连接测试流程

```
用户选择 CMDB 配置项 + Secrets Source
         ↓
从 CMDB 获取 IP 地址
         ↓
调用 Secrets Source 接口获取凭据
         ↓
使用凭据尝试 SSH 连接
         ↓
返回测试结果（凭据不保存）
```

**说明：**
- 密钥不保存在系统中，每次通过 Secrets Source 动态获取
- 支持的认证类型：`ssh_key`（SSH 密钥）、`password`（密码）
- 批量测试最多支持 50 个配置项
