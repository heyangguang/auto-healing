# 执行任务管理 API 文档

**Base URL**: `/api/v1`  
**权限**: 已登录用户（租户内数据隔离）

---

## 任务模板（Execution Tasks）

**路径前缀**: `/api/v1/execution-tasks`

### 1. 获取任务模板列表

**GET** `/api/v1/execution-tasks`

**权限**: `task:list`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称、描述） |
| `executor_type` | string | ❌ | 执行器类型：`ansible` / `shell` / `python` |
| `status` | string | ❌ | 状态：`active` / `inactive` |
| `playbook_id` | uuid | ❌ | 按 Playbook 筛选 |
| `playbook_name` | string | ❌ | 按 Playbook 名称模糊筛选 |
| `repository_name` | string | ❌ | 按仓库名称模糊筛选 |
| `target_hosts` | string | ❌ | 按目标主机筛选 |
| `needs_review` | bool | ❌ | 是否需要审核：`true` / `false` |
| `has_runs` | bool | ❌ | 是否有执行记录 |
| `min_run_count` | int | ❌ | 最少执行次数 |
| `last_run_status` | string | ❌ | 最后执行状态：`success` / `failed` / `running` |
| `created_from` | string | ❌ | 创建时间起始（RFC3339） |
| `created_to` | string | ❌ | 创建时间结束（RFC3339） |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` / `run_count` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

#### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "name": "磁盘清理任务",
        "description": "清理 /tmp 目录下的临时文件",
        "executor_type": "ansible",
        "playbook_id": "uuid",
        "playbook": {
          "id": "uuid",
          "name": "磁盘清理",
          "file_path": "playbooks/disk_cleanup.yml"
        },
        "target_hosts": "prod-web-*",
        "target_type": "pattern",
        "extra_vars": {
          "target_path": "/tmp",
          "max_age_days": "30"
        },
        "secrets_source_ids": ["uuid1"],
        "needs_review": false,
        "status": "active",
        "run_count": 50,
        "last_run_at": "2026-02-18T09:00:00Z",
        "last_run_status": "success",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": "2026-02-18T10:00:00Z"
      }
    ],
    "total": 20,
    "page": 1,
    "page_size": 20
  }
}
```

---

### 2. 创建任务模板

**POST** `/api/v1/execution-tasks`

**权限**: `playbook:execute`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 任务名称 |
| `description` | string | ❌ | 描述 |
| `executor_type` | string | ✅ | 执行器类型：`ansible` |
| `playbook_id` | uuid | ✅ | Playbook ID |
| `target_hosts` | string | ✅ | 目标主机（IP、主机名或模式） |
| `target_type` | string | ❌ | 目标类型：`ip` / `hostname` / `pattern` / `cmdb_query` |
| `extra_vars` | object | ❌ | 额外变量（键值对） |
| `secrets_source_ids` | []uuid | ❌ | 密钥源 ID 列表 |
| `needs_review` | bool | ❌ | 是否需要审核，默认 false |
| `skip_notification` | bool | ❌ | 是否跳过通知，默认 false |

---

### 3. 获取任务模板统计

**GET** `/api/v1/execution-tasks/stats`

**权限**: `task:list`

#### 响应

```json
{
  "code": 0,
  "data": {
    "total": 20,
    "active": 18,
    "inactive": 2,
    "needs_review": 3,
    "total_runs": 500
  }
}
```

---

### 4. 批量确认审核

**POST** `/api/v1/execution-tasks/batch-confirm-review`

**权限**: `task:update`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `task_ids` | []uuid | ✅ | 任务 ID 列表 |

---

### 5. 获取任务模板详情

**GET** `/api/v1/execution-tasks/:id`

**权限**: `task:detail`

---

### 6. 更新任务模板

**PUT** `/api/v1/execution-tasks/:id`

**权限**: `task:update`

---

### 7. 删除任务模板

**DELETE** `/api/v1/execution-tasks/:id`

**权限**: `task:delete`

---

### 8. 执行任务

**POST** `/api/v1/execution-tasks/:id/execute`

**权限**: `playbook:execute`

#### 请求体（可选）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `triggered_by` | string | ❌ | 触发来源标识 |
| `secrets_source_ids` | []uuid | ❌ | 覆盖密钥源 |
| `extra_vars` | object | ❌ | 覆盖额外变量 |
| `target_hosts` | string | ❌ | 覆盖目标主机 |
| `skip_notification` | bool | ❌ | 是否跳过通知 |

---

### 9. 确认变量审核

**POST** `/api/v1/execution-tasks/:id/confirm-review`

**权限**: `task:update`

---

### 10. 获取任务的执行历史

**GET** `/api/v1/execution-tasks/:id/runs`

**权限**: `task:detail`

#### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，默认 1 |
| `page_size` | int | 每页数量，默认 20 |

---

## 执行记录（Execution Runs）

**路径前缀**: `/api/v1/execution-runs`

### 11. 获取所有执行记录

**GET** `/api/v1/execution-runs`

**权限**: `task:list`

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索 |
| `task_id` | uuid | ❌ | 按任务模板筛选 |
| `status` | string | ❌ | 状态：`pending` / `running` / `success` / `failed` / `cancelled` |
| `triggered_by` | string | ❌ | 触发来源 |
| `started_after` | string | ❌ | 开始时间起始（RFC3339） |
| `started_before` | string | ❌ | 开始时间结束（RFC3339） |

---

### 12. 获取执行记录统计

**GET** `/api/v1/execution-runs/stats`

**权限**: `task:list`

---

### 13. 获取执行趋势

**GET** `/api/v1/execution-runs/trend`

**权限**: `task:list`

#### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `days` | int | 统计天数，默认 7 |

---

### 14. 获取触发方式分布

**GET** `/api/v1/execution-runs/trigger-distribution`

**权限**: `task:list`

---

### 15. 获取失败率最高的任务

**GET** `/api/v1/execution-runs/top-failed`

**权限**: `task:list`

#### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回数量，默认 5 |

---

### 16. 获取最活跃的任务

**GET** `/api/v1/execution-runs/top-active`

**权限**: `task:list`

#### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回数量，默认 5 |

---

### 17. 获取执行记录详情

**GET** `/api/v1/execution-runs/:id`

**权限**: `task:detail`

#### 响应

```json
{
  "code": 0,
  "data": {
    "id": "uuid",
    "task_id": "uuid",
    "task": {"id": "uuid", "name": "磁盘清理任务"},
    "status": "success",
    "triggered_by": "manual:admin",
    "target_hosts": "prod-web-01",
    "extra_vars": {"target_path": "/tmp"},
    "exit_code": 0,
    "stats": {"ok": 5, "changed": 2, "failed": 0, "skipped": 1, "unreachable": 0},
    "started_at": "2026-02-18T09:00:00Z",
    "completed_at": "2026-02-18T09:00:30Z",
    "duration_ms": 30000
  }
}
```

---

### 18. 获取执行日志

**GET** `/api/v1/execution-runs/:id/logs`

**权限**: `task:detail`

---

### 19. SSE 实时日志流

**GET** `/api/v1/execution-runs/:id/stream`

**权限**: `task:detail`

使用 Server-Sent Events（SSE）实时推送执行日志。

**响应格式（SSE）**:

```
event: log
data: {"sequence":1,"level":"info","message":"PLAY [all] *****","host":"prod-web-01"}

event: done
data: {"status":"success","exit_code":0,"stats":{"ok":5,"changed":2}}
```

---

### 20. 取消执行

**POST** `/api/v1/execution-runs/:id/cancel`

**权限**: `task:cancel`
