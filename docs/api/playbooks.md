# Playbook 管理 API 文档

**路径前缀**: `/api/v1/playbooks`  
**权限**: 已登录用户（租户内数据隔离）

> Playbook 是从 Git 仓库扫描出的 Ansible Playbook 文件，支持变量扫描和生命周期管理。

---

## 1. 获取 Playbook 列表

**GET** `/api/v1/playbooks`

**权限**: `playbook:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称、文件路径） |
| `repository_id` | uuid | ❌ | 按仓库筛选 |
| `name` | string | ❌ | 按名称精确筛选 |
| `file_path` | string | ❌ | 按文件路径筛选 |
| `status` | string | ❌ | 状态：`pending` / `ready` / `error` |
| `config_mode` | string | ❌ | 配置模式：`auto` / `enhanced` |
| `has_variables` | bool | ❌ | 是否有变量：`true` / `false` |
| `min_variables` | int | ❌ | 最少变量数 |
| `max_variables` | int | ❌ | 最多变量数 |
| `has_required_vars` | bool | ❌ | 是否有必填变量 |
| `created_from` | string | ❌ | 创建时间起始（RFC3339） |
| `created_to` | string | ❌ | 创建时间结束（RFC3339） |
| `sort_by` | string | ❌ | 排序字段：`name` / `created_at` / `updated_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

## 2. 创建 Playbook

**POST** `/api/v1/playbooks`

**权限**: `playbook:create`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `repository_id` | uuid | ✅ | Git 仓库 ID |
| `name` | string | ✅ | Playbook 名称 |
| `file_path` | string | ✅ | 仓库内文件路径 |
| `description` | string | ❌ | 描述 |
| `config_mode` | string | ✅ | 配置模式：`auto`（自动扫描）/ `enhanced`（增强配置） |

---

## 3. 获取 Playbook 统计

**GET** `/api/v1/playbooks/stats`

**权限**: `playbook:list`

### 响应

```json
{
  "code": 0,
  "data": {
    "total": 25,
    "ready": 20,
    "pending": 3,
    "error": 2,
    "by_config_mode": {"auto": 15, "enhanced": 10}
  }
}
```

---

## 4. 获取 Playbook 详情

**GET** `/api/v1/playbooks/:id`

**权限**: `playbook:list`

---

## 5. 更新 Playbook

**PUT** `/api/v1/playbooks/:id`

**权限**: `playbook:update`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ❌ | 名称 |
| `description` | string | ❌ | 描述 |

---

## 6. 删除 Playbook

**DELETE** `/api/v1/playbooks/:id`

**权限**: `playbook:delete`

---

## 7. 手动触发变量扫描

**POST** `/api/v1/playbooks/:id/scan`

**权限**: `playbook:update`

从入口 Playbook 递归扫描变量定义，并跟踪其依赖文件。

扫描顺序说明：

1. 入口文件本身
2. `import_playbook` / `include_tasks` / `import_tasks`
3. `roles` / `include_role` / `import_role`
4. `vars_files` / `template` 等依赖
5. 仓库根目录的 `.auto-healing.yml` 或 `.auto-healing.yaml`

增强配置说明：

- `.auto-healing.yml` 可为变量补充类型、描述、默认值、枚举等信息
- 支持 `exposure_mode: scoped`
- 当 `exposure_mode=scoped` 时，可用 `variables[].playbooks` 指定变量只暴露给哪些入口文件
- 未命中的增强变量不会暴露到该入口 Playbook 的可填写参数里

---

## 8. 更新变量配置

**PUT** `/api/v1/playbooks/:id/variables`

**权限**: `playbook:update`

手动配置 Playbook 的变量定义（`enhanced` 模式下使用）。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `variables` | []object | ✅ | 变量配置列表 |

```json
{
  "variables": [
    {
      "name": "target_path",
      "description": "清理目标路径",
      "type": "string",
      "required": true,
      "default": "/tmp",
      "options": []
    }
  ]
}
```

### 变量对象字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 变量名 |
| `description` | string | 变量描述 |
| `type` | string | 类型：`string` / `integer` / `boolean` / `list` / `select` |
| `required` | bool | 是否必填 |
| `default` | string | 默认值 |
| `options` | []string | 可选值列表（`select` 类型时使用） |

### `.auto-healing.yml` 示例

```yaml
exposure_mode: scoped

variables:
  - name: lab_script_path
    type: string
    required: true
    default: /opt/auto-healing-fault-lab/auto_healing_fault_lab.sh
    description: 远端故障实验脚本路径
    playbooks:
      - playbooks/fault_recovery_suite.yml
      - playbooks/service_down_recover.yml

  - name: fault_type
    type: select
    required: true
    enum:
      - service_down
      - cpu_high
      - disk_full
    playbooks:
      - playbooks/fault_recovery_suite.yml
```

字段说明：

- `exposure_mode=scoped`
  只暴露增强配置中明确声明且命中当前入口文件的变量
- `variables[].playbooks`
  入口文件白名单，使用仓库内相对路径
- 不写 `playbooks`
  默认对所有入口文件可见

---

## 9. 设置为 Ready 状态

**POST** `/api/v1/playbooks/:id/ready`

**权限**: `playbook:update`

将 Playbook 从 `pending` 状态切换为 `ready`，使其可被执行任务引用。

---

## 10. 下线 Playbook

**POST** `/api/v1/playbooks/:id/offline`

**权限**: `playbook:update`

将 Playbook 切换为 `pending` 状态（下线）。

---

## 11. 获取 Playbook 依赖文件列表

**GET** `/api/v1/playbooks/:id/files`

**权限**: `playbook:list`

获取该 Playbook 扫描到的入口文件和依赖文件。

返回规则：

- `relation=entry` 表示当前 Playbook 的入口文件
- `relation=dependency` 表示递归扫描到的依赖文件
- `type` 用于区分文件类型，例如 `playbook`、`task`、`template`、`include`

这可以帮助用户明确看到：

- 哪个文件是主入口
- 这个 Playbook 实际依赖了哪些 task/role/include 文件

### 响应示例

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "path": "playbooks/service_down_recover.yml",
      "type": "playbook",
      "relation": "entry"
    },
    {
      "path": "playbooks/fault_recovery_suite.yml",
      "type": "include",
      "relation": "dependency"
    },
    {
      "path": "playbooks/roles/fault_lab_service/tasks/main.yml",
      "type": "task",
      "relation": "dependency"
    }
  ]
}
```

---

## 12. 获取扫描日志

**GET** `/api/v1/playbooks/:id/scan-logs`

**权限**: `playbook:list`

---

## Playbook 生命周期

```
pending → ready → (offline) → pending
```

| 状态 | 说明 |
|------|------|
| `pending` | 待配置，不可被执行任务引用 |
| `ready` | 就绪，可被执行任务引用 |
| `error` | 扫描或配置出错 |
