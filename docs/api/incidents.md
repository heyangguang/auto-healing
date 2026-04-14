# 工单（Incidents）管理 API 文档

**路径前缀**: `/api/v1/incidents`  
**权限**: 已登录用户（租户内数据隔离）

> 工单由插件从第三方监控系统同步而来，是自愈流程的触发源。

---

## 1. 获取工单统计

**GET** `/api/v1/incidents/stats`

**权限**: `plugin:list`

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 1500,
    "scanned": 1200,
    "unscanned": 300,
    "matched": 900,
    "pending": 120,
    "processing": 45,
    "healed": 820,
    "failed": 10,
    "skipped": 3,
    "dismissed": 2
  }
}
```

---

## 2. 获取工单列表

**GET** `/api/v1/incidents`

**权限**: `plugin:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（标题、描述、外部 ID） |
| `plugin_id` | uuid | ❌ | 按插件 ID 筛选 |
| `source_plugin_name` | string | ❌ | 按插件名称筛选 |
| `status` | string | ❌ | 状态：`open` / `resolved` / `closed` |
| `healing_status` | string | ❌ | 自愈状态：`pending` / `processing` / `healed` / `failed` / `skipped` / `dismissed` |
| `severity` | string | ❌ | 严重级别：`critical` / `high` / `medium` / `low` |
| `has_plugin` | bool | ❌ | 是否关联插件：`true` / `false` |
| `sort_by` | string | ❌ | 排序字段：`created_at` / `severity` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

## 3. 获取工单详情

**GET** `/api/v1/incidents/:id`

**权限**: `plugin:list`

---

## 4. 关闭工单

**POST** `/api/v1/incidents/:id/close`

**权限**: `plugin:sync`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `resolution` | string | ✅ | 解决方案描述 |
| `work_notes` | string | ❌ | 工作备注 |
| `close_code` | string | ❌ | 关闭代码（如 `resolved` / `not_reproducible`） |
| `close_status` | string | ❌ | 关闭后状态，默认 `resolved` |

### 响应字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `message` | string | 结果消息 |
| `local_status` | string | 本地工单状态 |
| `source_updated` | bool | 是否已回写到源系统 |
| `writeback_log_id` | uuid | 本次回写日志 ID（如有） |

---

## 5. 获取工单回写日志

**GET** `/api/v1/incidents/:id/writeback-logs`

**权限**: `plugin:list`

返回当前工单的源系统回写记录，包括手动关单回写和流程自动关单回写。

| 字段 | 类型 | 说明 |
|------|------|------|
| `action` | string | 回写动作，如 `close` |
| `trigger_source` | string | 触发来源，如 `manual_close` / `flow_auto_close` |
| `status` | string | `pending` / `success` / `failed` / `skipped` |
| `request_method` | string | 请求方法 |
| `request_url` | string | 实际调用地址 |
| `request_payload` | object | 请求体 |
| `response_status_code` | int | 响应状态码 |
| `response_body` | string | 响应正文 |
| `error_message` | string | 错误信息 |

---

## 6. 重置工单扫描状态

**POST** `/api/v1/incidents/:id/reset-scan`

**权限**: `plugin:sync`

重置工单的规则匹配扫描状态，使其可以被重新扫描匹配。

---

## 7. 批量重置工单扫描状态

**POST** `/api/v1/incidents/batch-reset-scan`

**权限**: `plugin:sync`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uuid | ✅ | 工单 ID 列表 |
| `healing_status` | string | ❌ | 按自愈状态过滤，仅重置指定状态的工单：`pending` / `processing` / `healed` / `failed` / `skipped` / `dismissed` |

---

## 8. 手动触发工单自愈

**POST** `/api/v1/incidents/:id/trigger`

**权限**: `healing:trigger:execute`

对指定工单手动触发自愈流程（需要先匹配到规则）。

---

## 9. 忽略待触发工单

**POST** `/api/v1/incidents/:id/dismiss`

**权限**: `healing:trigger:execute`

将待触发工单标记为 `dismissed`，使其从待触发列表移除并进入“已忽略”列表。

---

## 工单状态说明

| 状态 | 说明 |
|------|------|
| `open` | 未解决 |
| `resolved` | 已解决 |
| `closed` | 已关闭 |

## 自愈状态说明

| 状态 | 说明 |
|------|------|
| `pending` | 待扫描或待人工触发 |
| `processing` | 已触发自愈流程，正在处理中 |
| `healed` | 自愈完成或工单关闭回写成功 |
| `failed` | 自愈执行失败 |
| `skipped` | 无匹配规则，被调度器跳过 |
| `dismissed` | 人工忽略或取消后不再触发 |
