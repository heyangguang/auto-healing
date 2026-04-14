# 前端 API 对接指南

> **版本**：v2.0 | **更新日期**：2026-02-18
> 本文档描述 API 路由的整体结构，重点说明**平台级**与**租户级**接口的划分规则，供前端开发对接使用。

---

## 核心概念

系统分为两个层次：

| 层次 | 说明 | 数据隔离 |
|------|------|----------|
| **平台级（Platform）** | 超级管理员管理整个平台，跨所有租户 | 无租户隔离，操作全局生效 |
| **租户级（Tenant）** | 普通用户在自己的租户内操作 | 严格按 `X-Tenant-ID` 隔离数据 |

### 识别规则

```
/api/v1/platform/...   → 平台级接口（需平台管理员权限）
/api/v1/common/...     → 认证后的公共入口（租户用户可带或省略 X-Tenant-ID；平台管理员仅在 Impersonation 时拥有租户上下文）
/api/v1/tenant/...     → 显式租户级接口（需租户上下文；租户用户可省略 X-Tenant-ID 使用默认租户）
```

### 请求头规范

```http
# 所有需要认证的接口
Authorization: Bearer <access_token>

# `/api/v1/tenant/*` 建议始终携带；`/api/v1/common/*` 对租户用户可选携带
X-Tenant-ID: <tenant_uuid>
```

---

## 一、公开接口（无需认证）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/auth/login` | 登录，返回 `access_token` + `refresh_token` |
| POST | `/api/v1/auth/refresh` | 刷新 Token |

---

## 二、平台级接口 `/api/v1/platform/`

> ⚠️ **仅平台管理员（`is_platform_admin: true`）可访问，普通租户用户调用会返回 403。**
> 不需要携带 `X-Tenant-ID` 请求头。

### 用户管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/platform/users` | `user:list` | 获取所有用户列表 |
| POST | `/api/v1/platform/users` | 平台管理员 | 创建用户 |
| GET | `/api/v1/platform/users/simple` | `user:list` | 简化用户列表（选择器用） |
| GET | `/api/v1/platform/users/:id` | `user:list` | 获取用户详情 |
| PUT | `/api/v1/platform/users/:id` | `user:update` | 更新用户信息 |
| DELETE | `/api/v1/platform/users/:id` | `user:delete` | 删除用户 |
| POST | `/api/v1/platform/users/:id/reset-password` | `user:reset_password` | 重置用户密码 |
| PUT | `/api/v1/platform/users/:id/roles` | `role:assign` | 分配用户角色 |

### 角色管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/platform/roles` | `role:list` | 获取角色列表 |
| POST | `/api/v1/platform/roles` | `role:create` | 创建角色 |
| GET | `/api/v1/platform/roles/:id` | `role:list` | 获取角色详情 |
| PUT | `/api/v1/platform/roles/:id` | `role:update` | 更新角色 |
| DELETE | `/api/v1/platform/roles/:id` | `role:delete` | 删除角色 |
| PUT | `/api/v1/platform/roles/:id/permissions` | `role:assign` | 分配角色权限 |

### 权限

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/platform/permissions` | 获取所有权限列表 |
| GET | `/api/v1/platform/permissions/tree` | 获取权限树 |

### 租户管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/platform/tenants` | `platform:tenants:manage` | 获取租户列表 |
| POST | `/api/v1/platform/tenants` | `platform:tenants:manage` | 创建租户 |
| GET | `/api/v1/platform/tenants/:id` | `platform:tenants:manage` | 获取租户详情 |
| PUT | `/api/v1/platform/tenants/:id` | `platform:tenants:manage` | 更新租户 |
| DELETE | `/api/v1/platform/tenants/:id` | `platform:tenants:manage` | 删除租户 |
| GET | `/api/v1/platform/tenants/:id/members` | `platform:tenants:manage` | 获取租户成员 |
| POST | `/api/v1/platform/tenants/:id/members` | `platform:tenants:manage` | 添加租户成员 |
| DELETE | `/api/v1/platform/tenants/:id/members/:userId` | `platform:tenants:manage` | 移除租户成员 |

### 平台设置

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/platform/settings` | `platform:settings:manage` | 获取平台配置 |
| PUT | `/api/v1/platform/settings/:key` | `platform:settings:manage` | 更新平台配置项 |

### 平台审计日志

> 记录平台管理员的所有操作（登录、用户管理、角色管理等），写入 `platform_audit_logs` 表。

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/platform/audit-logs` | `platform:audit:list` | 审计日志列表 |
| GET | `/api/v1/platform/audit-logs/:id` | `platform:audit:list` | 审计日志详情 |
| GET | `/api/v1/platform/audit-logs/stats` | `platform:audit:list` | 统计数据 |
| GET | `/api/v1/platform/audit-logs/trend` | `platform:audit:list` | 趋势图（按天） |
| GET | `/api/v1/platform/audit-logs/user-ranking` | `platform:audit:list` | 用户操作排行 |
| GET | `/api/v1/platform/audit-logs/high-risk` | `platform:audit:list` | 高危操作日志 |

**查询参数（列表接口）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，默认 1 |
| `page_size` | int | 每页条数，默认 20 |
| `search` | string | 模糊搜索（用户名/资源名/请求路径） |
| `category` | string | `login` \| `operation` |
| `action` | string | 操作类型，如 `create`、`delete` |
| `resource_type` | string | 资源类型，如 `users`、`roles` |
| `username` | string | 用户名精确过滤 |
| `user_id` | UUID | 用户 ID 过滤 |
| `status` | string | `success` \| `failed` |
| `created_after` | RFC3339 | 开始时间 |
| `created_before` | RFC3339 | 结束时间 |
| `sort_by` | string | 排序字段，如 `created_at` |
| `sort_order` | string | `asc` \| `desc`，默认 `desc` |

**响应字段（列表 & 详情）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 日志 ID |
| `user_id` | UUID | 操作用户 ID |
| `username` | string | 操作用户名 |
| `ip_address` | string | 客户端 IP |
| `user_agent` | string | 浏览器 UA |
| `category` | string | `login` \| `operation` |
| `action` | string | 操作类型 |
| `resource_type` | string | 资源类型 |
| `resource_id` | UUID | 资源 ID |
| `resource_name` | string | 资源名称 |
| `request_method` | string | HTTP 方法 |
| `request_path` | string | 请求路径 |
| `request_body` | object | 请求体（JSON） |
| `response_status` | int | HTTP 响应状态码 |
| `changes` | object | 变更前后对比（JSON） |
| `status` | string | `success` \| `failed` |
| `error_message` | string | 失败原因 |
| `risk_level` | string | `high` \| `normal`（后端计算） |
| `risk_reason` | string | 高危原因描述（`risk_level=high` 时有值） |
| `created_at` | RFC3339 | 创建时间 |

> **注意：** 平台级审计日志响应中**不包含** `user` 关联对象字段。

---

## 三、租户级接口 `/api/v1/`

> 所有接口需携带 `Authorization` 请求头。
> `/api/v1/common/*` 在租户用户场景下可选携带 `X-Tenant-ID`，未携带时默认使用当前默认租户；平台管理员仅在 Impersonation 场景下拥有租户上下文。

### 认证 & 个人信息

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/auth/me` | 获取当前用户信息 |
| GET | `/api/v1/auth/profile` | 获取个人资料 |
| PUT | `/api/v1/auth/profile` | 更新个人资料 |
| PUT | `/api/v1/auth/password` | 修改密码 |
| POST | `/api/v1/auth/logout` | 登出；可选携带 `refresh_token` 以同时吊销 refresh token |

### 用户偏好 & 活动

| 方法 | 路径 | 说明 |
|------|------|------|
| GET/PUT/PATCH | `/api/v1/common/user/preferences` | 用户偏好设置 |
| GET | `/api/v1/common/user/favorites` | 收藏列表 |
| POST | `/api/v1/common/user/favorites` | 添加收藏 |
| DELETE | `/api/v1/common/user/favorites/:menu_key` | 删除收藏 |
| GET | `/api/v1/common/user/recents` | 最近访问 |
| POST | `/api/v1/common/user/recents` | 记录最近访问 |
| GET | `/api/v1/common/user/tenants` | 当前用户所属租户列表 |

### 租户级用户管理

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| POST | `/api/v1/tenant/users` | `user:create` | 在当前租户内创建用户 |

> 注意：租户内创建的用户会自动关联到当前租户，无法跨租户操作。

### 租户级审计日志

> 记录租户内用户的操作，写入 `audit_logs` 表。

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| GET | `/api/v1/audit-logs` | `audit:list` | 审计日志列表 |
| GET | `/api/v1/audit-logs/:id` | `audit:list` | 审计日志详情 |
| GET | `/api/v1/audit-logs/stats` | `audit:list` | 统计数据 |
| GET | `/api/v1/audit-logs/trend` | `audit:list` | 趋势图 |
| GET | `/api/v1/audit-logs/user-ranking` | `audit:list` | 用户操作排行 |
| GET | `/api/v1/audit-logs/action-grouping` | `audit:list` | 操作分类统计 |
| GET | `/api/v1/audit-logs/resource-stats` | `audit:list` | 资源类型统计 |
| GET | `/api/v1/audit-logs/high-risk` | `audit:list` | 高危操作 |
| GET | `/api/v1/audit-logs/export` | `audit:export` | 导出（CSV 格式） |

**查询参数（列表接口）：**

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | int | 页码，默认 1 |
| `page_size` | int | 每页条数，默认 20 |
| `search` | string | 模糊搜索（用户名/资源名/请求路径） |
| `category` | string | `login` \| `operation` |
| `action` | string | 操作类型，如 `create`、`delete` |
| `resource_type` | string | 资源类型，如 `plugin`、`cmdb` |
| `exclude_action` | string | 排除操作类型（逗号分隔，如 `login,list`） |
| `exclude_resource_type` | string | 排除资源类型（逗号分隔） |
| `username` | string | 用户名精确过滤 |
| `user_id` | UUID | 用户 ID 过滤 |
| `status` | string | `success` \| `failed` |
| `risk_level` | string | `high` \| `normal` |
| `created_after` | RFC3339 | 开始时间 |
| `created_before` | RFC3339 | 结束时间 |
| `sort_by` | string | 排序字段，如 `created_at` |
| `sort_order` | string | `asc` \| `desc`，默认 `desc` |

**响应字段（列表 & 详情）：**

| 字段 | 类型 | 说明 |
|------|------|------|
| `id` | UUID | 日志 ID |
| `user_id` | UUID | 操作用户 ID |
| `username` | string | 操作用户名 |
| `ip_address` | string | 客户端 IP |
| `user_agent` | string | 浏览器 UA |
| `category` | string | `login` \| `operation` |
| `action` | string | 操作类型 |
| `resource_type` | string | 资源类型 |
| `resource_id` | UUID | 资源 ID |
| `resource_name` | string | 资源名称 |
| `request_method` | string | HTTP 方法 |
| `request_path` | string | 请求路径 |
| `request_body` | object | 请求体（JSON） |
| `response_status` | int | HTTP 响应状态码 |
| `changes` | object | 变更前后对比（JSON） |
| `status` | string | `success` \| `failed` |
| `error_message` | string | 失败原因 |
| `risk_level` | string | `high` \| `normal`（后端计算） |
| `risk_reason` | string | 高危原因描述（`risk_level=high` 时有值） |
| `user` | object | 关联用户对象（含 `id`、`username`、`email` 等） |
| `created_at` | RFC3339 | 创建时间 |

**辅助接口参数：**

| 接口 | 参数 | 说明 |
|------|------|------|
| `/trend` | `days=30` | 统计最近 N 天趋势 |
| `/user-ranking` | `limit=10&days=7` | Top N 用户，最近 N 天 |
| `/action-grouping` | `action=delete&days=30` | 按操作类型分组 |
| `/resource-stats` | `days=30` | 按资源类型统计 |
| `/high-risk` | `page=1&page_size=20` | 分页高危日志 |
| `/export` | 同列表接口参数 | 导出为 CSV 文件 |



| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/plugins` | 插件列表 |
| POST | `/api/v1/plugins` | 创建插件 |
| GET | `/api/v1/plugins/stats` | 统计 |
| GET/PUT/DELETE | `/api/v1/plugins/:id` | 详情/更新/删除 |
| POST | `/api/v1/plugins/:id/test` | 测试 |
| POST | `/api/v1/plugins/:id/activate` | 激活 |
| POST | `/api/v1/plugins/:id/deactivate` | 停用 |
| POST | `/api/v1/plugins/:id/sync` | 同步 |
| GET | `/api/v1/plugins/:id/logs` | 同步日志 |

### 工单/事件

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/incidents` | 事件列表 |
| GET | `/api/v1/incidents/stats` | 统计 |
| GET | `/api/v1/incidents/:id` | 事件详情 |
| POST | `/api/v1/incidents/:id/close` | 关闭工单 |
| GET | `/api/v1/incidents/:id/writeback-logs` | 查看工单回写日志 |
| POST | `/api/v1/incidents/:id/trigger` | 手动触发自愈 |
| POST | `/api/v1/incidents/:id/reset-scan` | 重置扫描 |
| POST | `/api/v1/incidents/batch-reset-scan` | 批量重置扫描 |

### CMDB 配置项

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/cmdb` | 配置项列表 |
| GET | `/api/v1/cmdb/stats` | 统计 |
| GET | `/api/v1/cmdb/ids` | 仅返回 ID 列表（全选用） |
| GET | `/api/v1/cmdb/:id` | 详情 |
| POST | `/api/v1/cmdb/:id/test-connection` | 测试连接 |
| POST | `/api/v1/cmdb/:id/maintenance` | 进入维护模式 |
| POST | `/api/v1/cmdb/:id/resume` | 退出维护模式 |
| GET | `/api/v1/cmdb/:id/maintenance-logs` | 维护日志 |
| POST | `/api/v1/cmdb/batch-test-connection` | 批量测试连接 |
| POST | `/api/v1/cmdb/batch/maintenance` | 批量进入维护 |
| POST | `/api/v1/cmdb/batch/resume` | 批量退出维护 |

### 密钥管理

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/secrets-sources` | 密钥源列表 |
| POST | `/api/v1/secrets-sources` | 创建密钥源 |
| GET | `/api/v1/secrets-sources/stats` | 统计 |
| GET/PUT/DELETE | `/api/v1/secrets-sources/:id` | 详情/更新/删除 |
| POST | `/api/v1/secrets-sources/:id/test` | 测试连接 |
| POST | `/api/v1/secrets-sources/:id/test-query` | 测试查询 |
| POST | `/api/v1/secrets-sources/:id/enable` | 启用 |
| POST | `/api/v1/secrets-sources/:id/disable` | 禁用 |
| POST | `/api/v1/secrets/query` | 查询密钥值 |

### Git 仓库

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/git-repos` | 仓库列表 |
| POST | `/api/v1/git-repos` | 创建仓库 |
| POST | `/api/v1/git-repos/validate` | 验证仓库地址 |
| GET | `/api/v1/git-repos/stats` | 统计 |
| GET/PUT/DELETE | `/api/v1/git-repos/:id` | 详情/更新/删除 |
| POST | `/api/v1/git-repos/:id/sync` | 同步 |
| POST | `/api/v1/git-repos/:id/reset-status` | 重置状态 |
| GET | `/api/v1/git-repos/:id/logs` | 同步日志 |
| GET | `/api/v1/git-repos/:id/commits` | 提交记录 |
| GET | `/api/v1/git-repos/:id/files` | 文件列表 |

### Playbook 模板

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/playbooks` | Playbook 列表 |
| POST | `/api/v1/playbooks` | 创建 |
| GET | `/api/v1/playbooks/stats` | 统计 |
| GET/PUT/DELETE | `/api/v1/playbooks/:id` | 详情/更新/删除 |
| POST | `/api/v1/playbooks/:id/scan` | 扫描变量 |
| PUT | `/api/v1/playbooks/:id/variables` | 更新变量 |
| POST | `/api/v1/playbooks/:id/ready` | 设为就绪 |
| POST | `/api/v1/playbooks/:id/offline` | 下线 |
| GET | `/api/v1/playbooks/:id/files` | 文件列表 |
| GET | `/api/v1/playbooks/:id/scan-logs` | 扫描日志 |

### 执行任务

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/execution-tasks` | 任务列表 |
| POST | `/api/v1/execution-tasks` | 创建任务 |
| GET | `/api/v1/execution-tasks/stats` | 统计 |
| GET/PUT/DELETE | `/api/v1/execution-tasks/:id` | 详情/更新/删除 |
| POST | `/api/v1/execution-tasks/:id/execute` | 执行 |
| POST | `/api/v1/execution-tasks/:id/confirm-review` | 确认审核 |
| POST | `/api/v1/execution-tasks/batch-confirm-review` | 批量确认审核 |
| GET | `/api/v1/execution-tasks/:id/runs` | 执行记录 |

### 执行记录

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/execution-runs` | 执行记录列表 |
| GET | `/api/v1/execution-runs/stats` | 统计 |
| GET | `/api/v1/execution-runs/trend` | 趋势 |
| GET | `/api/v1/execution-runs/trigger-distribution` | 触发方式分布 |
| GET | `/api/v1/execution-runs/top-failed` | 失败最多的任务 |
| GET | `/api/v1/execution-runs/top-active` | 最活跃的任务 |
| GET | `/api/v1/execution-runs/:id` | 执行详情 |
| GET | `/api/v1/execution-runs/:id/logs` | 执行日志 |
| GET | `/api/v1/execution-runs/:id/stream` | 日志流（SSE） |
| POST | `/api/v1/execution-runs/:id/cancel` | 取消执行 |

### 定时调度

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/execution-schedules` | 调度列表 |
| POST | `/api/v1/execution-schedules` | 创建调度 |
| GET | `/api/v1/execution-schedules/stats` | 统计 |
| GET | `/api/v1/execution-schedules/timeline` | 时间线（可视化） |
| GET/PUT/DELETE | `/api/v1/execution-schedules/:id` | 详情/更新/删除 |
| POST | `/api/v1/execution-schedules/:id/enable` | 启用 |
| POST | `/api/v1/execution-schedules/:id/disable` | 禁用 |

### 通知

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/channels` | 通知渠道列表 |
| POST | `/api/v1/channels` | 创建渠道 |
| GET/PUT/DELETE | `/api/v1/channels/:id` | 详情/更新/删除 |
| POST | `/api/v1/channels/:id/test` | 测试渠道 |
| GET | `/api/v1/templates` | 通知模板列表 |
| POST | `/api/v1/templates` | 创建模板 |
| GET/PUT/DELETE | `/api/v1/templates/:id` | 详情/更新/删除 |
| POST | `/api/v1/templates/:id/preview` | 预览模板 |
| GET | `/api/v1/template-variables` | 可用变量列表 |
| POST | `/api/v1/notifications/send` | 发送通知 |
| GET | `/api/v1/notifications` | 通知记录列表 |
| GET | `/api/v1/notifications/stats` | 统计 |
| GET | `/api/v1/notifications/:id` | 通知详情 |

### 自愈引擎

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/healing/flows` | 自愈流程列表 |
| POST | `/api/v1/healing/flows` | 创建流程 |
| GET | `/api/v1/healing/flows/stats` | 统计 |
| GET | `/api/v1/healing/flows/node-schema` | 节点 Schema |
| GET/PUT/DELETE | `/api/v1/healing/flows/:id` | 详情/更新/删除 |
| POST | `/api/v1/healing/flows/:id/dry-run` | 模拟运行 |
| POST | `/api/v1/healing/flows/:id/dry-run-stream` | 模拟运行流（SSE） |
| GET | `/api/v1/healing/rules` | 自愈规则列表 |
| POST | `/api/v1/healing/rules` | 创建规则 |
| GET | `/api/v1/healing/rules/stats` | 统计 |
| GET/PUT/DELETE | `/api/v1/healing/rules/:id` | 详情/更新/删除 |
| POST | `/api/v1/healing/rules/:id/activate` | 激活规则 |
| POST | `/api/v1/healing/rules/:id/deactivate` | 停用规则 |
| GET | `/api/v1/healing/instances` | 流程实例列表 |
| GET | `/api/v1/healing/instances/stats` | 统计 |
| GET | `/api/v1/healing/instances/:id` | 实例详情 |
| GET | `/api/v1/healing/instances/:id/recovery-logs` | 实例恢复记录 |
| POST | `/api/v1/healing/instances/:id/cancel` | 取消实例 |
| POST | `/api/v1/healing/instances/:id/recover` | 手动恢复实例 |
| POST | `/api/v1/healing/instances/:id/retry` | 重试实例 |
| GET | `/api/v1/healing/instances/:id/events` | 实例事件流（SSE） |
| GET | `/api/v1/healing/approvals` | 审批任务列表 |
| GET | `/api/v1/healing/approvals/pending` | 待审批列表 |
| GET | `/api/v1/healing/approvals/:id` | 审批详情 |
| POST | `/api/v1/healing/approvals/:id/approve` | 审批通过 |
| POST | `/api/v1/healing/approvals/:id/reject` | 审批拒绝 |
| GET | `/api/v1/healing/pending/trigger` | 待触发工单列表 |

### Dashboard & 站内信

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/dashboard/overview` | 总览数据 |
| GET/PUT | `/api/v1/dashboard/config` | 仪表盘配置 |
| GET | `/api/v1/dashboard/workspaces` | 工作区列表 |
| POST/PUT/DELETE | `/api/v1/dashboard/workspaces[/:id]` | 工作区管理 |
| GET/PUT | `/api/v1/dashboard/roles/:roleId/workspaces` | 角色工作区 |
| GET | `/api/v1/site-messages` | 站内信列表 |
| POST | `/api/v1/site-messages` | 发送站内信 |
| GET | `/api/v1/site-messages/unread-count` | 未读数量 |
| GET | `/api/v1/site-messages/categories` | 消息分类 |
| GET/PUT | `/api/v1/site-messages/settings` | 消息设置 |
| PUT | `/api/v1/site-messages/read` | 标记已读 |
| PUT | `/api/v1/site-messages/read-all` | 全部标记已读 |
| GET | `/api/v1/search` | 全局搜索 |

---

## 四、审计日志分流说明

系统维护两张独立的审计日志表，前端展示时需注意来源：

| 表 | 对应接口 | 记录内容 |
|---|---|---|
| `platform_audit_logs` | `/api/v1/platform/audit-logs` | 平台管理员的登录、用户/角色/权限管理操作 |
| `audit_logs` | `/api/v1/audit-logs` | 租户内用户的登录和业务操作 |

**`category` 字段值：**
- `login` — 登录事件
- `operation` — 业务操作事件

---

## 五、已废弃的路径（Breaking Change）

以下路径已**永久移除**，调用会返回 `404`：

| 废弃路径 | 迁移到 |
|---|---|
| `GET/POST /api/v1/users` | `/api/v1/platform/users` |
| `GET /api/v1/users/:id` | `/api/v1/platform/users/:id` |
| `PUT /api/v1/users/:id` | `/api/v1/platform/users/:id` |
| `DELETE /api/v1/users/:id` | `/api/v1/platform/users/:id` |
| `POST /api/v1/users/:id/reset-password` | `/api/v1/platform/users/:id/reset-password` |
| `PUT /api/v1/users/:id/roles` | `/api/v1/platform/users/:id/roles` |
| `GET/POST /api/v1/roles` | `/api/v1/platform/roles` |
| `GET/PUT/DELETE /api/v1/roles/:id` | `/api/v1/platform/roles/:id` |
| `PUT /api/v1/roles/:id/permissions` | `/api/v1/platform/roles/:id/permissions` |
| `GET /api/v1/permissions` | `/api/v1/platform/permissions` |
| `GET /api/v1/permissions/tree` | `/api/v1/platform/permissions/tree` |
