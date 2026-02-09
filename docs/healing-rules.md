# 自愈规则 API 文档

自愈规则定义了触发自愈流程的条件，当事件匹配规则时，自动或手动执行关联的自愈流程。

## 数据模型

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string (uuid) | - | 规则 ID (自动生成) |
| name | string | ✅ | 规则名称 |
| description | string | - | 规则描述 |
| priority | number | - | 优先级 (数字越大优先级越高，默认 0) |
| trigger_mode | string | - | 触发模式: `auto` (自动) / `manual` (手动，默认 auto) |
| conditions | array | - | 匹配条件列表 (JSONB) |
| match_mode | string | - | 匹配模式: `all` (全部满足) / `any` (任一满足，默认 all) |
| flow_id | string (uuid) | ⚡ | 关联的自愈流程 ID（激活规则时必填） |
| is_active | boolean | - | 是否启用 (默认 false) |
| last_run_at | string | - | 最后执行时间 |
| created_by | string (uuid) | - | 创建人 ID |
| created_at | string | - | 创建时间 |
| updated_at | string | - | 更新时间 |
| flow | object | - | 关联的流程对象 (列表/详情时返回) |

### 匹配条件 (conditions)

每个条件包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| field | string | 匹配字段 (见下方字段列表) |
| operator | string | 操作符 |
| value | any | 匹配值 |

#### 可匹配字段

**固定字段（来自 Incident 模型）**

| 字段名 | 说明 |
|--------|------|
| `title` | 工单标题 |
| `description` | 工单描述 |
| `severity` | 严重程度 |
| `priority` | 优先级 |
| `status` | 状态 |
| `category` | 类别 |
| `affected_ci` | 受影响的配置项 |
| `affected_service` | 受影响的服务 |
| `assignee` | 处理人 |
| `reporter` | 上报人 |
| `source_plugin_name` | 来源插件名称 |

**动态字段（来自 raw_data）**

除上述固定字段外，还可使用工单 `raw_data` 中的任意自定义字段。直接填写字段名即可，系统会自动从 `raw_data` 中查找。

#### 操作符类型


| 操作符 | 说明 | 示例 |
|--------|------|------|
| `equals` | 等于 | `{"field": "severity", "operator": "equals", "value": "1"}` |
| `contains` | 包含 | `{"field": "title", "operator": "contains", "value": "disk"}` |
| `in` | 在列表中 | `{"field": "category", "operator": "in", "value": ["network", "storage"]}` |
| `regex` | 正则匹配 | `{"field": "title", "operator": "regex", "value": "^ALERT-.*"}` |
| `gt` | 大于 | `{"field": "priority", "operator": "gt", "value": 2}` |
| `lt` | 小于 | `{"field": "priority", "operator": "lt", "value": 3}` |
| `gte` | 大于等于 | `{"field": "severity", "operator": "gte", "value": 2}` |
| `lte` | 小于等于 | `{"field": "severity", "operator": "lte", "value": 3}` |

#### 条件示例
```json
[
  {"field": "title", "operator": "contains", "value": "E2E-HEALING"},
  {"field": "severity", "operator": "lte", "value": 3},
  {"field": "category", "operator": "in", "value": ["software", "network"]}
]
```

### 匹配模式

| 模式 | 说明 |
|------|------|
| `all` | 所有条件都必须满足 (AND 逻辑) |
| `any` | 任一条件满足即可 (OR 逻辑) |

### 触发模式

| 模式 | 说明 |
|------|------|
| `auto` | 规则匹配后自动触发流程执行 |
| `manual` | 规则匹配后需要人工确认触发 |

---

## 嵌套条件逻辑（高级功能）

### 概念说明

支持使用条件组（Condition Group）来构建复杂的嵌套逻辑，实现 `(A && B) || C` 这样的组合。

**条件类型：**
- **单个条件** (`type: "condition"`): 包含 `field`, `operator`, `value`
- **条件组** (`type: "group"`): 包含 `logic` (AND/OR) 和嵌套的 `conditions`

### 嵌套条件示例

#### 示例1: (标题包含"磁盘" AND 严重度>=2) OR 类别="storage"

```json
{
  "conditions": [
    {
      "type": "group",
      "logic": "AND",
      "conditions": [
        {"type": "condition", "field": "title", "operator": "contains", "value": "磁盘"},
        {"type": "condition", "field": "severity", "operator": "gte", "value": 2}
      ]
    },
    {
      "type": "condition",
      "field": "category",
      "operator": "equals",
      "value": "storage"
    }
  ],
  "match_mode": "any"
}
```

**逻辑说明：**
- 外层 `match_mode: "any"` → 条件组之间是 OR 关系
- 内层 `logic: "AND"` → 条件组内部是 AND 关系
- 相当于: `(title 包含 "磁盘" AND severity >= 2) OR category = "storage"`

#### 示例2: 深度嵌套 ((A && B) || C) && D

```json
{
  "conditions": [
    {
      "type": "group",
      "logic": "OR",
      "conditions": [
        {
          "type": "group",
          "logic": "AND",
          "conditions": [
            {"type": "condition", "field": "title", "operator": "contains", "value": "网络"},
            {"type": "condition", "field": "priority", "operator": "equals", "value": "1"}
          ]
        },
        {"type": "condition", "field": "category", "operator": "equals", "value": "critical"}
      ]
    },
    {
      "type": "condition",
      "field": "assignee",
      "operator": "equals",
      "value": "运维团队"
    }
  ],
  "match_mode": "all"
}
```

**逻辑说明：**
- 相当于: `((title 包含 "网络" AND priority = 1) OR category = "critical") AND assignee = "运维团队"`

### 向后兼容

旧格式（扁平条件列表）仍然支持：
```json
{
  "conditions": [
    {"field": "title", "operator": "contains", "value": "E2E-HEALING"}
  ],
  "match_mode": "all"
}
```

> [!TIP]
> 如果条件的 `type` 字段为空，系统会自动识别为 `type: "condition"`（单个条件）。


---

## API 接口

### 1. 获取规则列表

**GET** `/api/v1/healing/rules`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| is_active | boolean | 筛选启用状态 |
| flow_id | string (uuid) | 筛选关联流程 |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "04bb30f6-85e4-4ac6-9a9a-fe72df03817f",
      "name": "E2E规则-服务重启",
      "description": "匹配标题包含 E2E-HEALING 的事件",
      "priority": 100,
      "trigger_mode": "auto",
      "conditions": [
        {"field": "title", "operator": "contains", "value": "E2E-HEALING"}
      ],
      "match_mode": "all",
      "flow_id": "db050a65-2c93-4abf-b3f1-281b7890e446",
      "is_active": true,
      "last_run_at": "2026-01-09T15:38:14.198662+08:00",
      "created_at": "2026-01-06T07:34:31.456362+08:00",
      "updated_at": "2026-01-09T15:38:14.198897+08:00",
      "flow": {
        "id": "db050a65-2c93-4abf-b3f1-281b7890e446",
        "name": "E2E流程-服务重启",
        "description": "端到端测试流程",
        "is_active": true,
        "nodes": [...],
        "edges": [...]
      }
    }
  ],
  "total": 8,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 创建规则

**POST** `/api/v1/healing/rules`

**请求体**
```json
{
  "name": "磁盘告警自愈规则",
  "description": "自动处理磁盘空间告警",
  "priority": 50,
  "trigger_mode": "auto",
  "conditions": [
    {"field": "title", "operator": "contains", "value": "磁盘"},
    {"field": "category", "operator": "equals", "value": "storage"}
  ],
  "match_mode": "all",
  "flow_id": "db050a65-2c93-4abf-b3f1-281b7890e446",
  "is_active": false
}
```

**响应示例** (HTTP 201)
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174001",
    "name": "磁盘告警自愈规则",
    "priority": 50,
    "trigger_mode": "auto",
    "conditions": [...],
    "match_mode": "all",
    "flow_id": "db050a65-2c93-4abf-b3f1-281b7890e446",
    "is_active": false,
    "created_at": "2026-01-10T10:00:00+08:00",
    "updated_at": "2026-01-10T10:00:00+08:00"
  }
}
```

> [!NOTE]
> 新创建的规则默认 `is_active = false`，需要手动启用。

---

### 3. 获取规则详情

**GET** `/api/v1/healing/rules/:id`

**响应**: 同列表中的单个规则对象

---

### 4. 更新规则

**PUT** `/api/v1/healing/rules/:id`

**请求体** (所有字段可选)
```json
{
  "name": "更新后的规则名称",
  "description": "更新后的描述",
  "priority": 100,
  "trigger_mode": "manual",
  "conditions": [...],
  "match_mode": "any",
  "flow_id": "new-flow-id",
  "is_active": true
}
```

**响应**: 返回更新后的规则对象

---

### 5. 删除规则

**DELETE** `/api/v1/healing/rules/:id`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| force | boolean | 强制删除 (自动解除关联的流程实例) |

**成功响应**
```json
{
  "code": 0,
  "message": "删除成功"
}
```

**失败响应（有关联的执行记录）**
```json
{
  "code": 40900,
  "message": "规则存在关联的执行记录，请使用 force=true 强制删除"
}
```

---

### 6. 启用规则

**POST** `/api/v1/healing/rules/:id/activate`

**成功响应**
```json
{
  "code": 0,
  "message": "规则已启用"
}
```

---

### 7. 停用规则

**POST** `/api/v1/healing/rules/:id/deactivate`

**成功响应**
```json
{
  "code": 0,
  "message": "规则已停用"
}
```

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /healing/rules | healing:rules:view |
| POST /healing/rules | healing:rules:create |
| GET /healing/rules/:id | healing:rules:view |
| PUT /healing/rules/:id | healing:rules:update |
| DELETE /healing/rules/:id | healing:rules:delete |
| POST /healing/rules/:id/activate | healing:rules:update |
| POST /healing/rules/:id/deactivate | healing:rules:update |
