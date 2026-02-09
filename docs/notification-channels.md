# 通知渠道 API 文档

通知渠道是发送通知的通道配置，支持 Webhook、邮件、钉钉三种类型。

## 数据模型

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string (uuid) | - | 渠道 ID (自动生成) |
| name | string | ✅ | 渠道名称 (唯一) |
| type | string | ✅ | 渠道类型: `webhook`, `email`, `dingtalk` |
| description | string | - | 描述 |
| config | object | ✅ | 渠道配置 (根据类型不同，仅创建/更新时使用，列表接口不返回) |
| retry_config | object | - | 重试配置 |
| recipients | string[] | - | 接收人列表 |
| is_active | boolean | - | 是否启用 (默认 true) |
| is_default | boolean | - | 是否默认渠道（当发送时未指定渠道时使用） |
| rate_limit_per_minute | number | - | 每分钟速率限制（超出限制的通知会失败） |
| created_at | string | - | 创建时间 |
| updated_at | string | - | 更新时间 |

### 渠道配置 (config)

> [!NOTE]
> `config` 字段包含敏感信息（如密码、token），在列表和详情接口中**不会返回**，仅在创建和更新时使用。

#### Webhook 类型
```json
{
  "url": "https://example.com/webhook",
  "method": "POST",
  "headers": {
    "X-Custom-Header": "custom-value"
  },
  "timeout_seconds": 30,
  "username": "api_user",
  "password": "api_password"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| url | string | ✅ | Webhook URL |
| method | string | - | HTTP 方法，默认 POST |
| headers | object | - | 自定义请求头 |
| timeout_seconds | number | - | 超时时间（秒），默认 30 |
| username | string | - | Basic Auth 用户名 |
| password | string | - | Basic Auth 密码 |

> [!TIP]
> 如果配置了 `username` 和 `password`，系统会自动生成 `Authorization: Basic xxx` 请求头。

发送时的请求体格式（固定）：
```json
{
  "subject": "通知主题",
  "body": "通知内容",
  "format": "text",
  "recipients": ["..."],
  "timestamp": "2026-01-10T10:00:00+08:00"
}
```

#### 邮件类型 (email)
```json
{
  "smtp_host": "smtp.example.com",
  "smtp_port": 587,
  "username": "user@example.com",
  "password": "xxx",
  "from_address": "noreply@example.com",
  "use_tls": true
}
```

#### 钉钉类型 (dingtalk)
```json
{
  "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
  "secret": "SEC..."
}
```

### 重试配置 (retry_config)
```json
{
  "max_retries": 3,
  "retry_intervals": [1, 5, 15]
}
```
- `max_retries`: 最大重试次数
- `retry_intervals`: 重试间隔（分钟），如 `[1, 5, 15]` 表示第1次重试在1分钟后，第2次在5分钟后，第3次在15分钟后

### 接收人列表 (recipients)

配置此渠道的接收人列表。发送通知时将使用这里配置的接收人。

**使用场景**：为通知渠道指定固定的接收人，如运维负责人的手机号或邮箱。

```json
{
  "recipients": ["13800138001", "admin@example.com", "ops-team@example.com"]
}
```

### 默认渠道 (is_default)

当发送通知时如果没有指定 `channel_ids`，系统会自动使用 `is_default=true` 且 `is_active=true` 的渠道。

> [!NOTE]
> 建议只设置一个渠道为默认渠道。

### 速率限制 (rate_limit_per_minute)

限制每分钟通过此渠道发送的通知数量。超出限制的通知会立即失败，错误信息为"超出速率限制: X 条/分钟"。

**使用场景**：防止短时间内大量告警轰炸接收人。

---

## API 接口

### 1. 获取渠道列表

**GET** `/api/v1/channels`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| type | string | 筛选渠道类型: `webhook`, `email`, `dingtalk` |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "46f98331-ffa7-4a58-85f6-3a5f4ec06b45",
      "name": "钉钉-运维群",
      "type": "dingtalk",
      "description": "运维告警通知群",
      "retry_config": {
        "max_retries": 3,
        "retry_intervals": [1, 5, 15]
      },
      "recipients": ["13800138001"],
      "is_active": true,
      "is_default": true,
      "rate_limit_per_minute": 60,
      "created_at": "2026-01-10T04:17:40.359041+08:00",
      "updated_at": "2026-01-10T04:17:40.359041+08:00"
    }
  ],
  "total": 15,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 创建渠道

**POST** `/api/v1/channels`

**请求体**
```json
{
  "name": "告警 Webhook",
  "type": "webhook",
  "description": "发送告警通知",
  "config": {
    "url": "https://example.com/alert",
    "method": "POST",
    "headers": {
      "Content-Type": "application/json"
    }
  },
  "retry_config": {
    "max_retries": 3,
    "retry_intervals": [1, 5, 15]
  },
  "recipients": ["admin@example.com"],
  "is_default": false,
  "rate_limit_per_minute": 100
}
```

**响应示例** (HTTP 201)
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174001",
    "name": "告警 Webhook",
    "type": "webhook",
    "description": "发送告警通知",
    "retry_config": {
      "max_retries": 3,
      "retry_intervals": [1, 5, 15]
    },
    "recipients": ["admin@example.com"],
    "is_active": true,
    "is_default": false,
    "rate_limit_per_minute": 100,
    "created_at": "2026-01-10T10:00:00+08:00",
    "updated_at": "2026-01-10T10:00:00+08:00"
  }
}
```

---

### 3. 获取渠道详情

**GET** `/api/v1/channels/:id`

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174001",
    "name": "告警 Webhook",
    "type": "webhook",
    "description": "发送告警通知",
    "retry_config": {
      "max_retries": 3,
      "retry_intervals": [1, 5, 15]
    },
    "recipients": ["admin@example.com"],
    "is_active": true,
    "is_default": false,
    "rate_limit_per_minute": 100,
    "created_at": "2026-01-10T10:00:00+08:00",
    "updated_at": "2026-01-10T10:00:00+08:00"
  }
}
```

---

### 4. 更新渠道

**PUT** `/api/v1/channels/:id`

**请求体** (所有字段可选)
```json
{
  "name": "更新后的名称",
  "description": "更新后的描述",
  "config": {
    "url": "https://new-url.com/webhook"
  },
  "recipients": ["new-admin@example.com"],
  "is_active": true,
  "is_default": false,
  "rate_limit_per_minute": 50
}
```

**响应**: 返回更新后的渠道对象

---

### 5. 删除渠道

**DELETE** `/api/v1/channels/:id`

> [!IMPORTANT]
> **保护性删除**：以下情况禁止删除
> - 有通知模板支持此渠道类型（`supported_channels` 包含该渠道类型）
> - 有任务模板使用此渠道（`notification_config` 中引用 `channel_ids`）
> - 有自愈流程使用此渠道（流程节点配置中引用该渠道）

**成功响应**
```json
{
  "code": 0,
  "message": "删除成功"
}
```

**失败响应（被模板引用）**
```json
{
  "code": 40000,
  "message": "无法删除：有 2 个通知模板支持此渠道类型（webhook），请先修改这些模板的 supported_channels"
}
```

**失败响应（被任务模板引用）**
```json
{
  "code": 40000,
  "message": "无法删除：有 3 个任务模板使用此通知渠道，请先修改这些任务的通知配置"
}
```

**失败响应（被自愈流程引用）**
```json
{
  "code": 40000,
  "message": "无法删除：有 1 个自愈流程使用此通知渠道，请先修改这些流程的通知节点配置"
}
```

---

### 6. 测试渠道

**POST** `/api/v1/channels/:id/test`

测试渠道连通性，发送测试消息。

**响应示例**
```json
{
  "code": 0,
  "message": "测试成功"
}
```

**失败响应**
```json
{
  "code": 40000,
  "message": "连接超时"
}
```

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /channels | channel:list |
| POST /channels | channel:create |
| GET /channels/:id | channel:list |
| PUT /channels/:id | channel:update |
| DELETE /channels/:id | channel:delete |
| POST /channels/:id/test | channel:list |
