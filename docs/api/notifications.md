# 通知管理 API 文档

**权限**: 已登录用户（租户内数据隔离）

> 注意：通知渠道、模板、发送日志分别使用独立的路径前缀，不在同一个 `/notifications` 下。

---

## 通知渠道（Channels）

**路径前缀**: `/api/v1/channels`

### 1. 获取渠道列表

**GET** `/api/v1/channels`

**权限**: `channel:list`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称） |
| `type` | string | ❌ | 渠道类型：`email` / `dingtalk` / `wecom` / `slack` / `teams` / `webhook` |

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "uuid",
      "name": "运维企业微信群",
      "description": "运维团队告警通知",
      "type": "wecom",
      "retry_config": {
        "max_retries": 3,
        "retry_intervals": [1, 5, 15]
      },
      "recipients": ["ops-team@example.com"],
      "is_active": true,
      "is_default": false,
      "created_at": "2026-01-01T00:00:00Z",
      "updated_at": "2026-02-18T10:00:00Z"
    }
  ],
  "total": 5,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 创建渠道

**POST** `/api/v1/channels`

**权限**: `channel:create`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 渠道名称 |
| `description` | string | ❌ | 描述 |
| `type` | string | ✅ | 渠道类型：`email` / `dingtalk` / `wecom` / `slack` / `teams` / `webhook` |
| `config` | object | ✅ | 渠道配置（根据类型不同） |
| `retry_config` | object | ❌ | 重试配置 |
| `recipients` | []string | ❌ | 默认接收人列表 |
| `is_default` | bool | ❌ | 是否默认渠道 |
| `rate_limit_per_minute` | int | ❌ | 每分钟限流阈值 |

**企业微信配置**:
```json
{
  "name": "运维企业微信群",
  "type": "wecom",
  "config": {
    "webhook_url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
  }
}
```

**DingTalk 配置**:
```json
{
  "name": "运维钉钉群",
  "type": "dingtalk",
  "config": {
    "webhook_url": "https://oapi.dingtalk.com/robot/send?access_token=xxx",
    "secret": "SEC..."
  }
}
```

> 历史兼容说明：已经错误创建为 `type=dingtalk` 且 `webhook_url`
> 指向 `https://qyapi.weixin.qq.com/cgi-bin/webhook/send?...` 的旧渠道，
> 系统仍会按企业微信机器人格式发送，避免存量配置立即失效。
> 新建企业微信渠道请统一使用 `type=wecom`。

**Slack 配置**:
```json
{
  "name": "运维 Slack Channel",
  "type": "slack",
  "config": {
    "webhook_url": "https://hooks.slack.com/services/xxx/yyy/zzz",
    "channel": "#ops-alerts",
    "username": "auto-healing",
    "icon_emoji": ":robot_face:"
  }
}
```

**Teams 配置**:
```json
{
  "name": "运维 Teams Channel",
  "type": "teams",
  "config": {
    "webhook_url": "https://example.webhook.office.com/webhookb2/...",
    "theme_color": "6264A7"
  }
}
```

**Email 配置**:
```json
{
  "name": "运维邮件",
  "type": "email",
  "config": {
    "smtp_host": "smtp.example.com",
    "smtp_port": 465,
    "smtp_user": "noreply@example.com",
    "smtp_password": "password",
    "from_name": "自愈系统",
    "to_addresses": ["ops@example.com"]
  }
}
```

**Webhook 配置**:
```json
{
  "name": "自定义 Webhook",
  "type": "webhook",
  "config": {
    "url": "https://hooks.example.com/notify",
    "method": "POST",
    "headers": {"Authorization": "Bearer token"},
    "auth_type": "basic",
    "auth_username": "user",
    "auth_password": "pass"
  }
}
```

---

### 3. 获取渠道详情

**GET** `/api/v1/channels/:id`

**权限**: `channel:list`

---

### 4. 更新渠道

**PUT** `/api/v1/channels/:id`

**权限**: `channel:update`

---

### 5. 删除渠道

**DELETE** `/api/v1/channels/:id`

**权限**: `channel:delete`

> 如果渠道正在被模板引用，删除会失败（引用计数保护）。

---

### 6. 测试渠道

**POST** `/api/v1/channels/:id/test`

**权限**: `channel:update`

---

## 通知模板（Templates）

**路径前缀**: `/api/v1/templates`

### 7. 获取模板列表

**GET** `/api/v1/templates`

**权限**: `template:list`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `name` | string | ❌ | 按模板名称模糊搜索 |
| `event_type` | string | ❌ | 事件类型筛选（见下方枚举） |
| `format` | string | ❌ | 消息格式：`text` / `markdown` / `html` |
| `supported_channel` | string | ❌ | 支持的渠道类型：`email` / `dingtalk` / `wecom` / `slack` / `teams` / `webhook` |
| `is_active` | bool | ❌ | 是否启用：`true` / `false` |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` / `updated_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

### 8. 创建模板

**POST** `/api/v1/templates`

**权限**: `template:create`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 模板名称 |
| `description` | string | ❌ | 描述 |
| `event_type` | string | ❌ | 事件类型（见下方枚举） |
| `supported_channels` | []string | ❌ | 支持的渠道类型列表：`email` / `dingtalk` / `wecom` / `slack` / `teams` / `webhook` |
| `subject_template` | string | ❌ | 主题模板（Email 类型） |
| `body_template` | string | ✅ | 消息体模板（支持 Go 模板语法） |
| `format` | string | ❌ | 消息格式：`text` / `markdown` / `html`，默认 `text` |
| `is_active` | bool | ❌ | 是否启用 |

---

### 9. 获取模板详情

**GET** `/api/v1/templates/:id`

**权限**: `template:list`

---

### 10. 更新模板

**PUT** `/api/v1/templates/:id`

**权限**: `template:update`

---

### 11. 删除模板

**DELETE** `/api/v1/templates/:id`

**权限**: `template:delete`

---

### 12. 预览模板

**POST** `/api/v1/templates/:id/preview`

**权限**: `template:list`

使用示例数据渲染模板内容预览（不实际发送）。

---

### 13. 获取可用模板变量

**GET** `/api/v1/template-variables`

返回所有事件类型可用的模板变量列表，用于模板编辑器提示。

---

## 通知发送记录（Notifications）

**路径前缀**: `/api/v1/notifications`

### 14. 手动发送通知

**POST** `/api/v1/notifications/send`

**权限**: `notification:send`

---

### 15. 获取通知发送记录列表

**GET** `/api/v1/notifications`

**权限**: `notification:list`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `status` | string | ❌ | 状态：`pending` / `sent` / `delivered` / `failed` / `bounced` |
| `task_name` | string | ❌ | 按任务名称筛选 |
| `triggered_by` | string | ❌ | 触发来源筛选 |
| `subject` | string | ❌ | 按通知标题模糊搜索 |
| `channel_id` | uuid | ❌ | 按渠道筛选 |
| `template_id` | uuid | ❌ | 按模板筛选 |
| `task_id` | uuid | ❌ | 按执行任务筛选 |
| `execution_run_id` | uuid | ❌ | 按执行记录筛选 |
| `created_after` | string | ❌ | 创建时间起始（RFC3339） |
| `created_before` | string | ❌ | 创建时间结束（RFC3339） |
| `sort_by` | string | ❌ | 排序字段：`created_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

### 16. 获取通知统计

**GET** `/api/v1/notifications/stats`

**权限**: `notification:list`

---

### 17. 获取通知详情

**GET** `/api/v1/notifications/:id`

**权限**: `notification:list`

---

## 事件类型（event_type）枚举

可选值：`execution_started` / `execution_result` / `flow_result` / `approval_required` / `manual_notification`

| 值 | 说明 |
|----|------|
| `execution_started` | 执行任务开始 |
| `execution_result` | 执行任务结果（成功/失败） |
| `flow_result` | 自愈流程结果（完成/失败） |
| `approval_required` | 自愈流程等待审批 |
| `manual_notification` | 手动发送或通用通知模板 |
