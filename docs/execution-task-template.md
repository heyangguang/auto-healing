# 任务模板管理

## 概述

任务模板是对 Playbook 模板的预设执行配置，包含目标主机、变量值等，用于快速复用执行。

> [!NOTE]
> 定时任务配置已移至独立的 [定时调度](execution-schedule.md) 模块管理。

---

## 模块关系

```
Playbook 模板
    │ 1:N
    ▼
任务模板（预设执行参数）
    │         │
    │ 1:N     │ 1:N
    ▼         ▼
定时调度     执行记录
(Schedule)   (Run)
```

---

## 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 模板 ID |
| `name` | string | 模板名称 |
| `playbook_id` | UUID | 关联的 Playbook |
| `target_hosts` | string | 目标主机 |
| `extra_vars` | object | 预设变量值 |
| `executor_type` | string | 执行器：`local` / `docker` |
| `description` | string | 任务描述 |
| `secrets_source_ids` | array | 关联的密钥源 ID 列表 |
| `notification_config` | object | 通知配置 |
| `playbook_variables_snapshot` | array | Playbook 变量快照 |
| `needs_review` | boolean | 是否需要审核（Playbook 变量变更后自动标记） |
| `changed_variables` | array | 变更的变量名列表 |

---

## API 接口

### 基础路径
```
/api/v1/execution-tasks
```

---

## 1. 创建任务模板

```http
POST /api/v1/execution-tasks
```

**请求体:**
```json
{
  "name": "生产环境日志清理",
  "playbook_id": "uuid-xxx",
  "target_hosts": "192.168.1.10,192.168.1.11",
  "extra_vars": {
    "cleanup_threshold": 85,
    "log_retention": 7
  },
  "executor_type": "docker",
  "description": "每日清理日志文件",
  "secrets_source_ids": ["uuid-secret-1"],
  "notification_config": {
    "enabled": true,
    "on_start": {
      "enabled": true,
      "channel_ids": ["钉钉渠道ID"],
      "template_id": "开始通知模板ID"
    },
    "on_success": {
      "enabled": true,
      "channel_ids": ["邮件渠道ID"],
      "template_id": "成功模板ID"
    },
    "on_failure": {
      "enabled": true,
      "channel_ids": ["钉钉渠道ID", "邮件渠道ID"],
      "template_id": "失败告警模板ID"
    }
  }
}
```

---

## 2. 获取任务模板列表

```http
GET /api/v1/execution-tasks
```

### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | integer | 页码，默认 1 |
| `page_size` | integer | 每页条数，默认 20 |
| `search` | string | 模糊搜索（匹配任务名称或描述） |
| `target_hosts` | string | 模糊搜索（匹配目标主机） |
| `executor_type` | string | 执行器类型筛选：`local` / `docker` |
| `status` | string | 状态筛选：`pending_review`（需审核）/ `ready`（就绪） |
| `playbook_id` | UUID | 关联的 Playbook ID |
| `playbook_name` | string | 模糊搜索（关联的 Playbook 名称） |
| `repository_name` | string | 模糊搜索（关联的仓库名称） |
| `has_runs` | boolean | 筛选有执行记录（true）或无执行记录（false）的任务 |
| `min_run_count` | integer | 最小执行次数，筛选 run_count >= N 的任务 |
| `last_run_status` | string | 按最后执行状态筛选：`pending` / `running` / `success` / `failed` / `cancelled` / `timeout` |

### 示例

```bash
# 基础分页
GET /api/v1/execution-tasks?page=1&page_size=20

# 搜索任务名称
GET /api/v1/execution-tasks?search=日志清理

# 筛选 Docker 执行器
GET /api/v1/execution-tasks?executor_type=docker

# 筛选需要审核的任务
GET /api/v1/execution-tasks?status=pending_review

# 组合筛选
GET /api/v1/execution-tasks?executor_type=local&status=ready&search=nginx
```

---

## 3. 获取任务模板详情

```http
GET /api/v1/execution-tasks/{id}
```

---

## 4. 更新任务模板

```http
PUT /api/v1/execution-tasks/{id}
```

**请求体:**
```json
{
  "name": "更新后的名称",
  "playbook_id": "uuid-xxx",
  "target_hosts": "192.168.1.20,192.168.1.21",
  "description": "任务描述文本",
  "secrets_source_ids": ["sec-001", "sec-002"],
  "extra_vars": {
    "cleanup_threshold": 90
  },
  "executor_type": "local"
}
```

---

## 5. 删除任务模板

```http
DELETE /api/v1/execution-tasks/{id}
```

> [!IMPORTANT]
> **保护性删除**：如果该任务模板下有关联的调度任务，将返回错误，需先删除调度任务。

**可能的错误响应：**
```json
{
  "code": 400,
  "message": "无法删除：该任务模板下有 2 个调度任务，请先删除关联的调度任务"
}
```

---

## 6. 执行任务

执行任务接口的详细说明请参考 [任务执行管理](execution-run.md)。

```http
POST /api/v1/execution-tasks/{id}/execute
```

**响应:**
```json
{
  "code": 0,
  "data": {
    "id": "run-xxx",
    "task_id": "task-xxx",
    "status": "pending"
  }
}
```

---

## 7. 获取执行历史

```http
GET /api/v1/execution-tasks/{id}/runs?page=1&page_size=20
```

---

## 通知配置

| 字段 | 类型 | 说明 |
|------|------|------|
| `channel_id` | UUID | 通知渠道 |
| `template_id` | UUID | 通知模板 |
| `on_success` | boolean | 成功时通知 |
| `on_failure` | boolean | 失败时通知 |
| `on_start` | boolean | 开始时通知 |

---

## Playbook 变量变更检测

当 Playbook 变量发生变更（扫描或手动修改）后，关联的任务模板会被标记为需要审核。

### 相关字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `playbook_variables_snapshot` | array | Playbook 变量快照（创建任务时复制） |
| `needs_review` | boolean | 是否需要审核（Playbook 变量已变更） |
| `changed_variables` | array | 变更的变量名列表 |

### 变更检测流程

```
1. 创建任务模板
   └─→ 保存 Playbook 当前变量到 playbook_variables_snapshot

2. Playbook 变量变更（扫描/手动修改）
   └─→ 对比关联任务的 snapshot 与最新变量
   └─→ 如有差异：设置 needs_review = true, changed_variables = [...]

3. 用户审核确认
   └─→ 调用 confirm-review 接口
   └─→ 更新 snapshot，清除 needs_review
```

### 执行阻止

> [!WARNING]
> 当 `needs_review = true` 时，任务模板**不能执行**：
> - 手动执行 `/execute` 返回错误
> - 定时调度执行被阻止
> - 自愈触发执行被阻止

---

## 8. 确认审核变量变更

```http
POST /api/v1/execution-tasks/{id}/confirm-review
```

**功能：**
- 清除 `needs_review` 状态
- 同步更新 `playbook_variables_snapshot` 为最新

**响应：**
```json
{
  "code": 0,
  "data": {
    "id": "task-xxx",
    "name": "任务名称",
    "needs_review": false,
    "changed_variables": []
  }
}
```
