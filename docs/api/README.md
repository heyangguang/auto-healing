# Auto-Healing System API 文档索引

**Base URL**: `http://your-server:8080/api/v1`  
**认证方式**: Bearer Token（JWT）

---

## 快速开始

```bash
# 1. 登录获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.data.access_token')

# 2. 使用 Token 调用接口
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/execution-tasks" | jq '.'
```

---

## 通用响应格式

业务接口统一使用如下响应包裹格式：

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 列表响应格式

```json
{
  "code": 0,
  "message": "success",
  "data": [...],
  "total": 100,
  "page": 1,
  "page_size": 20
}
```

### 错误响应格式

```json
{
  "code": 40000,
  "message": "参数错误：xxx"
}
```

---

## 通用查询参数

| 参数 | 说明 |
|------|------|
| `page` | 页码，默认 1 |
| `page_size` | 每页数量，默认 20，最大 100 |
| `sort_by` | 排序字段（各接口不同） |
| `sort_order` | 排序方向：`asc` / `desc` |

---

## 模块文档列表

### 认证与用户

| 模块 | 文档 | 路径前缀 |
|------|------|---------|
| 认证 | [auth.md](./auth.md) | `/api/v1/auth` |
| 用户偏好 | [auth.md](./auth.md#用户偏好设置) | `/api/v1/common/user/preferences` |
| 用户收藏 & 最近访问 | [user-activity.md](./user-activity.md) | `/api/v1/common/user/favorites`, `/api/v1/common/user/recents` |
| 全局搜索 | [search.md](./search.md) | `/api/v1/search` |

### 平台管理（Platform Admin）

| 模块 | 文档 | 路径前缀 |
|------|------|---------|
| 平台用户管理 | [platform-users.md](./platform-users.md) | `/api/v1/platform/users` |
| 平台角色 & 权限 | [platform-roles.md](./platform-roles.md) | `/api/v1/platform/roles`, `/api/v1/platform/permissions` |
| 租户管理 | [platform-tenants.md](./platform-tenants.md) | `/api/v1/platform/tenants` |
| 平台审计日志 | [platform-audit-logs.md](./platform-audit-logs.md) | `/api/v1/platform/audit-logs` |
| 平台设置 | [platform-settings.md](./platform-settings.md) | `/api/v1/platform/settings` |

### 租户功能

| 模块 | 文档 | 路径前缀 |
|------|------|---------|
| 插件管理 | [plugins.md](./plugins.md) | `/api/v1/plugins` |
| 工单（Incidents） | [incidents.md](./incidents.md) | `/api/v1/incidents` |
| CMDB 配置管理 | [cmdb.md](./cmdb.md) | `/api/v1/cmdb` |
| 密钥源管理 | [secrets.md](./secrets.md) | `/api/v1/secrets-sources`, `/api/v1/secrets/query` |
| Git 仓库管理 | [git-repos.md](./git-repos.md) | `/api/v1/git-repos` |
| Playbook 管理 | [playbooks.md](./playbooks.md) | `/api/v1/playbooks` |
| 执行任务模板 | [execution.md](./execution.md) | `/api/v1/execution-tasks` |
| 执行记录 | [execution.md](./execution.md#执行记录execution-runs) | `/api/v1/execution-runs` |
| 定时调度 | [schedules.md](./schedules.md) | `/api/v1/execution-schedules` |
| 自愈管理 | [healing.md](./healing.md) | `/api/v1/healing/flows`, `/api/v1/healing/rules`, `/api/v1/healing/instances`, `/api/v1/healing/approvals`, `/api/v1/healing/pending` |
| 通知渠道 | [notifications.md](./notifications.md) | `/api/v1/channels` |
| 通知模板 | [notifications.md](./notifications.md#通知模板templates) | `/api/v1/templates` |
| 通知发送记录 | [notifications.md](./notifications.md#通知发送记录notifications) | `/api/v1/notifications` |
| 站内信 | [site-messages.md](./site-messages.md) | `/api/v1/site-messages` |
| 仪表盘 | [dashboard.md](./dashboard.md) | `/api/v1/dashboard` |
| 租户审计日志 | [audit-logs.md](./audit-logs.md) | `/api/v1/audit-logs` |

---

## 权限说明

所有接口均需要在 Header 中携带 JWT Token：

```
Authorization: Bearer <access_token>
```

Token 有效期为 24 小时，可通过 `/api/v1/auth/refresh` 刷新。

### 权限代码说明

| 权限代码 | 说明 |
|---------|------|
| `user:list` / `user:create` / `user:update` / `user:delete` | 用户管理 |
| `role:list` / `role:create` / `role:update` / `role:delete` / `role:assign` | 租户角色管理 |
| `platform:roles:list` / `platform:roles:manage` / `platform:permissions:list` | 平台角色与权限管理 |
| `plugin:list` / `plugin:create` / `plugin:sync` | 插件管理 |
| `task:list` / `task:detail` / `task:cancel` | 执行任务 |
| `playbook:execute` | 执行 Playbook |
| `healing:flows:view` / `healing:flows:create` / `healing:flows:update` | 自愈流程 |
| `healing:rules:view` / `healing:rules:create` | 自愈规则 |
| `healing:instances:view` | 自愈实例 |
| `healing:approvals:view` / `healing:approvals:approve` | 审批任务 |
| `healing:trigger:view` / `healing:trigger:execute` | 工单触发 |
| `audit:list` / `audit:export` | 审计日志 |
| `platform:audit:list` | 平台审计日志 |
| `platform:tenants:manage` | 租户管理 |
| `platform:settings:manage` | 平台设置 |
| `dashboard:workspace:manage` | 工作区管理 |
| `site-message:list` / `site-message:settings:view` / `site-message:settings:manage` | 站内信查询与设置 |
| `platform:messages:send` | 平台站内信发送 |

---

## 文件列表

```
docs/api/
├── README.md                  # 本文件（总索引）
├── auth.md                    # 认证、个人信息、用户偏好
├── user-activity.md           # 收藏 & 最近访问
├── search.md                  # 全局搜索
├── platform-users.md          # 平台用户管理
├── platform-roles.md          # 平台角色 & 权限
├── platform-tenants.md        # 租户管理
├── platform-audit-logs.md     # 平台审计日志
├── platform-settings.md       # 平台设置
├── plugins.md                 # 插件管理
├── incidents.md               # 工单管理
├── cmdb.md                    # CMDB 配置管理
├── secrets.md                 # 密钥源管理
├── git-repos.md               # Git 仓库管理
├── playbooks.md               # Playbook 管理
├── execution.md               # 执行任务模板 & 执行记录
├── schedules.md               # 定时调度
├── healing.md                 # 自愈流程、规则、实例、审批
├── notifications.md           # 通知渠道、模板、发送记录
├── site-messages.md           # 站内信
├── dashboard.md               # 仪表盘
└── audit-logs.md              # 租户审计日志
```
