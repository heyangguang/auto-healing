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
  "data": {
    "total": 1500,
    "open": 20,
    "resolved": 1480,
    "by_severity": {"critical": 5, "high": 15, "medium": 100, "low": 1380},
    "by_plugin": [
      {"plugin_id": "uuid", "plugin_name": "Zabbix", "count": 1000}
    ]
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
| `healing_status` | string | ❌ | 自愈状态：`pending` / `triggered` / `success` / `failed` / `skipped` |
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
| `close_status` | string | ❌ | 关闭后状态，默认 `closed` |

---

## 5. 重置工单扫描状态

**POST** `/api/v1/incidents/:id/reset-scan`

**权限**: `plugin:sync`

重置工单的规则匹配扫描状态，使其可以被重新扫描匹配。

---

## 6. 批量重置工单扫描状态

**POST** `/api/v1/incidents/batch-reset-scan`

**权限**: `plugin:sync`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uuid | ✅ | 工单 ID 列表 |
| `healing_status` | string | ❌ | 按自愈状态过滤，仅重置指定状态的工单 |

---

## 7. 手动触发工单自愈

**POST** `/api/v1/incidents/:id/trigger`

**权限**: `healing:trigger:execute`

对指定工单手动触发自愈流程（需要先匹配到规则）。

---

## 工单状态说明

| 状态 | 说明 |
|------|------|
| `open` | 未解决 |
| `resolved` | 已解决 |
| `closed` | 已关闭 |
