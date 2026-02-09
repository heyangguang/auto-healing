# 通知记录 API 文档

通知记录保存了所有发送的通知的历史记录，包括发送状态、失败原因、重试信息等。

## 数据模型

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string (uuid) | 记录 ID |
| template_id | string (uuid) | 使用的模板 ID (可空) |
| channel_id | string (uuid) | 发送渠道 ID |
| execution_run_id | string (uuid) | 关联的执行记录 ID (可空) |
| workflow_instance_id | string (uuid) | 关联的流程实例 ID (可空) |
| incident_id | string (uuid) | 关联的事件 ID (可空) |
| recipients | string[] | 接收人列表（来自渠道的 recipients 配置） |
| subject | string | 通知主题 |
| body | string | 通知内容 |
| status | string | 状态: `pending`, `sent`, `failed`, `delivered`, `bounced` |
| external_message_id | string | 外部消息 ID (来自渠道返回) |
| response_data | object | 渠道响应数据 |
| error_message | string | 错误信息 (失败时) |
| retry_count | number | 已重试次数 |
| next_retry_at | string | 下次重试时间 (可空) |
| sent_at | string | 发送成功时间 (可空) |
| created_at | string | 创建时间 |
| template | object | 关联的模板对象 (列表/详情接口返回) |
| channel | object | 关联的渠道对象 (列表/详情接口返回) |
| execution_run | object | 关联的执行记录对象，包含任务模板信息 |

### execution_run 嵌套对象

| 字段 | 说明 |
|------|------|
| `id` | 执行记录 ID |
| `triggered_by` | 触发类型：`manual`(手动) / `scheduler:cron`(定时) / `scheduler:once`(单次调度) / `healing`(自愈) |
| `status` | 执行状态 |
| `task` | 任务模板对象 |
| `task.id` | 任务模板 ID |
| `task.name` | **任务模板名称** |

### 状态说明

| 状态 | 说明 |
|------|------|
| pending | 待发送 |
| sent | 已发送 |
| failed | 发送失败（包括超出速率限制） |
| delivered | 已送达 (部分渠道支持回执) |
| bounced | 退信 (邮件) |

---

## API 接口

### 1. 获取通知记录列表

**GET** `/api/v1/notifications`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| status | string | 筛选状态: `pending`, `sent`, `failed`, `delivered`, `bounced` |
| channel_id | string (uuid) | 筛选渠道 ID |
| template_id | string (uuid) | 筛选模板 ID |
| task_id | string (uuid) | 筛选任务模板 ID |
| task_name | string | 模糊搜索任务模板名称 |
| triggered_by | string | 筛选触发类型: `manual`, `scheduler:cron`, `scheduler:once`, `healing` |
| execution_run_id | string (uuid) | 筛选执行记录 ID |
| search | string | 模糊搜索通知主题 |
| created_after | string (RFC3339) | 创建时间起始，如 `2026-01-01T00:00:00+08:00` |
| created_before | string (RFC3339) | 创建时间结束 |
| sort_by | string | 排序字段: `created_at`, `status`, `subject`, `sent_at` |
| sort_order | string | 排序方向: `asc`, `desc` (默认 desc) |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "d4714f3e-9933-482b-afd2-5513c5f90535",
      "template_id": "2514a26f-0f73-4b3e-883a-35f9af8af00e",
      "channel_id": "46f98331-ffa7-4a58-85f6-3a5f4ec06b45",
      "execution_run_id": "f26be30e-e974-49e0-93d0-d4bd25bf468a",
      "recipients": ["13800138001"],
      "subject": "【success】日志清理任务",
      "body": "任务执行完成\n状态: success ✅\n耗时: 2m 35s",
      "status": "sent",
      "retry_count": 0,
      "sent_at": "2026-01-12T18:52:24+08:00",
      "created_at": "2026-01-12T18:52:23.753201+08:00",
      "template": {
        "id": "2514a26f-0f73-4b3e-883a-35f9af8af00e",
        "name": "任务执行通知",
        "description": "任务执行结果通知模板",
        "event_type": "execution_result",
        "supported_channels": ["webhook", "email", "dingtalk"],
        "format": "markdown",
        "is_active": true,
        "created_at": "2026-01-10T04:17:40.722912+08:00",
        "updated_at": "2026-01-10T04:17:40.722912+08:00"
      },
      "channel": {
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
        "created_at": "2026-01-10T04:17:40.359041+08:00",
        "updated_at": "2026-01-10T04:17:40.359041+08:00"
      }
    }
  ],
  "total": 13,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 获取通知记录详情

**GET** `/api/v1/notifications/:id`

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "d4714f3e-9933-482b-afd2-5513c5f90535",
    "template_id": "2514a26f-0f73-4b3e-883a-35f9af8af00e",
    "channel_id": "46f98331-ffa7-4a58-85f6-3a5f4ec06b45",
    "execution_run_id": "f26be30e-e974-49e0-93d0-d4bd25bf468a",
    "recipients": ["13800138001"],
    "subject": "【success】日志清理任务",
    "body": "任务执行完成\n状态: success ✅\n耗时: 2m 35s",
    "status": "sent",
    "retry_count": 0,
    "sent_at": "2026-01-12T18:52:24+08:00",
    "created_at": "2026-01-12T18:52:23.753201+08:00",
    "template": { "..." },
    "channel": { "..." }
  }
}
```

---

### 3. 手动发送通知

**POST** `/api/v1/notifications/send`

手动发送通知，可使用模板或直接指定内容。

> [!NOTE]
> 这个接口主要用于测试或特殊场景的手动触发。正常的任务执行通知由系统自动发送，使用任务模板中的 `notification_config` 配置。

**请求体 - 使用模板**
```json
{
  "template_id": "2514a26f-0f73-4b3e-883a-35f9af8af00e",
  "channel_ids": ["46f98331-ffa7-4a58-85f6-3a5f4ec06b45"],
  "variables": {
    "execution": {
      "status": "success",
      "status_emoji": "✅",
      "duration": "2m 35s"
    },
    "task": {
      "name": "日志清理任务",
      "target_hosts": "192.168.1.1"
    },
    "timestamp": "2026-01-10 10:00:00"
  }
}
```

> [!TIP]
> 如果 `channel_ids` 为空或不传，系统会自动使用 `is_default=true` 的默认渠道。

**请求体 - 直接指定内容**
```json
{
  "channel_ids": ["46f98331-ffa7-4a58-85f6-3a5f4ec06b45"],
  "subject": "系统通知",
  "body": "这是一条测试通知",
  "format": "text"
}
```

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "notification_ids": ["new12345-e89b-12d3-a456-426614174000"],
    "logs": [
      {
        "id": "new12345-e89b-12d3-a456-426614174000",
        "channel_id": "46f98331-ffa7-4a58-85f6-3a5f4ec06b45",
        "status": "sent",
        "sent_at": "2026-01-10T10:10:00+08:00"
      }
    ]
  }
}
```

---

## 失败通知示例

### 发送失败（网络/渠道错误）

```json
{
  "id": "d4714f3e-9933-482b-afd2-5513c5f90535",
  "status": "failed",
  "error_message": "钉钉发送失败: token is not exist (errcode: 300005)",
  "retry_count": 1,
  "next_retry_at": "2026-01-12T18:57:23+08:00",
  "created_at": "2026-01-12T18:52:23.753201+08:00"
}
```

系统会根据渠道的 `retry_config` 自动重试失败的通知。

### 超出速率限制

```json
{
  "id": "e5825f4f-0044-593c-b0e3-6624d6a01646",
  "status": "failed",
  "error_message": "超出速率限制: 60 条/分钟",
  "retry_count": 0,
  "created_at": "2026-01-12T18:52:23.753201+08:00"
}
```

超出速率限制的通知**不会自动重试**。

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /notifications | notification:list |
| GET /notifications/:id | notification:list |
| POST /notifications/send | notification:send |

---

## 前端展示建议

### 列表页面
- 显示时间、渠道名称、状态、接收人
- 状态使用标签颜色：
  - `sent` / `delivered` = 绿色
  - `failed` / `bounced` = 红色
  - `pending` = 黄色
- 支持按状态和渠道筛选
- 失败通知显示重试次数和下次重试时间

### 详情页面
- 显示完整的通知内容 (subject + body)
- 显示关联资源 (执行记录、流程实例等)
- 失败时显示错误信息 (`error_message`)
- 显示渠道响应数据 (`response_data`)
- 利用返回的 `template` 和 `channel` 对象展示关联信息
