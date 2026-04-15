# 后端字典 API 需求文档

> **版本**: v1.0  
> **日期**: 2026-02-23  
> **提出方**: 前端团队  
> **优先级**: 中（基础设施改进）

---

## 一、背景与问题

### 1.1 现状

前端目前有 **9 个静态常量文件**，包含 **20+ 组字典映射**（枚举状态码 → 中文标签 / 颜色 / 图标等），全部硬编码在 TypeScript 代码中：

| 文件 | 字典数量 | 涉及领域 |
|---|---|---|
| `incidentDicts.ts` | 6 组 | 事件严重程度、类别、工单状态、自愈状态、图表颜色/标签 |
| `auditDicts.ts` | 6 组 | 审计资源类型、操作类型、操作颜色、操作动词、HTTP Method |
| `instanceDicts.ts` | 3 组 | 自愈实例状态颜色/标签/MAP |
| `executionDicts.ts` | 4 组 | 执行器类型、运行状态颜色/标签/MAP |
| `notificationDicts.tsx` | 1 组 | 通知渠道类型 |
| `cmdbDicts.tsx` | 4 组 | CMDB 资产类型/状态/环境/状态标签 |
| `pluginDicts.ts` | 3 组 | 插件状态颜色/标签/MAP |
| `permissionDicts.ts` | 2 组 | 权限模块标签/颜色 |
| `nodeConfig.ts` | 4 组 | 流程节点类型颜色/标签/编辑器标签/组合配置 |

### 1.2 痛点

1. **后端每新增一个枚举值（如新的通知渠道类型 `slack`），前端必须手动同步修改 + 重新发版**
2. **前后端定义分散，容易出现不一致**（如后端加了 `dismissed` 状态，前端不知道，UI 显示原始英文）
3. **无法动态扩展**（比如租户想自定义资产类型或事件类别，当前架构完全不支持）
4. **多语言完全不可能**（中英文标签硬编码在前端代码里）

### 1.3 目标

实现一个 **后端字典管理 API**，使前端可以在运行时获取所有枚举值的展示信息（标签、颜色、图标等），**后端新增枚举值后前端零修改自动生效**。

---

## 二、数据模型设计

### 2.1 核心表结构

建议新建 `sys_dictionaries` 表（或类似命名），字段如下：

| 字段名 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `id` | UUID | ✅ | 主键 |
| `dict_type` | varchar(64) | ✅ | 字典类型标识，如 `instance_status`、`incident_severity` |
| `dict_key` | varchar(64) | ✅ | 枚举值 key，如 `completed`、`critical` |
| `label` | varchar(128) | ✅ | 中文显示标签，如 `已完成`、`严重` |
| `label_en` | varchar(128) | | 英文标签（预留多语言），如 `Completed`、`Critical` |
| `color` | varchar(32) | | 主色值（CSS 颜色），如 `#52c41a`、`red` |
| `tag_color` | varchar(32) | | antd Tag 组件色值，如 `success`、`error`、`processing` |
| `badge` | varchar(32) | | antd Badge 状态，如 `success`、`error`、`warning`、`default` |
| `icon` | varchar(64) | | 图标标识（前端约定的 icon key），如 `DesktopOutlined`、`MailOutlined` |
| `bg` | varchar(32) | | 背景色（卡片等场景），如 `#f6ffed` |
| `extra` | jsonb | | 扩展属性（灵活应对未来需求），如 `{"verb": "创建了", "verb_color": "#52c41a"}` |
| `sort_order` | int | | 排序权重（越小越靠前），默认 0 |
| `is_active` | bool | | 是否启用，默认 true |
| `created_at` | timestamp | ✅ | 创建时间 |
| `updated_at` | timestamp | ✅ | 更新时间 |

**唯一约束**: `(dict_type, dict_key)` 联合唯一

### 2.2 关于 `extra` 字段（JSONB）

部分字典有特殊属性不适合列式存储，放入 `extra`：

| 字典类型 | extra 用途举例 |
|---|---|
| `audit_action` | `{"verb": "创建了", "verb_color": "#52c41a"}` — 审计日志动词形式 |
| `notification_channel_type` | `{"label_upper": "WEBHOOK"}` — 大写英文标签 |
| `node_type` | `{"full_label": "开始节点"}` — 编辑器侧边栏完整标签 |

---

## 三、API 设计

### 3.1 批量查询字典

```
GET /api/v1/dictionaries
```

**查询参数：**

| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `types` | string | | 逗号分隔的字典类型，如 `instance_status,incident_severity`。为空则返回所有类型 |
| `active_only` | bool | | 是否只返回 `is_active=true` 的记录，默认 `true` |

**响应示例：**

```json
{
  "data": {
    "instance_status": [
      {
        "key": "completed",
        "label": "已完成",
        "label_en": "Completed",
        "color": "#52c41a",
        "tag_color": "success",
        "badge": "success",
        "icon": null,
        "bg": "#f6ffed",
        "extra": null,
        "sort_order": 1
      },
      {
        "key": "running",
        "label": "运行中",
        "label_en": "Running",
        "color": "#1677ff",
        "tag_color": "processing",
        "badge": "processing",
        "icon": null,
        "bg": "#e6f7ff",
        "extra": null,
        "sort_order": 2
      }
    ],
    "incident_severity": [
      {
        "key": "critical",
        "label": "严重",
        "label_en": "Critical",
        "color": "#cf1322",
        "tag_color": "red",
        "badge": null,
        "icon": null,
        "bg": null,
        "extra": null,
        "sort_order": 1
      }
    ]
  },
  "meta": {
    "total_types": 20,
    "total_entries": 156,
    "cached_at": "2026-02-23T09:30:00Z"
  }
}
```

> [!IMPORTANT]
> **响应格式核心要求**：按 `dict_type` 分组为 `Record<string, DictEntry[]>` 结构，每个 entry 内按 `sort_order` 升序排列。前端会基于这个结构构建全局缓存。

### 3.2 查询可用的字典类型列表

```
GET /api/v1/dictionaries/types
```

**响应示例：**

```json
{
  "data": [
    { "type": "instance_status", "label": "自愈实例状态", "entry_count": 7 },
    { "type": "incident_severity", "label": "事件严重程度", "entry_count": 5 },
    { "type": "audit_action", "label": "审计操作类型", "entry_count": 26 }
  ]
}
```

### 3.3 管理接口（可选，供后台管理用）

如果需要 Web 管理界面来维护字典数据（非必须，可以后期做）：

| 方法 | 路径 | 说明 |
|---|---|---|
| `POST` | `/api/v1/dictionaries` | 新增字典项 |
| `PUT` | `/api/v1/dictionaries/:id` | 更新字典项 |
| `DELETE` | `/api/v1/dictionaries/:id` | 删除字典项 |
| `POST` | `/api/v1/dictionaries/batch` | 批量导入（首次迁移用） |

---

## 四、需要迁移的完整字典清单

以下是前端现有全部字典的 **精确枚举值列表**，后端需要初始化种子数据。

### 4.1 事件严重程度 (`incident_severity`)

| key | label | color | tag_color | 备注 |
|---|---|---|---|---|
| `critical` | 严重 | `#cf1322` | `red` | 也支持数字 key `1` |
| `high` | 高 | `#fa541c` | `orange` | `2` |
| `medium` | 中 | `#faad14` | `gold` | `3` |
| `low` | 低 | `#52c41a` | `green` | `4` |
| `info` | 信息 | `#8c8c8c` | `default` | |

> [!NOTE]
> `critical`/`1`、`high`/`2` 等是同一个值的两种 key 表示。建议后端用 `extra` 存数字别名：`{"alias": "1"}`，前端根据 alias 做双向查找。

### 4.2 事件类别 (`incident_category`)

| key | label |
|---|---|
| `network` | 网络 |
| `application` | 应用 |
| `database` | 数据库 |
| `security` | 安全 |
| `hardware` | 硬件 |
| `storage` | 存储 |

### 4.3 工单状态 (`incident_status`)

| key | label | color(tag) |
|---|---|---|
| `open` | 打开 | `blue` |
| `in_progress` | 处理中 | `orange` |
| `resolved` | 已解决 | `green` |
| `closed` | 已关闭 | `default` |

### 4.4 自愈状态 (`healing_status`)

| key | label | color | badge |
|---|---|---|---|
| `pending` | 待处理 | `#d9d9d9` | `default` |
| `matched` | 已匹配 | `#1677ff` | `processing` |
| `healing` | 自愈中 | `#1677ff` | `processing` |
| `processing` | 处理中 | `#1677ff` | `processing` |
| `healed` | 已自愈 | `#52c41a` | `success` |
| `failed` | 失败 | `#ff4d4f` | `error` |
| `skipped` | 已跳过 | `#8c8c8c` | `default` |
| `dismissed` | 已忽略 | `#bfbfbf` | `default` |

### 4.5 自愈实例状态 (`instance_status`)

| key | label | color | tag_color | badge | bg |
|---|---|---|---|---|---|
| `pending` | 等待中 | `#faad14` | `warning` | `warning` | `#fffbe6` |
| `running` | 执行中 | `#1677ff` | `processing` | `processing` | `#e6f7ff` |
| `waiting_approval` | 待审批 | `#722ed1` | `warning` | `warning` | `#fffbe6` |
| `completed` | 已完成 | `#52c41a` | `success` | `success` | `#f6ffed` |
| `success` | 成功 | `#52c41a` | `success` | `success` | |
| `approved` | 已通过 | | | | |
| `failed` | 失败 | `#ff4d4f` | `error` | `error` | `#fff2f0` |
| `rejected` | 已拒绝 | | | | |
| `error` | 错误 | | | | |
| `partial` | 部分成功 | | | | |
| `cancelled` | 已取消 | `#d9d9d9` | `default` | `default` | `#fafafa` |
| `skipped` | 已跳过 | `#8c8c8c` | `default` | `default` | `#fafafa` |
| `simulated` | 模拟通过 | | | | |
| `triggered` | 已触发 | | | | |

### 4.6 执行器类型 (`executor_type`)

| key | label | color |
|---|---|---|
| `local` | 本地 | `#52c41a` |
| `docker` | Docker | `#1677ff` |
| `ssh` | SSH | `#722ed1` |

### 4.7 执行运行状态 (`run_status`)

| key | label | color | tag_color | badge |
|---|---|---|---|---|
| `success` | 成功 | `#52c41a` | `success` | `success` |
| `failed` | 失败 | `#ff4d4f` | `error` | `error` |
| `running` | 运行中 | `#1677ff` | `processing` | `processing` |
| `partial` | 部分成功 | `#faad14` | `warning` | `warning` |
| `cancelled` | 已取消 | `#d9d9d9` | `default` | `default` |

### 4.8 通知渠道类型 (`notification_channel_type`)

| key | label | color | icon | bg | extra |
|---|---|---|---|---|---|
| `webhook` | Webhook | `#722ed1` | `ApiOutlined` | `#f9f0ff` | `{"label_upper": "WEBHOOK"}` |
| `email` | 邮件 | `#1890ff` | `MailOutlined` | `#e6f7ff` | `{"label_upper": "EMAIL"}` |
| `dingtalk` | 钉钉 | `#0079f2` | `DingdingOutlined` | `#f0f5ff` | `{"label_upper": "DINGTALK"}` |
| `wecom` | 企业微信 | `#07c160` | `WecomOutlined` | `#f6ffed` | `{"label_upper": "WECOM"}` |
| `slack` | Slack | `#4a154b` | `SlackOutlined` | `#fff0f6` | `{"label_upper": "SLACK"}` |
| `teams` | Teams | `#6264a7` | `WindowsOutlined` | `#f0f5ff` | `{"label_upper": "TEAMS"}` |

### 4.9 CMDB 资产类型 (`cmdb_type`)

| key | label | color | icon |
|---|---|---|---|
| `server` | 服务器 | `#1890ff` | `DesktopOutlined` |
| `application` | 应用 | `#13c2c2` | `AppstoreOutlined` |
| `network` | 网络 | `#722ed1` | `GlobalOutlined` |
| `database` | 数据库 | `#fa8c16` | `HddOutlined` |

### 4.10 CMDB 资产状态 (`cmdb_status`)

| key | label | color | badge |
|---|---|---|---|
| `active` | 活跃 | `#52c41a` | `success` |
| `inactive` | 停用 | `#ff4d4f` | `error` |
| `maintenance` | 维护中 | `#faad14` | `warning` |

### 4.11 环境 (`cmdb_environment`)

| key | label | color(tag) |
|---|---|---|
| `production` | 生产 | `red` |
| `staging` | 预发 | `orange` |
| `test` | 测试 | `green` |
| `development` | 开发 | `blue` |

### 4.12 插件状态 (`plugin_status`)

| key | label | color | badge |
|---|---|---|---|
| `active` | 活跃 | `#52c41a` | `success` |
| `inactive` | 停用 | `#d9d9d9` | `default` |
| `error` | 异常 | `#ff4d4f` | `error` |

### 4.13 审计操作类型 (`audit_action`)

| key | label | tag_color | extra |
|---|---|---|---|
| `activate` | 激活 | `green` | `{"verb": "激活了", "verb_color": "#52c41a"}` |
| `approve` | 审批通过 | `green` | `{"verb": "通过了", "verb_color": "#52c41a"}` |
| `assign_permission` | 分配权限 | `magenta` | `{"verb": "分配了权限", "verb_color": "#eb2f96"}` |
| `assign_role` | 分配角色 | `magenta` | `{"verb": "分配了角色", "verb_color": "#eb2f96"}` |
| `batch_create` | 批量创建 | `lime` | `{"verb": "批量创建了", "verb_color": "#52c41a"}` |
| `confirm_review` | 确认复核 | `cyan` | `{"verb": "确认复核了", "verb_color": "#13c2c2"}` |
| `create` | 创建 | `green` | `{"verb": "创建了", "verb_color": "#52c41a"}` |
| `deactivate` | 停用 | `orange` | `{"verb": "停用了", "verb_color": "#fa8c16"}` |
| `delete` | 删除 | `red` | `{"verb": "删除了", "verb_color": "#f5222d"}` |
| `disable` | 禁用 | `orange` | `{"verb": "禁用了", "verb_color": "#fa8c16"}` |
| `dismiss` | 驳回 | `volcano` | `{"verb": "驳回了", "verb_color": "#fa541c"}` |
| `enable` | 启用 | `green` | `{"verb": "启用了", "verb_color": "#52c41a"}` |
| `execute` | 执行 | `geekblue` | `{"verb": "执行了", "verb_color": "#fa8c16"}` |
| `login` | 登录 | `purple` | `{"verb": "登录了", "verb_color": "#722ed1"}` |
| `logout` | 登出 | `purple` | `{"verb": "登出了", "verb_color": "#722ed1"}` |
| `maintenance` | 维护 | `orange` | `{"verb": "维护了", "verb_color": "#fa8c16"}` |
| `preview` | 预览 | `default` | `{"verb": "预览了", "verb_color": "#8c8c8c"}` |
| `ready` | 就绪 | `cyan` | `{"verb": "就绪了", "verb_color": "#13c2c2"}` |
| `reject` | 审批拒绝 | `red` | `{"verb": "拒绝了", "verb_color": "#f5222d"}` |
| `reset_password` | 重置密码 | `orange` | `{"verb": "重置了密码", "verb_color": "#fa8c16"}` |
| `reset_scan` | 重置扫描 | `gold` | `{"verb": "重置了扫描", "verb_color": "#faad14"}` |
| `resume` | 恢复 | `geekblue` | `{"verb": "恢复了", "verb_color": "#1890ff"}` |
| `scan` | 扫描 | `blue` | `{"verb": "扫描了", "verb_color": "#1890ff"}` |
| `sync` | 同步 | `purple` | `{"verb": "同步了", "verb_color": "#722ed1"}` |
| `test` | 测试 | `default` | `{"verb": "测试了", "verb_color": "#8c8c8c"}` |
| `trigger` | 触发 | `gold` | `{"verb": "触发了", "verb_color": "#fa8c16"}` |
| `update` | 更新 | `blue` | `{"verb": "更新了", "verb_color": "#1890ff"}` |
| `update_variables` | 更新变量 | `blue` | `{"verb": "更新了变量", "verb_color": "#1890ff"}` |

### 4.14 审计资源类型 - 租户 (`audit_resource_tenant`)

| key | label |
|---|---|
| `auth` | 认证 |
| `auth-logout` | 登出 |
| `channels` | 通知渠道 |
| `cmdb` | 资产管理 |
| `execution-schedules` | 定时任务 |
| `execution-tasks` | 执行任务 |
| `git-repos` | Git 仓库 |
| `healing` | 自愈管理 |
| `healing-flows` | 自愈流程 |
| `healing-rules` | 自愈规则 |
| `healing-approvals` | 自愈审批 |
| `incidents` | 事件管理 |
| `playbooks` | Playbook |
| `plugins` | 插件管理 |
| `secrets-sources` | 密钥管理 |
| `site-messages` | 站内信 |
| `templates` | 通知模板 |
| `users` | 用户管理 |

### 4.15 审计资源类型 - 平台 (`audit_resource_platform`)

| key | label |
|---|---|
| `auth` | 认证 |
| `auth-profile` | 个人资料 |
| `auth-logout` | 登出 |
| `healing-approvals` | 自愈审批 |
| `healing-flows` | 自愈流程 |
| `healing-rules` | 自愈规则 |
| `permissions` | 权限管理 |
| `roles` | 角色管理 |
| `settings` | 平台设置 |
| `tenant` | 租户 |
| `tenant-users` | 租户用户 |
| `tenants` | 租户管理 |
| `users` | 用户管理 |

### 4.16 权限模块 (`permission_module`)

| key | label | color |
|---|---|---|
| `system` | 系统管理 | `#722ed1` |
| `user` | 用户管理 | `#1677ff` |
| `role` | 角色管理 | `#13c2c2` |
| `plugin` | 插件管理 | `#52c41a` |
| `execution` | 执行管理 | `#fa8c16` |
| `notification` | 通知管理 | `#eb2f96` |
| `healing` | 自愈引擎 | `#f5222d` |
| `workflow` | 工作流 | `#2f54eb` |
| `dashboard` | 仪表盘 | `#faad14` |
| `platform` | 平台管理 | `#531dab` |
| `site-message` | 站内信 | `#08979c` |

### 4.17 HTTP Method (`http_method`)

| key | label | color |
|---|---|---|
| `GET` | GET | `#61affe` |
| `POST` | POST | `#49cc90` |
| `PUT` | PUT | `#fca130` |
| `PATCH` | PATCH | `#50e3c2` |
| `DELETE` | DELETE | `#f93e3e` |

### 4.18 流程节点类型 (`node_type`)

| key | label | color | icon | extra |
|---|---|---|---|---|
| `start` | 开始 | `#52c41a` | `PlayCircleOutlined` | `{"full_label": "开始节点"}` |
| `end` | 结束 | `#ff4d4f` | `StopOutlined` | `{"full_label": "结束节点"}` |
| `execution` | 执行 | `#8B5A2B` | `CodeOutlined` | `{"full_label": "任务执行"}` |
| `approval` | 审批 | `#faad14` | `AuditOutlined` | `{"full_label": "人工审批"}` |
| `notification` | 通知 | `#52c41a` | `BellOutlined` | `{"full_label": "发送通知"}` |
| `condition` | 条件分支 | `#722ed1` | `BranchesOutlined` | `{"full_label": "条件分支"}` |
| `host_extractor` | 主机提取 | `#1890ff` | `ClusterOutlined` | `{"full_label": "主机提取"}` |
| `cmdb_validator` | CMDB 校验 | `#13c2c2` | `SafetyCertificateOutlined` | `{"full_label": "CMDB验证"}` |
| `set_variable` | 变量设置 | `#eb2f96` | `FunctionOutlined` | `{"full_label": "设置变量"}` |
| `compute` | 计算 | `#2f54eb` | `CalculatorOutlined` | `{"full_label": "计算节点"}` |

---

## 五、前后端图标约定

### 5.1 约定规则

后端 `icon` 字段存储的是 **Ant Design 图标名称字符串**（不带尖括号），前端维护一个 `iconKey → React.ReactNode` 映射表。

**前端映射示例**（已有约 30 个图标）：

```typescript
import {
    DesktopOutlined,
    MailOutlined,
    ApiOutlined,
    DingdingOutlined,
    WechatWorkOutlined,
    SlackOutlined,
    WindowsOutlined,
    ...
} from '@ant-design/icons';

const ICON_REGISTRY: Record<string, React.ReactNode> = {
    DesktopOutlined: <DesktopOutlined />,
    MailOutlined: <MailOutlined />,
    ApiOutlined: <ApiOutlined />,
    DingdingOutlined: <DingdingOutlined />,
    WecomOutlined: <WechatWorkOutlined />,
    SlackOutlined: <SlackOutlined />,
    WindowsOutlined: <WindowsOutlined />,
    // ... 按需扩展
};

export function resolveIcon(iconKey: string | null): React.ReactNode {
    return iconKey ? ICON_REGISTRY[iconKey] ?? null : null;
}
```

### 5.2 后端职责

- `icon` 字段只需存储字符串 key（如 `"MailOutlined"`）
- 后端 **无需** 关心图标的实际渲染
- 新增枚举如果需要特殊图标，告知前端在 `ICON_REGISTRY` 中添加；如果没有特殊图标，可留空（前端会使用默认图标）

---

## 六、类型安全方案

### 6.1 问题

字典 API 返回的是动态数据，TypeScript 无法在编译期推断出具体的联合类型。例如本来 `status` 的类型是 `'completed' | 'failed' | 'running'`，改为 API 驱动后会退化为 `string`。

### 6.2 建议方案

后端在返回字典数据时，增加一个 **类型定义导出接口**（可选实现）：

```
GET /api/v1/dictionaries/typescript
```

返回自动生成的 TypeScript 类型定义文件：

```typescript
// 自动生成，请勿手动修改
export type InstanceStatus = 'pending' | 'running' | 'waiting_approval' | 'completed' | 'success' | 'approved' | 'failed' | 'rejected' | 'error' | 'partial' | 'cancelled' | 'skipped' | 'simulated' | 'triggered';
export type IncidentSeverity = 'critical' | 'high' | 'medium' | 'low' | 'info';
export type RunStatus = 'success' | 'failed' | 'running' | 'partial' | 'cancelled';
// ...
```

前端可以将这个接口的输出保存为 `.d.ts` 文件，作为开发期辅助。这个接口只在开发时使用，不影响运行时行为。

> [!TIP]
> 这个接口不是高优先级，可以后期再实现。前端在过渡期会用 `string` 类型 + fallback 默认值来保证安全。

---

## 七、缓存策略

### 7.1 建议

- 后端对 `/api/v1/dictionaries` 设置 **`Cache-Control: max-age=300`**（5 分钟客户端缓存）
- 后端内部对字典数据做 **内存缓存**，字典变更时清除缓存
- 如果字典数据有变更，可选返回 `ETag` 或 `Last-Modified` 头，前端用 `If-None-Match` 做条件请求

### 7.2 前端使用策略

- 前端在应用初始化（`getInitialState`）时拉取一次全量字典
- 缓存到全局 state，整个会话期间不再重复请求
- 未来可选：切换租户时重新拉取（如果字典是租户级别的）

---

## 八、迁移计划

> [!IMPORTANT]
> 此功能采用 **渐进式迁移**，不需要一次性完成。

### Phase 1：基础设施（后端）
1. 创建 `sys_dictionaries` 表
2. 实现 `GET /api/v1/dictionaries` 查询接口
3. 用数据库 migration 或 seed 脚本，将上述第四节的 20+ 组字典数据导入

### Phase 2：前端过渡
1. 前端实现 `useDictionary` hook，启动时拉取字典
2. 各组件逐步切换到动态字典（带 fallback 到静态常量）
3. 验证所有页面正常后，删除静态常量文件

### Phase 3：增强（可选）
1. 实现字典管理后台页面
2. 实现 TypeScript 类型导出接口
3. 支持租户级字典覆盖

---

## 九、验收标准

1. ✅ `GET /api/v1/dictionaries` 返回全部 20+ 组字典，结构符合 3.1 节定义
2. ✅ 支持 `types` 参数按需过滤
3. ✅ 所有枚举值数据完整，与第四节清单一致
4. ✅ 结果按 `sort_order` 排序
5. ✅ 响应时间 < 50ms（内存缓存后）
6. ✅ `(dict_type, dict_key)` 唯一约束正确工作
