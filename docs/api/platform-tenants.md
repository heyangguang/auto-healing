# 平台租户管理 API 文档

**路径前缀**: `/api/v1/platform/tenants`  
**权限**: 平台管理员（`platform:tenants:manage`）

---

## 1. 获取租户列表

**GET** `/api/v1/platform/tenants`

**权限**: `platform:tenants:manage`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 20 |
| `keyword` | string | ❌ | 模糊搜索（名称） |

### 响应

返回租户列表，每个租户包含 `member_count` 表示成员总数。

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "name": "默认租户",
      "code": "default",
      "description": "...",
      "icon": "bank",
      "status": "active",
      "member_count": 5,
      "created_at": "...",
      "updated_at": "..."
    }
  ],
  "total": 1,
  "page": 1,
  "page_size": 20
}
```

## 2. 创建租户

**POST** `/api/v1/platform/tenants`

**权限**: `platform:tenants:manage`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 租户名称 |
| `code` | string | ✅ | 租户唯一标识码 |
| `description` | string | ❌ | 描述 |
| `icon` | string | ❌ | 图标名称（如 `bank` / `shop` / `team` / `cloud`） |

---

## 3. 获取租户详情

**GET** `/api/v1/platform/tenants/:id`

**权限**: `platform:tenants:manage`

---

## 4. 更新租户

**PUT** `/api/v1/platform/tenants/:id`

**权限**: `platform:tenants:manage`

### 请求体（所有字段可选）

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 名称 |
| `description` | string | 描述 |
| `icon` | string | 图标名称 |
| `status` | string | 状态：`active` / `disabled`（`default` 租户不可禁用） |

---

## 5. 删除租户

**DELETE** `/api/v1/platform/tenants/:id`

**权限**: `platform:tenants:manage`

---

## 6. 获取租户成员列表

**GET** `/api/v1/platform/tenants/:id/members`

**权限**: `platform:tenants:manage`

---

## 7. 添加租户成员

**POST** `/api/v1/platform/tenants/:id/members`

**权限**: `platform:tenants:manage`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `user_id` | uuid | ✅ | 用户 ID |
| `role_id` | uuid | ✅ | 分配的角色 ID |

---

## 8. 移除租户成员

**DELETE** `/api/v1/platform/tenants/:id/members/:userId`

**权限**: `platform:tenants:manage`

---

## 用户租户列表（当前用户）

**GET** `/api/v1/common/user/tenants`

**权限**: 无特殊要求（已登录即可）

获取当前用户所属的所有租户列表，用于租户切换功能。

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "name": "运维团队",
      "icon": "team",
      "is_current": true
    }
  ]
}
```
