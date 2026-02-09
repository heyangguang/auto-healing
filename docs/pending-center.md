# 待办中心 API 文档

待办中心（Pending Center）统一管理所有需要人工决策的事项，包括待触发工单和待审批任务。

---

## 概念说明

###  待触发工单 vs 待审批任务

| 对比项 | 待触发工单 (Pending Trigger) | 待审批任务 (Pending Approval) |
|-------|----------------------------|----------------------------|
| **时机** | 流程尚未开始 | 流程已运行到审批节点 |
| **状态** | 规则已匹配，但未创建流程实例 | 流程实例状态为 `waiting_approval` |
| **操作** | 点击"启动自愈" → 创建并执行流程 | 点击"批准/拒绝" → 流程继续/中止 |
| **来源** | Manual 触发模式的规则 | 流程中的 Approval 节点 |

---

## API 接口

### 1. 获取待触发工单列表

**GET** `/api/v1/healing/pending/trigger`

获取已匹配 Manual 规则但尚未触发的工单列表。

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| search | string | 模糊搜索：title, external_id, affected_ci |
| severity | string | 级别过滤：1=严重, 2=高, 3=中, 4=低 |
| date_from | string | 起始日期 (YYYY-MM-DD) |
| date_to | string | 结束日期 (YYYY-MM-DD) |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "a63982b8-a0e6-4900-83c9-8a91554e3f27",
      "plugin_id": "e6f1c6a5-5482-4b86-9679-56e89e04ceb7",
      "source_plugin_name": "Mock ITSM测试",
      "external_id": "E2E-INC-1767940113-002",
      "title": "E2E-HEALING 服务重启请求",
      "description": "应用服务需要重启",
      "severity": "2",
      "priority": "2",
      "status": "1",
      "category": "software",
      "affected_ci": "192.168.31.103",
      "affected_service": "web-service",
      "assignee": "李四",
      "reporter": "运维人员",
      "healing_status": "pending",
      "scanned": true,
      "matched_rule_id": "04bb30f6-85e4-4ac6-9a9a-fe72df03817f",
      "healing_flow_instance_id": null,
      "created_at": "2026-01-09T14:28:33.054613+08:00",
      "updated_at": "2026-01-09T14:28:33.054613+08:00",
      "plugin": {
        "id": "e6f1c6a5-5482-4b86-9679-56e89e04ceb7",
        "name": "Mock ITSM测试",
        "type": "itsm"
      }
    }
  ],
  "total": 15,
  "page": 1,
  "page_size": 20
}
```

**筛选逻辑**
- `scanned = true` (已扫描)
- `matched_rule_id IS NOT NULL` (匹配了规则)
- `healing_flow_instance_id IS NULL` (未创建流程实例，说明是Manual模式)

---

### 2. 手动触发自愈流程

**POST** `/api/v1/incidents/:id/trigger`

为指定工单手动触发自愈流程（用于待办中心的"启动自愈"按钮）。

**Path 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| id | string (uuid) | 工单 ID |

**响应示例** (HTTP 201)
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "f96c81e4-e180-4a45-8506-855140b8d1ff",
    "flow_id": "db050a65-2c93-4abf-b3f1-281b7890e446",
    "rule_id": "04bb30f6-85e4-4ac6-9a9a-fe72df03817f",
    "incident_id": "a63982b8-a0e6-4900-83c9-8a91554e3f27",
    "status": "pending",
    "context": {
      "incident": {...}
    },
    "created_at": "2026-01-13T15:30:00+08:00",
    "updated_at": "2026-01-13T15:30:00+08:00"
  }
}
```

**失败响应（工单未匹配规则）**
```json
{
  "code": 40000,
  "message": "此工单未匹配任何规则"
}
```

**失败响应（已触发过）**
```json
{
  "code": 40000,
  "message": "此工单已经触发过自愈流程"
}
```

---

### 3. 获取待审批任务列表

**GET** `/api/v1/healing/approvals/pending`

获取所有状态为 `pending` 的审批任务。

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| search | string | 模糊搜索：node_id, flow_instance_id |
| date_from | string | 起始日期 (YYYY-MM-DD) |
| date_to | string | 结束日期 (YYYY-MM-DD) |

---

## 前端集成建议

### 待办中心菜单结构

```
📁 待办中心
  ├── [Tab] 待触发工单
  │   └── API: GET /healing/pending/trigger
  │   └── 操作: POST /incidents/:id/trigger
  └── [Tab] 待审批任务
      └── API: GET /healing/approvals/pending
      └── 操作: POST /healing/approvals/:id/approve
              POST /healing/approvals/:id/reject
```

### 操作按钮

**待触发工单操作列**
```tsx
<Button 
  type="primary" 
  onClick={() => triggerHealing(record.id)}
>
  启动自愈
</Button>
```

**待审批任务操作列**
```tsx
<Space>
  <Button type="primary" onClick={() => approve(record.id)}>批准</Button>
  <Button danger onClick={() => reject(record.id)}>拒绝</Button>
</Space>
```

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /healing/pending/trigger | healing:trigger:view |
| POST /incidents/:id/trigger | healing:trigger:execute |
| GET /healing/approvals/pending | healing:approvals:view |
| POST /healing/approvals/:id/approve | healing:approvals:approve |
| POST /healing/approvals/:id/reject | healing:approvals:approve |
