# 自愈流程 API 文档

自愈流程是自动化故障修复的核心定义，使用可视化节点编排方式定义修复步骤。

## 数据模型

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| id | string (uuid) | - | 流程 ID (自动生成) |
| name | string | ✅ | 流程名称 |
| description | string | - | 流程描述 |
| nodes | array | - | 节点列表 (JSONB) |
| edges | array | - | 连线列表 (JSONB) |
| is_active | boolean | - | 是否启用 (默认 true) |
| created_by | string (uuid) | - | 创建人 ID |
| created_at | string | - | 创建时间 |
| updated_at | string | - | 更新时间 |

### 节点定义 (nodes)

每个节点包含以下字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | 节点唯一标识 (如 `start_1`) |
| type | string | 节点类型 |
| config | object | 节点配置 (根据类型不同) |
| name | string | 节点名称 (可选) |
| position | object | 节点位置 `{x, y}` (可选，用于可视化) |

#### 节点类型

| 类型 | 说明 | 配置参数 |
|------|------|----------|
| `start` | 开始节点 | 无 |
| `end` | 结束节点 | 无 |
| `host_extractor` | 主机提取 | `source_field`, `extract_mode`, `split_by`, `output_key` |
| `cmdb_validator` | CMDB 验证 | `input_key`, `output_key` |
| `approval` | 人工审批 | `title`, `description`, `approvers`, `approver_roles`, `timeout_hours` |
| `execution` | 任务执行 | `task_template_id`, `hosts_key`, `extra_vars`, `executor_type` |
| `notification` | 通知发送 | `template_id`, `channel_ids` |
| `condition` | 条件分支 | `condition`, `true_target`, `false_target` |
| `set_variable` | 设置变量 | `key`, `value` |

#### 节点配置示例

**主机提取节点**
```json
{
  "id": "host_extractor_1",
  "type": "host_extractor",
  "config": {
    "source_field": "raw_data.cmdb_ci",
    "extract_mode": "split",
    "split_by": ",",
    "output_key": "hosts"
  }
}
```

**CMDB 验证节点**
```json
{
  "id": "cmdb_validator_1",
  "type": "cmdb_validator",
  "config": {
    "input_key": "hosts",
    "output_key": "validated_hosts"
  }
}
```

**审批节点**
```json
{
  "id": "approval_1",
  "type": "approval",
  "config": {
    "title": "服务重启审批",
    "description": "请确认是否执行自愈操作",
    "approvers": ["admin"],
    "approver_roles": ["admin"],
    "timeout_hours": 1
  }
}
```

**执行节点**
```json
{
  "id": "execution_1",
  "type": "execution",
  "config": {
    "task_template_id": "8cbf586d-065d-4421-987e-0eb240b179f2",
    "hosts_key": "validated_hosts",
    "executor_type": "local",
    "extra_vars": {
      "service_name": "nginx",
      "service_action": "restart"
    }
  }
}
```

**通知节点**
```json
{
  "id": "notification_1",
  "type": "notification",
  "config": {
    "template_id": "8cbf586d-065d-4421-987e-0eb240b179f2",
    "channel_ids": ["164c0c2b-d4ae-4d1e-993d-8bd36aeffc51"]
  }
}
```

### 连线定义 (edges)

```json
{
  "source": "start_1",
  "target": "host_extractor_1"
}
```

> [!NOTE]
> 连线支持两种格式：`source/target` 或 `from/to`，后端会自动兼容。

---

## API 接口

### 1. 获取流程列表

**GET** `/api/v1/healing/flows`

**Query 参数**
| 参数 | 类型 | 说明 |
|------|------|------|
| page | number | 页码 (默认 1) |
| page_size | number | 每页数量 (默认 20) |
| is_active | boolean | 筛选启用状态 |

**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "db050a65-2c93-4abf-b3f1-281b7890e446",
      "name": "E2E流程-服务重启",
      "description": "端到端测试流程（含审批节点）",
      "nodes": [
        {"id": "start_1", "type": "start", "config": {}},
        {"id": "host_extractor_1", "type": "host_extractor", "config": {"extract_mode": "split", "output_key": "hosts", "source_field": "raw_data.cmdb_ci", "split_by": ","}},
        {"id": "cmdb_validator_1", "type": "cmdb_validator", "config": {"input_key": "hosts", "output_key": "validated_hosts"}},
        {"id": "approval_1", "type": "approval", "config": {"approver_roles": ["admin"], "approvers": ["admin"], "description": "请确认是否执行自愈操作", "timeout_hours": 1, "title": "自愈执行审批"}},
        {"id": "execution_1", "type": "execution", "config": {"executor_type": "local", "hosts_key": "validated_hosts", "task_template_id": "..."}},
        {"id": "notification_1", "type": "notification", "config": {"channel_ids": ["..."], "template_id": "..."}},
        {"id": "end_1", "type": "end", "config": {}}
      ],
      "edges": [
        {"source": "start_1", "target": "host_extractor_1"},
        {"source": "host_extractor_1", "target": "cmdb_validator_1"},
        {"source": "cmdb_validator_1", "target": "approval_1"},
        {"source": "approval_1", "target": "execution_1"},
        {"source": "execution_1", "target": "notification_1"},
        {"source": "notification_1", "target": "end_1"}
      ],
      "is_active": true,
      "created_at": "2026-01-06T07:34:31.406555+08:00",
      "updated_at": "2026-01-06T07:34:31.406555+08:00"
    }
  ],
  "total": 5,
  "page": 1,
  "page_size": 20
}
```

---

### 2. 创建流程

**POST** `/api/v1/healing/flows`

**请求体**
```json
{
  "name": "服务重启流程",
  "description": "自动重启故障服务",
  "nodes": [
    {"id": "start_1", "type": "start", "config": {}},
    {"id": "host_extractor_1", "type": "host_extractor", "config": {"source_field": "raw_data.cmdb_ci", "extract_mode": "split", "split_by": ",", "output_key": "hosts"}},
    {"id": "execution_1", "type": "execution", "config": {"task_template_id": "xxx", "hosts_key": "hosts"}},
    {"id": "end_1", "type": "end", "config": {}}
  ],
  "edges": [
    {"source": "start_1", "target": "host_extractor_1"},
    {"source": "host_extractor_1", "target": "execution_1"},
    {"source": "execution_1", "target": "end_1"}
  ],
  "is_active": true
}
```

**响应示例** (HTTP 201)
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "123e4567-e89b-12d3-a456-426614174001",
    "name": "服务重启流程",
    "description": "自动重启故障服务",
    "nodes": [...],
    "edges": [...],
    "is_active": true,
    "created_at": "2026-01-10T10:00:00+08:00",
    "updated_at": "2026-01-10T10:00:00+08:00"
  }
}
```

---

### 3. 获取流程详情

**GET** `/api/v1/healing/flows/:id`

**响应**: 同列表中的单个流程对象

---

### 4. 更新流程

**PUT** `/api/v1/healing/flows/:id`

**请求体** (所有字段可选)
```json
{
  "name": "更新后的流程名称",
  "description": "更新后的描述",
  "nodes": [...],
  "edges": [...],
  "is_active": false
}
```

**响应**: 返回更新后的流程对象

---

### 5. 删除流程

**DELETE** `/api/v1/healing/flows/:id`

**成功响应**
```json
{
  "code": 0,
  "message": "删除成功"
}
```

---

### 6. Dry-Run 模拟执行

**POST** `/api/v1/healing/flows/:id/dry-run`

模拟执行流程，验证配置是否正确，不产生真实副作用。支持从指定节点重试。

**请求体**
```json
{
  "mock_incident": {
    "title": "测试工单",
    "severity": "1",
    "priority": "1",
    "category": "software",
    "affected_ci": "192.168.1.100,192.168.1.101",
    "raw_data": {
      "cmdb_ci": "server-01"
    }
  },
  "from_node_id": "",
  "context": {}
}
```

**请求参数说明**

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| mock_incident | object | ✅ | 模拟工单数据 |
| from_node_id | string | - | 从哪个节点开始执行（用于重试） |
| context | object | - | 初始上下文（用于重试，包含之前执行的变量） |
| mock_approvals | object | - | 模拟审批结果，格式: `{"节点ID": "approved \| rejected"}` |

**mock_approvals 示例**
```json
{
  "mock_incident": {"title": "测试告警", "raw_data": {"cmdb_ci": "192.168.1.100"}},
  "mock_approvals": {
    "approval_1": "rejected"
  }
}
```
- 未指定的审批节点默认 `approved`（通过）
- 设置为 `rejected` 时，流程走 `rejected` 分支

> [!TIP]
> **重试场景**：前端保存每次执行的 `output`，用户点击某节点"从这里重试"时，把之前的输出作为 `context` 传入。


**响应示例**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "success": true,
    "message": "Dry-Run 完成，共执行 5 个节点",
    "nodes": [
      {
        "node_id": "start_1",
        "node_type": "start",
        "status": "ok",
        "message": "流程开始"
      },
      {
        "node_id": "host_extractor_1",
        "node_type": "host_extractor",
        "status": "ok",
        "message": "提取主机: [192.168.1.100 192.168.1.101]",
        "output": {"hosts": ["192.168.1.100", "192.168.1.101"]}
      },
      {
        "node_id": "approval_1",
        "node_type": "approval",
        "status": "simulated",
        "message": "审批节点「服务重启审批」(模拟通过)"
      },
      {
        "node_id": "execution_1",
        "node_type": "execution",
        "status": "would_execute",
        "message": "将执行任务「服务重启」，目标主机: [192.168.1.100]",
        "output": {"task_template": "服务重启", "target_hosts": ["192.168.1.100"]}
      },
      {
        "node_id": "end_1",
        "node_type": "end",
        "status": "ok",
        "message": "流程结束"
      }
    ]
  }
}
```

**节点状态说明**

Dry-Run 和真实执行使用**完全一致**的状态值：

| status | 说明 |
|--------|------|
| `success` | 执行成功 |
| `failed` | 执行失败 |
| `skipped` | 未执行（条件/审批分支未选中） |
| `error` | 配置错误（缺少必要参数等） |

**节点日志结构**

每个节点返回三类日志，便于排错。Dry-Run 和真实执行结构**完全一致**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `input` | object | 节点输入（上游数据 + 当前全局上下文快照） |
| `process` | string[] | 执行过程日志（详细记录每一步操作） |
| `output` | object | 节点输出（传给下游的数据） |

**示例**：
```json
{
  "node_id": "host_extractor_1",
  "status": "success",
  "message": "提取主机: [192.168.1.100]",
  "input": {
    "incident": {"title": "测试", "raw_data": {"cmdb_ci": "192.168.1.100"}}
  },
  "process": [
    "读取配置 source_field: raw_data.cmdb_ci",
    "从字段 raw_data.cmdb_ci 提取主机",
    "成功提取 1 个主机: [192.168.1.100]",
    "写入上下文 hosts"
  ],
  "output": {
    "hosts": ["192.168.1.100"]
  }
}
```

**节点失败条件**

Dry-Run 会验证每个节点的必要输入条件，不满足时返回 `error` 状态并终止流程：

| 节点类型 | 失败条件 |
|---------|---------|
| `host_extractor` | 1. 未配置 `source_field`<br>2. 提取的主机列表为空 |
| `cmdb_validator` | 1. 输入主机列表为空<br>2. 所有主机验证失败 |
| `approval` | 未配置 `title`（审批标题） |
| `execution` | 1. 未配置 `task_template_id`<br>2. 任务模板不存在<br>3. 目标主机列表为空 |
| `notification` | 1. 未配置 `template_id`<br>2. 通知模板不存在<br>3. 未配置 `channel_ids` |

**Dry-Run 执行模式**

| 节点类型 | 执行模式 | 说明 |
|---------|---------|------|
| `start` | 真实 | 初始化流程上下文 |
| `host_extractor` | 真实 | 真正从工单数据提取主机 |
| `cmdb_validator` | 真实 | 真正查询 CMDB 数据库验证主机 |
| `condition` | 真实 | 真正执行条件表达式 |
| `set_variable` | 真实 | 真正设置变量 |
| `approval` | **模拟** | 模拟通过/拒绝（可通过 `mock_approvals` 配置）|
| `execution` | **模拟** | 模拟执行（不实际运行 Ansible） |
| `notification` | **模拟** | 模拟发送（不实际发送通知） |
| `end` | 真实 | 结束流程 |

---

### Dry-Run 与真实执行对比

Dry-Run 和真实执行设计上**尽可能一致**，方便前端用同一套逻辑处理：

| 维度 | Dry-Run (测试模式) | 真实执行 |
|------|-------------------|---------|
| **状态值** | `success` / `failed` / `error` | `success` / `failed` / `error` |
| **日志结构** | `input` / `process` / `output` | `input` / `process` / `output` |
| **SSE 事件流** | ✅ `node_start` / `node_log` / `node_complete` | ✅ 相同 |
| **画布状态流动** | ✅ 实时更新节点状态 | ✅ 相同 |
| **数据持久化** | ❌ 不写数据库 | ✅ 写 `flow_execution_logs` 表 |
| **历史查看** | ❌ 仅当次结果 | ✅ 可查历史执行记录 |

**唯一区别**：
- Dry-Run 不写数据库，仅返回当次执行结果
- 真实执行写入数据库，支持事后查看历史记录

**节点日志输出**

每个节点执行时都会记录日志，前端点击节点可查看：

| 节点类型 | 日志内容 |
|---------|---------|
| `start` | `incident` 工单数据 |
| `end` | 流程结束 |
| `host_extractor` | 提取的主机列表 |
| `cmdb_validator` | 验证通过的主机列表 |
| `approval` | 审批标题、审批人、超时时间 |
| `execution` | 任务名称、目标主机、执行结果 |
| `notification` | 通知模板、发送渠道、发送结果 |
| `condition` | 条件表达式、匹配分支 |
| `set_variable` | 设置的变量名和值 |

---

## 节点执行状态定义

| 状态 | 英文 | 说明 | 适用节点 |
|------|------|------|---------|
| 等待 | `pending` | 还没轮到执行 | 所有 |
| 执行中 | `running` | 正在执行 | 所有 |
| 成功 | `success` | 执行成功 | 所有 |
| 部分成功 | `partial` | 部分成功 | execution |
| 失败 | `failed` | 执行失败 | 所有 |
| 跳过 | `skipped` | 因分支条件被跳过 | 所有 |
| 等待审批 | `waiting_approval` | 等待人工审批 | approval |

### 状态流转图

```
                    ┌─────────┐
                    │ pending │
                    └────┬────┘
                         │ 开始执行
                         ▼
                    ┌─────────┐
                    │ running │
                    └────┬────┘
                         │
        ┌────────┬───────┼───────┬──────────┐
        ↓        ↓       ↓       ↓          ↓
   ┌────────┐ ┌─────────┐ ┌────────┐ ┌─────────────────┐
   │success │ │ partial │ │ failed │ │waiting_approval │
   └────────┘ └─────────┘ └────────┘ └────────┬────────┘
                                              │
                                    ┌─────────┴─────────┐
                                    ↓                   ↓
                               ┌────────┐          ┌────────┐
                               │success │          │ failed │
                               └────────┘          └────────┘
```

---

### 6.2 Dry-Run SSE 流式输出

**POST** `/api/v1/healing/flows/:id/dry-run-stream`

与 `/dry-run` 相同的请求体，但返回 SSE 事件流，实时推送节点执行状态。

**SSE 事件类型**

| 事件 | 说明 |
|------|------|
| `flow_start` | 流程开始 |
| `node_start` | 节点开始执行 |
| `node_log` | 节点日志 |
| `node_complete` | 节点执行完成 |
| `flow_complete` | 流程结束 |

**事件数据示例**

```json
event: node_start
data: {"event":"node_start","data":{"node_id":"execution_1","node_type":"execution","node_name":"任务执行","status":"running"},"timestamp":"2026-01-13T22:16:00+08:00"}

event: node_complete
data: {"event":"node_complete","data":{"node_id":"execution_1","node_type":"execution","status":"success","output":{"run_id":"xxx"}},"timestamp":"2026-01-13T22:16:30+08:00"}

event: flow_complete
data: {"event":"flow_complete","data":{"success":true,"message":"Dry-Run 完成，共执行 5 个节点"},"timestamp":"2026-01-13T22:16:35+08:00"}
```

**前端使用示例**

```javascript
const eventSource = new EventSource('/api/v1/healing/flows/xxx/dry-run-stream');

eventSource.addEventListener('node_start', (event) => {
  const data = JSON.parse(event.data);
  updateNodeStatus(data.data.node_id, 'running');
});

eventSource.addEventListener('node_complete', (event) => {
  const data = JSON.parse(event.data);
  updateNodeStatus(data.data.node_id, data.data.status);
});

eventSource.addEventListener('flow_complete', (event) => {
  eventSource.close();
});
```

---


### 7. 获取节点类型定义

**GET** `/api/v1/healing/flows/node-schema`

返回所有节点类型的配置项、输入输出变量、分支信息，用于前端流程设计器。

**响应结构**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "initial_context": {
      "incident": {
        "type": "object",
        "description": "触发流程的工单数据",
        "properties": {
          "title": {"type": "string", "description": "工单标题"},
          "severity": {"type": "string", "description": "严重级别"},
          "affected_ci": {"type": "string", "description": "影响的CI"},
          "raw_data": {"type": "object", "description": "原始数据"}
        }
      }
    },
    "nodes": {
      "start": {...},
      "host_extractor": {...},
      "approval": {...}
    }
  }
}
```

**节点定义结构**
```json
{
  "name": "审批节点",
  "description": "等待人工审批，有两个输出分支",
  "config": {
    "title": {"type": "string", "required": "true", "description": "审批标题"},
    "timeout_hours": {"type": "number", "default": "24", "description": "超时时间"}
  },
  "ports": {
    "in": 1,
    "out": 2,
    "out_ports": [
      {"id": "approved", "name": "通过", "condition": "审批通过时"},
      {"id": "rejected", "name": "拒绝", "condition": "审批拒绝或超时时"}
    ]
  },
  "inputs": [],
  "outputs": []
}
```

**节点分支信息 (ports)**

| 节点类型 | 输入口 | 输出口 | 输出口ID | 说明 |
|---------|-------|-------|---------|------|
| `start` | 0 | 1 | `default` | 起始节点 |
| `end` | 1 | 0 | - | 结束节点 |
| `host_extractor` | 1 | 1 | `default` | 单分支 |
| `cmdb_validator` | 1 | 1 | `default` | 单分支 |
| `approval` | 1 | 2 | `approved`, `rejected` | 通过/拒绝两个分支 |
| `execution` | 1 | 3 | `success`, `partial`, `failed` | 成功/部分成功/失败三个分支 |
| `notification` | 1 | 1 | `default` | 单分支 |
| `condition` | 1 | 2 | `true`, `false` | 条件判断两个分支 |
| `set_variable` | 1 | 1 | `default` | 单分支 |

**Edge 数据结构**

```json
{
  "source": "execution_1",
  "target": "notification_success",
  "sourceHandle": "success"
}
```

> [!NOTE]
> `sourceHandle` 用于指定边从源节点的哪个输出口连出。前端画布上每个输出口对应一个连接点。


---

## 权限要求

| 接口 | 权限 |
|------|------|
| GET /healing/flows/node-schema | healing:flows:view |
| GET /healing/flows | healing:flows:view |
| POST /healing/flows | healing:flows:create |
| GET /healing/flows/:id | healing:flows:view |
| PUT /healing/flows/:id | healing:flows:update |
| DELETE /healing/flows/:id | healing:flows:delete |
| POST /healing/flows/:id/dry-run | healing:flows:update |

