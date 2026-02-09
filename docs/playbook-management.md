# Playbook 模板管理

## 概述

Playbook 模板模块用于管理 Ansible Playbook 执行模板。每个 Playbook 模板关联一个 Git 仓库中的入口文件，系统自动扫描并管理其中的变量，供执行任务时使用。

---

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Git 仓库                                  │
│  (一个远程仓库 = 一个 Git 仓库配置)                              │
│                                                                 │
│  ├── site.yml           ← Playbook 模板 A                       │
│  ├── maintenance.yml    ← Playbook 模板 B                       │
│  ├── roles/                                                     │
│  │   ├── cleanup/                                               │
│  │   └── restart/                                               │
│  └── .auto-healing.yml  ← 增强模式配置文件（可选）               │
└─────────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────────┐
│                     Playbook 模板                                │
│  (每个入口文件 = 一个 Playbook 模板)                             │
│                                                                 │
│  ┌──────────────────────────┐                                   │
│  │ 主站点日志清理             │                                   │
│  │ config_mode: auto        │  ← 扫描模式（创建时指定）          │
│  │ status: ready            │  ← 状态                           │
│  │ variables: [...]         │  ← 扫描到的变量                    │
│  │ last_scanned_at: ...     │  ← 最后扫描时间                    │
│  └──────────────────────────┘                                   │
└─────────────────────────────────────────────────────────────────┘
```

### Git 仓库与 Playbook 的关系

| 关系 | 说明 |
|------|------|
| **1:N** | 一个 Git 仓库可以包含多个 Playbook 模板 |
| **依赖** | Playbook 必须关联一个已同步的 Git 仓库 |
| **同步** | Git 仓库同步后，**仅对已手动扫描过的 Playbook** 自动重新扫描变量 |

---

## 生命周期流程

```
                    ┌────────────────┐
                    │   创建 Playbook │
                    │ POST /playbooks │
                    │                 │
                    │ 必填：          │
                    │ - repository_id │
                    │ - name          │
                    │ - file_path     │
                    │ - config_mode   │ ← 这时选择 auto/enhanced
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │ status=pending │
                    │ (待扫描)        │
                    └────────┬───────┘
                             │
                             ▼
                    ┌────────────────┐
                    │   手动扫描      │
                    │POST /{id}/scan │
                    └────────┬───────┘
                             ▼
                    ┌────────────────┐
                    │ status=scanned │
                    │ (已扫描,待上线) │
                    └────────┬───────┘
                             │
              ┌──────────────┴──────────────┐
              │                              │
              ▼                              ▼
     ┌─────────────────┐           ┌─────────────────┐
     │ 编辑变量（可选） │           │ 直接上线         │
     │PUT /{id}/variables│          │                 │
     └────────┬────────┘           │                 │
              │                     │                 │
              └──────────┬─────────┘                 │
                         │                            │
                         ▼                            │
                ┌─────────────────┐                   │
                │    上线          │◄─────────────────┘
                │ POST /{id}/ready│
                └────────┬────────┘
                         │
                         ▼
                ┌─────────────────┐
                │ status=ready    │
                │ (可执行任务)     │
                └────────┬────────┘
                         │
          ┌──────────────┴──────────────┐
          │                              │
          ▼                              ▼
   ┌─────────────────┐           ┌─────────────────┐
   │ 执行任务使用     │           │ 下线（维护）     │
   │ POST /execution │           │POST /{id}/offline│
   │ -tasks          │           │ → status=pending │
   └─────────────────┘           └─────────────────┘
```

---

## Playbook 状态

| 状态 | 说明 | 可被执行任务选择 |
|------|------|------------------|
| `pending` | 刚创建，待扫描 | ❌ |
| `scanned` | 已扫描，待上线 | ❌ |
| `ready` | 已上线，可用 | ✅ |
| `error` | 扫描出错 | ❌ |
| `invalid` | 入口文件不存在 | ❌ |

---

## 扫描模式（config_mode）

| 模式 | 说明 | 何时使用 |
|------|------|---------|
| `auto` | 自动模式：从代码智能扫描变量 | 普通场景 |
| `enhanced` | 增强模式：结合 `.auto-healing.yml` | 需要更精细控制 |

> ⚠️ **重要**：`config_mode` 在**创建时必须指定**，之后**不可更改**。

---

## Git 同步后的自动扫描

```
Git 仓库定时同步
       ↓
同步成功（检测到代码变更）
       ↓
检查关联的所有 Playbook
       ↓
┌──────────────────────────────────────────┐
│ 对于每个 Playbook：                        │
│                                           │
│ IF last_scanned_at != null               │
│ (用户已手动扫描过)                         │
│    → 自动触发扫描                          │
│    → trigger_type = 'repo_sync'          │
│                                           │
│ ELSE                                      │
│ (用户从未扫描过)                           │
│    → 不做任何操作                          │
│    → 等待用户手动扫描                      │
└──────────────────────────────────────────┘
---

## 变量扫描机制

### 扫描流程

```
入口文件 (site.yml)
    │
    ├── 解析 YAML 结构，提取 {{ var }} 变量引用
    │
    ├── 递归扫描 include_tasks / import_tasks
    │   └── tasks/*.yml
    │
    ├── 递归扫描 vars_files（支持动态路径）
    │   └── vars/*.yml（如 vars/{{ env }}.yml → 扫描 vars/ 下所有 .yml）
    │
    ├── 递归扫描 roles（所有官方标准目录）
    │   ├── tasks/main.yml
    │   ├── handlers/main.yml
    │   ├── vars/main.yml
    │   ├── defaults/main.yml
    │   ├── files/*.yml, *.j2
    │   ├── templates/*.j2
    │   └── meta/main.yml
    │
    └── 扫描 template 模块引用的 .j2 文件
        └── 提取 {{ var }} 变量引用
```

### 支持的文件类型

| 扩展名 | 扫描方式 |
|--------|---------|
| `.yml` / `.yaml` | 解析 YAML 结构 + 提取 `{{ var }}` |
| `.j2` | 提取 `{{ var }}` 变量引用 |

### 动态路径处理

```yaml
# 动态 vars_files
vars_files:
  - vars/common.yml           # 静态：直接扫描
  - "vars/{{ env }}.yml"      # 动态：扫描 vars/ 下所有 .yml
```

> ⚠️ **动态 include_tasks** 无法静态分析，需在增强配置中手动声明变量。

### Ansible Role 标准目录

| 目录 | 用途 | 扫描文件类型 |
|------|------|-------------|
| `tasks/` | 任务 | .yml |
| `handlers/` | 处理器 | .yml |
| `vars/` | 变量 | .yml |
| `defaults/` | 默认变量 | .yml |
| `files/` | 静态文件 | .yml, .j2 |
| `templates/` | Jinja2 模板 | .j2 |
| `meta/` | 元数据 | .yml |

---

## API 接口

### 基础路径
```
/api/v1/playbooks
```

### 权限要求
| 操作 | 权限 |
|------|------|
| 查看列表/详情 | `playbook:list` |
| 创建 | `playbook:create` |
| 更新/扫描 | `playbook:update` |
| 删除 | `playbook:delete` |

---

## 1. 创建 Playbook 模板

```http
POST /api/v1/playbooks
Content-Type: application/json
```

**请求体:**
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `repository_id` | UUID | ✅ | 关联的 Git 仓库 ID |
| `name` | string | ✅ | 模板名称（全局唯一） |
| `file_path` | string | ✅ | 入口文件相对路径，如 `site.yml` |
| `config_mode` | string | ✅ | 扫描模式：`auto` 或 `enhanced` |
| `description` | string | ❌ | 描述 |

> ⚠️ **重要**：`config_mode` 创建时必须指定，之后不可更改。

**请求示例:**
```json
{
  "repository_id": "149a257d-022b-40b1-b1bd-f73ba4880742",
  "name": "主站点日志清理",
  "file_path": "site.yml",
  "config_mode": "auto",
  "description": "用于清理服务器日志和重启服务"
}
```

**响应:**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "7d00236b-165e-4a64-a4d2-8b3dd6965a02",
    "repository_id": "149a257d-022b-40b1-b1bd-f73ba4880742",
    "name": "主站点日志清理",
    "file_path": "site.yml",
    "config_mode": "auto",
    "status": "pending",
    "variables": [],
    "created_at": "2026-01-08T10:00:00Z"
  }
}
```

> **注意**：创建后状态为 `pending`，需要手动调用扫描接口扫描变量。

---

## 2. 获取 Playbook 列表

```http
GET /api/v1/playbooks?repository_id={id}&status={status}&page={page}&page_size={size}
```

**查询参数:**
| 参数 | 类型 | 说明 |
|------|------|------|
| `repository_id` | UUID | 按 Git 仓库筛选 |
| `status` | string | 按状态筛选: `pending`, `ready`, `error` |
| `page` | int | 页码，默认 1 |
| `page_size` | int | 每页数量，默认 20 |

**响应:**
```json
{
  "code": 0,
  "data": [
    {
      "id": "7d00236b-165e-4a64-a4d2-8b3dd6965a02",
      "name": "主站点日志清理",
      "file_path": "site.yml",
      "status": "ready",
      "variables_count": 82
    }
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 1
  }
}
```

---

## 3. 获取 Playbook 详情

```http
GET /api/v1/playbooks/{id}
```

**响应:**
```json
{
  "code": 0,
  "data": {
    "id": "7d00236b-165e-4a64-a4d2-8b3dd6965a02",
    "repository_id": "149a257d-022b-40b1-b1bd-f73ba4880742",
    "name": "主站点日志清理",
    "file_path": "site.yml",
    "description": "用于清理服务器日志和重启服务",
    "status": "ready",
    "variables": [
      {
        "name": "memory_threshold",
        "type": "number",
        "required": false,
        "default": "{{ threshold | default(90) }}",
        "description": "",
        "sources": [
          {"file": "site.yml", "line": 0},
          {"file": "roles/cleanup/tasks/main.yml", "line": 0}
        ],
        "primary_source": "site.yml",
        "in_code": true
      }
    ],
    "last_scan_at": "2026-01-08T11:00:00Z",
    "created_at": "2026-01-08T10:00:00Z"
  }
}
```

---

## 4. 更新 Playbook

```http
PUT /api/v1/playbooks/{id}
Content-Type: application/json
```

**请求体:**
| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 模板名称 |
| `description` | string | 描述 |

---

## 5. 删除 Playbook

```http
DELETE /api/v1/playbooks/{id}
```

> [!IMPORTANT]
> **保护性删除**：如果该 Playbook 下有关联的任务模板，将返回错误，需先删除任务模板。

**可能的错误响应：**
```json
{
  "code": 400,
  "message": "无法删除：该 Playbook 下有 3 个任务模板，请先删除关联的任务模板"
}
```

> ⚠️ 成功删除 Playbook 会同时删除关联的扫描日志

---

## 6. 扫描变量

```http
POST /api/v1/playbooks/{id}/scan
```

**功能说明:**
- 递归扫描 Playbook 及其引用的所有文件（roles、include_tasks 等）
- 智能推断变量类型
- 支持增强模式（.auto-healing.yml）
- 生成扫描日志

**响应:**
```json
{
  "code": 0,
  "data": {
    "id": "scan-log-uuid",
    "playbook_id": "7d00236b-165e-4a64-a4d2-8b3dd6965a02",
    "trigger_type": "manual",
    "files_scanned": 9,
    "variables_found": 82,
    "variables_added": 5,
    "variables_removed": 0,
    "created_at": "2026-01-08T11:00:00Z"
  }
}
```

> [!NOTE]
> 扫描完成后，如果变量发生变更，系统会自动标记关联的任务模板为 `needs_review = true`。

---

## 7. 更新变量配置

```http
PUT /api/v1/playbooks/{id}/variables
Content-Type: application/json
```

**用途:** 手动修改变量的 `required`、`description`、`default` 等配置

**请求体:**
```json
{
  "variables": [
    {
      "name": "memory_threshold",
      "type": "number",
      "required": true,
      "description": "内存使用率阈值（百分比）",
      "default": 90
    }
  ]
}
```

> 注意：手动配置会与自动扫描结果合并，手动配置优先

> [!WARNING]
> **变量变更影响关联任务模板**
> 
> 当 Playbook 变量发生变更（扫描或手动修改）后，系统会自动检测关联的任务模板：
> - 任务模板的 `needs_review` 会被标记为 `true`
> - `changed_variables` 会记录变更的变量名列表
> - **需要审核后才能执行**（手动/调度/自愈执行都会被阻止）
> 
> 用户需要在任务模板页面确认变更后，才能继续执行任务。

---

## 8. 上线（Ready）

```http
POST /api/v1/playbooks/{id}/ready
```

设置 Playbook 状态为 `ready`，表示可以用于创建执行任务。

**状态变更:** `pending` → `ready`

---

## 9. 下线（Offline）

```http
POST /api/v1/playbooks/{id}/offline
```

设置 Playbook 状态为 `pending`，下线后无法被执行任务使用。

**状态变更:** `ready` → `pending`

**使用场景:**
- Playbook 需要维护或修改时
- 临时禁止执行某个 Playbook

---

## 10. 获取文件列表

```http
GET /api/v1/playbooks/{id}/files
```

获取 Playbook 扫描过的所有文件列表（入口文件 + 引用的所有文件）。

**响应:**
```json
{
  "code": 0,
  "data": {
    "files": [
      {"path": "site.yml", "type": "entry"},
      {"path": "roles/cleanup/tasks/main.yml", "type": "task"},
      {"path": "roles/cleanup/defaults/main.yml", "type": "defaults"},
      {"path": "roles/cleanup/vars/main.yml", "type": "vars"},
      {"path": "roles/cleanup/handlers/main.yml", "type": "handlers"}
    ]
  }
}
```

**文件类型说明:**

| type | 说明 |
|------|------|
| `entry` | 入口文件 |
| `task` | tasks 目录下的任务文件 |
| `vars` | vars 目录下的变量文件 |
| `defaults` | defaults 目录下的默认变量文件 |
| `handlers` | handlers 目录下的处理器文件 |
| `template` | templates 目录下的模板文件 |
| `file` | files 目录下的静态文件 |
| `role` | role 相关文件 |
| `include` | include_tasks 引入的文件 |

---

## 11. 获取扫描日志

```http
GET /api/v1/playbooks/{id}/scan-logs?page={page}&page_size={size}
```

**响应:**
```json
{
  "code": 0,
  "data": [
    {
      "id": "scan-log-uuid",
      "trigger_type": "manual",
      "files_scanned": 9,
      "variables_found": 82,
      "variables_added": 5,
      "variables_removed": 0,
      "created_at": "2026-01-08T11:00:00Z"
    }
  ],
  "pagination": {...}
}
```

---

## 变量扫描详解

### 扫描流程

```
┌─────────────────────────────────────────────────────────────────┐
│                       扫描流程                                   │
└─────────────────────────────────────────────────────────────────┘

1. 检测增强配置
   └─> 解析 .auto-healing.yml（如果存在）

2. 递归扫描文件
   ├─> scanYAMLStructure: 扫描 vars 块
   ├─> scanVariableReferences: 扫描 {{ var }} 引用
   └─> scanIncludes: 递归扫描 include_tasks/roles

3. 智能类型推断
   ├─> inferTypeSmartly()
   │   ├─> 优先级1: 直接值类型
   │   ├─> 优先级2: Jinja2 default 表达式
   │   ├─> 优先级3: 变量名启发式
   │   └─> 优先级4: 默认 string
   └─> parseJinja2Default(): 解析 {{ var | default(90) }}

4. 多来源记录
   └─> addVariable(): 记录所有出现位置

5. 合并配置
   └─> mergeVariables(): 用户配置 + 新扫描结果
```

### 类型推断优先级

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1️⃣ | **增强模式** | `.auto-healing.yml` 中明确定义 |
| 2️⃣ | **Jinja2 default** | `{{ var \| default(90) }}` → number |
| 3️⃣ | **变量名启发式** | `*_timeout` → number, `is_*` → boolean |
| 4️⃣ | **默认** | string |

### 变量名启发式规则

**Boolean 类型:**
- 前缀: `is_`, `has_`, `can_`, `should_`, `enable_`, `disable_`, `use_`
- 完全匹配: `enabled`, `disabled`, `force`, `verbose`, `debug`, `compress`

**Number 类型:**
- 后缀: `_threshold`, `_count`, `_timeout`, `_port`, `_size`, `_limit`, `_max`, `_min`, `_interval`, `_retries`, `_percent`

**List 类型:**
- 后缀: `_hosts`, `_dirs`, `_files`, `_paths`, `_list`, `_items`, `_servers`

### 多来源记录

当同一变量在多个文件中出现时：

```json
{
  "name": "log_path",
  "type": "string",
  "sources": [
    {"file": "site.yml", "line": 0},
    {"file": "roles/log_rotation/tasks/main.yml", "line": 0},
    {"file": "roles/log_rotation/tasks/main.yml", "line": 0}
  ],
  "primary_source": "site.yml"
}
```

**主来源确定规则:**
1. 增强模式定义了此变量 → `.auto-healing.yml`
2. 代码中有 Jinja2 default → 第一个有 default 的位置
3. 都没有 → 第一次扫描到的位置

### 合并规则

| 字段 | 优先级规则 |
|------|-----------|
| **type** | 增强模式 > 新扫描精确类型 > 用户配置 > 默认 |
| **description** | 增强模式 > 用户配置 > 空 |
| **required** | 增强模式 > 用户配置 > false |
| **default** | 增强模式 > 用户配置 > 代码扫描 |

---

## 增强模式

在仓库根目录创建 `.auto-healing.yml` 文件：

```yaml
# .auto-healing.yml
variables:
  - name: memory_threshold
    type: number
    required: true
    default: 90
    description: "内存使用率阈值（百分比）"
    min: 50
    max: 100

  - name: environment
    type: enum
    enum: ["dev", "staging", "prod"]
    default: "prod"
    description: "目标环境"

  - name: admin_password
    type: password
    required: true
    description: "管理员密码（不会在日志中显示）"
```

### 增强模式独有功能

| 功能 | 说明 |
|------|------|
| `enum` 类型 | 下拉选择 |
| `password` 类型 | 密码输入框 |
| `min` / `max` | 数值范围限制 |
| `pattern` | 正则表达式校验 |
| `description` | 中文描述 |

---

## 使用流程

### 典型工作流程

```
1. 创建 Git 仓库
   POST /api/v1/git-repos
        ↓
2. 同步 Git 仓库
   POST /api/v1/git-repos/{id}/sync
        ↓
3. 创建 Playbook 模板
   POST /api/v1/playbooks
   {
     "repository_id": "...",
     "name": "日志清理",
     "file_path": "site.yml"
   }
        ↓
4. 扫描变量
   POST /api/v1/playbooks/{id}/scan
        ↓
5. （可选）手动配置变量
   PUT /api/v1/playbooks/{id}/variables
        ↓
6. 设置为 Ready
   POST /api/v1/playbooks/{id}/ready
        ↓
7. 创建执行任务
   POST /api/v1/execution-tasks
   {
     "playbook_id": "...",
     "variables": {"memory_threshold": 85}
   }
```

### 自动同步流程

```
Git 仓库定时同步
       ↓
检测到代码变更
       ↓
自动重新扫描关联的 Playbook
       ↓
合并变量配置
       ↓
记录扫描日志
```

---

## 变量字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 变量名 |
| `type` | string | 类型: `string`, `number`, `boolean`, `list`, `object`, `enum`, `password` |
| `required` | boolean | 是否必填 |
| `default` | any | 默认值 |
| `description` | string | 描述 |
| `sources` | array | 所有来源位置 |
| `primary_source` | string | 主来源文件 |
| `in_code` | boolean | 是否在代码中存在 |
| `enum` | array | 枚举值（type=enum） |
| `min` | number | 最小值（type=number） |
| `max` | number | 最大值（type=number） |
| `pattern` | string | 正则校验（type=string） |

---

## 前端展示

| 类型 | 控件 |
|------|------|
| `string` | 文本输入框 |
| `number` | 数字输入框（可带 min/max） |
| `boolean` | 开关 |
| `list` | 标签输入 |
| `object` | JSON 编辑器 |
| `enum` | 下拉选择框 |
| `password` | 密码输入框 |

---

## 数据模型

### Playbook

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `repository_id` | UUID | 关联的 Git 仓库 |
| `name` | string | 模板名称（唯一） |
| `file_path` | string | 入口文件路径 |
| `description` | string | 描述 |
| `status` | string | `pending`, `ready`, `error` |
| `variables` | array | 变量配置 |
| `last_scan_at` | datetime | 最后扫描时间 |
| `created_at` | datetime | 创建时间 |
| `updated_at` | datetime | 更新时间 |

### PlaybookScanLog

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `playbook_id` | UUID | 关联的 Playbook |
| `trigger_type` | string | `manual`, `auto`, `sync` |
| `files_scanned` | int | 扫描的文件数 |
| `variables_found` | int | 发现的变量数 |
| `variables_added` | int | 新增的变量数 |
| `variables_removed` | int | 移除的变量数 |
| `created_at` | datetime | 扫描时间 |
