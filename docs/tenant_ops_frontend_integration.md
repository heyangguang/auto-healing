# 租户运营模块 — 前端接入文档

> **后端已就绪**，前端需要接入 2 个 API 替换 Mock 数据。
> 权限要求：`platform:tenants:manage` 或 `platform:tenants:list`

---

## 接口一：`GET /api/v1/platform/tenants/stats`

> **已有接口**，本次扩展了返回字段。服务：`getTenantStats()`，已在 `services/auto-healing/platform/tenants.ts` 中定义。

### 响应结构

```json
{
  "code": 0,
  "data": {
    "summary": {
      "total_tenants": 5,
      "active_tenants": 5,
      "disabled_tenants": 0,
      "total_users": 20,
      "total_rules": 9,
      "total_instances": 0,
      "total_templates": 0
    },
    "tenants": [
      {
        "id": "uuid",
        "name": "默认租户",
        "code": "default",
        "status": "active",
        "icon": "bank",
        "member_count": 14,
        "rule_count": 9,
        "instance_count": 0,
        "template_count": 0,
        "audit_log_count": 146,
        "last_activity_at": "2026-02-23 21:30:14",

        "cmdb_count": 100,
        "git_count": 8,
        "playbook_count": 8,
        "secret_count": 82,
        "plugin_count": 148,
        "incident_count": 705,
        "flow_count": 8,
        "schedule_count": 11,
        "notification_channel_count": 17,
        "notification_template_count": 14,

        "healing_success_count": 12,
        "healing_total_count": 21,
        "incident_covered_count": 28
      }
    ]
  }
}
```

### 新增字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `cmdb_count` | `number` | CMDB 资产数量 |
| `git_count` | `number` | Git 仓库数量 |
| `playbook_count` | `number` | Playbook 数量 |
| `secret_count` | `number` | 凭据密钥数量 |
| `plugin_count` | `number` | 插件数量 |
| `incident_count` | `number` | 工单/告警总数 |
| `flow_count` | `number` | 自愈流程数量 |
| `schedule_count` | `number` | 定时任务数量 |
| `notification_channel_count` | `number` | 通知渠道数量 |
| `notification_template_count` | `number` | 通知模板数量 |
| `healing_success_count` | `number` | 自愈成功次数（流程实例 status=completed） |
| `healing_total_count` | `number` | 自愈总执行次数（流程实例总数） |
| `incident_covered_count` | `number` | 被自愈规则覆盖的工单数（有匹配规则的工单） |

### 前端可计算的派生指标

```typescript
// 自愈成功率（每租户）
const healingSuccessRate = t.healing_total_count > 0
  ? Math.round((t.healing_success_count / t.healing_total_count) * 100) : 0;

// 工单自愈覆盖率（每租户）
const incidentCoverageRate = t.incident_count > 0
  ? Math.round((t.incident_covered_count / t.incident_count) * 100) : 0;

// 自动化配置率（跨租户）
const total = tenants.length || 1;
const withRules = tenants.filter(t => t.rule_count > 0).length;
const ruleConfigRate = Math.round(withRules / total * 100); // XX% 租户已配置规则

// 类似可算：模板配置率、流程配置率、通知渠道配置率...
```

### TypeScript 类型更新

原本 `TenantStatsItem` 中的新增字段已经不是 `optional`，可以去掉 `?`：

```typescript
interface TenantStatsItem {
    id: string;
    name: string;
    code: string;
    status: string;
    icon: string;
    member_count: number;
    rule_count: number;
    instance_count: number;
    template_count: number;
    audit_log_count: number;
    last_activity_at: string | null;
    // ↓ 以下都是新增的，现在后端已返回，不再需要 ? 可选标记
    cmdb_count: number;
    git_count: number;
    playbook_count: number;
    secret_count: number;
    plugin_count: number;
    incident_count: number;
    flow_count: number;
    schedule_count: number;
    notification_channel_count: number;
    notification_template_count: number;
    healing_success_count: number;
    healing_total_count: number;
    incident_covered_count: number;
}
```

---

## 接口二：`GET /api/v1/platform/tenants/trends`（新增）

> **新接口**，需要在 `services/auto-healing/platform/tenants.ts` 中添加 service 函数。

### 请求

```
GET /api/v1/platform/tenants/trends?days=7
```

| 参数 | 类型 | 必填 | 默认值 | 说明 |
|------|------|------|-------|------|
| `days` | `number` | 否 | `7` | 统计天数，支持 1~90 |

### 响应结构

```json
{
  "code": 0,
  "data": {
    "dates": ["02/17", "02/18", "02/19", "02/20", "02/21", "02/22", "02/23"],
    "operations": [3, 24, 0, 18, 0, 32, 94],
    "audit_logs": [3, 18, 0, 18, 0, 28, 90],
    "task_executions": [26, 24, 26, 18, 25, 32, 25]
  }
}
```

### 字段说明

| 字段 | 类型 | 说明 |
|------|------|------|
| `dates` | `string[]` | 日期标签数组（格式 `MM/DD`），长度=days |
| `operations` | `number[]` | 操作趋势：所有**租户级**审计日志按天计数 |
| `audit_logs` | `number[]` | 安全审计趋势：仅登录/登出/代操作等安全事件 |
| `task_executions` | `number[]` | 任务执行趋势：执行记录按天计数 |

### Service 函数示例

```typescript
/** 获取租户运营趋势数据 */
export async function getTenantTrends(params?: { days?: number }) {
    return request<any>('/api/v1/platform/tenants/trends', {
        method: 'GET',
        params,
    });
}
```

---

## 页面接入指引

### 1. 租户运营总览（`tenant-overview/index.tsx`）

| 区域 | 数据来源 | 当前状态 | 改动 |
|------|---------|---------|------|
| 顶部 4 个核心指标卡片 | `stats.summary` | ✅ 已接入 | 无需改动 |
| 平台资源概览（6宫格） | `stats.tenants[].cmdb_count` 等 | ✅ 已接入 | 去掉 `\|\| 0` 兜底，字段已非可选 |
| 自愈能力概览（5格） | `stats.tenants[].schedule_count` 等 | ✅ 已接入 | 同上 |
| 通知与监控（6格） | `stats.tenants[].notification_*` | ✅ 已接入 | 同上 |
| 3 个 TOP 5 列表 | `stats.tenants[]` 排序 | ✅ 已接入 | 无需改动 |
| 2 个排行榜 | `stats.tenants[]` 排序 | ✅ 已接入 | 无需改动 |
| **3 个折线趋势图** | ~~Mock~~ → `trends` 接口 | ❌ Mock 中 | **需替换** |
| 3 个环形统计 | `stats.tenants[]` 计算 | ✅ 已接入 | 无需改动 |

**重点改动 — 折线趋势图替换 Mock：**

```typescript
// 1. 导入新的 service
import { getTenantStats, getTenantTrends } from '@/services/auto-healing/platform/tenants';

// 2. 新增 state
const [trendData, setTrendData] = useState<{
    dates: string[]; operations: number[]; audit_logs: number[]; task_executions: number[];
}>({ dates: [], operations: [], audit_logs: [], task_executions: [] });

// 3. 在 fetchStats 中同时请求 trends
const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
        const [statsRes, trendsRes] = await Promise.all([
            getTenantStats(),
            getTenantTrends({ days: 7 }),
        ]);
        // stats 处理...
        setTenants(statsRes?.data?.tenants || []);
        setSummary(statsRes?.data?.summary || summary);
        // trends 处理
        const td = trendsRes?.data || trendsRes;
        setTrendData({
            dates: td?.dates || [],
            operations: td?.operations || [],
            audit_logs: td?.audit_logs || [],
            task_executions: td?.task_executions || [],
        });
    } catch { /* ignore */ } finally { setLoading(false); }
}, []);

// 4. 替换 AreaChart 的数据源
// 操作趋势
<AreaChart data={trendData.operations} labels={trendData.dates} color="#1677ff" />
// 审计趋势
<AreaChart data={trendData.audit_logs} labels={trendData.dates} color="#722ed1" />
// 任务执行
<AreaChart data={trendData.task_executions} labels={trendData.dates} color="#13c2c2" />

// 5. Tag 总数也改为真实数据
extra={<Tag>{trendData.operations.reduce((s, v) => s + v, 0)} 次</Tag>}

// 6. 删除 Mock 数据（mock7Days, mockOperationValues, mockAuditValues, mockTaskValues）
```

### 2. 租户运营明细（`tenant-ops-detail/index.tsx`）

| 改动 | 说明 |
|------|------|
| **替换 Mock 数据** | 删除 `MOCK_TENANTS` 数组，从 `getTenantStats()` 获取真实数据 |
| **加入 API 调用** | 添加 `useEffect` + `useState` 调用 `getTenantStats()` |
| **TypeScript 类型** | 去掉 `TenantStatsItem` 重复定义，与总览页共用同一份类型 |

```typescript
// 在组件中：
const [tenants, setTenants] = useState<TenantStatsItem[]>([]);
const [loading, setLoading] = useState(true);

useEffect(() => {
    getTenantStats().then(res => {
        setTenants(res?.data?.tenants || []);
    }).finally(() => setLoading(false));
}, []);

// Table 的 dataSource 从 MOCK_TENANTS 改为 tenants
// Table 加上 loading={loading}
```

---

## curl 验证命令

```bash
# 登录获取 Token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

# 测试 stats
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/tenants/stats" | jq '.data.tenants[0]'

# 测试 trends（7天）
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/tenants/trends?days=7" | jq '.data'

# 测试 trends（30天）
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://localhost:8080/api/v1/platform/tenants/trends?days=30" | jq '.data'
```
