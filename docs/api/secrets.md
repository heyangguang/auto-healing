# 密钥源管理 API 文档

**路径前缀**: `/api/v1/secrets-sources`  
**权限**: 已登录用户（租户内数据隔离）

> 注意：路径前缀为 `/secrets-sources`（带连字符），不是 `/secrets`。

---

## 1. 获取密钥源列表

**GET** `/api/v1/secrets-sources`

**权限**: `plugin:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `type` | string | ❌ | 类型：`ssh_key` / `username_password` / `api_token` / `vault` |
| `status` | string | ❌ | 状态筛选 |
| `is_default` | bool | ❌ | 是否为默认密钥源：`true` / `false` |

> 注意：此接口**不支持分页**，返回所有符合条件的密鑰源列表。

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "name": "生产环境 SSH 密鑰",
      "type": "ssh_key",
      "auth_type": "ssh_key",
      "is_default": true,
      "priority": 1,
      "status": "enabled",
      "config": {"username": "ops", "private_key": "***"},
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-02-18T10:00:00Z"
    }
  ]
}
```

> 注意：响应中敏感字段（token、password、secret、api_key）会被隐藏为 `***`。

---

## 2. 创建密钥源

**POST** `/api/v1/secrets-sources`

**权限**: `plugin:create`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 名称 |
| `type` | string | ✅ | 类型：`ssh_key` / `username_password` / `api_token` / `vault` |
| `auth_type` | string | ✅ | 认证方式（与 type 一致或进一步区分） |
| `config` | object | ✅ | 密钥配置（JSON 对象，根据 type 包含对应字段，如 `username`、`password`、`private_key`、`token` 等） |
| `is_default` | bool | ❌ | 是否为默认密钥源，默认 false |
| `priority` | int | ❌ | 优先级，数字越小优先级越高 |

---

## 3. 获取密钥源统计

**GET** `/api/v1/secrets-sources/stats`

**权限**: `plugin:list`

---

## 4. 获取密钥源详情

**GET** `/api/v1/secrets-sources/:id`

**权限**: `plugin:list`

---

## 5. 更新密钥源

**PUT** `/api/v1/secrets-sources/:id`

**权限**: `plugin:update`

---

## 6. 删除密钥源

**DELETE** `/api/v1/secrets-sources/:id`

**权限**: `plugin:delete`

> 如果密钥源被任务模板引用，删除会失败。

---

## 7. 测试连接

**POST** `/api/v1/secrets-sources/:id/test`

**权限**: `plugin:test`

测试密钥源是否可用（SSH 连接测试等）。

---

## 8. 测试查询

**POST** `/api/v1/secrets-sources/:id/test-query`

**权限**: `plugin:test`

测试密鑰源能否为指定主机匹配到凭据，支持单选和多选主机注意模式。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `hostname` | string | ❌ | 主机名（单选模式） |
| `ip_address` | string | ❌ | IP 地址（单选模式） |
| `hosts` | []object | ❌ | 多主机列表（多选模式），每个元素包含 `hostname` 和 `ip_address` |

> 单选模式：传入 `hostname` 和/或 `ip_address`。多选模式：传入 `hosts` 数组。

### 响应

```json
{
  "code": 0,
  "data": {
    "results": [
      {
        "hostname": "prod-web-01",
        "ip_address": "192.168.1.100",
        "success": true,
        "auth_type": "ssh_key",
        "username": "ops",
        "has_credential": true,
        "message": "成功获取凭据"
      }
    ],
    "success_count": 1,
    "fail_count": 0
  }
}
```

---

## 9. 启用密钥源

**POST** `/api/v1/secrets-sources/:id/enable`

**权限**: `plugin:update`

---

## 10. 禁用密钥源

**POST** `/api/v1/secrets-sources/:id/disable`

**权限**: `plugin:update`

---

## 密钥查询（独立接口）

### 11. 查询密钥值

**POST** `/api/v1/secrets/query`

**权限**: `plugin:list`

用于在执行任务时动态查询密钥值（如 Vault 中的密钥）。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `source_id` | uuid | ✅ | 密钥源 ID |
| `path` | string | ❌ | 查询路径（Vault 类型） |
| `key` | string | ❌ | 查询键名 |
