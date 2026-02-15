# 用户管理 API 文档

> 基础路径: `http://localhost:8080/api/v1`
> 认证方式: `Authorization: Bearer <access_token>`

---

## 目录

- [认证 (Auth)](#认证-auth)
  - [登录](#1-登录)
  - [登出](#2-登出)
  - [刷新令牌](#3-刷新令牌)
  - [获取当前用户](#4-获取当前用户)
  - [获取个人资料](#5-获取个人资料)
  - [更新个人资料](#6-更新个人资料)
  - [修改密码](#7-修改密码)
- [用户管理 (Users)](#用户管理-users)
  - [获取用户列表](#8-获取用户列表)
  - [创建用户](#9-创建用户)
  - [获取用户详情](#10-获取用户详情)
  - [更新用户](#11-更新用户)
  - [删除用户](#12-删除用户)
  - [重置密码](#13-重置密码)
  - [分配用户角色](#14-分配用户角色)
- [角色管理 (Roles)](#角色管理-roles)
  - [获取角色列表](#15-获取角色列表)
  - [创建角色](#16-创建角色)
  - [获取角色详情](#17-获取角色详情)
  - [更新角色](#18-更新角色)
  - [删除角色](#19-删除角色)
  - [分配角色权限](#20-分配角色权限)
- [权限 (Permissions)](#权限-permissions)
  - [获取权限列表](#21-获取权限列表)
  - [获取权限树](#22-获取权限树)
- [用户偏好 (Preferences)](#用户偏好-preferences)
  - [获取偏好设置](#23-获取偏好设置)
  - [全量更新偏好](#24-全量更新偏好)
  - [部分更新偏好](#25-部分更新偏好)

---

## 通用响应格式

### 成功响应

```json
{
  "code": 0,
  "message": "success",
  "data": { ... }
}
```

### 列表响应（分页）

```json
{
  "code": 0,
  "message": "success",
  "data": [ ... ],
  "total": 30,
  "page": 1,
  "page_size": 20
}
```

### 错误响应

```json
{
  "code": -1,
  "message": "错误描述"
}
```

---

## 认证 (Auth)

### 1. 登录

> ⚠️ 此接口无需认证，响应格式与标准格式不同（直接返回 token 对象）

```
POST /auth/login
```

**请求体:**

```json
{
  "username": "admin",
  "password": "admin123456"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| username | string | ✅ | 用户名 |
| password | string | ✅ | 密码 |

**响应:**

```json
{
  "access_token": "eyJhbGciOiJI...",
  "refresh_token": "eyJhbGciOiJI...",
  "token_type": "Bearer",
  "expires_in": 3600,
  "user": {
    "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "username": "admin",
    "email": "admin@example.com",
    "display_name": "管理员",
    "roles": ["super_admin"],
    "permissions": ["*"]
  }
}
```

**示例:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}'
```

---

### 2. 登出

```
POST /auth/logout
```

**请求头:** `Authorization: Bearer <access_token>`

**响应:**

```json
{
  "code": 0,
  "message": "登出成功"
}
```

**示例:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer $TOKEN"
```

---

### 3. 刷新令牌

> ⚠️ 响应格式与登录一致（直接返回 token 对象）

```
POST /auth/refresh
```

**请求体:**

```json
{
  "refresh_token": "eyJhbGciOiJI..."
}
```

**响应:** 同登录响应格式

**示例:**

```bash
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"eyJhbGciOiJI..."}'
```

---

### 4. 获取当前用户

> 返回当前登录用户的基本信息（角色和权限为字符串数组）

```
GET /auth/me
```

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "username": "admin",
    "email": "admin@example.com",
    "display_name": "管理员",
    "roles": ["super_admin"],
    "permissions": ["*"]
  }
}
```

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/auth/me
```

---

### 5. 获取个人资料

> 返回当前用户的详细信息（用于个人中心页面），角色为对象数组

```
GET /auth/profile
```

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "username": "admin",
    "email": "admin@example.com",
    "display_name": "管理员",
    "phone": "",
    "avatar_url": "",
    "status": "active",
    "last_login_at": "2026-02-14T01:23:58.104464+08:00",
    "last_login_ip": "::1",
    "password_changed_at": "2026-01-03T18:04:04.745722+08:00",
    "created_at": "2026-01-03T18:04:04.745722+08:00",
    "roles": [
      {
        "id": "12220598-8abf-4adf-9406-41f5b9cab04b",
        "name": "super_admin",
        "display_name": "超级管理员",
        "is_system": true
      }
    ],
    "permissions": ["*"]
  }
}
```

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/auth/profile
```

---

### 6. 更新个人资料

```
PUT /auth/profile
```

**请求体:**

```json
{
  "display_name": "新名称",
  "email": "newemail@example.com",
  "phone": "13800138000"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| display_name | string | ❌ | 显示名称 |
| email | string | ❌ | 邮箱 |
| phone | string | ❌ | 手机号 |

**响应:**

```json
{
  "code": 0,
  "message": "更新成功"
}
```

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/auth/profile \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"display_name":"新名称","phone":"13800138000"}'
```

---

### 7. 修改密码

```
PUT /auth/password
```

**请求体:**

```json
{
  "old_password": "旧密码",
  "new_password": "新密码至少8位"
}
```

| 字段 | 类型 | 必填 | 校验 | 说明 |
|------|------|------|------|------|
| old_password | string | ✅ | - | 旧密码 |
| new_password | string | ✅ | min=8 | 新密码，最少8位 |

**响应:**

```json
{
  "code": 0,
  "message": "密码修改成功"
}
```

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/auth/password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"old_password":"admin123456","new_password":"newpass1234"}'
```

---

## 用户管理 (Users)

### 8. 获取用户列表

> 权限: `user:list`

```
GET /users
```

**查询参数:**

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| page | int | 1 | 页码 |
| page_size | int | 20 | 每页数量 |
| search | string | - | 模糊搜索（用户名/邮箱/显示名） |
| username | string | - | 按用户名精确筛选 |
| email | string | - | 按邮箱精确筛选 |
| display_name | string | - | 按显示名筛选 |
| role_id | uuid | - | 按角色 ID 筛选 |
| status | string | - | 按状态筛选：`active` / `inactive` |
| created_from | string | - | 创建时间起始（ISO 8601） |
| created_to | string | - | 创建时间结束（ISO 8601） |
| sort_field | string | - | 排序字段：`username` / `email` / `display_name` / `created_at` / `status` |
| sort_order | string | - | 排序方向：`asc` / `desc` |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "2973e1d2-c4b3-4f34-991f-1c3503e1e384",
      "username": "testuser",
      "email": "test@example.com",
      "display_name": "测试用户",
      "status": "active",
      "created_at": "2026-02-13T02:43:27.139149+08:00",
      "updated_at": "2026-02-13T02:43:27.139149+08:00",
      "roles": [
        {
          "id": "897fac14-f4ba-490a-b1cb-cecdce561c9e",
          "name": "test_role",
          "display_name": "测试角色",
          "is_system": false,
          "created_at": "2026-01-06T14:59:47.8864+08:00",
          "updated_at": "2026-01-06T14:59:47.8864+08:00"
        }
      ]
    }
  ],
  "total": 30,
  "page": 1,
  "page_size": 20
}
```

**示例:**

```bash
# 基本列表
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/users?page=1&page_size=20"

# 搜索 + 按状态筛选
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/users?search=admin&status=active"

# 按角色筛选 + 排序
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/users?role_id=897fac14-...&sort_field=created_at&sort_order=desc"
```

---

### 9. 创建用户

> 权限: `user:create`

```
POST /users
```

**请求体:**

```json
{
  "username": "newuser",
  "email": "newuser@example.com",
  "password": "password123",
  "display_name": "新用户",
  "phone": "13800138000",
  "role_ids": ["897fac14-f4ba-490a-b1cb-cecdce561c9e"]
}
```

| 字段 | 类型 | 必填 | 校验 | 说明 |
|------|------|------|------|------|
| username | string | ✅ | min=3, max=50 | 用户名 |
| email | string | ✅ | 邮箱格式 | 邮箱 |
| password | string | ✅ | min=8 | 密码 |
| display_name | string | ❌ | - | 显示名称 |
| phone | string | ❌ | - | 手机号 |
| role_ids | uuid[] | ❌ | - | 角色 ID 列表 |

**响应:** `201 Created`

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "新用户UUID",
    "username": "newuser",
    "email": "newuser@example.com",
    "display_name": "新用户",
    "status": "active",
    "created_at": "...",
    "updated_at": "...",
    "roles": [...]
  }
}
```

**示例:**

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "newuser",
    "email": "newuser@example.com",
    "password": "password123",
    "display_name": "新用户",
    "role_ids": ["897fac14-f4ba-490a-b1cb-cecdce561c9e"]
  }'
```

---

### 10. 获取用户详情

> 权限: `user:list`

```
GET /users/:id
```

**路径参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| id | uuid | 用户 ID |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "username": "admin",
    "email": "admin@example.com",
    "display_name": "管理员",
    "status": "active",
    "last_login_at": "2026-02-14T01:24:14.798519+08:00",
    "last_login_ip": "::1",
    "created_at": "2026-01-03T18:04:04.745722+08:00",
    "updated_at": "2026-02-14T01:24:14.799004+08:00",
    "roles": [
      {
        "id": "12220598-8abf-4adf-9406-41f5b9cab04b",
        "name": "super_admin",
        "display_name": "超级管理员",
        "description": "拥有系统所有权限",
        "is_system": true,
        "created_at": "...",
        "updated_at": "...",
        "permissions": [
          {
            "id": "60453e09-...",
            "code": "user:list",
            "name": "查看用户列表",
            "module": "user",
            "resource": "user",
            "action": "read",
            "created_at": "..."
          }
        ]
      }
    ]
  }
}
```

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/users/81186ca0-b38b-455e-b585-84c0ea7a0e2e
```

---

### 11. 更新用户

> 权限: `user:update`

```
PUT /users/:id
```

**请求体:**

```json
{
  "display_name": "新名称",
  "phone": "13800138000",
  "status": "inactive"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| display_name | string | ❌ | 显示名称 |
| phone | string | ❌ | 手机号 |
| status | string | ❌ | 状态: `active` / `inactive` |

**响应:** 返回更新后的用户对象（同获取用户详情）

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/users/2973e1d2-... \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"display_name":"新名称","status":"inactive"}'
```

---

### 12. 删除用户

> 权限: `user:delete`
> ⚠️ 不能删除自己的账户

```
DELETE /users/:id
```

**响应:**

```json
{
  "code": 0,
  "message": "删除成功"
}
```

**示例:**

```bash
curl -X DELETE http://localhost:8080/api/v1/users/2973e1d2-... \
  -H "Authorization: Bearer $TOKEN"
```

---

### 13. 重置密码

> 权限: `user:reset_password`

```
POST /users/:id/reset-password
```

**请求体:**

```json
{
  "new_password": "newpass1234"
}
```

| 字段 | 类型 | 必填 | 校验 | 说明 |
|------|------|------|------|------|
| new_password | string | ✅ | min=8 | 新密码，最少8位 |

**响应:**

```json
{
  "code": 0,
  "message": "密码重置成功"
}
```

**示例:**

```bash
curl -X POST http://localhost:8080/api/v1/users/2973e1d2-.../reset-password \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"new_password":"newpass1234"}'
```

---

### 14. 分配用户角色

> 权限: `role:assign`
> ⚠️ 全量替换：传入的角色列表会替换用户的所有角色

```
PUT /users/:id/roles
```

**请求体:**

```json
{
  "role_ids": [
    "12220598-8abf-4adf-9406-41f5b9cab04b",
    "897fac14-f4ba-490a-b1cb-cecdce561c9e"
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| role_ids | uuid[] | ✅ | 角色 ID 列表（全量替换） |

**响应:** 返回更新后的用户对象（含角色信息）

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/users/2973e1d2-.../roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"role_ids":["12220598-8abf-4adf-9406-41f5b9cab04b"]}'
```

---

## 角色管理 (Roles)

### 15. 获取角色列表

> 权限: `role:list`
> 返回所有角色，含用户数和权限数统计

```
GET /roles
```

**查询参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| search | string | 按角色名/显示名模糊搜索 |

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "12220598-8abf-4adf-9406-41f5b9cab04b",
      "name": "super_admin",
      "display_name": "超级管理员",
      "description": "拥有系统所有权限，可管理所有资源",
      "is_system": true,
      "created_at": "2026-01-03T18:03:48.250507+08:00",
      "updated_at": "2026-02-14T01:23:35.088108+08:00",
      "permissions": [...],
      "user_count": 2,
      "permission_count": 45
    }
  ]
}
```

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/roles"

# 搜索
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/roles?search=admin"
```

---

### 16. 创建角色

> 权限: `role:create`

```
POST /roles
```

**请求体:**

```json
{
  "name": "custom_role",
  "display_name": "自定义角色",
  "description": "角色描述"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| name | string | ✅ | 角色标识（英文，唯一） |
| display_name | string | ✅ | 显示名称 |
| description | string | ❌ | 描述 |

**响应:** `201 Created`，返回创建的角色对象

**示例:**

```bash
curl -X POST http://localhost:8080/api/v1/roles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"custom_role","display_name":"自定义角色","description":"角色描述"}'
```

---

### 17. 获取角色详情

> 权限: `role:list`

```
GET /roles/:id
```

**响应:** 返回角色对象，含完整权限列表

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "12220598-...",
    "name": "super_admin",
    "display_name": "超级管理员",
    "description": "...",
    "is_system": true,
    "created_at": "...",
    "updated_at": "...",
    "permissions": [
      {
        "id": "60453e09-...",
        "code": "user:list",
        "name": "查看用户列表",
        "module": "user",
        "resource": "user",
        "action": "read",
        "created_at": "..."
      }
    ]
  }
}
```

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/roles/12220598-8abf-4adf-9406-41f5b9cab04b
```

---

### 18. 更新角色

> 权限: `role:update`

```
PUT /roles/:id
```

**请求体:**

```json
{
  "display_name": "新名称",
  "description": "新描述"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| display_name | string | ❌ | 显示名称 |
| description | string | ❌ | 描述 |

**响应:** 返回更新后的角色对象

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/roles/897fac14-... \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"display_name":"新名称","description":"新描述"}'
```

---

### 19. 删除角色

> 权限: `role:delete`
> ⚠️ 系统内置角色（`is_system: true`）不可删除

```
DELETE /roles/:id
```

**响应:**

```json
{
  "code": 0,
  "message": "删除成功"
}
```

**示例:**

```bash
curl -X DELETE http://localhost:8080/api/v1/roles/897fac14-... \
  -H "Authorization: Bearer $TOKEN"
```

---

### 20. 分配角色权限

> 权限: `role:assign`
> ⚠️ 全量替换：传入的权限列表会替换角色的所有权限

```
PUT /roles/:id/permissions
```

**请求体:**

```json
{
  "permission_ids": [
    "60453e09-4dc9-4d24-b3b1-5b618e61a57b",
    "49c36208-aa48-44f6-ab5c-57ccdc29e38f"
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| permission_ids | uuid[] | ✅ | 权限 ID 列表（全量替换） |

**响应:** 返回更新后的角色对象（含权限列表）

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/roles/897fac14-.../permissions \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"permission_ids":["60453e09-...","49c36208-..."]}'
```

---

## 权限 (Permissions)

### 21. 获取权限列表

> 无特殊权限要求（已登录即可）

```
GET /permissions
```

**查询参数:**

| 参数 | 类型 | 说明 |
|------|------|------|
| search | string | 按名称/编码模糊搜索 |
| module | string | 按模块筛选，可选值见下方 |
| name | string | 按名称筛选 |
| code | string | 按编码筛选 |

**可用模块:** `dashboard`, `execution`, `healing`, `notification`, `plugin`, `role`, `system`, `user`, `workflow`

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "49c36208-aa48-44f6-ab5c-57ccdc29e38f",
      "code": "user:create",
      "name": "创建用户",
      "module": "user",
      "resource": "user",
      "action": "create",
      "created_at": "2026-01-03T18:03:48.252827+08:00"
    }
  ]
}
```

**示例:**

```bash
# 获取所有权限
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/permissions

# 按模块筛选
curl -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/permissions?module=user"
```

---

### 22. 获取权限树

> 按模块分组返回所有权限

```
GET /permissions/tree
```

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "user": [
      { "id": "...", "code": "user:list", "name": "查看用户列表", ... },
      { "id": "...", "code": "user:create", "name": "创建用户", ... }
    ],
    "role": [
      { "id": "...", "code": "role:list", "name": "查看角色列表", ... }
    ],
    "execution": [...],
    "healing": [...],
    "plugin": [...],
    "notification": [...],
    "system": [...],
    "workflow": [...],
    "dashboard": [...]
  }
}
```

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/permissions/tree
```

---

## 用户偏好 (Preferences)

> 所有偏好接口均针对当前登录用户，无需传 user_id

### 23. 获取偏好设置

```
GET /user/preferences
```

**响应:**

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "e32cf316-0bea-40e0-9390-2c01feb9e3ff",
    "user_id": "81186ca0-b38b-455e-b585-84c0ea7a0e2e",
    "preferences": {
      "user_list_columns": ["username", "display_name", "email", "roles", "status", "created_at", "actions"],
      "user_list_column_widths": {
        "email": 200,
        "roles": 180,
        "status": 172,
        "actions": 176,
        "username": 278,
        "created_at": 424,
        "display_name": 120
      }
    },
    "created_at": "2026-02-13T01:41:43.038292+08:00",
    "updated_at": "2026-02-13T21:58:04.427019+08:00"
  }
}
```

> 如果用户从未设置过偏好，返回空对象:
> `{ "user_id": "...", "preferences": {} }`

**示例:**

```bash
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/user/preferences
```

---

### 24. 全量更新偏好

> 全量替换整个 `preferences` 对象

```
PUT /user/preferences
```

**请求体:**

```json
{
  "preferences": {
    "user_list_columns": ["username", "email", "roles", "status"],
    "theme": "dark"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| preferences | object | ✅ | 完整偏好设置 JSON 对象（全量替换） |

**响应:** 返回更新后的偏好对象

**示例:**

```bash
curl -X PUT http://localhost:8080/api/v1/user/preferences \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"preferences":{"theme":"dark","language":"zh-CN"}}'
```

---

### 25. 部分更新偏好

> 合并更新：只更新传入的字段，不影响已有字段

```
PATCH /user/preferences
```

**请求体:**

```json
{
  "preferences": {
    "theme": "light"
  }
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| preferences | object | ✅ | 要合并的偏好字段（只更新传入的 key） |

**响应:** 返回合并后的完整偏好对象

**示例:**

```bash
curl -X PATCH http://localhost:8080/api/v1/user/preferences \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"preferences":{"theme":"light"}}'
```

---

## 快速获取 Token 脚本

```bash
# 获取 Token 并赋值给变量
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' \
  | python3 -c "import json,sys;print(json.load(sys.stdin)['access_token'])")

# 后续所有请求使用
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/users
```

---

## 权限代码速查表

| 模块 | 权限代码 | 说明 |
|------|---------|------|
| 用户 | `user:list` | 查看用户列表 |
| 用户 | `user:create` | 创建用户 |
| 用户 | `user:update` | 更新用户 |
| 用户 | `user:delete` | 删除用户 |
| 用户 | `user:reset_password` | 重置密码 |
| 角色 | `role:list` | 查看角色列表 |
| 角色 | `role:create` | 创建角色 |
| 角色 | `role:update` | 更新角色 |
| 角色 | `role:delete` | 删除角色 |
| 角色 | `role:assign` | 分配角色/权限 |
