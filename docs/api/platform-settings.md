# 平台设置 API 文档

**路径前缀**: `/api/v1/platform/settings`  
**权限**: `platform:settings:manage`

---

## 1. 获取所有设置项

**GET** `/api/v1/platform/settings`

**权限**: `platform:settings:manage`

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `module` | string | ❌ | 按模块过滤，如 `site_message`、`smtp`、`security` |

### 响应

```json
{
  "code": 0,
  "data": [
    {
      "module": "site",
      "settings": [
        {
          "key": "site_name",
          "value": "Auto-Healing 平台",
          "description": "平台名称",
          "type": "string",
          "updated_at": "2026-02-18T10:00:00Z"
        }
      ]
    }
  ]
}
```

---

## 2. 更新设置项

**PUT** `/api/v1/platform/settings/:key`

**权限**: `platform:settings:manage`

### 请求体

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `value` | string | ✅ | 设置值（统一以字符串传递） |

### 示例

```bash
curl -X PUT /api/v1/platform/settings/site_name \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"value": "我的自愈平台"}'
```

---

## 常见设置项

| Key | 类型 | 说明 |
|-----|------|------|
| `site_name` | string | 平台名称 |
| `site_logo` | string | 平台 Logo URL |
| `max_concurrent_executions` | integer | 最大并发执行数 |
| `execution_timeout_seconds` | integer | 执行超时时间（秒） |
| `audit_log_retention_days` | integer | 审计日志保留天数 |
| `allow_self_registration` | boolean | 是否允许自助注册 |
| `smtp_host` | string | 全局 SMTP 服务器 |
| `smtp_port` | integer | SMTP 端口 |
