# 用户活动 API 文档（收藏 & 最近访问）

**Base URL**: `/api/v1/user`  
**权限**: 已登录用户（数据按用户 + 当前租户上下文隔离；平台用户仅在 Impersonation 场景下具备租户上下文）

---

## 收藏（Favorites）

### 1. 获取收藏列表

**GET** `/api/v1/common/user/favorites`

#### 响应

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "menu_key": "plugin:uuid",
      "name": "Zabbix 监控",
      "path": "/plugins/uuid",
      "created_at": "2026-02-18T10:00:00Z"
    }
  ]
}
```

> 注意：响应为直接数组（非分页列表），无 `total`/`page`/`page_size` 字段。

---

### 2. 添加收藏

**POST** `/api/v1/common/user/favorites`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `menu_key` | string | ✅ | 菜单唯一标识（如 `plugin:uuid`、`healing_flow:uuid`） |
| `name` | string | ✅ | 菜单显示名称 |
| `path` | string | ✅ | 前端路由路径 |

```json
{
  "menu_key": "plugin:uuid",
  "name": "Zabbix 监控",
  "path": "/plugins/uuid"
}
```

> 同一 `menu_key` 重复收藏返回 409 Conflict。

---

### 3. 取消收藏

**DELETE** `/api/v1/common/user/favorites/:menu_key`

#### 路径参数

| 参数 | 类型 | 说明 |
|------|------|------|
| `menu_key` | string | 菜单唯一标识（需 URL 编码） |

#### 响应

```json
{
  "code": 0,
  "message": "取消收藏成功"
}
```

---

## 最近访问（Recents）

### 4. 获取最近访问列表

**GET** `/api/v1/common/user/recents`

#### 响应

```json
{
  "code": 0,
  "data": [
    {
      "id": "uuid",
      "user_id": "uuid",
      "menu_key": "healing_flow:uuid",
      "name": "磁盘告警自愈流程",
      "path": "/healing/flows/uuid",
      "visited_at": "2026-02-18T10:00:00Z"
    }
  ]
}
```

> 注意：响应为直接数组（非分页列表），无 `total`/`page`/`page_size` 字段。

---

### 5. 记录最近访问

**POST** `/api/v1/common/user/recents`

#### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `menu_key` | string | ✅ | 菜单唯一标识 |
| `name` | string | ✅ | 菜单显示名称 |
| `path` | string | ✅ | 前端路由路径 |

> 系统使用 Upsert 策略，若 `menu_key` 已存在则更新 `visited_at`；每个用户最多保留 50 条记录，超出时自动淘汰最旧的记录。
