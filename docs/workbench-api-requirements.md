# 工作台（Workbench）后端接口需求

> 前端页面：`src/pages/workbench/index.tsx`
> 当前状态：**全部使用硬编码 mock 数据**，需要后端实现以下接口

---

## 一、已有接口（可直接复用，无需开发）

| 模块 | 接口 | 说明 |
|---|---|---|
| 工单统计 | `GET /api/v1/incidents/stats` | 待处理/近7天总计 |
| 纳管主机统计 | `GET /api/v1/cmdb/stats` | 在线/离线数量 |
| 自愈流程统计 | `GET /api/v1/healing/flows/stats` | 总数/已启用数 |
| 自愈规则统计 | `GET /api/v1/healing/rules/stats` | 总数/已启用数 |
| 自愈实例统计 | `GET /api/v1/healing/instances/stats` | 今日成功/失败 |
| 执行运行统计 | `GET /api/v1/execution-runs/stats` | 执行成功/失败 |
| 待办审批列表 | `GET /api/v1/healing/approvals/pending` | 待审批列表 |
| 审计日志（变更记录） | `GET /api/v1/audit-logs` | 最近变更操作 |
| 定时任务列表 | `GET /api/v1/execution-schedules` | 定时任务 |
| 定时任务时间线 | `GET /api/v1/execution-schedules/timeline` | 日历展示用 |

---

## 二、需要新开发的接口

---

### 接口 1（P0）：工作台综合概览

```
GET /api/v1/common/workbench/overview
```

**说明**：聚合接口，一次性返回工作台核心统计数据。避免前端发 8+ 个并发请求。

**请求参数**：无（根据当前登录用户 + 租户自动过滤）

**响应格式**：

```json
{
  "code": 0,
  "message": "success",
  "data": {
    "system_health": {
      "status": "healthy",
      "version": "v2.5.0",
      "uptime_seconds": 1036800,
      "environment": "production",
      "api_latency_ms": 23,
      "db_latency_ms": 5
    },
    "healing_stats": {
      "today_success": 12,
      "today_failed": 1
    },
    "incident_stats": {
      "pending_count": 3,
      "last_7_days_total": 45
    },
    "host_stats": {
      "online_count": 145,
      "offline_count": 3
    },
    "resource_overview": {
      "flows": { "total": 18, "enabled": 5 },
      "rules": { "total": 24, "enabled": 18 },
      "hosts": { "total": 148, "offline": 3 },
      "playbooks": { "total": 36, "needs_review": 4 },
      "schedules": { "total": 12, "enabled": 10 },
      "notification_templates": { "total": 8, "channels": 3 },
      "secrets": { "total": 15, "types": "SSH + API" },
      "users": { "total": 12, "admins": 3 }
    }
  }
}
```

**字段说明**：

| 字段 | 说明 |
|---|---|
| `system_health.status` | 枚举：`healthy` / `degraded` / `down` |
| `system_health.uptime_seconds` | 系统运行秒数，前端格式化为 "X天 X小时" |
| `system_health.api_latency_ms` | 近期平均 API 响应延迟 |
| `system_health.db_latency_ms` | 数据库连接延迟 |
| `healing_stats` | 今日自愈执行成功/失败次数 |
| `incident_stats` | 工单：待处理数 + 近7天总计 |
| `host_stats` | CMDB 主机在线/离线数 |
| `resource_overview` | 各模块资源总数 + 关键子状态 |

> 如果聚合太复杂，也可以拆成 3 个独立接口：
> - `GET /api/v1/common/workbench/system-health`
> - `GET /api/v1/common/workbench/healing-stats`
> - `GET /api/v1/common/workbench/resource-overview`

---

### 接口 2（P1）：活动动态

```
GET /api/v1/common/workbench/activities
```

**说明**：获取当前租户下最近的活动动态（跨模块混合时间线）

**请求参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `limit` | int | 否 | 10 | 返回条数 |

**响应格式**：

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "type": "execution",
        "text": "执行完成：Web服务健康检查",
        "created_at": "2026-02-21T22:03:00Z"
      },
      {
        "id": "uuid",
        "type": "flow",
        "text": "创建流程：数据库自动备份",
        "created_at": "2026-02-21T21:50:00Z"
      }
    ]
  }
}
```

**type 枚举**：

| type | 含义 | 来源 |
|---|---|---|
| `execution` | 执行任务相关 | execution_runs 表 |
| `flow` | 自愈流程变更 | healing_flows 审计 |
| `rule` | 规则变更 | healing_rules 审计 |
| `system` | 系统通知/插件更新 | 系统事件 |

> 建议：可以直接从 `audit_logs` 表查询最近 N 条，按 resource_type 分类映射为上述 type。

---

### 接口 3（P1）：定时任务日历

```
GET /api/v1/common/workbench/schedule-calendar
```

**说明**：获取指定月份的定时任务日历数据，展示每天有哪些定时任务。

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `year` | int | 是 | 年份，如 2026 |
| `month` | int | 是 | 月份，如 2 |

**响应格式**：

```json
{
  "code": 0,
  "data": {
    "dates": {
      "2026-02-17": [
        { "name": "Web服务健康检查", "time": "00:00", "schedule_id": "uuid" },
        { "name": "日志归档", "time": "02:00", "schedule_id": "uuid" },
        { "name": "数据库备份", "time": "03:00", "schedule_id": "uuid" }
      ],
      "2026-02-18": [
        { "name": "Web服务健康检查", "time": "00:00", "schedule_id": "uuid" },
        { "name": "SSL证书检查", "time": "06:00", "schedule_id": "uuid" }
      ]
    }
  }
}
```

**实现说明**：
- 从 `execution_schedules` 表取出所有 `enabled=true` 的定时任务
- 根据 cron 表达式计算指定月份每天的执行时间
- 只返回有任务的日期，无任务的日期不含在 `dates` 中

---

### 接口 4（P2）：系统公告

```
GET /api/v1/common/workbench/announcements
```

**说明**：获取系统公告列表（版本更新说明、维护计划通知等）

**请求参数**：

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|---|---|---|---|---|
| `limit` | int | 否 | 5 | 返回条数 |

**响应格式**：

```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": "uuid",
        "title": "v2.5.0 版本发布更新说明",
        "content": "...",
        "created_at": "2026-02-12T00:00:00Z"
      },
      {
        "id": "uuid",
        "title": "系统维护计划通知",
        "content": "...",
        "created_at": "2026-02-10T00:00:00Z"
      }
    ]
  }
}
```

**实现建议**：
- 方案 A：新建 `announcements` 表（id, title, content, created_at, updated_at）
- 方案 B：复用 `site_messages` 表，新增 `type = 'announcement'` 类型

---

### 接口 5（P2）：我的收藏

#### 5.1 获取收藏

```
GET /api/v1/common/workbench/favorites
```

**请求参数**：无

**响应格式**：

```json
{
  "code": 0,
  "data": {
    "items": [
      { "key": "cmdb", "label": "资产管理", "icon": "DatabaseOutlined", "path": "/cmdb" },
      { "key": "rules", "label": "自愈规则", "icon": "ToolOutlined", "path": "/healing/rules" },
      { "key": "flows", "label": "自愈流程", "icon": "ThunderboltOutlined", "path": "/healing/flows" }
    ]
  }
}
```

#### 5.2 更新收藏

```
PUT /api/v1/common/workbench/favorites
```

**请求 Body**：

```json
{
  "items": [
    { "key": "cmdb", "label": "资产管理", "icon": "DatabaseOutlined", "path": "/cmdb" },
    { "key": "rules", "label": "自愈规则", "icon": "ToolOutlined", "path": "/healing/rules" }
  ]
}
```

**响应**：

```json
{
  "code": 0,
  "message": "success"
}
```

**实现建议**：存储在 `user_preferences` 表中，key 为 `workbench_favorites`，value 为 JSON。

---

## 三、开发优先级

| 优先级 | 接口 | 原因 |
|---|---|---|
| 🔴 P0 | `GET /api/v1/common/workbench/overview` | 工作台核心，没它页面全是空的 |
| 🟡 P1 | `GET /api/v1/common/workbench/activities` | 活动动态是运维人员高频查看区域 |
| 🟡 P1 | `GET /api/v1/common/workbench/schedule-calendar` | 定时任务日历是重要展示模块 |
| 🟢 P2 | `GET /api/v1/common/workbench/announcements` | 系统公告可以后期补充 |
| 🟢 P2 | `GET/PUT /api/v1/common/workbench/favorites` | 收藏暂时可硬编码，后续个性化 |

---

## 四、数据库表（如需新建）

### announcements（如果走方案 A）

```sql
CREATE TABLE announcements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title VARCHAR(255) NOT NULL,
    content TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

### user_preferences 扩展（收藏功能）

如果 `user_preferences` 表已存在，只需插入一条记录：

```sql
-- key: workbench_favorites
-- value: JSON 格式的收藏列表
```
