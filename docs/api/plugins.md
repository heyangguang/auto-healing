# 插件管理 API 文档

**Base URL**: `/api/v1/plugins`  
**权限**: 已登录用户（租户内数据隔离）

---

## 1. 获取插件列表

**GET** `/api/v1/plugins`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称、描述） |
| `type` | string | ❌ | 插件类型：`itsm` / `cmdb` |
| `status` | string | ❌ | 状态：`active` / `inactive` / `error` |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "uuid",
      "name": "Zabbix 监控",
      "description": "Zabbix 监控系统集成插件",
      "type": "itsm",
      "status": "active",
      "sync_enabled": true,
      "config": {
        "url": "http://zabbix.example.com",
        "username": "admin"
      },
      "last_sync_at": "2026-02-18T09:00:00Z",
      "next_sync_at": "2026-02-18T09:05:00Z",
      "sync_interval_minutes": 5,
      "max_failures": 5,
      "consecutive_failures": 0,
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

## 2. 创建插件

**POST** `/api/v1/plugins`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 插件名称 |
| `description` | string | ❌ | 描述 |
| `type` | string | ✅ | 插件类型 |
| `version` | string | ❌ | 版本号，默认 `1.0.0` |
| `config` | object | ✅ | 插件配置（根据类型不同而不同） |
| `field_mapping` | object | ❌ | 字段映射配置（JSON 对象） |
| `sync_filter` | object | ❌ | 同步过滤配置（JSON 对象） |
| `sync_enabled` | bool | ❌ | 是否启用自动同步，默认 false |
| `sync_interval_minutes` | int | ❌ | 同步间隔（分钟） |
| `max_failures` | int | ❌ | 最大连续失败次数，默认 5 |

```json
{
  "name": "Zabbix 监控",
  "description": "生产环境 Zabbix 监控",
  "type": "itsm",
  "config": {
    "url": "http://zabbix.example.com",
    "auth_type": "bearer",
    "token": "xxxxx"
  },
  "sync_enabled": true,
  "sync_interval_minutes": 5
}
```

适配器模式示例：

```json
{
  "name": "iTop 工单适配器",
  "type": "itsm",
  "config": {
    "url": "http://127.0.0.1:18085/api/incidents",
    "auth_type": "none",
    "close_incident_url": "http://127.0.0.1:18085/api/incidents/{external_id}/close",
    "close_incident_method": "POST"
  },
 "field_mapping": {},
  "sync_enabled": false,
  "sync_interval_minutes": 5
}
```

说明：

- `close_incident_url` 用于关闭源工单的回写接口
- 当用户在工单页面手动关闭工单，或自愈流程启用了“成功后自动关单”时，系统会调用该地址
- URL 支持变量：`{external_id}`

工单关闭协议：

- `close_status` 由 AHS 定义为“关闭后的生命周期状态”，如 `resolved` / `closed`
- `close_code` 由 AHS 定义为“关闭原因码”，如 `auto_healed` / `manual_fixed` / `not_reproducible`
- `resolution` 表达最终结论，`work_notes` 表达处理动作、执行结果与验证结果
- 适配器必须负责把这些字段映射到对端系统；如果对端不支持某字段，必须在适配器文档中显式说明

```json
{
  "name": "iTop 资产适配器",
  "type": "cmdb",
  "config": {
    "url": "http://127.0.0.1:18085/api/cmdb-items",
    "auth_type": "none"
  },
  "field_mapping": {},
  "sync_enabled": false,
  "sync_interval_minutes": 5
}
```

---

## 3. 获取插件详情

**GET** `/api/v1/plugins/:id`

### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 插件 ID |

---

## 3A. 解决方案模板

用于管理工单关闭/回写时使用的“解决方案模板”。模板可以被 Flow 的 `close_policy.solution_template_id` 引用，也可以在工单手动关闭时指定。

新版模板采用“静态方案 + 动态步骤”模型：

- `problem_template`：问题说明
- `solution_template`：标准解决方案说明
- `verification_template`：验证结果说明
- `conclusion_template`：最终结论
- `steps_render_mode`：执行步骤渲染模式，由系统根据真实运行过程自动生成

兼容说明：

- `resolution_template` / `work_notes_template` 仍保留，用于兼容旧模板
- 若配置了新版分段字段，系统会优先使用新版字段渲染

### 获取模板列表

**GET** `/api/v1/incident-solution-templates`

**权限**: `plugin:list`

### 创建模板

**POST** `/api/v1/incident-solution-templates`

**权限**: `plugin:create`

请求体字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 模板名称 |
| `description` | string | ❌ | 模板描述 |
| `problem_template` | string | ❌ | 问题说明模板 |
| `solution_template` | string | ❌ | 解决方案模板；描述标准处理方案 |
| `verification_template` | string | ❌ | 验证结果模板 |
| `conclusion_template` | string | ❌ | 最终结论模板 |
| `steps_render_mode` | string | ❌ | 步骤渲染模式：`summary` / `detailed` |
| `steps_max_count` | int | ❌ | 最多展示的步骤数量 |
| `step_output_max_length` | int | ❌ | 单步输出摘要最大长度 |
| `resolution_template` | string | ❌ | 兼容旧模型的结论模板 |
| `work_notes_template` | string | ❌ | 兼容旧模型的过程模板 |
| `default_close_code` | string | ❌ | 默认关闭原因码 |
| `default_close_status` | string | ❌ | 默认关闭状态，默认 `resolved` |

模板变量说明：

- 系统自动提供 `incident.*`、`system.*`、`operator.*`
- 自动关单场景通常还会提供 `flow.*`、`execution.*`、`steps`、`steps_text`
- 手动关单场景可通过 `template_vars` 补充变量
- 模板变量写法为 `{{ incident.title }}`、`{{ execution.run_id }}` 这类点路径

回写渲染规则：

- `resolution` 用于承载最终结论
- `work_notes` 用于承载“问题说明 / 解决方案 / 执行步骤 / 验证结果”等完整结构
- 适配器需要负责把完整结构映射到对端系统的可见字段；若对端仅支持单个“解决方案”字段，应优先保证完整结构可见

## 4. 更新插件

**PUT** `/api/v1/plugins/:id`

### 请求体（所有字段可选）

| 字段 | 类型 | 说明 |
|------|------|------|
| `description` | string | 描述 |
| `version` | string | 版本号 |
| `config` | object | 插件配置 |
| `field_mapping` | object | 字段映射配置 |
| `sync_filter` | object | 同步过滤配置 |
| `sync_enabled` | bool | 是否启用自动同步 |
| `sync_interval_minutes` | int | 同步间隔（分钟） |
| `max_failures` | int | 最大连续失败次数 |

---

## 5. 删除插件

**DELETE** `/api/v1/plugins/:id`

---

## 6. 测试插件连接

**POST** `/api/v1/plugins/:id/test`

**权限**: `plugin:test`

仅测试连接，不改变插件状态。

### 响应

```json
{
  "code": 0,
  "message": "连接测试成功"
}
```

---

## 7. 激活插件

**POST** `/api/v1/plugins/:id/activate`

**权限**: `plugin:update`

测试连接成功后，将插件状态设为 `active`。

### 响应

```json
{
  "code": 0,
  "message": "插件已激活"
}
```

---

## 8. 停用插件

**POST** `/api/v1/plugins/:id/deactivate`

**权限**: `plugin:update`

---

## 9. 手动触发同步

**POST** `/api/v1/plugins/:id/sync`

触发插件立即同步数据（CMDB 配置项、工单等）。

### 响应

```json
{
  "code": 0,
  "message": "同步已触发"
}
```

---

## 10. 获取插件同步日志

**GET** `/api/v1/plugins/:id/logs`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "uuid",
      "plugin_id": "uuid",
      "status": "success",
      "sync_type": "manual",
      "records_fetched": 20,
      "records_processed": 15,
      "records_failed": 0,
      "details": {
        "new_count": 5,
        "updated_count": 10
      },
      "started_at": "2026-02-18T09:00:00Z",
      "completed_at": "2026-02-18T09:00:05Z",
      "error_message": ""
    }
  ],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

---

## 11. 获取插件统计

**GET** `/api/v1/plugins/stats`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 5,
    "active": 4,
    "inactive": 1,
    "error": 0,
    "by_type": {
      "itsm": 4,
      "cmdb": 1
    }
  }
}
```

---

## 工单（Incident）管理

插件会自动从外部系统同步工单数据。工单接口为**独立路由组**，路径前缀为 `/api/v1/incidents`，详见 [incidents.md](./incidents.md)。

### 12. 获取工单列表

**GET** `/api/v1/incidents`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `plugin_id` | uuid | ❌ | 按插件筛选 |
| `status` | string | ❌ | 工单状态：`open` / `resolved` / `closed` 等 |
| `severity` | string | ❌ | 严重程度：`critical` / `high` / `medium` / `low` |
| `healing_status` | string | ❌ | 自愈状态：`pending` / `processing` / `healed` / `failed` / `skipped` / `dismissed` |
| `search` | string | ❌ | 模糊搜索（标题、外部 ID、受影响 CI） |

#### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "uuid",
      "plugin_id": "uuid",
      "external_id": "INC-12345",
      "title": "服务器 CPU 使用率过高",
      "description": "生产服务器 CPU 持续超过 90%",
      "severity": "high",
      "priority": "P1",
      "status": "open",
      "category": "performance",
      "affected_ci": "prod-server-01",
      "affected_service": "payment-service",
      "assignee": "zhangsan",
      "reporter": "zabbix-system",
      "healing_status": "processing",
      "matched_rule_id": "uuid",
      "healing_flow_instance_id": "uuid",
      "raw_data": {},
      "created_at": "2026-02-18T08:00:00Z",
      "updated_at": "2026-02-18T10:00:00Z"
    }
  ],
  "total": 50,
  "page": 1,
  "page_size": 20
}
```

### 13. 获取工单详情

**GET** `/api/v1/incidents/:id`

### 工单字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid | 工单 ID |
| `plugin_id` | uuid | 来源插件 ID |
| `external_id` | string | 外部系统工单 ID |
| `title` | string | 标题 |
| `description` | string | 描述 |
| `severity` | string | 严重程度：`critical` / `high` / `medium` / `low` |
| `priority` | string | 优先级：`P1` / `P2` / `P3` / `P4` |
| `status` | string | 工单状态 |
| `category` | string | 分类 |
| `affected_ci` | string | 受影响的配置项 |
| `affected_service` | string | 受影响的服务 |
| `assignee` | string | 负责人 |
| `reporter` | string | 上报人 |
| `healing_status` | string | 自愈状态：`pending` / `processing` / `healed` / `failed` / `skipped` / `dismissed` |
| `matched_rule_id` | uuid | 匹配的自愈规则 ID |
| `healing_flow_instance_id` | uuid | 触发的自愈流程实例 ID |
| `raw_data` | object | 原始数据（来自外部系统） |
