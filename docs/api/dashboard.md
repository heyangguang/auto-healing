# 仪表盘 API 文档

**路径前缀**: `/api/v1/dashboard`  
**权限**: 已登录用户

---

## 1. 获取概览统计

**GET** `/api/v1/dashboard/overview`

**权限**: `dashboard:view`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `sections` | string | ✅ | 需要的数据模块，逗号分隔，可选值：`incidents`、`cmdb`、`healing`、`execution`、`plugins`、`notifications`、`git`、`playbooks`、`secrets`、`users` |

### 响应

响应结构根据 `sections` 参数动态返回，每个模块的统计数据作为顶层字段返回。

```json
{
  "code": 0,
  "data": {
    "plugins": {"total": 5, "active": 4, "error": 1},
    "cmdb": {"total": 200, "active": 180, "maintenance": 5},
    "incidents": {"total": 1500, "open": 20, "resolved": 1480},
    "healing": {
      "instances": {"total": 500, "success": 450, "failed": 30, "running": 5},
      "success_rate": 0.9375
    }
  }
}
```

---

## 2. 获取用户仪表盘配置

**GET** `/api/v1/dashboard/config`

**权限**: `dashboard:view`

获取当前用户的仪表盘个性化配置（Widget 布局等）。

---

## 3. 保存用户仪表盘配置

**PUT** `/api/v1/dashboard/config`

**权限**: `dashboard:config:manage`

### 请求体

任意自由格式的 JSON 对象，全量覆盖目前配置。

```json
{
  "widgets": [{"id": "incidents", "x": 0, "y": 0, "w": 6, "h": 4}],
  "layout": "grid",
  "theme": "dark"
}
```

> 注意：请求体为自由 JSON，字段定义由前端自行设计。

---

## 4. 创建系统工作区

**POST** `/api/v1/dashboard/workspaces`

**权限**: `dashboard:workspace:manage`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `name` | string | ✅ | 工作区名称 |
| `description` | string | ❌ | 描述 |
| `config` | object | ✅ | 工作区配置（自由 JSON） |

---

## 5. 获取系统工作区列表

**GET** `/api/v1/dashboard/workspaces`

**权限**: `dashboard:view`

返回当前用户可见的系统工作区。

- 有 `dashboard:workspace:manage` 权限的管理员：返回当前租户全部系统工作区。
- 普通用户：只返回当前用户有权看到的系统工作区（含默认系统工作区）。

---

## 6. 更新系统工作区

**PUT** `/api/v1/dashboard/workspaces/:id`

**权限**: `dashboard:workspace:manage`

仅允许更新白名单字段：`name`、`description`、`config`。

业务约束：

- 默认系统工作区不能删除。
- `is_default`、`is_readonly` 等保护字段由后端控制，前端传入会被拒绝。

---

## 7. 删除系统工作区

**DELETE** `/api/v1/dashboard/workspaces/:id`

**权限**: `dashboard:workspace:manage`

---

## 8. 获取角色关联的工作区

**GET** `/api/v1/dashboard/roles/:roleId/workspaces`

**权限**: `dashboard:workspace:manage`

获取指定角色当前生效的工作区列表，默认系统工作区由后端自动包含。

---

## 9. 分配角色工作区

**PUT** `/api/v1/dashboard/roles/:roleId/workspaces`

**权限**: `dashboard:workspace:manage`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `workspace_ids` | []uuid | ✅ | 工作区 ID 列表（全量替换） |
