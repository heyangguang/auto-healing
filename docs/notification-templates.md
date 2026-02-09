# 通知模板 API 文档

通知模板定义了通知内容的格式和结构，支持 `{{变量名}}` 格式的变量替换。

## 数据模型

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string (uuid) | - | 模板 ID (自动生成) |
| name | string | ✅ | 模板名称 |
| description | string | - | 描述 |
| event_type | string | - | 事件类型: `execution_result`, `flow_execution`, `incident_created`, `approval_required`, `custom` |
| supported_channels | string[] | - | 支持的渠道类型，如 `["webhook", "email", "dingtalk"]` |
| subject_template | string | - | 主题模板 (用于邮件等) |
| body_template | string | ✅ | 内容模板 |
| format | string | - | 格式: `text`, `markdown`, `html` (默认 text) |
| available_variables | string[] | - | 模板中使用的变量列表 (自动提取) |
| is_active | boolean | - | 是否启用 (默认 true) |
| created_at | string | - | 创建时间 |
| updated_at | string | - | 更新时间 |

---

## 模板语法

使用 `{{变量名}}` 进行变量替换，支持嵌套路径如 `{{execution.status}}`。

### 示例模板
```
【{{execution.status}}】任务执行通知 {{execution.status_emoji}}

任务名称: {{task.name}}
执行时间: {{timestamp}}
目标主机: {{task.target_hosts}} (共 {{task.host_count}} 台)
执行耗时: {{execution.duration}}

执行统计:
- 成功: {{stats.ok}} | 变更: {{stats.changed}} | 失败: {{stats.failed}}

---
{{system.name}} | {{date}} {{time}}
```

---

## 可用变量

获取所有可用变量: **GET** `/api/v1/template-variables`

### 变量分类

| 分类 | 变量 | 说明 |
|------|------|------|
| **execution** | `execution.run_id` | 执行记录 ID |
| | `execution.status` | 执行状态 (success/failed/timeout/cancelled) |
| | `execution.status_emoji` | 状态表情 (✅/❌/⏱️/🚫) |
| | `execution.exit_code` | 退出码 |
| | `execution.triggered_by` | 触发者 |
| | `execution.trigger_type` | 触发类型 (manual/scheduled/workflow) |
| | `execution.started_at` | 开始时间 |
| | `execution.completed_at` | 完成时间 |
| | `execution.duration` | 执行时长 (如 2m 35s) |
| | `execution.duration_seconds` | 执行时长(秒) |
| **task** | `task.id` | 任务模板 ID |
| | `task.name` | 任务名称 |
| | `task.target_hosts` | 目标主机列表 |
| | `task.host_count` | 主机数量 |
| | `task.executor_type` | 执行器类型 (local/docker) |
| **repository** | `repository.id` | 仓库 ID |
| | `repository.name` | 仓库名称 |
| | `repository.url` | 仓库 URL |
| | `repository.branch` | 分支 |
| | `repository.playbook` | Playbook 文件 |
| **stats** | `stats.ok` | 成功任务数 |
| | `stats.changed` | 变更任务数 |
| | `stats.failed` | 失败任务数 |
| | `stats.unreachable` | 不可达主机数 |
| | `stats.skipped` | 跳过任务数 |
| **error** | `error.message` | 错误信息 |
| | `error.host` | 出错主机 |
| **system** | `system.name` | 系统名称 |
| | `system.url` | 系统 URL |
| | `system.version` | 系统版本 |
| | `timestamp` | 当前时间 (YYYY-MM-DD HH:MM:SS) |
| | `date` | 当前日期 (YYYY-MM-DD) |
| | `time` | 当前时间 (HH:MM:SS) |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "variables": [
      {"name": "execution.status", "description": "执行状态 (success/failed/timeout/cancelled)", "category": "execution"},
      {"name": "execution.status_emoji", "description": "状态表情 (✅/❌/⏱️/🚫)", "category": "execution"},
      {"name": "task.name", "description": "任务名称", "category": "task"},
      {"name": "task.target_hosts", "description": "目标主机列表", "category": "task"},
      {"name": "timestamp", "description": "当前时间 (YYYY-MM-DD HH:MM:SS)", "category": "system"}
    ]
  }
}
```

---

## API 接口

### 1. 获取模板列表

**GET** `/api/v1/templates`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| search | string | 模糊搜索模板名称 |
| event_type | string | 事件类型筛选 |
| is_active | boolean | 按启用状态筛选 (`true`/`false`) |
| format | string | 按格式筛选 (`text`/`markdown`/`html`) |
| supported_channel | string | 按支持渠道筛选 (`webhook`/`email`/`dingtalk`) |
| sort_by | string | 排序字段 (`name`/`created_at`/`updated_at`) |
| sort_order | string | 排序方向 (`asc`/`desc`) |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "2514a26f-0f73-4b3e-883a-35f9af8af00e",
      "name": "全渠道-通用通知",
      "description": "支持所有渠道类型的通用通知模板",
      "event_type": "custom",
      "supported_channels": ["webhook", "email", "dingtalk"],
      "subject_template": "通用通知",
      "body_template": "{{execution.status}} | {{timestamp}} | {{task.target_hosts}}",
      "format": "text",
      "available_variables": ["execution.status", "timestamp", "task.target_hosts"],
      "is_active": true,
      "created_at": "2026-01-10T04:17:40.722912+08:00",
      "updated_at": "2026-01-10T04:17:40.722912+08:00"
    }
  ],
  "total": 15,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 创建模板

**POST** `/api/v1/templates`

**请求体**
```json
{
  "name": "任务执行通知",
  "description": "任务执行完成后发送通知",
  "event_type": "execution_result",
  "supported_channels": ["webhook", "email", "dingtalk"],
  "subject_template": "【{{execution.status}}】任务执行通知",
  "body_template": "任务执行完成\n状态: {{execution.status}} {{execution.status_emoji}}\n耗时: {{execution.duration}}",
  "format": "markdown"
}
```

**响应示例** (HTTP 201)
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174001",
    "name": "任务执行通知",
    "description": "任务执行完成后发送通知",
    "event_type": "execution_result",
    "supported_channels": ["webhook", "email", "dingtalk"],
    "subject_template": "【{{execution.status}}】任务执行通知",
    "body_template": "任务执行完成\n状态: {{execution.status}} {{execution.status_emoji}}\n耗时: {{execution.duration}}",
    "format": "markdown",
    "available_variables": ["execution.status", "execution.status_emoji", "execution.duration"],
    "is_active": true,
    "created_at": "2026-01-10T10:00:00+08:00",
    "updated_at": "2026-01-10T10:00:00+08:00"
  }
}
```

---

### 3. 获取模板详情

**GET** `/api/v1/templates/:id`

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174001",
    "name": "任务执行通知",
    "description": "任务执行完成后发送通知",
    "event_type": "execution_result",
    "supported_channels": ["webhook", "email", "dingtalk"],
    "subject_template": "【{{execution.status}}】任务执行通知",
    "body_template": "...",
    "format": "markdown",
    "available_variables": ["execution.status", "execution.status_emoji", "execution.duration"],
    "is_active": true,
    "created_at": "2026-01-10T10:00:00+08:00",
    "updated_at": "2026-01-10T10:00:00+08:00"
  }
}
```

---

### 4. 更新模板

**PUT** `/api/v1/templates/:id`

**请求体** (所有字段可选)
```json
{
  "name": "更新后的名称",
  "description": "更新后的描述",
  "body_template": "新的模板内容",
  "is_active": true
}
```

**响应**: 返回更新后的模板对象

---

### 5. 删除模板

**DELETE** `/api/v1/templates/:id`

> [!IMPORTANT]
> **保护性删除**：如果有任务模板使用此通知模板（`notification_config` 中引用 `template_id`），将返回错误。

**成功响应**
```json
{
  "code": 0,
  "message": "删除成功"
}
```

**失败响应（被引用）**
```json
{
  "code": 40000,
  "message": "无法删除：有 3 个任务模板使用此通知模板，请先修改这些任务的通知配置"
}
```

---

### 6. 预览模板

**POST** `/api/v1/templates/:id/preview`

使用示例变量预览模板渲染结果。

**请求体**
```json
{
  "variables": {
    "execution": {
      "status": "success",
      "status_emoji": "✅",
      "duration": "2m 35s"
    },
    "task": {
      "name": "日志清理任务",
      "target_hosts": "192.168.1.1,192.168.1.2",
      "host_count": 2
    },
    "timestamp": "2026-01-10 10:00:00"
  }
}
```

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "subject": "【success】任务执行通知",
    "body": "任务执行完成\n状态: success ✅\n耗时: 2m 35s"
  }
}
```

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /templates | template:list |
| POST /templates | template:create |
| GET /templates/:id | template:list |
| PUT /templates/:id | template:update |
| DELETE /templates/:id | template:delete |
| POST /templates/:id/preview | template:list |
| GET /template-variables | (无需特殊权限) |
