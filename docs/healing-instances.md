# 执行实例 API 文档

执行实例（FlowInstance）是自愈流程的一次具体执行记录，包含执行状态、上下文数据和节点执行状态。

## 数据模型

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string (uuid) | - | 实例 ID (自动生成) |
| flow_id | string (uuid) | ✅ | 关联的流程 ID |
| rule_id | string (uuid) | - | 触发此实例的规则 ID |
| incident_id | string (uuid) | - | 关联的事件 ID |
| status | string | - | 执行状态 |
| current_node_id | string | - | 当前执行到的节点 ID |
| context | object | - | 执行上下文 (JSONB) |
| node_states | object | - | 各节点的执行状态 (JSONB) |
| error_message | string | - | 错误信息 |
| started_at | string | - | 开始时间 |
| completed_at | string | - | 完成时间 |
| created_at | string | - | 创建时间 |
| updated_at | string | - | 更新时间 |
| flow | object | - | 关联的流程对象 |
| rule | object | - | 关联的规则对象 |
| incident | object | - | 关联的事件对象 |

### 执行状态 (status)

| 状态 | 说明 |
|------|------|
| `pending` | 等待执行 |
| `running` | 正在执行 |
| `waiting_approval` | 等待审批 |
| `completed` | 执行完成 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |

### 执行上下文 (context)

执行上下文存储流程执行过程中的数据，由各节点动态写入和读取。

```json
{
  "hosts": ["192.168.31.103", "192.168.31.100"],
  "incident": {
    "id": "a63982b8-a0e6-4900-83c9-8a91554e3f27",
    "title": "E2E-HEALING 服务重启请求",
    "severity": "2",
    "affected_ci": "192.168.31.103"
  },
  "validated_hosts": [
    {
      "name": "healing-test-host",
      "ip_address": "192.168.31.103",
      "hostname": "healing-test-host",
      "cmdb_id": "25996c96-3d58-4928-bb03-758a0a1ec4f7",
      "os": "Linux",
      "os_version": "CentOS 8",
      "environment": "production",
      "status": "active",
      "valid": true
    }
  ],
  "validation_summary": {
    "total": 2,
    "valid": 2,
    "invalid": 0
  }
}
```

### 节点状态 (node_states)

记录每个已执行节点的状态信息：

```json
{
  "approval_1": {
    "status": "waiting_approval",
    "title": "E2E 自愈执行审批",
    "description": "请确认是否执行自愈操作",
    "task_id": "5dd0809d-823f-4480-90a6-2bf3ed3bbdef",
    "timeout_at": "2026-01-09T16:38:14+08:00",
    "created_at": "2026-01-09T15:38:14+08:00"
  },
  "execution_1": {
    "status": "completed",
    "run_id": "abc12345-...",
    "stats": {"ok": 2, "failed": 0}
  }
}
```

---

## API 接口

### 1. 获取实例列表

**GET** `/api/v1/healing/instances`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| status | string | 筛选执行状态 |
| flow_id | string (uuid) | 筛选关联流程 |
| rule_id | string (uuid) | 筛选触发规则 |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "f96c81e4-e180-4a45-8506-855140b8d1ff",
      "flow_id": "db050a65-2c93-4abf-b3f1-281b7890e446",
      "rule_id": "04bb30f6-85e4-4ac6-9a9a-fe72df03817f",
      "incident_id": "a63982b8-a0e6-4900-83c9-8a91554e3f27",
      "status": "failed",
      "current_node_id": "approval_1",
      "context": {
        "hosts": ["192.168.31.103", "192.168.31.100"],
        "incident": {...},
        "validated_hosts": [...],
        "validation_summary": {"total": 4, "valid": 4, "invalid": 0}
      },
      "node_states": {
        "approval_1": {
          "status": "waiting_approval",
          "title": "E2E 自愈执行审批",
          "task_id": "5dd0809d-823f-4480-90a6-2bf3ed3bbdef"
        }
      },
      "error_message": "审批超时",
      "started_at": "2026-01-09T15:38:14.201493+08:00",
      "completed_at": "2026-01-09T16:38:20.818028+08:00",
      "created_at": "2026-01-09T15:38:14.187549+08:00",
      "updated_at": "2026-01-09T16:38:20.818378+08:00",
      "flow": {
        "id": "db050a65-2c93-4abf-b3f1-281b7890e446",
        "name": "E2E流程-服务重启",
        "description": "端到端测试流程"
      },
      "rule": {
        "id": "04bb30f6-85e4-4ac6-9a9a-fe72df03817f",
        "name": "E2E规则-服务重启",
        "priority": 100
      },
      "incident": {
        "id": "a63982b8-a0e6-4900-83c9-8a91554e3f27",
        "title": "E2E-HEALING 四主机混合认证测试",
        "severity": "3",
        "healing_status": "failed"
      }
    }
  ],
  "total": 2147,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 获取实例详情

**GET** `/api/v1/healing/instances/:id`

**响应**: 同列表中的单个实例对象，包含完整的 `context`、`node_states` 以及关联对象

---

### 3. 取消实例

**POST** `/api/v1/healing/instances/:id/cancel`

取消正在执行或等待中的流程实例。

> [!NOTE]
> 取消实例会同时更新关联事件的 `healing_status` 为 `skipped`。

**成功响应**
```json
{
  "code": 0,
  "message": "流程实例已取消"
}
```

**失败响应（实例已完成）**
```json
{
  "code": 40000,
  "message": "无法取消已完成的实例"
}
```

---

### 4. 重试实例

**POST** `/api/v1/healing/instances/:id/retry`

重试失败的流程实例，支持从指定节点开始。

> [!NOTE]
> 只能重试状态为 `failed` 的实例。

**请求体（可选）**
```json
{
  "from_node_id": "execution_1"
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| from_node_id | string | - | 从哪个节点开始重试，为空则从失败节点继续 |

**成功响应**
```json
{
  "code": 0,
  "message": "流程实例正在重试"
}
```

**失败响应（非失败状态）**
```json
{
  "code": 40000,
  "message": "只能重试失败的流程实例"
}
```

---

### 5. 获取实例事件流 (SSE)

**GET** `/api/v1/healing/instances/:id/events`

返回 SSE 事件流，实时推送流程执行状态变化。

**SSE 事件类型**

| 事件 | 说明 |
|------|------|
| `connected` | 连接建立，返回实例当前状态 |
| `node_start` | 节点开始执行 |
| `node_log` | 节点日志 |
| `node_complete` | 节点执行完成 |
| `flow_complete` | 流程结束 |

**事件数据示例**

```json
event: connected
data: {"event":"connected","data":{"instance_id":"xxx","status":"running"},"timestamp":"..."}

event: node_start
data: {"event":"node_start","data":{"node_id":"execution_1","node_type":"execution","status":"running"},"timestamp":"..."}

event: node_log
data: {"event":"node_log","data":{"node_id":"execution_1","level":"info","message":"开始执行任务","details":{...}},"timestamp":"..."}

event: node_complete
data: {"event":"node_complete","data":{"node_id":"execution_1","status":"success","output":{...}},"timestamp":"..."}

event: flow_complete
data: {"event":"flow_complete","data":{"success":true,"status":"completed","message":"流程执行完成"},"timestamp":"..."}
```

**前端使用示例**

```javascript
const eventSource = new EventSource('/api/v1/healing/instances/xxx/events');

eventSource.addEventListener('node_log', (event) => {
  const data = JSON.parse(event.data);
  appendLog(data.data.node_id, data.data.message);
});

eventSource.addEventListener('flow_complete', (event) => {
  const data = JSON.parse(event.data);
  showResult(data.data.success, data.data.message);
  eventSource.close();
});
```

---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /healing/instances | healing:instances:view |
| GET /healing/instances/:id | healing:instances:view |
| POST /healing/instances/:id/cancel | healing:instances:view |
| POST /healing/instances/:id/retry | healing:instances:view |
| GET /healing/instances/:id/events | healing:instances:view |
