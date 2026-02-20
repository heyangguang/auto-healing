# 平台管理员边界设计说明

> 本文档是**前端 + 后端**的共同规范，定义平台管理员的职责边界和接口归属。

---

## 一、角色体系（最终版）

系统共有 **4 个角色**，分两个层级：

| 层级 | 角色名 | 说明 |
|------|--------|------|
| **全局** | `platform_admin` | 平台管理员。可访问所有租户，管理租户和租户管理员账号。可有多个，但至少保留一个。 |
| **租户内** | `admin` | 租户管理员。管理本租户内部的用户、资源等。 |
| **租户内** | `operator` | 运维人员。执行操作，无管理权限。 |
| **租户内** | `viewer` | 只读用户。只能查看，不能操作。 |

> **注**：`super_admin` 角色已废弃删除，统一使用 `platform_admin`。

---

## 二、平台管理员的能力边界

```
platform_admin
├── ✅ 可切换到 任意租户（is_platform_admin = true，不受租户归属限制）
├── ✅ 权限：* （全量，登录时由角色动态计算，无需数据库字段）
├── ✅ 管理租户：增删改查
├── ✅ 指定/变更租户管理员
└── ✅ 管理平台管理员账号（创建/禁用/重置密码）

    ❌ 不管 租户内用户的日常增删改
    ❌ 不管 租户内功能配置（由租户管理员自己管）
```

**安全保护**：删除平台管理员时，若系统中只剩最后一个，后端会拒绝删除，返回 400 错误。

---

## 三、接口归属

### 3.1 平台用户管理（`/api/v1/platform/users`）

**定位：管理平台管理员账号。**

> 此接口只返回有 `platform_admin` 角色的用户，不是全量用户池。  
> 全量用户下拉（选人用）请用 `GET /platform/users/simple`。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/platform/users` | 列出所有平台管理员（自动过滤） |
| GET | `/platform/users/simple` | 全量轻量下拉（不过滤，用于选人） |
| POST | `/platform/users` | 创建平台管理员账号（**自动赋予 `platform_admin` 角色**） |
| GET | `/platform/users/:id` | 账号详情 |
| PUT | `/platform/users/:id` | 更新账号信息 |
| DELETE | `/platform/users/:id` | 删除账号（最后一个不可删） |
| POST | `/platform/users/:id/reset-password` | 重置密码 |
| PUT | `/platform/users/:id/roles` | 变更全局角色 |

### 3.2 平台租户管理（`/api/v1/platform/tenants`）

**定位：管理租户本身 + 指定每个租户的管理员。**

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/platform/tenants` | 列出所有租户 |
| POST | `/platform/tenants` | 创建租户（空租户） |
| GET | `/platform/tenants/:id` | 租户详情 |
| PUT | `/platform/tenants/:id` | 更新租户信息 / 禁用租户 |
| DELETE | `/platform/tenants/:id` | 删除租户 |
| GET | `/platform/tenants/:id/members` | 查看租户成员及角色 |
| POST | `/platform/tenants/:id/admin` | **设置租户管理员**（从 `/users/simple` 选人） |
| PUT | `/platform/tenants/:id/members/:userId/role` | **变更租户内角色（升/降级）** |

### 3.3 租户内部管理（`/api/v1/tenant/...`）

**平台管理员不干预，由租户自治。**

---

## 四、前端关键适配点

### 4.1 平台用户管理页面

- `GET /platform/users` 返回的就是平台管理员列表，无需前端过滤
- 「新建」按钮调用 `POST /platform/users`，后端自动赋予 `platform_admin` 角色，前端**不需要**让用户选择角色
- 响应字段中 `roles[].name === "platform_admin"` 可用于显示角色标签

### 4.2 选人下拉框（设置租户管理员时）

- 调用 `GET /platform/users/simple`（不过滤，返回全量用户池）
- 不要调用 `GET /platform/users`（只有平台管理员）

### 4.3 租户详情页 —「设置管理员」流程

```
1. GET /platform/users/simple?search=xxx   → 搜索选人
2. POST /platform/tenants/:id/admin        → { "user_id": "..." }
3. 若需降级管理员 → PUT /platform/tenants/:id/members/:userId/role
                     → { "role_id": "<operator 或 viewer 的 UUID>" }
```

### 4.4 is_platform_admin 标志

- 登录响应 `user.is_platform_admin === true` 时，前端应显示平台管理入口（租户管理、平台设置等）
- 此标志由后端根据 `platform_admin` 角色**动态计算**，无需前端单独请求

---

## 五、已废弃接口（前端需移除）

| 接口 | 状态 |
|------|------|
| ~~`POST /platform/tenants/:id/members`~~ | 已废弃，后端不再注册此路由 |
| ~~`DELETE /platform/tenants/:id/members/:userId`~~ | 已废弃，后端不再注册此路由 |

---

## 六、操作流程速查

### 添加新的平台管理员
```
POST /platform/users
{ "username": "...", "email": "...", "password": "...", "display_name": "..." }
→ 后端自动赋予 platform_admin 角色
→ 该用户登录后 is_platform_admin = true，权限 = *
```

### 创建租户并指定管理员
```
1. POST /platform/tenants          → 创建租户
2. GET  /platform/users/simple     → 搜索候选人（全量池）
3. POST /platform/tenants/:id/admin → { "user_id": "..." }
```

### 变更租户内成员角色
```
PUT /platform/tenants/:id/members/:userId/role
→ { "role_id": "<角色 UUID>" }
```
