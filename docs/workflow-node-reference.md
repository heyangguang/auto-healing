# 工作流节点参考文档

本文档详细描述了自愈引擎工作流中所有节点类型及其配置参数。

## 目录

1. [概述](#概述)
2. [节点类型](#节点类型)
   - [start - 开始节点](#start---开始节点)
   - [end - 结束节点](#end---结束节点)
   - [host_extractor - 主机提取节点](#host_extractor---主机提取节点)
   - [cmdb_validator - CMDB校验节点](#cmdb_validator---cmdb校验节点)
   - [approval - 审批节点](#approval---审批节点)
   - [execution - 执行节点](#execution---执行节点)
   - [notification - 通知节点](#notification---通知节点)
   - [condition - 条件判断节点](#condition---条件判断节点)
   - [set_variable - 变量设置节点](#set_variable---变量设置节点)
3. [表达式语法](#表达式语法)
4. [Context 数据流](#context-数据流)
5. [边（Edge）定义](#边edge定义)
6. [完整 E2E 示例](#完整-e2e-示例)

---

## 概述

自愈流程使用 DAG（有向无环图）结构定义，由**节点（Nodes）**和**边（Edges）**组成。

### 基础结构

```json
{
  "name": "自愈流程名称",
  "description": "流程描述",
  "is_active": true,
  "nodes": [...],
  "edges": [...]
}
```

### 节点公共字段

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `id` | string | 是 | 节点唯一标识符 |
| `type` | string | 是 | 节点类型（见下文） |
| `config` | object | 是 | 节点配置对象（可为空 `{}`） |
| `name` | string | 否 | 节点显示名称 |
| `position` | object | 否 | 前端可视化坐标 `{x, y}` |

---

## 节点类型

### start - 开始节点

流程的入口点，每个流程必须有且只有一个开始节点。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| - | - | - | 无需配置参数 |

**示例**：
```json
{
  "id": "start_1",
  "type": "start",
  "config": {}
}
```

---

### end - 结束节点

流程的终点节点。一个流程可以有多个结束节点（用于不同分支）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| - | - | - | 无需配置参数 |

**示例**：
```json
{
  "id": "end_1",
  "type": "end",
  "config": {}
}
```

---

### host_extractor - 主机提取节点

从工单数据中提取目标主机列表。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `source_field` | string | 是 | - | 数据来源字段路径（支持嵌套如 `raw_data.cmdb_ci`） |
| `extract_mode` | string | 否 | `split` | 提取模式（见下表） |
| `split_by` | string | 否 | `,` | 分隔符（`split` 模式使用） |
| `regex_pattern` | string | 否 | - | 正则表达式（`regex` 模式使用） |
| `output_key` | string | 否 | `hosts` | 输出到 context 的 key 名称 |

**提取模式说明**：

| 模式 | 说明 | 示例 |
|------|------|------|
| `direct` | 直接使用字段值作为单个主机 | `"web-01"` → `["web-01"]` |
| `split` | 按分隔符拆分 | `"web-01,web-02"` → `["web-01", "web-02"]` |
| `regex` | 正则表达式匹配提取 | 使用 `regex_pattern` |
| `json_path` | 从 JSON 数组提取（自动） | `["web-01", "web-02"]` → 直接使用 |

**示例**：
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

---

### cmdb_validator - CMDB校验节点

验证提取的主机是否存在于内部 CMDB 中，并获取主机详细信息（IP 地址等）。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `input_key` | string | 否 | `hosts` | 输入主机列表的 context key |
| `output_key` | string | 否 | `validated_hosts` | 输出验证后主机的 context key |
| `fail_on_not_found` | bool | 否 | `true` | 主机不存在时是否失败 |
| `skip_missing` | bool | 否 | `false` | 跳过缺失的主机继续执行 |

**输出结构**（存入 `output_key`）：
```json
[
  {
    "original_name": "web-server-01",
    "ip_address": "192.168.31.100",
    "name": "WebServer01",
    "hostname": "web-server-01.local",
    "status": "active",
    "environment": "production",
    "os": "Ubuntu",
    "os_version": "22.04",
    "owner": "ops-team",
    "location": "DC1",
    "valid": true,
    "cmdb_id": "uuid-xxx"
  }
]
```

**示例**：
```json
{
  "id": "cmdb_validator_1",
  "type": "cmdb_validator",
  "config": {
    "input_key": "hosts",
    "output_key": "validated_hosts",
    "fail_on_not_found": true,
    "skip_missing": false
  }
}
```

---

### approval - 审批节点

创建审批任务，暂停流程执行直到审批通过或拒绝。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `title` | string | 是 | - | 审批任务标题 |
| `description` | string | 否 | - | 审批任务描述 |
| `approvers` | []string | 否 | `[]` | 审批人用户名列表 |
| `approver_roles` | []string | 否 | `[]` | 审批角色列表 |
| `timeout_hours` | float | 否 | `24` | 超时时间（小时） |

**流程状态变化**：
```
running → waiting_approval → (批准后) running → ...
                          → (拒绝后) failed
```

**API 交互**：
- `GET /healing/approvals/pending` - 获取待审批列表
- `GET /healing/approvals/{id}` - 获取审批详情
- `POST /healing/approvals/{id}/approve` - 批准
- `POST /healing/approvals/{id}/reject` - 拒绝

**示例**：
```json
{
  "id": "approval_1",
  "type": "approval",
  "config": {
    "title": "自愈执行审批",
    "description": "请确认是否执行自愈操作",
    "approvers": ["admin", "ops"],
    "approver_roles": ["admin"],
    "timeout_hours": 1
  }
}
```

---

### execution - 执行节点

执行 Ansible Playbook，支持本地执行和 Docker 容器执行。

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `git_repo_id` | string | **是** | - | Git 仓库 UUID（必须已激活） |
| `playbook_path` | string | 否 | 仓库 `main_playbook` | Playbook 相对路径 |
| `executor_type` | string | 否 | `local` | 执行器类型：`local`、`docker` |
| `docker_image` | string | 否 | `ansible:latest` | Docker 镜像（`docker` 模式） |
| `hosts_key` | string | 否 | `validated_hosts` | 目标主机 context key |
| `secrets_source_id` | string | 否 | - | 密钥源 UUID（用于 SSH 认证） |
| `extra_vars` | object | 否 | `{}` | Ansible 额外变量 |
| `timeout_minutes` | int | 否 | `10` | 执行超时时间（分钟） |
| `verbosity` | int | 否 | `1` | Ansible 详细程度（0-4） |
| `keep_credentials` | bool | 否 | `false` | **调试选项**：保留凭证文件 |

**认证方式**（由 `secrets_source_id` 关联的密钥源决定）：

| auth_type | 描述 | 返回字段 |
|-----------|------|----------|
| `password` | 密码认证 | `password`, `username` |
| `ssh_key` | SSH 密钥认证 | `private_key`, `username` |

**凭证安全**：
- 默认执行完成后自动清理：`.ssh_keys/`、`inventory.ini`、`ansible.cfg`
- 设置 `keep_credentials: true` 可保留这些文件用于调试

**输出结构**（写入 context 的 `execution_result`）：
```json
{
  "status": "completed",
  "message": "执行成功",
  "exit_code": 0,
  "stdout": "PLAY RECAP...",
  "stderr": "",
  "started_at": "2026-01-06T03:00:00Z",
  "finished_at": "2026-01-06T03:00:30Z",
  "duration_ms": 30000,
  "stats": {
    "ok": 4,
    "changed": 1,
    "unreachable": 0,
    "failed": 0,
    "skipped": 0
  }
}
```

**示例**：
```json
{
  "id": "execution_1",
  "type": "execution",
  "config": {
    "git_repo_id": "uuid-of-git-repo",
    "executor_type": "local",
    "hosts_key": "validated_hosts",
    "secrets_source_id": "uuid-of-secrets-source",
    "extra_vars": {
      "service_name": "nginx",
      "service_action": "restart"
    },
    "timeout_minutes": 30,
    "keep_credentials": false
  }
}
```

---

### notification - 通知节点

发送通知到指定渠道（Webhook、钉钉、邮件等）。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `channel_ids` | []string | 是 | 通知渠道 UUID 列表 |
| `template_id` | string | 否 | 通知模板 UUID（不指定则使用默认模板） |
| `webhook_url` | string | 否 | 直接指定 Webhook URL（备用） |

**可用模板变量**（40+）：

| 分类 | 变量 |
|------|------|
| 时间 | `timestamp`, `date`, `time` |
| 流程 | `flow_instance_id`, `flow_status` |
| 系统 | `system_name`, `system_version`, `system_env` |
| 工单 | `incident_id`, `incident_title`, `incident_severity`, `incident_source` |
| 执行 | `execution_status`, `execution_message`, `execution_exit_code`, `execution_stdout` |
| 统计 | `stats.ok`, `stats.changed`, `stats.failed`, `stats.unreachable`, `stats.total` |
| 主机 | `target_hosts`, `host_count` |

详情参见 `/api/v1/template-variables` 接口。

**示例**：
```json
{
  "id": "notification_1",
  "type": "notification",
  "config": {
    "channel_ids": ["uuid-channel-1", "uuid-channel-2"],
    "template_id": "uuid-template"
  }
}
```

---

### condition - 条件判断节点

根据条件表达式决定流程分支走向。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `conditions` | []Condition | 是 | 条件列表（按顺序匹配，第一个为 true 的生效） |
| `default_target` | string | 否 | 无条件匹配时的默认目标节点 ID |

**Condition 结构**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `expression` | string | 条件表达式（见[表达式语法](#表达式语法)） |
| `target` | string | 条件成立时跳转的节点 ID |

**示例**：
```json
{
  "id": "condition_1",
  "type": "condition",
  "config": {
    "conditions": [
      {"expression": "execution_result.status == 'completed'", "target": "notification_success"},
      {"expression": "execution_result.exit_code != 0", "target": "notification_failed"},
      {"expression": "is_low_risk == true", "target": "skip_approval"}
    ],
    "default_target": "end_1"
  }
}
```

---

### set_variable - 变量设置节点

设置流程变量，可用于后续节点的条件判断或参数传递。

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `variables` | object | 是 | 变量键值对（key 为变量名，value 为值） |

**支持的值类型**：
- 字符串：`"high"`
- 布尔值：`true`, `false`
- 数字：`42`, `3.14`

**示例**：
```json
{
  "id": "set_var_1",
  "type": "set_variable",
  "config": {
    "variables": {
      "is_low_risk": false,
      "env": "production",
      "max_retries": 3,
      "custom_message": "自愈流程开始执行"
    }
  }
}
```

**使用场景**：
- 在条件节点前设置「风险等级」变量，决定后续分支
- 设置运行时参数供 execution 节点使用
- 记录流程过程中的中间状态

---

## 表达式语法

condition 和其他节点使用的表达式语法。

### 支持的运算符

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `==` | 等于 | `status == 'completed'` |
| `!=` | 不等于 | `exit_code != 0` |
| `>` | 大于 | `stats.failed > 0` |
| `<` | 小于 | `host_count < 5` |
| `>=` | 大于等于 | `stats.ok >= 1` |
| `<=` | 小于等于 | `duration_ms <= 60000` |

### 变量路径

| 路径 | 说明 | 示例 |
|------|------|------|
| `execution_result.xxx` | 执行节点结果 | `execution_result.status`, `execution_result.exit_code` |
| `execution.xxx` | 同上（别名） | `execution.stats.failed` |
| `context.xxx` | context 中的变量 | `context.is_low_risk` |
| 直接变量名 | set_variable 设置的变量 | `is_low_risk`, `env` |

### 字面量

| 类型 | 写法 | 示例 |
|------|------|------|
| 字符串 | 单/双引号 | `'completed'`, `"failed"` |
| 布尔值 | true/false | `true`, `false` |
| 数字 | 整数/浮点数 | `0`, `100`, `3.14` |

### 表达式示例

```
# 执行成功判断
execution_result.status == 'completed'
execution_result.exit_code == 0

# 失败判断  
execution_result.status != 'completed'
execution.stats.failed > 0

# 自定义变量判断
is_low_risk == true
env == 'production'
max_retries >= 3

# 统计信息判断
stats.ok > 0
host_count <= 10
```

---

## Context 数据流

流程执行过程中，各节点会向 `context` 中读写数据：

```
┌─────────────────┐
│     incident    │  (系统注入)
│   工单原始数据   │
└────────┬────────┘
         ▼
┌─────────────────┐
│  host_extractor │  读取: incident.raw_data.cmdb_ci
│                 │  写入: hosts[]
└────────┬────────┘
         ▼
┌─────────────────┐
│  cmdb_validator │  读取: hosts[]
│                 │  写入: validated_hosts[], validation_summary
└────────┬────────┘
         ▼
┌─────────────────┐
│  set_variable   │  写入: 自定义变量 (is_low_risk, env, ...)
└────────┬────────┘
         ▼
┌─────────────────┐
│    approval     │  写入: node_states.approval_1
└────────┬────────┘
         ▼
┌─────────────────┐
│   execution     │  读取: validated_hosts[], secrets_source_id
│                 │  写入: execution_result
└────────┬────────┘
         ▼
┌─────────────────┐
│   condition     │  读取: execution_result, 自定义变量
│                 │  决定: 下一跳节点
└────────┬────────┘
         ▼
┌─────────────────┐
│  notification   │  读取: 所有 context 变量构建模板  
└─────────────────┘
```

### 主要 Context Key

| Key | 来源节点 | 类型 | 说明 |
|-----|---------|------|------|
| `incident` | 系统 | object | 触发工单原始数据 |
| `hosts` | host_extractor | []string | 提取的主机名列表 |
| `validated_hosts` | cmdb_validator | []object | CMDB 验证后的主机详情 |
| `validation_summary` | cmdb_validator | object | 验证统计 `{total, valid, invalid}` |
| `execution_result` | execution | object | Ansible 执行结果 |
| `自定义变量` | set_variable | any | 用户设置的变量 |

---

## 边（Edge）定义

边定义节点之间的连接关系。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `source` | string | 是 | 源节点 ID |
| `target` | string | 是 | 目标节点 ID |
| `condition` | string | 否 | 条件表达式（用于条件节点的分支） |

**注意**：系统同时支持 `from/to` 格式和 `source/target` 格式。

**示例**：
```json
{
  "edges": [
    {"source": "start_1", "target": "host_extractor_1"},
    {"source": "host_extractor_1", "target": "cmdb_validator_1"},
    {"source": "cmdb_validator_1", "target": "approval_1"},
    {"source": "approval_1", "target": "execution_1"},
    {"source": "execution_1", "target": "condition_1"},
    {"source": "notification_success", "target": "end_1"},
    {"source": "notification_failed", "target": "end_2"}
  ]
}
```

---

## 完整 E2E 示例

### 示例 1: 基础自愈流程（含审批）

完整流程路径：
```
start → host_extractor → cmdb_validator → approval → execution → notification → end
```

**API 调用**：
```bash
# 1. 创建流程
curl -X POST "http://localhost:8080/api/v1/healing/flows" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Nginx 重启自愈流程",
    "description": "自动检测并重启 Nginx 服务",
    "is_active": true,
    "nodes": [
      {"id": "start_1", "type": "start", "config": {}},
      {"id": "host_extractor_1", "type": "host_extractor", "config": {
        "source_field": "raw_data.cmdb_ci",
        "extract_mode": "split",
        "split_by": ","
      }},
      {"id": "cmdb_validator_1", "type": "cmdb_validator", "config": {
        "input_key": "hosts",
        "output_key": "validated_hosts"
      }},
      {"id": "approval_1", "type": "approval", "config": {
        "title": "Nginx 重启审批",
        "description": "请审批是否重启 Nginx 服务",
        "timeout_hours": 1
      }},
      {"id": "execution_1", "type": "execution", "config": {
        "git_repo_id": "YOUR_GIT_REPO_UUID",
        "executor_type": "local",
        "secrets_source_id": "YOUR_SECRETS_SOURCE_UUID",
        "extra_vars": {"service_name": "nginx"}
      }},
      {"id": "notification_1", "type": "notification", "config": {
        "channel_ids": ["YOUR_CHANNEL_UUID"]
      }},
      {"id": "end_1", "type": "end", "config": {}}
    ],
    "edges": [
      {"source": "start_1", "target": "host_extractor_1"},
      {"source": "host_extractor_1", "target": "cmdb_validator_1"},
      {"source": "cmdb_validator_1", "target": "approval_1"},
      {"source": "approval_1", "target": "execution_1"},
      {"source": "execution_1", "target": "notification_1"},
      {"source": "notification_1", "target": "end_1"}
    ]
  }'
```

---

### 示例 2: 带条件分支的流程

流程路径：
```
start → host_extractor → set_variable → cmdb_validator → execution → condition
                                                                        ├─ (成功) → notify_success → end_success  
                                                                        └─ (失败) → notify_failed → end_failed
```

**流程定义**：
```json
{
  "name": "带条件分支的自愈流程",
  "is_active": true,
  "nodes": [
    {"id": "start_1", "type": "start", "config": {}},
    {"id": "host_extractor_1", "type": "host_extractor", "config": {"source_field": "raw_data.cmdb_ci"}},
    {"id": "set_var_1", "type": "set_variable", "config": {
      "variables": {"is_low_risk": false, "env": "production"}
    }},
    {"id": "cmdb_validator_1", "type": "cmdb_validator", "config": {}},
    {"id": "execution_1", "type": "execution", "config": {
      "git_repo_id": "YOUR_GIT_REPO_UUID",
      "secrets_source_id": "YOUR_SECRETS_SOURCE_UUID"
    }},
    {"id": "condition_1", "type": "condition", "config": {
      "conditions": [
        {"expression": "execution_result.status == 'completed'", "target": "notify_success"},
        {"expression": "execution_result.exit_code != 0", "target": "notify_failed"}
      ],
      "default_target": "notify_failed"
    }},
    {"id": "notify_success", "type": "notification", "config": {"channel_ids": ["..."]}},
    {"id": "notify_failed", "type": "notification", "config": {"channel_ids": ["..."]}},
    {"id": "end_success", "type": "end", "config": {}},
    {"id": "end_failed", "type": "end", "config": {}}
  ],
  "edges": [
    {"source": "start_1", "target": "host_extractor_1"},
    {"source": "host_extractor_1", "target": "set_var_1"},
    {"source": "set_var_1", "target": "cmdb_validator_1"},
    {"source": "cmdb_validator_1", "target": "execution_1"},
    {"source": "execution_1", "target": "condition_1"},
    {"source": "notify_success", "target": "end_success"},
    {"source": "notify_failed", "target": "end_failed"}
  ]
}
```

---

### 示例 3: 使用 set_variable 控制分支

根据自定义变量 `is_low_risk` 决定是否跳过审批：

```json
{
  "nodes": [
    {"id": "set_var_1", "type": "set_variable", "config": {
      "variables": {"is_low_risk": true}
    }},
    {"id": "condition_1", "type": "condition", "config": {
      "conditions": [
        {"expression": "is_low_risk == true", "target": "execution_1"},
        {"expression": "is_low_risk == false", "target": "approval_1"}
      ],
      "default_target": "approval_1"
    }}
  ]
}
```

---

## 附录：前置资源准备

创建自愈流程前，需要准备以下资源：

| 资源 | API | 说明 |
|------|-----|------|
| ITSM 插件 | `POST /api/v1/plugins` | 对接 ITSM 系统 |
| CMDB 插件 | `POST /api/v1/plugins` | 同步 CMDB 数据 |
| 密钥源 | `POST /api/v1/secrets-sources` | SSH 凭证管理 |
| Git 仓库 | `POST /api/v1/git-repos` | Playbook 仓库 |
| 通知渠道 | `POST /api/v1/channels` | Webhook/钉钉/邮件 |
| 通知模板 | `POST /api/v1/templates` | 可选，自定义消息格式 |

参见 [API 测试指南](api-testing-guide.md) 获取详细步骤。
