# 用户偏好设置 API 文档

**路径前缀**: `/api/v1/user/preferences`  
**权限**: 已登录用户（数据按用户隔离）

---

## 1. 获取当前用户偏好设置

**GET** `/api/v1/user/preferences`

**权限**: 无特殊要求（已登录即可）

### 响应

```json
{
  "code": 0,
  "data": {
    "user_id": "uuid",
    "preferences": {
      "theme": "dark",
      "language": "zh-CN",
      "sidebar_collapsed": false
    },
    "updated_at": "2026-02-19T09:00:00Z"
  }
}
```

> 若用户尚未设置任何偏好，返回 `preferences: {}`（空对象）。

---

## 2. 全量更新偏好设置

**PUT** `/api/v1/user/preferences`

**权限**: 无特殊要求

全量覆盖当前用户的偏好设置（Upsert）。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `preferences` | object | ✅ | 偏好设置（自由 JSON 对象） |

```json
{
  "preferences": {
    "theme": "dark",
    "language": "zh-CN",
    "sidebar_collapsed": false,
    "default_page_size": 20
  }
}
```

> 该接口完全替换原有偏好，如需保留旧值请使用 PATCH 接口。

---

## 3. 部分更新偏好设置

**PATCH** `/api/v1/user/preferences`

**权限**: 无特殊要求

以合并（merge）方式更新偏好设置，只更新请求中包含的字段，其余字段保持不变。

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `preferences` | object | ✅ | 需要更新的偏好字段（自由 JSON 对象） |

```json
{
  "preferences": {
    "theme": "light"
  }
}
```
