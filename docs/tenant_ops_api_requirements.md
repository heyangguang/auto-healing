# 租户运营模块 — 后端接口需求文档

> **前端页面**：平台管理 → 租户运营总览 / 租户运营明细
> **当前状态**：部分已实现 (`GET /api/v1/platform/tenants/stats`)，缺失多个资源维度的统计和趋势数据
> **权限要求**：所有接口均需 `super_admin` 或 `platform_admin` 角色

---

## 一、现有接口增强（高优先级）

### 1. `GET /api/v1/platform/tenants/stats` — 租户运营统计

> [!IMPORTANT]
> 该接口已在后端实现，但返回字段不足。需要在 **现有接口上扩展**，不需要新增路由。

#### 1.1 `TenantStatsItem` 需要新增的字段

| 字段名 | 类型 | 来源表 | 说明 |
|--------|------|--------|------|
| `cmdb_count` | `int64` | `cmdb_hosts` | CMDB 主机数量 |
| `git_count` | `int64` | `git_repositories` | Git 仓库数量 |
| `playbook_count` | `int64` | `playbooks` | Playbook 数量 |
| `secret_count` | `int64` | `secrets` | 凭据密钥数量 |
| `plugin_count` | `int64` | `plugins` | 插件数量 |
| `incident_count` | `int64` | `incidents` | 工单/告警数量 |
| `flow_count` | `int64` | `healing_flows` | 自愈流程数量 |
| `schedule_count` | `int64` | `scheduled_tasks` | 定时任务数量 |
| `notification_channel_count` | `int64` | `notification_channels` | 通知渠道数量 |
| `notification_template_count` | `int64` | `notification_templates` | 通知模板数量 |
| `healing_success_count` | `int64` | `healing_instances` | 自愈成功次数（`status = 'success'`） |
| `healing_total_count` | `int64` | `healing_instances` | 自愈总执行次数 |
| `incident_covered_count` | `int64` | `incidents` | 被自愈规则覆盖的工单数（有关联 healing_rule 的工单） |

> [!NOTE]
> 现有的 `CountTenantTable` 通用方法可以直接复用统计大部分表的记录数，只需在 handler 里多调用几次。
> `healing_success_count` 和 `incident_covered_count` 需要带条件查询（WHERE status/条件），可能需要新的 repository 方法。

#### 1.2 实现建议

```go
// 在 GetTenantStats handler 中，对每个 tenant 补充以下调用：
item.CmdbCount = h.repo.CountTenantTable(ctx, tenant.ID, "cmdb_hosts")
item.GitCount = h.repo.CountTenantTable(ctx, tenant.ID, "git_repositories")
item.PlaybookCount = h.repo.CountTenantTable(ctx, tenant.ID, "playbooks")
item.SecretCount = h.repo.CountTenantTable(ctx, tenant.ID, "secrets")
item.PluginCount = h.repo.CountTenantTable(ctx, tenant.ID, "plugins")
item.IncidentCount = h.repo.CountTenantTable(ctx, tenant.ID, "incidents")
item.FlowCount = h.repo.CountTenantTable(ctx, tenant.ID, "healing_flows")
item.ScheduleCount = h.repo.CountTenantTable(ctx, tenant.ID, "scheduled_tasks")
item.NotificationChannelCount = h.repo.CountTenantTable(ctx, tenant.ID, "notification_channels")
item.NotificationTemplateCount = h.repo.CountTenantTable(ctx, tenant.ID, "notification_templates")

// === 需要新 repository 方法 ===
item.HealingSuccessCount = h.repo.CountTenantTableWhere(ctx, tenant.ID, "healing_instances", "status = 'success'")
item.HealingTotalCount = item.InstanceCount  // 复用已有的 instance_count
item.IncidentCoveredCount = h.repo.CountTenantTableWhere(ctx, tenant.ID, "incidents", "healing_rule_id IS NOT NULL")
```

> [!WARNING]
> 表名请以实际数据库中的表名为准，上面列出的表名仅供参考。后端工程师需要确认每个资源对应的准确表名。
> `incidents` 表如果不存在，`CountTenantTable` 已经有表存在性检查，会返回 0。

#### 1.3 预期响应格式（增强后）

```json
{
  "data": {
    "tenants": [
      {
        "id": "uuid",
        "name": "生产运维中心",
        "code": "prod-ops",
        "status": "active",
        "icon": "",
        "member_count": 28,
        "rule_count": 15,
        "instance_count": 142,
        "template_count": 12,
        "audit_log_count": 1580,
        "last_activity_at": "2026-02-23 21:30:00",
        "cmdb_count": 356,
        "git_count": 8,
        "playbook_count": 24,
        "secret_count": 18,
        "plugin_count": 6,
        "incident_count": 45,
        "flow_count": 9,
        "schedule_count": 7,
        "notification_channel_count": 4,
        "notification_template_count": 8,
        "healing_success_count": 128,
        "healing_total_count": 142,
        "incident_covered_count": 41
      }
    ],
    "summary": {
      "total_tenants": 8,
      "active_tenants": 7,
      "disabled_tenants": 1,
      "total_users": 141,
      "total_rules": 78,
      "total_instances": 631,
      "total_templates": 64
    }
  }
}
```

---

## 二、新增接口（中优先级）

### 2. `GET /api/v1/platform/tenants/trends` — 运营趋势数据

> 前端当前使用 Mock 数据展示 **近 7 天** 的三条折线图（操作趋势、审计趋势、任务执行趋势）。需要后端提供真实的趋势数据。

#### 2.1 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `days` | `int` | 否 | `7` | 统计天数，支持 7 / 14 / 30 |

#### 2.2 响应格式

```json
{
  "data": {
    "dates": ["02/17", "02/18", "02/19", "02/20", "02/21", "02/22", "02/23"],
    "operations": [45, 62, 38, 71, 55, 48, 67],
    "audit_logs": [18, 24, 12, 32, 28, 15, 22],
    "task_executions": [12, 18, 9, 24, 15, 8, 21]
  }
}
```

#### 2.3 各维度数据来源

| 维度 | 来源 | SQL 逻辑 |
|------|------|----------|
| `operations` | `audit_logs` | 按天 `GROUP BY DATE(created_at)` 统计所有操作数 |
| `audit_logs` | `audit_logs` | 同上，或可按 `action` 类型过滤 |
| `task_executions` | `execution_runs` 或 `healing_instances` | 按天统计执行次数 |

> [!NOTE]
> `operations` 和 `audit_logs` 可能是同一数据源但不同维度。如果审计日志包含所有操作，可以区分：
> - `operations`：所有审计记录（增删改操作总数）
> - `audit_logs`：安全相关审计（登录/权限变更等）
> - `task_executions`：从 `execution_runs` 表按 `created_at` 按天分组统计
>
> 具体分类逻辑由后端工程师根据实际业务定义。

---

## 三、新增接口（低优先级 / 未来增强）

### 3. `GET /api/v1/platform/tenants/ops-detail` — 租户运营明细（分页）

> 目前租户运营明细页面使用的是前端 Mock 数据。如果租户数量较少（< 50 个），可以直接复用 `GET /tenants/stats` 接口返回全量数据，前端做前端分页即可，**不需要**此独立接口。
>
> 但如果未来租户数量增长到数百个，建议新增此接口支持后端分页和搜索。

#### 3.1 请求参数

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|--------|------|
| `page` | `int` | 否 | `1` | 页码 |
| `page_size` | `int` | 否 | `15` | 每页条数 |
| `name` | `string` | 否 | | 按租户名称模糊搜索 |
| `code` | `string` | 否 | | 按租户代码模糊搜索 |
| `status` | `string` | 否 | | 按状态过滤 (`active` / `inactive`) |
| `sort_by` | `string` | 否 | `name` | 排序字段 |
| `sort_order` | `string` | 否 | `asc` | 排序方向 (`asc` / `desc`) |

#### 3.2 响应格式

```json
{
  "data": {
    "items": [
      {
        "id": "uuid",
        "name": "...",
        "code": "...",
        "status": "active",
        "member_count": 28,
        "rule_count": 15,
        "instance_count": 142,
        "template_count": 12,
        "audit_log_count": 1580,
        "last_activity_at": "2026-02-23 21:30:00",
        "cmdb_count": 356,
        "git_count": 8,
        "playbook_count": 24,
        "secret_count": 18,
        "plugin_count": 6,
        "incident_count": 45,
        "flow_count": 9,
        "schedule_count": 7,
        "notification_channel_count": 4,
        "notification_template_count": 8,
        "healing_success_count": 128,
        "healing_total_count": 142,
        "incident_covered_count": 41
      }
    ],
    "total": 8,
    "page": 1,
    "page_size": 15
  }
}
```

> [!TIP]
> 短期内建议直接让前端复用 `GET /tenants/stats` 接口（接口一），只有当性能成为问题时再独立此接口。

---

## 四、需要新增的 Repository 方法

### 4.1 `CountTenantTableWhere` — 带条件的通用统计

```go
// CountTenantTableWhere 统计某租户在指定表中满足额外条件的记录数
func (r *TenantRepository) CountTenantTableWhere(
    ctx context.Context, tenantID uuid.UUID, tableName string, extraWhere string,
) int64
```

用于统计：
- `healing_success_count`：`CountTenantTableWhere(ctx, id, "healing_instances", "status = 'success'")`
- `incident_covered_count`：`CountTenantTableWhere(ctx, id, "incidents", "healing_rule_id IS NOT NULL")`

> [!CAUTION]
> `extraWhere` 接收原始 SQL 片段，必须确保只在内部使用，不接受用户输入，防止 SQL 注入。

### 4.2 `GetTrends` — 趋势统计

```go
// GetTrends 按天统计某张表最近 N 天的记录数
func (r *TenantRepository) GetTrends(
    ctx context.Context, tableName string, days int,
) (dates []string, counts []int64, err error)
```

SQL 逻辑示例：
```sql
SELECT DATE(created_at) AS date, COUNT(*) AS cnt
FROM audit_logs
WHERE created_at >= NOW() - INTERVAL '7 days'
GROUP BY DATE(created_at)
ORDER BY date ASC;
```

---

## 五、开发优先级总结

| 优先级 | 接口/任务 | 工作量 | 说明 |
|--------|---------- |--------|------|
| 🔴 **P0** | 扩展 `GET /tenants/stats` 返回字段 | **小** | 只需在 handler 里补充 `CountTenantTable` 调用，约 10 行代码 |
| 🔴 **P0** | 新增 `CountTenantTableWhere` repository 方法 | **小** | 复制 `CountTenantTable` 加一个 WHERE 条件 |
| 🟡 **P1** | 新增 `GET /tenants/trends` 趋势接口 | **中** | 需要写 SQL 按天分组统计 |
| 🟢 **P2** | 新增 `GET /tenants/ops-detail` 分页接口 | **可选** | 租户数量少时不需要 |

---

## 六、表名确认清单

后端工程师请确认以下表名是否与实际数据库一致：

| 前端字段 | 预期表名 | 备注 |
|----------|---------|------|
| `cmdb_count` | `cmdb_hosts` | 可能叫 `hosts` / `assets` |
| `git_count` | `git_repositories` | 可能叫 `repositories` |
| `playbook_count` | `playbooks` | — |
| `secret_count` | `secrets` | 可能叫 `credentials` |
| `plugin_count` | `plugins` | — |
| `incident_count` | `incidents` | 可能叫 `alerts` / `tickets` |
| `flow_count` | `healing_flows` | 可能叫 `flows` / `workflows` |
| `schedule_count` | `scheduled_tasks` | 可能叫 `schedules` |
| `notification_channel_count` | `notification_channels` | — |
| `notification_template_count` | `notification_templates` | — |
| `healing_success_count` | `healing_instances` WHERE `status='success'` | 需确认 `status` 可选值 |
| `incident_covered_count` | `incidents` WHERE 有关联规则 | 需确认关联字段名称 |

> [!IMPORTANT]
> `CountTenantTable` 已有表存在性检查（`information_schema.tables`），如果表名不对会安全返回 0，不会报错。
