# 工单管理 API 文档

## 概述

工单（Incident）是从 ITSM 插件同步进来的事件/故障记录。工单管理模块提供查询和扫描状态管理功能。

**工单来源：** 工单不是手动创建的，而是通过 ITSM 类型的插件定时同步进来。

**工单关闭：** 工单关闭由自愈流程内部执行，不暴露 API 接口。

---

## API 接口

| 方法 | 路径 | 说明 |
|-----|------|------|
| GET | `/api/v1/incidents` | 获取工单列表 |
| GET | `/api/v1/incidents/{id}` | 获取工单详情 |
| POST | `/api/v1/incidents/{id}/reset-scan` | 重置单个工单扫描状态 |
| POST | `/api/v1/incidents/batch-reset-scan` | 批量重置工单扫描状态 |

---

## 工单数据模型

| 字段 | 类型 | 说明 |
|-----|------|------|
| `id` | UUID | 主键 |
| `plugin_id` | UUID | 关联插件 ID（可为 null） |
| `source_plugin_name` | string | 来源插件名 |
| `external_id` | string | 外部系统工单 ID |
| `title` | string | 标题 |
| `description` | string | 描述 |
| `severity` | string | 严重程度（来自外部系统原始值，如 ServiceNow 的 `"1"`, `"2"`, `"3"`, `"4"`） |
| `priority` | string | 优先级 |
| `status` | string | 工单状态：`open`/`in_progress`/`resolved`/`closed` |
| `category` | string | 分类 |
| `affected_ci` | string | 受影响配置项 |
| `affected_service` | string | 受影响服务 |
| `assignee` | string | 处理人 |
| `reporter` | string | 报告人 |
| `healing_status` | string | 自愈状态（见下表） |
| `scanned` | bool | 是否已被自愈引擎扫描 |
| `matched_rule_id` | UUID | 匹配的自愈规则 ID |
| `healing_flow_instance_id` | UUID | 关联的自愈流程实例 ID |
| `raw_data` | JSON | 原始数据 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

### 自愈状态 (healing_status)

| 值 | 说明 | 触发时机 |
|----|------|---------|
| `pending` | 待处理 | 工单同步创建时（默认状态） |
| `processing` | 自愈中 | 创建流程实例开始执行时 |
| `healed` | 已自愈 | 自愈流程执行成功完成时 |
| `failed` | 自愈失败 | 自愈流程执行失败时 |
| `skipped` | 已跳过 | 工单扫描后无匹配规则时 |

**状态流转图：**

```
工单同步创建 → pending
        ↓
   调度器扫描
        ↓
   匹配规则?
   ├── 否 → skipped
   └── 是 → processing（创建流程实例）
              ↓
         等待审批?
         ├── 通过 → 继续执行
         ├── 拒绝 → failed
         ├── 超时 → failed
         └── 取消 → skipped
              ↓
         流程执行完成?
         ├── 成功 → healed
         └── 失败 → failed
```

---

## 接口详情

### 1. 获取工单统计

```http
GET /api/v1/incidents/stats
```

**响应示例：**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 100,
    "scanned": 80,
    "unscanned": 20,
    "matched": 50,
    "pending": 30,
    "processing": 5,
    "healed": 40,
    "failed": 5,
    "skipped": 10
  }
}
```

| 字段 | 说明 |
|------|------|
| `total` | 总工单数 |
| `scanned` | 已扫描数 |
| `unscanned` | 待扫描数 |
| `matched` | 已匹配规则数 |
| `pending` | 待处理数 |
| `processing` | 自愈中数 |
| `healed` | 已自愈数 |
| `failed` | 自愈失败数 |
| `skipped` | 已跳过数 |

---

### 2. 获取工单列表

```http
GET /api/v1/incidents
```

**查询参数：**

| 参数 | 类型 | 说明 |
|-----|------|------|
| `page` | int | 页码，默认 1 |
| `page_size` | int | 每页数量，默认 20 |
| `plugin_id` | UUID | 按插件 ID 筛选 |
| `status` | string | 按自愈状态筛选 |
| `severity` | string | 按严重程度筛选 |
| `source_plugin_name` | string | 按来源插件名筛选（支持模糊匹配） |
| `has_plugin` | bool | `true`=有关联插件，`false`=无关联（插件已删除） |

**响应示例：**
```json
{
  "data": [
    {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "title": "服务器 CPU 使用率过高",
      "severity": "high",
      "status": "open",
      "healing_status": "pending",
      "scanned": false,
      ...
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 100
  }
}
```

---

### 2. 获取工单详情

```http
GET /api/v1/incidents/{id}
```

---

### 3. 重置单个工单扫描状态

```http
POST /api/v1/incidents/{id}/reset-scan
```

**功能：**
- 将 `scanned` 设为 `false`
- 清除 `matched_rule_id`
- 清除 `healing_flow_instance_id`

**使用场景：**
- 自愈规则修改后，让工单重新匹配
- 流程执行失败后，让工单重新进入自愈流程

**响应：**
```json
{
  "message": "工单扫描状态已重置，将被重新扫描"
}
```

---

### 4. 批量重置工单扫描状态

```http
POST /api/v1/incidents/batch-reset-scan
```

**请求体：**
```json
{
  "ids": ["uuid1", "uuid2", "uuid3"],
  "healing_status": "failed"
}
```

| 字段 | 类型 | 说明 |
|-----|------|------|
| `ids` | UUID[] | 指定工单 ID 列表（为空时按条件筛选） |
| `healing_status` | string | 按自愈状态筛选 |

**使用示例：**

1. **重置指定工单**：
```json
{
  "ids": ["uuid1", "uuid2"]
}
```

2. **重置所有失败的工单**：
```json
{
  "healing_status": "failed"
}
```

3. **同时指定 ID 和状态**：
```json
{
  "ids": ["uuid1", "uuid2"],
  "healing_status": "failed"
}
```

**响应：**
```json
{
  "affected_count": 5,
  "message": "成功重置 5 条工单的扫描状态"
}
```

---

## 工单生命周期

```
ITSM 插件同步 → 工单创建 (healing_status=pending, scanned=false)
       ↓
自愈调度器扫描 → scanned=true
       ↓
匹配规则 → matched_rule_id=xxx
       ↓
创建流程实例 → healing_flow_instance_id=xxx, healing_status=processing
       ↓
流程执行完成 → healing_status=healed 或 failed
       ↓
(可选) 重置扫描 → scanned=false, 重新进入扫描队列
```

---

## 权限要求

| 接口 | 权限 |
|-----|------|
| 获取工单列表/详情 | `plugin:list` |
| 重置扫描状态 | `plugin:sync` |
