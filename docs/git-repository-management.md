# Git 仓库管理

Git 仓库模块用于管理 Ansible Playbook 仓库，支持从 Git 远程仓库同步代码。

## 核心概念

### Git 仓库与 Playbook 的关系

```
┌─────────────────────────────────────────────────────────────────┐
│                        Git 仓库                                  │
│  (1个远程仓库对应1个 Git 仓库配置)                               │
│                                                                 │
│  ├── site.yml           ← Playbook 模板 A                       │
│  ├── maintenance.yml    ← Playbook 模板 B                       │
│  ├── roles/                                                     │
│  │   ├── cleanup/                                               │
│  │   └── restart/                                               │
│  └── .auto-healing.yml  ← 增强模式配置                          │
└─────────────────────────────────────────────────────────────────┘
```

| 关系 | 说明 |
|------|------|
| **1:N** | 一个 Git 仓库可以包含多个 Playbook 模板 |
| **依赖** | Playbook 必须关联一个已同步的 Git 仓库 |
| **同步联动** | Git 仓库同步后，关联的 Playbook 会自动重新扫描变量 |

> 📖 关于 Playbook 模板的详细文档，请参阅 [Playbook 模板管理](playbook-management.md)

---

## API 接口

### 基础路径
```
/api/v1/git-repos
```

---

## 1. 验证仓库（创建前获取分支列表）

```http
POST /api/v1/git-repos/validate
Content-Type: application/json
```

**用途:** 在创建仓库之前，验证 URL 和认证是否有效，并获取可用分支列表

**请求体:**
```json
{
  "url": "https://gitee.com/company/ansible-playbooks.git",
  "auth_type": "token",
  "auth_config": {
    "token": "your-access-token"
  }
}
```

**响应:**
```json
{
  "code": 0,
  "data": {
    "branches": ["master", "develop", "feature/cleanup"],
    "default_branch": "master"
  }
}
```

**错误响应示例:**
```json
{
  "code": 400,
  "message": "认证失败: 用户名或密码/令牌无效"
}
```

---

## 2. 创建仓库

```http
POST /api/v1/git-repos
Content-Type: application/json
```

**流程:** 验证 URL/认证/分支 → 创建记录 → 首次 Clone → 返回结果

**请求体:**
| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 仓库名称（唯一） |
| `url` | string | ✅ | Git 仓库 URL |
| `default_branch` | string | ❌ | 默认分支，默认从 validate 获取 |
| `auth_type` | string | ❌ | `none`, `token`, `password`, `ssh_key` |
| `auth_config` | object | ❌ | 认证配置 |
| `sync_enabled` | boolean | ❌ | 是否启用定时同步 |
| `sync_interval` | string | ❌ | 同步间隔: `1h`, `6h`, `12h`, `24h` |

**auth_config 格式:**
```json
// token
{"token": "xxx"}

// password
{"username": "user", "password": "pass"}

// ssh_key
{"private_key": "-----BEGIN...", "passphrase": "可选"}
```

---

## 3-6. CRUD 接口

| 接口 | 说明 |
|------|------|
| `GET /git-repos` | 列表 |
| `GET /git-repos/{id}` | 详情 |
| `PUT /git-repos/{id}` | 更新 |
| `DELETE /git-repos/{id}` | 删除 |

---

## 7. 同步仓库

```http
POST /api/v1/git-repos/{id}/sync
```

手动触发同步（git pull）

---

## 8. 获取 Commit 历史

```http
GET /api/v1/git-repos/{id}/commits?limit=10
```

**响应:**
```json
{
  "code": 0,
  "data": [
    {
      "commit_id": "a1b2c3d",
      "full_id": "a1b2c3d4e5f6...",
      "message": "修复内存泄漏问题",
      "author": "张三",
      "author_email": "zhangsan@example.com",
      "date": "2026-01-08T10:30:00Z"
    }
  ]
}
```

---

## 9. 获取文件树/内容

```http
GET /api/v1/git-repos/{id}/files
GET /api/v1/git-repos/{id}/files?path=site.yml
```

---

## 10. 获取同步日志

```http
GET /api/v1/git-repos/{id}/logs?page=1&page_size=20
```

---

## 使用流程

```
1. 验证仓库（获取分支列表）
   POST /api/v1/git-repos/validate
   { "url": "...", "auth_type": "token", "auth_config": {...} }
   → 返回 branches: ["master", "develop"]
        ↓
2. 选择分支并创建仓库
   POST /api/v1/git-repos
   { "name": "...", "url": "...", "default_branch": "master", ... }
   → 自动 Clone + 返回 201
        ↓
3. 创建 Playbook 模板
   POST /api/v1/playbooks
   { "repository_id": "...", "name": "日志清理", "file_path": "site.yml" }
        ↓
4. 扫描变量
   POST /api/v1/playbooks/{id}/scan
        ↓
5. 设置为 Ready
   POST /api/v1/playbooks/{id}/ready
        ↓
6. 创建执行任务
   POST /api/v1/execution-tasks
```

---

## 数据模型

### GitRepository

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 主键 |
| `name` | string | 仓库名称（唯一） |
| `url` | string | Git URL |
| `default_branch` | string | 默认分支 |
| `auth_type` | string | `none`, `token`, `password`, `ssh_key` |
| `status` | string | `pending`, `ready`, `syncing`, `error` |
| `last_sync_at` | datetime | 最后同步时间 |
| `last_commit_id` | string | 最后同步的 commit ID |
| `sync_enabled` | boolean | 是否启用定时同步 |
| `sync_interval` | string | 同步间隔 |
| `created_at` | datetime | 创建时间 |

> **注意:** `auth_config` 字段不会在 API 响应中输出（安全考虑）
