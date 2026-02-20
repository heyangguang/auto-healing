# 自愈管理 API 文档

**权限**: 已登录用户（租户内数据隔离）

---

## 自愈流程（Healing Flows）

**路径前缀**: `/api/v1/healing/flows`

### 1. 获取节点类型 Schema

**GET** `/api/v1/healing/flows/node-schema`

**权限**: `healing:flows:view`

返回所有节点类型的配置定义，用于前端流程设计器。包含节点输入/输出端口、配置项、变量定义等。

---

### 2. 获取流程列表

**GET** `/api/v1/healing/flows`

**权限**: `healing:flows:view`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称、描述） |
| `name` | string | ❌ | 按名称精确/模糊筛选 |
| `description` | string | ❌ | 按描述关键词筛选 |
| `node_type` | string | ❌ | 按节点类型筛选（如 `approval`、`execution`） |
| `is_active` | bool | ❌ | 是否激活：`true` / `false` |
| `min_nodes` | int | ❌ | 最少节点数 |
| `max_nodes` | int | ❌ | 最多节点数 |
| `created_from` | string | ❌ | 创建时间起始（RFC3339 或 `YYYY-MM-DD`） |
| `created_to` | string | ❌ | 创建时间结束 |
| `updated_from` | string | ❌ | 更新时间起始 |
| `updated_to` | string | ❌ | 更新时间结束 |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` / `updated_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

### 3. 创建流程

**POST** `/api/v1/healing/flows`

**权限**: `healing:flows:create`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 流程名称 |
| `description` | string | ❌ | 描述 |
| `nodes` | array | ❌ | 节点定义（JSON） |
| `edges` | array | ❌ | 边定义（JSON） |
| `is_active` | bool | ❌ | 是否激活，默认 true |

---

### 4. 获取流程统计

**GET** `/api/v1/healing/flows/stats`

**权限**: `healing:flows:view`

---

### 5. 获取流程详情

**GET** `/api/v1/healing/flows/:id`

**权限**: `healing:flows:view`

---

### 6. 更新流程

**PUT** `/api/v1/healing/flows/:id`

**权限**: `healing:flows:update`

---

### 7. 删除流程

**DELETE** `/api/v1/healing/flows/:id`

**权限**: `healing:flows:delete`

---

### 8. 试运行（Dry Run）

**POST** `/api/v1/healing/flows/:id/dry-run`

**权限**: `healing:flows:update`

使用模拟工单数据对流程进行试运行，验证流程逻辑是否正确，不会产生实际执行。

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `mock_incident` | object | ✅ | 模拟工单数据 |
| `from_node_id` | string | ❌ | 从指定节点开始执行（用于重试） |
| `context` | object | ❌ | 初始上下文（用于重试） |
| `mock_approvals` | object | ❌ | 模拟审批结果，key为 node_id，value为 `approved`或`rejected` |

**`mock_incident` 子字段**

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | ✅ | 工单标题 |
| `description` | string | ❌ | 工单描述 |
| `severity` | string | ❌ | 严重级别 |
| `priority` | string | ❌ | 优先级 |
| `status` | string | ❌ | 状态 |
| `category` | string | ❌ | 分类 |
| `affected_ci` | string | ❌ | 影响的 CI |
| `affected_service` | string | ❌ | 影响的服务 |
| `assignee` | string | ❌ | 处理人 |
| `reporter` | string | ❌ | 报告人 |
| `raw_data` | object | ❌ | 原始数据 |

---

### 9. 试运行 SSE 流（实时输出）

**POST** `/api/v1/healing/flows/:id/dry-run-stream`

**权限**: `healing:flows:update`

与 dry-run 相同，但通过 SSE 实时推送试运行过程中的节点执行状态。

---

## 自愈规则（Healing Rules）

**路径前缀**: `/api/v1/healing/rules`

### 10. 获取规则列表

**GET** `/api/v1/healing/rules`

**权限**: `healing:rules:view`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索 |
| `flow_id` | uuid | ❌ | 按流程筛选 |
| `is_active` | bool | ❌ | 是否激活 |
| `trigger_mode` | string | ❌ | 触发模式：`auto` / `manual` |
| `match_mode` | string | ❌ | 匹配模式：`all` / `any` |
| `priority` | int | ❌ | 按优先级筛选 |
| `has_flow` | bool | ❌ | 是否已关联流程：`true` / `false` |
| `created_from` | string | ❌ | 创建时间起始（RFC3339 或 `YYYY-MM-DD`） |
| `created_to` | string | ❌ | 创建时间结束 |
| `sort_by` | string | ❌ | 排序字段 |
| `sort_order` | string | ❌ | 排序方向 |

---

### 11. 创建规则

**POST** `/api/v1/healing/rules`

**权限**: `healing:rules:create`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 规则名称 |
| `description` | string | ❌ | 描述 |
| `flow_id` | uuid | ❌ | 关联的自愈流程 ID |
| `plugin_id` | uuid | ❌ | 关联的插件 ID（限定工单来源） |
| `conditions` | []object | ❌ | 匹配条件列表 |
| `match_mode` | string | ❌ | 匹配模式：`all`（全匹配）/ `any`（任意匹配），默认 `all` |
| `trigger_mode` | string | ❌ | 触发模式：`auto`（自动）/ `manual`（手动），默认 `auto` |
| `is_active` | bool | ❌ | 是否激活，默认 false |
| `priority` | int | ❌ | 优先级（数字越小优先级越高），默认 0 |

#### 条件对象字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `field` | string | 匹配字段：`severity` / `title` / `category` / `affected_ci` / `affected_service` 等 |
| `operator` | string | 操作符：`eq` / `ne` / `contains` / `not_contains` / `starts_with` / `ends_with` / `regex` |
| `value` | string | 匹配值 |

---

### 12. 获取规则统计

**GET** `/api/v1/healing/rules/stats`

**权限**: `healing:rules:view`

---

### 13. 获取规则详情

**GET** `/api/v1/healing/rules/:id`

**权限**: `healing:rules:view`

---

### 14. 更新规则

**PUT** `/api/v1/healing/rules/:id`

**权限**: `healing:rules:update`

---

### 15. 删除规则

**DELETE** `/api/v1/healing/rules/:id`

**权限**: `healing:rules:delete`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `force` | bool | ❌ | `true` 时强制删除（自动解除关联的流程实例），默认 false |

---

### 16. 激活规则

**POST** `/api/v1/healing/rules/:id/activate`

**权限**: `healing:rules:update`

---

### 17. 停用规则

**POST** `/api/v1/healing/rules/:id/deactivate`

**权限**: `healing:rules:update`

---

## 自愈实例（Flow Instances）

**路径前缀**: `/api/v1/healing/instances`

### 18. 获取实例列表

**GET** `/api/v1/healing/instances`

**权限**: `healing:instances:view`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `flow_id` | uuid | ❌ | 按流程筛选 |
| `rule_id` | uuid | ❌ | 按规则筛选 |
| `incident_id` | uuid | ❌ | 按工单筛选 |
| `status` | string | ❌ | 状态：`running` / `success` / `failed` / `cancelled` / `pending_approval` |
| `search` | string | ❌ | 模糊搜索 |
| `flow_name` | string | ❌ | 按流程名称筛选 |
| `rule_name` | string | ❌ | 按规则名称筛选 |
| `incident_title` | string | ❌ | 按工单标题筛选 |
| `current_node_id` | string | ❌ | 当前节点 ID |
| `error_message` | string | ❌ | 错误信息关键词 |
| `has_error` | bool | ❌ | 是否有错误：`true` / `false` |
| `created_from` | string | ❌ | 创建时间起始（RFC3339 或 `YYYY-MM-DD`） |
| `created_to` | string | ❌ | 创建时间结束 |
| `started_from` | string | ❌ | 开始时间起始 |
| `started_to` | string | ❌ | 开始时间结束 |
| `completed_from` | string | ❌ | 完成时间起始 |
| `completed_to` | string | ❌ | 完成时间结束 |
| `min_nodes` | int | ❌ | 最少节点数 |
| `max_nodes` | int | ❌ | 最多节点数 |
| `min_failed_nodes` | int | ❌ | 最少失败节点数 |
| `max_failed_nodes` | int | ❌ | 最多失败节点数 |
| `sort_by` | string | ❌ | 排序字段：`created_at` / `updated_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

### 19. 获取实例统计

**GET** `/api/v1/healing/instances/stats`

**权限**: `healing:instances:view`

#### 响应

```json
{
  "code": 0,
  "data": {
    "total": 500,
    "running": 5,
    "success": 450,
    "failed": 30,
    "cancelled": 10,
    "pending_approval": 5,
    "success_rate": 0.9375
  }
}
```

---

### 20. 获取实例详情

**GET** `/api/v1/healing/instances/:id`

**权限**: `healing:instances:view`

---

### 21. 取消实例

**POST** `/api/v1/healing/instances/:id/cancel`

**权限**: `healing:instances:view`

---

### 22. 重试实例

**POST** `/api/v1/healing/instances/:id/retry`

**权限**: `healing:instances:view`

#### 请求体（可选）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `from_node_id` | string | ❌ | 从指定节点 ID 开始重试，不传则从头重试 |

---

### 23. 实例事件 SSE 流

**GET** `/api/v1/healing/instances/:id/events`

**权限**: `healing:instances:view`

通过 SSE 实时推送实例执行过程中的节点状态变化事件。

---

## 审批任务（Approvals）

**路径前缀**: `/api/v1/healing/approvals`

### 24. 获取审批任务列表

**GET** `/api/v1/healing/approvals`

**权限**: `healing:approvals:view`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `flow_instance_id` | uuid | ❌ | 按流程实例筛选 |
| `status` | string | ❌ | 状态：`pending` / `approved` / `rejected` / `timeout` |

---

### 25. 获取待审批列表

**GET** `/api/v1/healing/approvals/pending`

**权限**: `healing:approvals:view`

返回当前用户待处理的审批任务。

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（node_id、flow_instance_id） |
| `date_from` | string | ❌ | 创建时间起始（`YYYY-MM-DD`） |
| `date_to` | string | ❌ | 创建时间结束 |

---

### 26. 获取审批任务详情

**GET** `/api/v1/healing/approvals/:id`

**权限**: `healing:approvals:view`

---

### 27. 审批通过

**POST** `/api/v1/healing/approvals/:id/approve`

**权限**: `healing:approvals:approve`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `comment` | string | ❌ | 审批意见 |

---

### 28. 审批拒绝

**POST** `/api/v1/healing/approvals/:id/reject`

**权限**: `healing:approvals:approve`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `comment` | string | ❌ | 拒绝原因 |

---

## 待触发工单（Pending Center）

**路径前缀**: `/api/v1/healing/pending`

### 29. 获取待触发工单列表

**GET** `/api/v1/healing/pending/trigger`

**权限**: `healing:trigger:view`

返回已匹配规则但等待人工确认触发的工单列表。

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（标题、external_id、affected_ci） |
| `severity` | string | ❌ | 严重级别：`critical` / `high` / `medium` / `low` |
| `date_from` | string | ❌ | 创建时间起始（`YYYY-MM-DD`） |
| `date_to` | string | ❌ | 创建时间结束 |

---

## 工单手动触发

### 30. 手动触发工单自愈

**POST** `/api/v1/incidents/:id/trigger`

**权限**: `healing:trigger:execute`

对指定工单手动触发自愈流程。

---

## 实例状态说明

| 状态 | 说明 |
|------|------|
| `running` | 执行中 |
| `success` | 执行成功 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |
| `pending_approval` | 等待审批 |
| `pending_trigger` | 等待触发（手动触发模式） |
