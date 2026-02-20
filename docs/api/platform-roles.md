# 平台角色与权限管理 API 文档

**路径前缀**: `/api/v1/platform/roles`  
**权限**: 平台管理员

---

## 角色管理

### 1. 获取角色列表

**GET** `/api/v1/platform/roles`

**权限**: `role:list`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `search` | string | ❌ | 模糊搜索（名称） |

> 注意：角色列表不分页，返回全量数据。

---

### 2. 创建角色

**POST** `/api/v1/platform/roles`

**权限**: `role:create`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 角色名称 |
| `description` | string | ❌ | 描述 |

---

### 3. 获取角色详情

**GET** `/api/v1/platform/roles/:id`

**权限**: `role:list`

---

### 4. 更新角色

**PUT** `/api/v1/platform/roles/:id`

**权限**: `role:update`

---

### 5. 删除角色

**DELETE** `/api/v1/platform/roles/:id`

**权限**: `role:delete`

---

### 6. 分配角色权限

**PUT** `/api/v1/platform/roles/:id/permissions`

**权限**: `role:assign`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `permission_ids` | []uuid | ✅ | 权限 ID 列表（全量替换） |

---

## 权限管理

### 7. 获取权限列表

**GET** `/api/v1/platform/permissions`

**权限**: 无特殊要求（已登录即可）

#### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `search` | string | ❌ | 模糊搜索（名称、代码） |
| `module` | string | ❌ | 按模块筛选（如 `user`、`role`、`plugin`） |
| `name` | string | ❌ | 按权限名称精确筛选 |
| `code` | string | ❌ | 按权限代码精确筛选（如 `user:list`） |

---

### 8. 获取权限树

**GET** `/api/v1/platform/permissions/tree`

**权限**: 无特殊要求（已登录即可）

返回按模块分组的权限树结构，用于角色权限配置界面。

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "module": "用户管理",
      "permissions": [
        {"id": "uuid", "code": "user:list", "name": "查看用户"},
        {"id": "uuid", "code": "user:create", "name": "创建用户"}
      ]
    }
  ]
}
```
