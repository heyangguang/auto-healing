# 平台用户管理 API 文档

**路径前缀**: `/api/v1/platform/users`  
**权限**: 平台管理员

---

## 1. 获取用户列表

**GET** `/api/v1/platform/users`

**权限**: `user:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `search` | string | ❌ | 模糊搜索（用户名、邮箱、昵称） |
| `status` | string | ❌ | 状态：`active` / `inactive` |
| `username` | string | ❌ | 按用户名精确筛选 |
| `email` | string | ❌ | 按邮箱精确筛选 |
| `display_name` | string | ❌ | 按显示名称筛选 |
| `role_id` | uuid | ❌ | 按角色筛选 |
| `created_from` | string | ❌ | 创建时间起始（ISO 8601） |
| `created_to` | string | ❌ | 创建时间截止（ISO 8601） |
| `sort_field` | string | ❌ | 排序字段：`username` / `created_at` / `last_login_at` |
| `sort_order` | string | ❌ | 排序方向：`asc` / `desc` |

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "username": "admin",
        "email": "admin@example.com",
        "display_name": "管理员",
        "status": "active",
        "is_platform_admin": true,
        "roles": [{"id": "uuid", "name": "平台管理员"}],
        "last_login_at": "2026-02-18T09:00:00Z",
        "created_at": "2026-01-01T00:00:00Z"
      }
    ],
    "total": 50,
    "page": 1,
    "page_size": 20
  }
}
```

---

## 2. 创建用户

**POST** `/api/v1/platform/users`

**权限**: 平台管理员（`RequirePlatformAdmin`）

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | ✅ | 用户名（唯一，3-50 字符） |
| `email` | string | ✅ | 邮箱（唯一） |
| `password` | string | ✅ | 初始密码（至少 8 位） |
| `display_name` | string | ❌ | 显示名称 |
| `phone` | string | ❌ | 手机号 |
| `role_ids` | []uuid | ❌ | 初始分配的角色 ID 列表 |
| `status` | string | ❌ | 状态：`active` / `inactive`，默认 `active` |
| `is_platform_admin` | bool | ❌ | 是否为平台管理员，默认 false |

---

## 3. 获取简单用户列表（轻量接口）

**GET** `/api/v1/platform/users/simple`

**权限**: `user:list`

返回用户的 ID、用户名、昵称，不含分页，用于下拉选择器等场景。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `search` | string | ❌ | 模糊搜索（用户名、邮箱、昵称） |
| `status` | string | ❌ | 状态筛选，默认 `active` |

### 响应

```json
{
  "code": 0,
  "data": [
    {"id": "uuid", "username": "admin", "nickname": "管理员"}
  ]
}
```

---

## 4. 获取用户详情

**GET** `/api/v1/platform/users/:id`

**权限**: `user:list`

---

## 5. 更新用户

**PUT** `/api/v1/platform/users/:id`

**权限**: `user:update`

### 请求体（所有字段可选）

| 字段 | 类型 | 说明 |
|------|------|------|
| `display_name` | string | 显示名称 |
| `phone` | string | 手机号 |
| `status` | string | 状态：`active` / `inactive` |

---

## 6. 删除用户

**DELETE** `/api/v1/platform/users/:id`

**权限**: `user:delete`

---

## 7. 重置密码

**POST** `/api/v1/platform/users/:id/reset-password`

**权限**: `user:reset_password`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `new_password` | string | ✅ | 新密码 |

---

## 8. 分配用户角色

**PUT** `/api/v1/platform/users/:id/roles`

**权限**: `role:assign`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `role_ids` | []uuid | ✅ | 角色 ID 列表（全量替换） |

---

## 租户级用户创建

**POST** `/api/v1/tenant/users`

**权限**: `user:create`

在当前租户内创建用户（非平台级）。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `username` | string | ✅ | 用户名 |
| `email` | string | ✅ | 邮箱 |
| `password` | string | ✅ | 初始密码 |
| `nickname` | string | ❌ | 昵称 |
