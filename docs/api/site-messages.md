# 站内信 API 文档

**路径前缀**: `/api/v1/site-messages`  
**权限**: 已登录用户

---

## 1. 获取未读消息数

**GET** `/api/v1/site-messages/unread-count`

**权限**: 无特殊要求（已登录即可）

### 响应

```json
{
  "code": 0,
  "data": {
    "unread_count": 5
  }
}
```

---

## 2. 获取消息分类列表

**GET** `/api/v1/site-messages/categories`

**权限**: 无特殊要求

返回所有可用的消息分类枚举值。

---

## 3. 获取消息接收设置

**GET** `/api/v1/site-messages/settings`

**权限**: 无特殊要求

### 响应

```json
{
  "code": 0,
  "data": {
    "retention_days": 90,
    "updated_at": "2026-02-18T10:00:00Z"
  }
}
```

---

## 4. 更新消息接收设置

**PUT** `/api/v1/site-messages/settings`

**权限**: `site-message:create`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `retention_days` | int | ✅ | 消息保留天数（1-3650） |

---

## 5. 标记消息为已读

**PUT** `/api/v1/site-messages/read`

**权限**: 无特殊要求

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `ids` | []uuid | ✅ | 消息 ID 列表 |

---

## 6. 标记所有消息为已读

**PUT** `/api/v1/site-messages/read-all`

**权限**: 无特殊要求

---

## 7. 获取消息列表

**GET** `/api/v1/site-messages`

**权限**: 无特殊要求

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `page` | int | ❌ | 页码，默认 1 |
| `page_size` | int | ❌ | 每页数量，默认 10，最大 100 |
| `keyword` | string | ❌ | 模糊搜索（标题、内容） |
| `category` | string | ❌ | 消息分类（见分类枚举） |

### 响应

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "title": "执行任务成功",
        "content": "任务「磁盘清理」执行成功",
        "category": "execution",
        "is_read": false,
        "link": "/execution-runs/uuid",
        "created_at": "2026-02-18T09:00:00Z"
      }
    ],
    "total": 10,
    "page": 1,
    "page_size": 20
  }
}
```

---

## 8. 创建消息（管理员）

**POST** `/api/v1/site-messages`

**权限**: `site-message:create`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `category` | string | ✅ | 分类（见分类枚举） |
| `title` | string | ✅ | 消息标题 |
| `content` | string | ✅ | 消息内容 |

---

## 消息分类枚举

| 分类 | 说明 |
|------|------|
| `execution` | 执行任务相关 |
| `healing` | 自愈流程相关 |
| `approval` | 审批任务相关 |
| `system` | 系统通知 |
| `alert` | 告警通知 |

---

## 9. SSE 实时推送（新消息通知）

**GET** `/api/v1/tenant/site-messages/events?token=xxx`

**权限**: 已登录（通过 URL query 传 token）  
**协议**: Server-Sent Events (SSE) 长连接

### 说明

前端通过 `EventSource` 连接此端点，后端会实时推送新消息通知。收到通知后，前端自行调 `GET /unread-count` 刷新角标即可。

- **心跳**: 每 30 秒发送 `: heartbeat` 注释保活
- **断线重连**: 浏览器 `EventSource` 原生自动重连，无需手动处理
- **多客户端**: 同一用户可同时有多个浏览器/标签页连接

### 事件类型

| 事件名 | 触发时机 | data 格式 |
|--------|----------|-----------|
| `init` | 连接建立后立即推送 | `{"type":"init","unread_count":5}` |
| `new_message` | 有新站内信创建时 | `{"type":"new_message"}` |

### 前端接入示例

```typescript
// 建议在全局 Layout 层建立连接（登录后）
const token = localStorage.getItem('access_token');
const es = new EventSource(`/api/v1/tenant/site-messages/events?token=${token}`);

// 1. 连接建立 → 获取初始未读数
es.addEventListener('init', (e) => {
  const { unread_count } = JSON.parse(e.data);
  setUnreadCount(unread_count); // 设置角标
});

// 2. 新消息 → 刷新未读数
es.addEventListener('new_message', () => {
  // 调接口刷新未读数
  fetchUnreadCount().then(count => setUnreadCount(count));
  // 可选：显示桌面通知 / 消息提示
});

// 3. 连接错误处理（可选，浏览器会自动重连）
es.onerror = () => {
  console.warn('SSE 连接断开，浏览器将自动重连');
};

// 4. 页面卸载时关闭连接
window.addEventListener('beforeunload', () => es.close());
```

### 注意事项

1. **token 传递**: SSE 使用 `EventSource`，不支持自定义 Header，所以通过 URL query `?token=xxx` 传递 JWT token（后端已支持）
2. **保留轮询兜底**: 建议保留现有 60 秒轮询作为降级方案，SSE 连接期间可将轮询间隔拉长到 5 分钟
3. **登出清理**: 用户退出登录时需调用 `es.close()` 关闭连接
4. **Token 过期**: token 过期后 SSE 连接会断开（401），前端应在 `onerror` 中检测并刷新 token 后重新建立连接

### curl 测试

```bash
# 获取 token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

# 连接 SSE（会立即看到 init 事件，然后每 30 秒心跳）
curl -N "http://localhost:8080/api/v1/tenant/site-messages/events?token=$TOKEN"

# 另开终端发消息，上面的连接会立即收到 new_message 事件
curl -X POST http://localhost:8080/api/v1/platform/site-messages \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"category":"system_update","title":"测试","content":"SSE测试"}'
```
