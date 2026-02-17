# 站内信（Site Messages）API 接口文档

> **Base URL**: `http://localhost:8080/api/v1`
> **认证方式**: Bearer Token（所有接口需要登录）
> **日期**: 2026-02-18

---

## 目录

1. [数据模型](#数据模型)
2. [消息分类枚举](#1-获取消息分类枚举)
3. [消息列表查询](#2-查询消息列表)
4. [未读数量](#3-获取未读消息数量)
5. [批量标记已读](#4-批量标记已读)
6. [全部标记已读](#5-全部标记已读)
7. [创建消息](#6-创建站内信管理员)
8. [获取设置](#7-获取站内信设置)
9. [修改设置](#8-修改站内信设置管理员)
10. [前端集成建议](#前端集成建议)

---

## 统一响应格式

所有接口使用统一的 JSON 响应结构：

```typescript
interface ApiResponse<T> {
  code: number;       // 0 = 成功, 非0 = 错误
  message: string;    // 提示信息
  data?: T;           // 业务数据
  total?: number;     // 列表总数（仅分页接口）
  page?: number;      // 当前页码（仅分页接口）
  page_size?: number; // 每页数量（仅分页接口）
}
```

**错误码**:
| 错误码 | HTTP 状态码 | 说明 |
|--------|-------------|------|
| 0 | 200/201 | 成功 |
| 40000 | 400 | 请求参数错误 |
| 40100 | 401 | 未认证 |
| 40300 | 403 | 无权限 |
| 50000 | 500 | 服务器内部错误 |

---

## 数据模型

### SiteMessage — 站内信消息

```typescript
interface SiteMessage {
  id: string;              // UUID，消息唯一标识
  category: string;        // 消息分类（见枚举）
  title: string;           // 标题
  content: string;         // 正文内容（HTML 格式）
  created_at: string;      // 创建时间 ISO 8601
  expires_at?: string;     // 过期时间 ISO 8601（可选）
  is_read: boolean;        // 当前用户是否已读
}
```

### SiteMessageCategory — 消息分类

```typescript
interface SiteMessageCategory {
  value: string;  // 分类标识
  label: string;  // 中文名称
}
```

**固定分类值**:
| value | label | 建议图标/颜色 |
|-------|-------|---------------|
| `system_update` | 系统更新 | 🔄 蓝色 |
| `fault_alert` | 故障通知 | ⚠️ 红色 |
| `service_notice` | 服务消息 | 📋 绿色 |
| `product_news` | 产品消息 | 🎉 紫色 |
| `activity` | 活动通知 | 🎯 橙色 |
| `security` | 安全公告 | 🔒 红色/橙色 |

### SiteMessageSettings — 站内信设置

```typescript
interface SiteMessageSettings {
  id: string;              // UUID
  retention_days: number;  // 消息保留天数（默认 90）
  updated_at: string;      // 最后修改时间 ISO 8601
}
```

---

## 接口详情

### 1. 获取消息分类枚举

返回所有可用的消息分类列表，用于筛选下拉框。

**请求**:
```
GET /api/v1/site-messages/categories
Authorization: Bearer <token>
```

**参数**: 无

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    { "value": "system_update", "label": "系统更新" },
    { "value": "fault_alert", "label": "故障通知" },
    { "value": "service_notice", "label": "服务消息" },
    { "value": "product_news", "label": "产品消息" },
    { "value": "activity", "label": "活动通知" },
    { "value": "security", "label": "安全公告" }
  ]
}
```

---

### 2. 查询消息列表

支持分页、关键词搜索（标题模糊匹配）和分类筛选。已过期消息自动排除。

**请求**:
```
GET /api/v1/site-messages
Authorization: Bearer <token>
```

**Query 参数**:
| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `page` | int | 否 | 1 | 页码，从 1 开始 |
| `page_size` | int | 否 | 10 | 每页数量，最大 100 |
| `keyword` | string | 否 | - | 按标题模糊搜索（不区分大小写） |
| `category` | string | 否 | - | 按分类筛选（需传入 value 值） |

**请求示例**:
```
GET /api/v1/site-messages?page=1&page_size=10&keyword=安全&category=security
```

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "id": "9a65653b-3588-4671-ada0-a3360b6abeb2",
      "category": "security",
      "title": "安全公告：请立即更新访问密钥",
      "content": "<p>🔒 <b>安全等级：高</b></p><p>我们检测到部分用户的 API 访问密钥可能存在泄露风险。</p>...",
      "created_at": "2026-02-18T00:53:32.689+08:00",
      "expires_at": "2026-05-19T00:53:32.688+08:00",
      "is_read": false
    },
    {
      "id": "dc81e762-1050-40ba-99a4-1cab4421bf48",
      "category": "security",
      "title": "安全策略更新：密码复杂度要求提升",
      "content": "<p>为提升平台安全性，自 <b>2026年3月1日</b> 起...</p>",
      "created_at": "2026-02-11T00:53:32.689+08:00",
      "expires_at": "2026-05-19T00:53:32.688+08:00",
      "is_read": true
    }
  ],
  "total": 2,
  "page": 1,
  "page_size": 10
}
```

**注意事项**:
- `content` 为 **HTML** 格式，前端需使用 `dangerouslySetInnerHTML` 或类似方式渲染
- `is_read` 为当前登录用户的已读状态
- 结果按 `created_at` 降序排列（最新的在前）
- 已过期的消息（`expires_at < 当前时间`）不会返回

---

### 3. 获取未读消息数量

用于导航栏铃铛显示未读角标，轻量接口，建议 **60 秒轮询**一次。

**请求**:
```
GET /api/v1/site-messages/unread-count
Authorization: Bearer <token>
```

**参数**: 无

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "unread_count": 15
  }
}
```

**前端使用建议**:
```typescript
// 轮询示例
const pollInterval = 60000; // 60 秒

const fetchUnreadCount = async () => {
  const res = await request.get('/api/v1/site-messages/unread-count');
  return res.data.unread_count; // number
};
```

---

### 4. 批量标记已读

将指定的消息标记为当前用户已读。支持单条或多条同时操作。

**请求**:
```
PUT /api/v1/site-messages/read
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体**:
```json
{
  "ids": [
    "9a65653b-3588-4671-ada0-a3360b6abeb2",
    "dc81e762-1050-40ba-99a4-1cab4421bf48"
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | string[] | 是 | 消息 UUID 数组，不能为空 |

**响应**:
```json
{
  "code": 0,
  "message": "标记已读成功"
}
```

**注意事项**:
- 重复标记不会报错（幂等操作）
- 标记后应刷新消息列表和未读计数

---

### 5. 全部标记已读

将当前用户所有未读消息标记为已读。

**请求**:
```
PUT /api/v1/site-messages/read-all
Authorization: Bearer <token>
```

**请求体**: 无（不需要 body）

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "marked_count": 13
  }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `marked_count` | number | 本次标记的消息数量 |

---

### 6. 创建站内信（管理员）

> 需要 `site-message:create` 权限（admin / super_admin 角色拥有）

**请求**:
```
POST /api/v1/site-messages
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体**:
```json
{
  "category": "system_update",
  "title": "平台 v3.3.0 版本发布",
  "content": "<h3>更新内容</h3><p>新增<b>站内信</b>功能...</p>"
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `category` | string | 是 | 消息分类，必须为合法枚举值 |
| `title` | string | 是 | 消息标题 |
| `content` | string | 是 | 消息正文（支持 HTML） |

**响应** (HTTP 201):
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "category": "system_update",
    "title": "平台 v3.3.0 版本发布",
    "content": "<h3>更新内容</h3><p>新增<b>站内信</b>功能...</p>",
    "created_at": "2026-02-18T00:57:30.123+08:00",
    "expires_at": "2026-05-19T00:57:30.123+08:00"
  }
}
```

**注意事项**:
- `expires_at` 根据当前系统设置的 `retention_days` 自动计算
- 无权限用户调用会返回 403

---

### 7. 获取站内信设置

获取全局站内信配置，当前仅包含消息保留天数。

**请求**:
```
GET /api/v1/site-messages/settings
Authorization: Bearer <token>
```

**参数**: 无

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "7baa3e38-8a0e-4a10-b53c-4822a4f65e36",
    "retention_days": 90,
    "updated_at": "2026-02-18T00:54:43.003909+08:00"
  }
}
```

---

### 8. 修改站内信设置（管理员）

> 需要 `site-message:create` 权限

**请求**:
```
PUT /api/v1/site-messages/settings
Authorization: Bearer <token>
Content-Type: application/json
```

**请求体**:
```json
{
  "retention_days": 60
}
```

| 字段 | 类型 | 必填 | 范围 | 说明 |
|------|------|------|------|------|
| `retention_days` | int | 是 | 1 ~ 3650 | 消息保留天数 |

**响应**:
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "id": "7baa3e38-8a0e-4a10-b53c-4822a4f65e36",
    "retention_days": 60,
    "updated_at": "2026-02-18T00:57:39.228+08:00"
  }
}
```

**注意事项**:
- 修改 `retention_days` 只影响**新创建**的消息，已有消息的 `expires_at` 不变
- 范围限制 1 ~ 3650 天（约 10 年），超出范围返回 400

---

## 前端集成建议

### 1. 导航栏铃铛（未读角标）

```
┌─────────────────────────────────────────┐
│  Logo   菜单...              🔔(3)  头像 │
└─────────────────────────────────────────┘
```

- 调用 `GET /site-messages/unread-count`，**60 秒轮询**
- 当 `unread_count > 0` 时显示红色角标
- 当 `unread_count > 99` 时显示 `99+`
- 点击铃铛跳转到站内信列表页

### 2. 消息列表页

```
┌─────────────────────────────────────────┐
│  站内信                                  │
│  ┌──────────┐ ┌──────────┐ ┌─────────┐  │
│  │ 搜索框    │ │ 分类筛选  │ │全部已读  │  │
│  └──────────┘ └──────────┘ └─────────┘  │
│                                          │
│  🔴 安全公告：请立即更新访问密钥      1小时前│
│  🔴 生产环境数据库连接异常告警       30分钟前│
│  ⚪ 平台 v3.2.0 版本发布公告         2小时前│
│  ⚪ 您的定时任务执行报告             12小时前│
│                                          │
│  ← 1 / 3 →                              │
└─────────────────────────────────────────┘
```

- `GET /site-messages?page=1&page_size=10` 分页加载
- 搜索框输入后拼接 `keyword` 参数
- 分类下拉框通过 `GET /categories` 获取选项，选择后拼接 `category` 参数
- 🔴 表示 `is_read: false`，⚪ 表示 `is_read: true`
- 点击消息展开详情（`content` 用 HTML 渲染）
- 展开时调用 `PUT /read` 标记该条为已读
- 「全部已读」按钮调用 `PUT /read-all`

### 3. 管理员发送消息（可选页面）

- 需要 `site-message:create` 权限
- 表单：分类下拉 + 标题输入 + 富文本编辑器（content 为 HTML）
- 提交调用 `POST /site-messages`

### 4. 设置页面（可选）

- 管理员可调整消息保留天数
- `GET /settings` 获取当前值
- `PUT /settings` 修改

### 5. TypeScript 类型定义参考

```typescript
// typings.d.ts

declare namespace SiteMessage {
  /** 消息分类 */
  interface Category {
    value: string;
    label: string;
  }

  /** 站内信消息 */
  interface Message {
    id: string;
    category: string;
    title: string;
    content: string;
    created_at: string;
    expires_at?: string;
    is_read: boolean;
  }

  /** 设置 */
  interface Settings {
    id: string;
    retention_days: number;
    updated_at: string;
  }

  /** 创建消息请求 */
  interface CreateRequest {
    category: string;
    title: string;
    content: string;
  }

  /** 批量标记已读请求 */
  interface MarkReadRequest {
    ids: string[];
  }

  /** 修改设置请求 */
  interface UpdateSettingsRequest {
    retention_days: number;
  }

  /** 未读数量响应 */
  interface UnreadCountResponse {
    unread_count: number;
  }

  /** 全部已读响应 */
  interface MarkAllReadResponse {
    marked_count: number;
  }
}
```

---

## curl 测试命令速查

```bash
# 获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

# 1. 分类枚举
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/site-messages/categories" | jq '.'

# 2. 消息列表
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/site-messages?page=1&page_size=10" | jq '.'

# 3. 搜索 + 分类筛选
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/site-messages?keyword=安全&category=security" | jq '.'

# 4. 未读数量
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/site-messages/unread-count" | jq '.'

# 5. 批量标记已读
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"ids":["<message-id-1>","<message-id-2>"]}' \
  "http://localhost:8080/api/v1/site-messages/read" | jq '.'

# 6. 全部标记已读
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/site-messages/read-all" | jq '.'

# 7. 创建消息（需 admin 权限）
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"category":"system_update","title":"测试消息","content":"<p>Hello</p>"}' \
  "http://localhost:8080/api/v1/site-messages" | jq '.'

# 8. 获取设置
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/site-messages/settings" | jq '.'

# 9. 修改过期天数
curl -s -X PUT -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"retention_days":60}' \
  "http://localhost:8080/api/v1/site-messages/settings" | jq '.'
```
