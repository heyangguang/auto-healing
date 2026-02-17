# 审计日志 API 变更文档（前端对接指南）

> **更新时间**：2026-02-18
> **影响范围**：`GET /api/v1/audit-logs` 列表接口、审计日志详情展示

---

## 一、变更概要

本次后端更新为审计日志增加了 **变更追踪** 能力：

| 变更项 | 说明 |
|--------|------|
| **新增字段** `changes` | 列表接口新增，记录 PUT/PATCH/DELETE 操作的具体变更内容 |
| **新增字段** `request_body` | 列表接口新增，记录请求体内容 |
| **后端逻辑** | 自动对比资源修改前后的值，生成 `old`/`new` 差异 |

> **注意**：`GetAuditLog` 详情接口（`GET /api/v1/audit-logs/:id`）原本已有 `changes` 和 `request_body`，此次仅修复了 **列表接口** 的遗漏。

---

## 二、完整字段列表

### `GET /api/v1/audit-logs` 响应字段

| 字段 | 类型 | 是否新增 | 说明 |
|------|------|:--------:|------|
| `id` | `string (UUID)` | ❌ | 审计日志 ID |
| `user_id` | `string (UUID) \| null` | ❌ | 操作用户 ID |
| `username` | `string` | ❌ | 操作用户名 |
| `ip_address` | `string` | ❌ | 操作者 IP 地址 |
| `user_agent` | `string` | ❌ | 用户代理字符串 |
| `action` | `string` | ❌ | 操作类型：`create`/`update`/`delete`/`execute`/`login`/`logout` |
| `resource_type` | `string` | ❌ | 资源类型：`plugins`/`users`/`roles` 等 |
| `resource_id` | `string (UUID) \| null` | ❌ | 操作的资源 ID |
| `resource_name` | `string` | ❌ | 资源名称（自动解析） |
| `request_method` | `string` | ❌ | HTTP 方法：`GET`/`POST`/`PUT`/`DELETE` |
| `request_path` | `string` | ❌ | 请求路径 |
| `request_body` | `object \| null` | ✅ **新增** | 请求体内容（JSON 格式，< 10KB 时记录） |
| `response_status` | `number` | ❌ | HTTP 响应状态码 |
| `changes` | `object \| null` | ✅ **新增** | 变更记录（详见下方格式说明） |
| `status` | `string` | ❌ | 操作状态：`success` / `failed` |
| `error_message` | `string` | ❌ | 错误信息（失败时） |
| `risk_level` | `string` | ❌ | 风险等级：`normal` / `high` |
| `risk_reason` | `string` | ❌ | 高风险原因 |
| `created_at` | `string (ISO8601)` | ❌ | 创建时间 |
| `user` | `object \| null` | ❌ | 关联用户对象 |

---

## 三、`changes` 字段格式详解

### 3.1 UPDATE 操作（PUT/PATCH）

当资源被更新且有字段发生变化时，`changes` 记录每个变化字段的旧值和新值：

```json
{
  "changes": {
    "description": {
      "old": "原始描述内容",
      "new": "更新后的描述"
    },
    "name": {
      "old": "旧名称",
      "new": "新名称"
    }
  }
}
```

**规则**：
- 仅记录**实际发生变化**的字段（旧值 ≠ 新值）
- 仅对比**请求体中包含且旧状态中也存在**的字段
- 如果没有字段发生变化，`changes` 为 `null`
- 过长的值会被截断（> 200 字符显示 `[内容过长，已截断...]`）

### 3.2 DELETE 操作

当资源被删除时，`changes` 记录被删除资源的关键信息：

```json
{
  "changes": {
    "deleted": {
      "name": "被删除的资源名称",
      "description": "资源描述",
      "status": "active"
    }
  }
}
```

**保留的关键字段**：`name`、`username`、`title`、`hostname`、`flow_name`、`description`、`status`

### 3.3 CREATE 操作

创建操作 **不记录** `changes`（值为 `null`），因为没有"旧值"可对比。创建的内容可从 `request_body` 获取。

### 3.4 LOGIN 等其他操作

登录、登出等操作 `changes` 和 `request_body` 均为 `null`（出于安全考虑不记录密码等信息）。

---

## 四、`changes` 不同操作的值示例

| 操作类型 | `changes` 示例 | `request_body` |
|----------|---------------|----------------|
| `update` | `{"description": {"old": "旧值", "new": "新值"}}` | `{"name": "xxx", "description": "新值"}` |
| `delete` | `{"deleted": {"name": "xxx", "status": "active"}}` | `null` |
| `create` | `null` | `{"name": "xxx", ...}` |
| `login` | `null` | `null`（安全原因） |

---

## 五、敏感字段过滤

以下字段 **不会出现** 在 `changes` 中（安全过滤）：

| 过滤字段 |
|----------|
| `password`, `password_hash`, `old_password`, `new_password` |
| `access_key_hash`, `secret_key_hash`, `secret_key` |
| `private_key`, `passphrase`, `credential` |
| `token`, `api_key` |
| `id`, `created_at`, `updated_at`, `deleted_at` |

---

## 六、支持变更追踪的资源类型

以下资源在 UPDATE/DELETE 时会自动记录 `changes`：

| API 路径 | 资源类型 | 数据库表 |
|----------|---------|---------|
| `/api/v1/plugins/:id` | 插件 | `plugins` |
| `/api/v1/users/:id` | 用户 | `users` |
| `/api/v1/roles/:id` | 角色 | `roles` |
| `/api/v1/notifications/channels/:id` | 通知渠道 | `notification_channels` |
| `/api/v1/notifications/templates/:id` | 通知模板 | `notification_templates` |
| `/api/v1/execution-tasks/:id` | 执行任务 | `execution_tasks` |
| `/api/v1/execution-schedules/:id` | 执行计划 | `execution_schedules` |
| `/api/v1/execution-runs/:id` | 执行记录 | `execution_runs` |
| `/api/v1/cmdb/:id` | CMDB 资产 | `cmdb_items` |
| `/api/v1/secrets-sources/:id` | 密钥源 | `secrets_sources` |
| `/api/v1/git-repos/:id` | Git 仓库 | `git_repositories` |
| `/api/v1/playbooks/:id` | Playbook | `playbook_templates` |
| `/api/v1/incidents/:id` | 事件 | `incidents` |
| `/api/v1/healing/flows/:id` | 自愈流程 | `healing_flows` |
| `/api/v1/healing/rules/:id` | 自愈规则 | `healing_rules` |
| `/api/v1/healing/instances/:id` | 流程实例 | `flow_instances` |

> 不在此列表中的资源（如 `auth`、`dashboard`）不支持变更追踪。

---

## 七、前端建议实现

### 7.1 审计日志列表页

- 在列表中可添加一个 **"变更"列** 或 **图标指示器**，当 `changes` 不为 `null` 时显示
- 点击可弹出 **变更详情 Drawer/Modal**

### 7.2 变更详情展示建议

```
┌─ 变更详情 ─────────────────────────┐
│                                     │
│  字段: description                  │
│  旧值: 原始描述内容                   │
│  新值: 更新后的描述                   │
│                                     │
│  字段: status                       │
│  旧值: inactive                     │
│  新值: active                       │
│                                     │
└─────────────────────────────────────┘
```

### 7.3 TypeScript 类型定义参考

```typescript
// changes 字段类型
interface AuditChanges {
  // UPDATE 操作：每个变化字段的 old/new 值
  [fieldName: string]: {
    old: string | number | boolean | null;
    new: string | number | boolean | null;
  } | {
    // DELETE 操作的 deleted 对象
    [key: string]: string | number | boolean | null;
  };
}

// 审计日志完整类型（列表项已更新）
interface AuditLogItem {
  id: string;
  user_id: string | null;
  username: string;
  ip_address: string;
  user_agent: string;
  action: 'create' | 'update' | 'delete' | 'execute' | 'login' | 'logout';
  resource_type: string;
  resource_id: string | null;
  resource_name: string;
  request_method: string;
  request_path: string;
  request_body: Record<string, any> | null;   // ✅ 新增
  response_status: number;
  changes: AuditChanges | null;               // ✅ 新增
  status: 'success' | 'failed';
  error_message: string;
  risk_level: 'normal' | 'high';
  risk_reason: string;
  created_at: string;
  user: UserInfo | null;
}
```

### 7.4 判断逻辑参考

```typescript
// 判断是否有变更记录
const hasChanges = (log: AuditLogItem) => log.changes !== null;

// 判断是否为 DELETE 操作的变更
const isDeleteChanges = (changes: AuditChanges) => 'deleted' in changes;

// 获取变更的字段列表（UPDATE）
const getChangedFields = (changes: AuditChanges) =>
  Object.keys(changes).filter(k => k !== 'deleted');

// 获取删除资源的信息
const getDeletedInfo = (changes: AuditChanges) =>
  changes.deleted as Record<string, any>;
```
