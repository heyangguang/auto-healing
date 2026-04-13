# 认证模块 API 文档

**Base URL**: `/api/v1`  
**认证方式**: Bearer Token（除登录/刷新接口外，所有接口均需要 `Authorization: Bearer <token>` 请求头）

---

## 1. 用户登录

**POST** `/api/v1/auth/login`

**权限**: 公开（无需认证）

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | ✅ | 用户名 |
| `password` | string | ✅ | 密码 |

```json
{
  "username": "admin",
  "password": "admin123456"
}
```

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 86400,
    "token_type": "Bearer",
    "user": {
      "id": "uuid",
      "username": "admin",
      "email": "admin@example.com",
      "display_name": "管理员",
      "is_platform_admin": true,
      "roles": ["admin"],
      "permissions": ["platform:users:view"]
    },
    "tenants": [
      {
        "id": "uuid",
        "name": "Default Tenant",
        "code": "default"
      }
    ],
    "current_tenant_id": "uuid"
  }
}
```

> **注意**: 登录失败超过限制次数会触发账户锁定。
> **审计说明**: 所有登录尝试（包括不存在账号、密码错误、锁定账户）都会作为 `category=auth` 的全局认证安全事件记录到平台审计日志。

---

## 2. 刷新 Token

**POST** `/api/v1/auth/refresh`

**权限**: 公开（使用 refresh_token）

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `refresh_token` | string | ✅ | 刷新令牌 |

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refresh_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in": 86400,
    "token_type": "Bearer",
    "user": {
      "id": "uuid",
      "username": "admin",
      "email": "admin@example.com",
      "display_name": "管理员",
      "is_platform_admin": true,
      "roles": ["admin"],
      "permissions": ["platform:users:view"]
    },
    "tenants": [
      {
        "id": "uuid",
        "name": "Default Tenant",
        "code": "default"
      }
    ],
    "current_tenant_id": "uuid"
  }
}
```

---

## 3. 用户登出

**POST** `/api/v1/auth/logout`

**权限**: 已登录用户

### 请求体（可选）

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `refresh_token` | string | ❌ | 若携带，则服务端会同时吊销该刷新令牌 |

### 响应

```json
{
  "code": 0,
  "message": "登出成功"
}
```

> **审计说明**: 登出事件会统一记录到平台审计日志，语义为 `category=auth`、`action=logout`、`resource_type=auth`。

---

## 4. 获取当前用户信息

**GET** `/api/v1/auth/me`

**权限**: 已登录用户

### 响应

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "uuid",
    "username": "admin",
    "email": "admin@example.com",
    "display_name": "管理员",
    "phone": "13800138000",
    "avatar": "https://...",
    "is_platform_admin": true,
    "tenant_id": "uuid",
    "status": "active",
    "roles": [
      {
        "id": "uuid",
        "name": "admin",
        "display_name": "管理员"
      }
    ],
    "permissions": ["user:read", "user:write"],
    "last_login_at": "2026-02-18T10:00:00Z",
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-02-18T10:00:00Z"
  }
}
```

---

## 5. 获取个人资料

**GET** `/api/v1/auth/profile`

**权限**: 已登录用户

与 `/auth/me` 类似，返回当前用户的个人资料信息（不含权限列表）。

---

## 6. 修改密码

**PUT** `/api/v1/auth/password`

**权限**: 已登录用户

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `old_password` | string | ✅ | 当前密码 |
| `new_password` | string | ✅ | 新密码 |

```json
{
  "old_password": "oldpass123",
  "new_password": "newpass456"
}
```

### 响应

```json
{
  "code": 0,
  "message": "密码修改成功"
}
```

---

## 7. 更新个人信息

**PUT** `/api/v1/auth/profile`

**权限**: 已登录用户

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `display_name` | string | ❌ | 显示名称 |
| `email` | string | ❌ | 邮箱 |
| `phone` | string | ❌ | 手机号 |

```json
{
  "display_name": "张三",
  "email": "zhangsan@example.com",
  "phone": "13900139000"
}
```

### 响应

返回更新后的用户信息对象（同 `/auth/me` 响应结构）。

---

## 8. 获取个人登录历史

**GET** `/api/v1/auth/profile/login-history`

**权限**: 已登录用户

> 返回当前用户的认证历史，数据源统一来自平台认证审计，不依赖当前租户上下文是否可见或是否已停用。

---

## 9. 通过邀请注册

**POST** `/api/v1/auth/register`

**权限**: 公开（需有效邀请）

> **审计说明**: 邀请注册成功或失败都会记录为 `category=auth`、`action=register`、`resource_type=auth` 的认证安全事件。

---

## 10. 获取用户偏好设置

**GET** `/api/v1/common/user/preferences`

**权限**: 已登录用户

### 响应

```json
{
  "code": 0,
  "data": {
    "user_id": "uuid",
    "preferences": {
      "theme": "dark",
      "language": "zh-CN",
      "notification_enabled": true
    }
  }
}
```

---

## 9. 全量更新偏好设置

**PUT** `/api/v1/common/user/preferences`

**权限**: 已登录用户

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `preferences` | object | ✅ | 偏好设置 JSON 对象（自由格式） |

```json
{
  "preferences": {
    "theme": "dark",
    "language": "zh-CN",
    "notification_enabled": true,
    "sidebar_collapsed": false
  }
}
```

---

## 10. 部分更新偏好设置

**PATCH** `/api/v1/common/user/preferences`

**权限**: 已登录用户

与 PUT 相同的请求体，但只合并更新指定字段，不覆盖未传入的字段。

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

### 错误响应

```json
{
  "code": 40000,
  "message": "错误描述"
}
```

### 常见错误码

| 业务错误码 | 说明 |
|------------|------|
| 40000 | 请求参数错误 |
| 40100 | 未认证或 Token 失效 |
| 40300 | 无权限 |
| 40400 | 资源不存在 |
| 50000 | 服务器内部错误 |
