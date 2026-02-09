# Playbook 变量配置指南

本文档面向 Playbook 开发人员，介绍如何编写 Playbook 以便系统能够正确识别和管理变量。

## 概述

系统支持两种变量配置模式：

| 模式 | 说明 | 适用场景 |
|------|------|----------|
| **自动模式** | 自动扫描 Playbook 代码，智能推断变量类型 | 大多数场景，快速接入 |
| **增强模式** | 通过 `.auto-healing.yml` 文件精确定义变量配置 | 需要精确控制时使用 |

---

## 自动模式

### 变量类型智能推断

系统会按以下优先级自动推断变量类型：

#### 1. Jinja2 default 表达式

推荐使用 Ansible 标准写法，系统会解析 `default()` 中的值来推断类型：

```yaml
vars:
  # 推断为 number（default 值为 90）
  memory_threshold: "{{ threshold | default(90) }}"
  
  # 推断为 boolean（default 值为 false）
  force_cleanup: "{{ force | default(false) }}"
  
  # 推断为 list（default 值为 []）
  target_servers: "{{ servers | default([]) }}"
  
  # 推断为 object（default 值为 {}）
  extra_config: "{{ config | default({}) }}"
```

| default 值示例 | 推断类型 |
|---------------|---------|
| `default(90)` | number |
| `default(3.14)` | number |
| `default(true)` / `default(false)` | boolean |
| `default([])` / `default(['a', 'b'])` | list |
| `default({})` / `default({"key": "val"})` | object |
| 其他 | string |

#### 2. 变量名启发式推断

如果没有 default 表达式，系统会根据变量名称推断类型：

**Boolean 类型**
```yaml
# 以下命名会被推断为 boolean
enabled: null
force: null
is_active: null
has_permission: null
enable_backup: null
compress_logs: null
verbose: null
debug: null
```

规则：
- 完整匹配：`enabled`, `disabled`, `force`, `verbose`, `debug`, `compress`, `allow`, `require`, `skip`, `dry_run`
- 前缀匹配：`is_`, `has_`, `can_`, `should_`, `enable_`, `disable_`, `use_`
- 后缀匹配：`_enabled`, `_disabled`, `_flag`, `_mode`

**Number 类型**
```yaml
# 以下命名会被推断为 number
memory_threshold: null
retry_count: null
connection_timeout: null
ssh_port: null
buffer_size: null
```

规则（后缀匹配）：
`_threshold`, `_count`, `_timeout`, `_port`, `_size`, `_limit`, `_max`, `_min`, `_interval`, `_retries`, `_delay`, `_seconds`, `_minutes`, `_hours`, `_days`, `_percent`, `_percentage`, `_rate`, `_number`

**List 类型**
```yaml
# 以下命名会被推断为 list
target_hosts: null
log_dirs: null
config_files: null
```

规则（后缀匹配）：
`_hosts`, `_dirs`, `_files`, `_paths`, `_list`, `_items`, `_servers`, `_nodes`, `_addresses`

---

## 增强模式

### 配置文件

在仓库根目录创建 `.auto-healing.yml` 文件（或 `playbook.meta.yml`）：

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

  - name: force_cleanup
    type: boolean
    default: false
    description: "是否强制清理缓存"

  - name: environment
    type: enum
    enum: ["dev", "staging", "prod"]
    default: "prod"
    description: "目标环境"

  - name: admin_password
    type: password
    required: true
    description: "管理员密码（不会在日志中显示）"

  - name: target_servers
    type: list
    description: "目标服务器列表"
```

### 字段说明

| 字段 | 必填 | 说明 |
|------|------|------|
| `name` | ✅ | 变量名称 |
| `type` | ❌ | 类型：`string`, `number`, `boolean`, `list`, `object`, `enum`, `password` |
| `required` | ❌ | 是否必填，默认 `false` |
| `default` | ❌ | 默认值 |
| `description` | ❌ | 变量描述（支持中文） |
| `enum` | ❌ | 枚举值列表（仅 type=enum 时有效） |
| `min` | ❌ | 最小值（仅 type=number 时有效） |
| `max` | ❌ | 最大值（仅 type=number 时有效） |
| `pattern` | ❌ | 正则校验（仅 type=string 时有效） |

### 增强模式独有功能

以下功能**只能通过增强模式实现**，自动模式无法推断：

| 功能 | 说明 |
|------|------|
| `enum` 类型 | 下拉选择，需要明确的枚举值列表 |
| `password` 类型 | 密码输入框，值不会在日志中显示 |
| `min` / `max` | 数值范围限制 |
| `pattern` | 正则表达式校验 |
| `description` | 中文描述 |
| `required` | 必填标记 |

---

## 最佳实践

### 1. 使用 default 过滤器

```yaml
# ✅ 推荐：系统能正确推断类型
memory_threshold: "{{ threshold | default(90) }}"

# ❌ 不推荐：系统只能推断为 string
memory_threshold: "{{ threshold }}"
```

### 2. 变量命名规范

```yaml
# ✅ 推荐：命名清晰，系统能正确推断
retry_count: 3
is_enabled: true
target_hosts: []

# ❌ 不推荐：命名模糊，需要增强模式
count: 3
flag: true
hosts: []
```

### 3. 复杂场景使用增强模式

```yaml
# 场景：需要下拉选择环境
# 解决方案：在 .auto-healing.yml 中定义
variables:
  - name: environment
    type: enum
    enum: ["dev", "staging", "prod"]
    description: "选择部署环境"
```

---

## 前端展示效果

| 类型 | 前端控件 |
|------|----------|
| `string` | 文本输入框 |
| `number` | 数字输入框 |
| `boolean` | 开关 |
| `list` | 标签输入 |
| `object` | JSON 编辑器 |
| `enum` | 下拉选择框 |
| `password` | 密码输入框 |

---

## 变量来源追踪

### 多来源记录

当同一变量在多个位置出现时，系统会记录所有来源：

```json
{
  "name": "threshold",
  "type": "number",
  "sources": [
    {"file": "site.yml", "line": 10},
    {"file": "roles/memory/tasks/main.yml", "line": 5},
    {"file": "roles/disk/tasks/main.yml", "line": 8}
  ],
  "primary_source": "roles/memory/tasks/main.yml",
  "in_code": true
}
```

### 主来源确定规则

| 优先级 | 条件 | 主来源 |
|--------|------|--------|
| 1️⃣ | 增强模式定义了此变量 | `.auto-healing.yml` |
| 2️⃣ | 代码中有 Jinja2 default | 第一个有 default 的位置 |
| 3️⃣ | 都没有 | 第一次扫描到的位置 |

---

## 类型推断优先级

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1️⃣ **增强模式** | `.auto-healing.yml` | 最高优先，明确定义的类型 |
| 2️⃣ **Jinja2 default** | `{{ var \| default(90) }}` | 从代码推断的精确类型 |
| 3️⃣ **变量名启发式** | `*_timeout`, `is_*` | 基于命名规则推断 |
| 4️⃣ **默认** | - | 兜底返回 `string` |

---

## 变量合并规则

当用户配置与新扫描结果合并时：

| 字段 | 优先级规则 |
|------|-----------|
| **type** | 增强模式 > 新扫描精确类型 > 用户配置 > 默认 |
| **description** | 增强模式 > 用户配置 > 空 |
| **required** | 增强模式 > 用户配置 > false |
| **default** | 增强模式 > 用户配置 > 代码扫描 |
| **enum/min/max/pattern** | 只有增强模式能定义 |

### 关键原则

1. **用户手动修改的 `required`、`description` 优先保留**
2. **类型可以被更精确的值覆盖**（string → number）
3. **增强模式定义的值始终最高优先**

### 示例流程

```
第一次扫描:
  扫描代码 → threshold: type=string (变量名推断)
  
第二次扫描:
  扫描代码 → threshold: type=number (Jinja2 default(90))
  合并: 发现 number 比 string 精确 → 更新为 number
  
用户修改:
  用户设置 → threshold: required=true, description="内存阈值"
  
第三次扫描:
  扫描代码 → threshold: type=number
  合并: 保留用户的 required 和 description，类型仍为 number
  
增强模式:
  .auto-healing.yml 定义 → threshold: type=number, min=0, max=100
  合并: 使用增强模式的 type/min/max，保留用户的 required/description
```
