# 密钥管理

密钥管理模块负责 SSH 凭据的集中管理和分发，支持多种密钥源类型。

---

## 设计思路

### 为什么需要 query_key？

**问题**：不同客户的密钥存储方式不同：
- 有的按 IP 存储：`/secrets/192.168.1.100`
- 有的按主机名存储：`/secrets/web-server-01`

**方案**：新增 `query_key` 字段，用户只需填写基础路径，系统自动拼接：

```json
{
  "secret_path": "secret/data/hosts",
  "query_key": "ip"
}
// 查询时自动变成: secret/data/hosts/192.168.1.100
```

> **注意**：系统会自动处理路径末尾的斜杠，无论用户填 `/hosts` 还是 `/hosts/` 都能正确工作。

### 为什么有两个测试接口？

| 接口 | 用途 | 是否影响启用 |
|------|------|-------------|
| `/test` | 测试密钥源能否连通 | ✅ 是启用的前提 |
| `/test-query` | 测试某台主机能否获取凭据 | ❌ 仅作查询工具 |

**原因**：
- `/test` 验证配置是否正确（地址、认证等）
- `/test-query` 让用户可以测试某台主机是否在密钥源中

### 启用流程

1. 创建密钥源 → 默认 `inactive`
2. 用户可调 `/test` 手动测试连接
3. 调 `/enable` 时，系统**自动再测一次连接**，通过才启用

> **设计原因**：避免用户启用了一个无法连通的密钥源，导致后续执行任务失败。

---

## API 接口

### 密钥源管理

| 操作 | API |
|------|-----|
| 列表 | `GET /api/v1/secrets-sources` |
| 创建 | `POST /api/v1/secrets-sources` |
| 详情 | `GET /api/v1/secrets-sources/{id}` |
| 更新 | `PUT /api/v1/secrets-sources/{id}` |
| 删除 | `DELETE /api/v1/secrets-sources/{id}` |

### 列表筛选参数

```http
GET /api/v1/secrets-sources?type=vault&status=active&is_default=true
```

| 参数 | 说明 |
|------|------|
| `type` | 类型筛选：`file`, `vault`, `webhook` |
| `status` | 状态筛选：`active`, `inactive` |
| `is_default` | 是否默认：`true`, `false` |

### 测试与启用

| 操作 | API | 说明 |
|------|-----|------|
| 测试连接 | `POST /{id}/test` | 更新 `last_test_at` |
| 测试主机 | `POST /{id}/test-query` | 测试指定主机能否获取凭据 |
| 启用 | `POST /{id}/enable` | 自动测试后启用 |
| 禁用 | `POST /{id}/disable` | 直接禁用 |

### test-query 请求/响应

支持**单主机**和**多主机**两种测试方式。

#### 单主机测试

**请求**：
```json
{
  "hostname": "web-server-01",
  "ip_address": "192.168.1.100"
}
```

**响应**：
```json
{
  "success": true,
  "auth_type": "ssh_key",
  "username": "root",
  "has_credential": true,
  "message": "成功获取凭据"
}
```

#### 多主机批量测试

**请求**：
```json
{
  "hosts": [
    {"hostname": "web-server-01", "ip_address": "192.168.1.100"},
    {"hostname": "db-server-01", "ip_address": "192.168.1.101"},
    {"hostname": "unknown-host", "ip_address": "192.168.1.200"}
  ]
}
```

**响应**：
```json
{
  "success_count": 2,
  "fail_count": 1,
  "results": [
    {
      "hostname": "web-server-01",
      "ip_address": "192.168.1.100",
      "success": true,
      "auth_type": "ssh_key",
      "username": "root",
      "has_credential": true,
      "message": "成功获取凭据"
    },
    {
      "hostname": "db-server-01",
      "ip_address": "192.168.1.101",
      "success": true,
      "auth_type": "password",
      "username": "root",
      "has_credential": true,
      "message": "成功获取凭据"
    },
    {
      "hostname": "unknown-host",
      "ip_address": "192.168.1.200",
      "success": false,
      "message": "获取凭据失败: host not found"
    }
  ]
}
```

> **注意**：
> - 即使成功，也不返回实际的密钥内容（出于安全考虑）
> - 批量测试时，即使部分主机失败，接口仍返回 200

---

## 数据模型

```
SecretsSource
├── id               # UUID
├── name             # 名称
├── type             # 类型 (file, vault, webhook)
├── auth_type        # 认证类型 (ssh_key, password)
├── config           # 配置信息 (JSON)
├── is_default       # 是否默认密钥源
├── priority         # 优先级
├── status           # 状态 (active, inactive)
├── last_test_at     # 最后测试时间（调用 /test 时更新）
└── created_at       # 创建时间
```

---

## 配置示例

### File（所有主机共用）

所有主机共用一个 SSH 密钥文件，适用于统一部署公钥的场景：

```json
{
  "name": "本地密钥",
  "type": "file",
  "auth_type": "ssh_key",
  "config": {
    "key_path": "/etc/auto-healing/secrets/id_rsa",
    "username": "root"
  }
}
```

> **注意**：
> - File 模式下所有主机使用同一个密钥
> - 如果设为默认密钥源，所有执行任务都会用这个密钥

> **安全限制**：
> - 只允许目录：`/etc/auto-healing/secrets/`
> - 禁止路径包含 `..`

### Vault（按主机查询）

```json
{
  "name": "Vault 密钥",
  "type": "vault",
  "auth_type": "ssh_key",
  "config": {
    "address": "https://vault.example.com:8200",
    "secret_path": "secret/data/hosts",
    "query_key": "ip",
    "auth": {
      "type": "token",
      "token": "hvs.xxxxx"
    }
  }
}
```

### Webhook（按主机查询）

```json
{
  "name": "密钥 API",
  "type": "webhook",
  "auth_type": "password",
  "config": {
    "url": "https://secrets-api.example.com/query",
    "method": "POST",
    "query_key": "ip",
    "auth": {
      "type": "bearer",
      "token": "your-token"
    }
  }
}
```

> **必填字段**：`url`、`method`、`query_key`

---

## query_key 说明

| 类型 | query_key | 行为 |
|------|-----------|------|
| `file` | 不需要 | 所有主机共用密钥文件 |
| `webhook` | **必填** | 按 IP 或 hostname 查询 |
| `vault` | **必填** | 按 IP 或 hostname 查询 |

| query_key 值 | 查询方式 | 示例 |
|-----|----------|------|
| `ip` | 按 IP 查询 | `url/192.168.1.100` |
| `hostname` | 按主机名查询 | `url/web-server-01` |

> **智能处理**：执行任务时，系统从 CMDB 获取主机的真实 IP 和 hostname，无论用户在 `target_hosts` 填的是什么。
