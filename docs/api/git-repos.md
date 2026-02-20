# Git 仓库管理 API 文档

**路径前缀**: `/api/v1/git-repos`  
**权限**: 已登录用户（租户内数据隔离）

---

## 1. 验证仓库连接

**POST** `/api/v1/git-repos/validate`

**权限**: `plugin:list`

在创建仓库前验证 URL 和认证信息是否有效。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `url` | string | ✅ | 仓库 URL |
| `auth_type` | string | ❌ | 认证方式：`none` / `token` / `ssh_key` / `username_password` |
| `auth_config` | object | ❌ | 认证配置（JSON 对象，如 `{"token": "xxx"}` 或 `{"username": "x", "password": "x"}`） |

---

## 2. 获取仓库列表

**GET** `/api/v1/git-repos`

**权限**: `plugin:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（名称、URL） |
| `name` | string | ❌ | 按名称精确/模糊筛选 |
| `url` | string | ❌ | 按仓库 URL 筛选 |
| `status` | string | ❌ | 状态：`active` / `inactive` / `error` / `syncing` |
| `auth_type` | string | ❌ | 认证方式：`none` / `token` / `ssh_key` / `username_password` |
| `sync_enabled` | bool | ❌ | 是否启用自动同步：`true` / `false` |
| `created_from` | string | ❌ | 创建时间起始（RFC3339） |
| `created_to` | string | ❌ | 创建时间结束（RFC3339） |
| `sort_field` | string | ❌ | 排序字段：`name` / `created_at` / `last_sync_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

---

## 3. 创建仓库

**POST** `/api/v1/git-repos`

**权限**: `plugin:create`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 仓库名称 |
| `url` | string | ✅ | 仓库 URL（支持 HTTPS 和 SSH） |
| `default_branch` | string | ❌ | 默认分支，默认 `main` |
| `auth_type` | string | ❌ | 认证方式：`none` / `token` / `ssh_key` / `username_password` |
| `auth_config` | object | ❌ | 认证配置（JSON 对象，与 `auth_type` 对应） |
| `sync_enabled` | bool | ❌ | 是否启用自动同步，默认 false |
| `sync_interval` | string | ❌ | 同步间隔表达式（如 `@every 1h`） |
| `max_failures` | int | ❌ | 最大连续失败次数 |

**auth_config 示例（token 认证）**:
```json
{"token": "ghp_xxxxx"}
```

**auth_config 示例（SSH 密鑰）**:
```json
{"private_key": "-----BEGIN RSA PRIVATE KEY-----\n..."}
```

---

## 4. 获取仓库统计

**GET** `/api/v1/git-repos/stats`

**权限**: `plugin:list`

---

## 5. 获取仓库详情

**GET** `/api/v1/git-repos/:id`

**权限**: `plugin:list`

---

## 6. 更新仓库

**PUT** `/api/v1/git-repos/:id`

**权限**: `plugin:update`

### 请求体（所有字段可选）

| 字段 | 类型 | 说明 |
|------|------|------|
| `default_branch` | string | 默认分支 |
| `auth_type` | string | 认证方式 |
| `auth_config` | object | 认证配置（JSON 对象） |
| `sync_enabled` | bool | 是否启用自动同步 |
| `sync_interval` | string | 同步间隔表达式 |
| `max_failures` | int | 最大连续失败次数 |

## 7. 删除仓库

**DELETE** `/api/v1/git-repos/:id`

**权限**: `plugin:delete`

> 删除仓库会同时删除该仓库下的所有 Playbook 记录。

---

## 8. 手动触发同步

**POST** `/api/v1/git-repos/:id/sync`

**权限**: `plugin:sync`

---

## 9. 重置仓库状态

**POST** `/api/v1/git-repos/:id/reset-status`

**权限**: `plugin:update`

将仓库状态强制重置（当仓库卡在 `syncing` 状态时使用）。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `status` | string | ❌ | 目标状态：`pending`（默认）/ `error` |

---

## 10. 获取同步日志

**GET** `/api/v1/git-repos/:id/logs`

**权限**: `plugin:list`

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "repo_id": "uuid",
      "status": "success",
      "trigger": "manual",
      "commit_hash": "abc123def456",
      "commit_message": "Add new playbook for disk cleanup",
      "files_added": 2,
      "files_modified": 1,
      "files_deleted": 0,
      "playbooks_scanned": 25,
      "error_message": "",
      "started_at": "2026-02-18T09:00:00Z",
      "completed_at": "2026-02-18T09:00:10Z",
      "duration_ms": 10000
    }
  ]
}
```

---

## 11. 获取提交历史

**GET** `/api/v1/git-repos/:id/commits`

**权限**: `plugin:list`

返回仓库最近的 Git 提交记录。

### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `limit` | int | 返回数量，默认 10 |

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "hash": "abc123def456",
      "message": "Add new playbook for disk cleanup",
      "author": "ops-team",
      "timestamp": "2026-02-18T09:00:00Z"
    }
  ]
}
```

---

## 12. 获取仓库文件列表 / 文件内容

**GET** `/api/v1/git-repos/:id/files`

**权限**: `plugin:list`

不传 `path` 时返回文件树；传 `path` 时返回指定文件的内容。

### 查询参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `path` | string | 文件路径（不传则返回文件树，传则返回文件内容） |

### 响应（文件树）

```json
{
  "code": 0,
  "data": {
    "files": [
      "playbooks/disk_cleanup.yml",
      "playbooks/service_restart.yml"
    ]
  }
}
```

### 响应（文件内容，传 path 时）

```json
{
  "code": 0,
  "data": {
    "path": "playbooks/disk_cleanup.yml",
    "content": "---\n- name: Disk Cleanup\n  hosts: all\n  ..."
  }
}
```
