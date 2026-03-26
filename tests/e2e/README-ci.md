# E2E / CI 运行说明

更新时间：2026-03-22

本文档说明如何在本地或 CI 中运行当前的自动化验收脚本。

## 当前可用脚本

全量验收：

- [test_acceptance_real.sh](./test_acceptance_real.sh)

模块化验收：

- [test_acceptance_auth.sh](./test_acceptance_auth.sh)
- [test_acceptance_platform_tenants.sh](./test_acceptance_platform_tenants.sh)
- [test_acceptance_settings_secrets_dictionaries.sh](./test_acceptance_settings_secrets_dictionaries.sh)
- [test_acceptance_common.sh](./test_acceptance_common.sh)
- [test_acceptance_profile_rbac_misc.sh](./test_acceptance_profile_rbac_misc.sh)
- [test_acceptance_workbench_site_messages.sh](./test_acceptance_workbench_site_messages.sh)
- [test_acceptance_dashboard_overview_stats.sh](./test_acceptance_dashboard_overview_stats.sh)
- [test_acceptance_dashboard.sh](./test_acceptance_dashboard.sh)
- [test_acceptance_tenant_boundaries.sh](./test_acceptance_tenant_boundaries.sh)
- [test_acceptance_impersonation.sh](./test_acceptance_impersonation.sh)
- [test_acceptance_healing.sh](./test_acceptance_healing.sh)
- [test_acceptance_healing_queries.sh](./test_acceptance_healing_queries.sh)
- [test_acceptance_execution.sh](./test_acceptance_execution.sh)
- [test_acceptance_execution_queries.sh](./test_acceptance_execution_queries.sh)
- [test_acceptance_plugin_cmdb.sh](./test_acceptance_plugin_cmdb.sh)
- [test_acceptance_notifications_audit.sh](./test_acceptance_notifications_audit.sh)
- [test_acceptance_notification_variables.sh](./test_acceptance_notification_variables.sh)
- [test_acceptance_notification_failures.sh](./test_acceptance_notification_failures.sh)
- [test_acceptance_notification_retry.sh](./test_acceptance_notification_retry.sh)
- [test_acceptance_notification_retry_exhaustion.sh](./test_acceptance_notification_retry_exhaustion.sh)
- [test_acceptance_notification_rate_limit.sh](./test_acceptance_notification_rate_limit.sh)
- [test_acceptance_notification_retry_tenant_scope.sh](./test_acceptance_notification_retry_tenant_scope.sh)
- [test_acceptance_secrets_default_fallback.sh](./test_acceptance_secrets_default_fallback.sh)
- [test_acceptance_secrets_disabled_usage.sh](./test_acceptance_secrets_disabled_usage.sh)
- [test_acceptance_secrets_runtime_override.sh](./test_acceptance_secrets_runtime_override.sh)
- [test_acceptance_secrets_reference_updates.sh](./test_acceptance_secrets_reference_updates.sh)
- [test_acceptance_secrets_update_constraints.sh](./test_acceptance_secrets_update_constraints.sh)
- [test_acceptance_blacklist_security.sh](./test_acceptance_blacklist_security.sh)
- [test_acceptance_blacklist_exemption_execution.sh](./test_acceptance_blacklist_exemption_execution.sh)
- [test_acceptance_audit_action_assertions.sh](./test_acceptance_audit_action_assertions.sh)
- [test_acceptance_filters_pagination.sh](./test_acceptance_filters_pagination.sh)

主脚本：

- [test_acceptance_real.py](./test_acceptance_real.py)

## 前置依赖

运行脚本前需要具备：

- `docker`
- `curl`
- `jq`
- `git`
- `go`（或通过 `ACCEPTANCE_GO_BIN` 指定本机 Go 二进制）
- `python3.11`

还需要本机存在以下容器：

- PostgreSQL 容器：默认按 phase 隔离，例如 `auto-healing-postgres-auth`
- Redis 容器：默认按 phase 隔离，例如 `auto-healing-redis-auth`

如果容器名称不同，可用环境变量覆盖：

```bash
export ACCEPTANCE_PG_CONTAINER=your-postgres-container
export ACCEPTANCE_REDIS_CONTAINER=your-redis-container
```

验收脚本会直接调用本机 Go 构建当前源码的静态二进制。若 `go` 不在 `PATH` 中，可显式指定：

```bash
export ACCEPTANCE_GO_BIN=/usr/local/go/bin/go
```

如需覆盖默认验收管理员密码 `admin123456`，可以同时设置验收脚本和初始化管理员密码：

```bash
export ACCEPTANCE_ADMIN_PASSWORD='your-admin-password'
export INIT_ADMIN_PASSWORD='your-admin-password'
```

## 全量运行

```bash
bash tests/e2e/test_acceptance_real.sh
```

脚本会自动：

1. 构建当前代码的静态二进制
2. 创建隔离验收数据库
3. 启动 Mock ITSM / Mock CMDB
4. 启动当前代码服务
5. 初始化管理员
6. 运行验收场景
7. 输出结果 JSON

默认情况下，主脚本会把验收环境中的初始化管理员密码固定为 `admin123456`；如果需要其他密码，可通过 `INIT_ADMIN_PASSWORD` 覆盖。

## 模块化运行

可以按模块单独执行，例如：

```bash
bash tests/e2e/test_acceptance_auth.sh
bash tests/e2e/test_acceptance_platform_tenants.sh
bash tests/e2e/test_acceptance_settings_secrets_dictionaries.sh
bash tests/e2e/test_acceptance_profile_rbac_misc.sh
bash tests/e2e/test_acceptance_dashboard.sh
bash tests/e2e/test_acceptance_dashboard_overview_stats.sh
bash tests/e2e/test_acceptance_healing.sh
bash tests/e2e/test_acceptance_execution.sh
bash tests/e2e/test_acceptance_plugin_cmdb.sh
bash tests/e2e/test_acceptance_notifications_audit.sh
bash tests/e2e/test_acceptance_notification_variables.sh
bash tests/e2e/test_acceptance_notification_failures.sh
bash tests/e2e/test_acceptance_notification_retry.sh
bash tests/e2e/test_acceptance_notification_retry_exhaustion.sh
bash tests/e2e/test_acceptance_notification_rate_limit.sh
bash tests/e2e/test_acceptance_notification_retry_tenant_scope.sh
bash tests/e2e/test_acceptance_secrets_default_fallback.sh
bash tests/e2e/test_acceptance_secrets_disabled_usage.sh
bash tests/e2e/test_acceptance_secrets_runtime_override.sh
bash tests/e2e/test_acceptance_real.py --phase interface_contract_smoke
bash tests/e2e/test_acceptance_secrets_reference_updates.sh
bash tests/e2e/test_acceptance_secrets_update_constraints.sh
bash tests/e2e/test_acceptance_blacklist_security.sh
bash tests/e2e/test_acceptance_blacklist_exemption_execution.sh
bash tests/e2e/test_acceptance_audit_action_assertions.sh
bash tests/e2e/test_acceptance_filters_pagination.sh
```

适合以下场景：

- 修复了单一模块后快速回归
- CI 分阶段并行
- 只对高风险模块做烟测

## 主脚本可选参数

直接调用 Python 主脚本时，可用：

```bash
python3.11 tests/e2e/test_acceptance_real.py --list-phases
python3.11 tests/e2e/test_acceptance_real.py --phase auth
python3.11 tests/e2e/test_acceptance_real.py --phase platform_tenants
python3.11 tests/e2e/test_acceptance_real.py --phase settings_secrets_dictionaries
python3.11 tests/e2e/test_acceptance_real.py --phase profile_rbac_misc
python3.11 tests/e2e/test_acceptance_real.py --phase healing
python3.11 tests/e2e/test_acceptance_real.py --phase healing_queries
python3.11 tests/e2e/test_acceptance_real.py --phase git_execution
python3.11 tests/e2e/test_acceptance_real.py --phase plugin_cmdb
python3.11 tests/e2e/test_acceptance_real.py --phase execution_queries
python3.11 tests/e2e/test_acceptance_real.py --phase interface_contract_smoke
python3.11 tests/e2e/test_acceptance_real.py --phase workbench_site_messages
python3.11 tests/e2e/test_acceptance_real.py --phase notifications_audit
python3.11 tests/e2e/test_acceptance_real.py --phase notification_variables
python3.11 tests/e2e/test_acceptance_real.py --phase notification_failures
python3.11 tests/e2e/test_acceptance_real.py --phase notification_retry
python3.11 tests/e2e/test_acceptance_real.py --phase notification_retry_exhaustion
python3.11 tests/e2e/test_acceptance_real.py --phase notification_rate_limit
python3.11 tests/e2e/test_acceptance_real.py --phase notification_retry_tenant_scope
python3.11 tests/e2e/test_acceptance_real.py --phase secrets_default_fallback
python3.11 tests/e2e/test_acceptance_real.py --phase secrets_disabled_usage
python3.11 tests/e2e/test_acceptance_real.py --phase secrets_runtime_override
python3.11 tests/e2e/test_acceptance_real.py --phase secrets_reference_updates
python3.11 tests/e2e/test_acceptance_real.py --phase secrets_update_constraints
python3.11 tests/e2e/test_acceptance_real.py --phase blacklist_security
python3.11 tests/e2e/test_acceptance_real.py --phase blacklist_exemption_execution
python3.11 tests/e2e/test_acceptance_real.py --phase audit_action_assertions
python3.11 tests/e2e/test_acceptance_real.py --phase dashboard_overview_stats
python3.11 tests/e2e/test_acceptance_real.py --phase filters_pagination
```

当前 phase：

- `auth`
- `platform_tenants`
- `settings_secrets_dictionaries`
- `common`
- `profile_rbac_misc`
- `workbench_site_messages`
- `dashboard_overview_stats`
- `dashboard`
- `tenant_boundaries`
- `search_site_messages`
- `impersonation`
- `healing`
- `healing_queries`
- `git_execution`
- `plugin_cmdb`
- `execution_queries`
- `interface_contract_smoke`
- `notifications_audit`
- `notification_variables`
- `notification_failures`
- `notification_retry`
- `notification_retry_exhaustion`
- `notification_rate_limit`
- `notification_retry_tenant_scope`
- `secrets_default_fallback`
- `secrets_disabled_usage`
- `secrets_runtime_override`
- `secrets_reference_updates`
- `secrets_update_constraints`
- `blacklist_security`
- `blacklist_exemption_execution`
- `audit_action_assertions`
- `filters_pagination`
- `query_token_sse`

## 产物

默认会保留验收临时目录，便于排查失败：

```bash
export KEEP_ACCEPTANCE_ARTIFACTS=1
```

脚本启动时会打印类似：

```text
acceptance artifacts: /tmp/ah-acceptance-xxxxxx
```

其中包含：

- 服务日志
- mock 服务日志
- 本地 Git 验收仓库
- 工作目录

如果不需要保留：

```bash
export KEEP_ACCEPTANCE_ARTIFACTS=0
```

## 结果文件

全量跑完会输出：

- [acceptance-automation-results-all-2026-03-22.json](../../docs/acceptance-automation-results-all-2026-03-22.json)

模块化运行会输出：

- `docs/acceptance-automation-results-<phase>-2026-03-22.json`

## 当前覆盖范围

已经自动覆盖的核心场景：

- 认证 / refresh / logout
- 平台租户管理：list / detail / update / stats / trends / members / invitations / cancel / delete
- 平台设置、字典公共读和平台 CRUD
- Secrets Sources：create / list / detail / stats / test / test-query / query / default / enable / disable / delete
- 多租户 `/common/*` 数据隔离
- Workbench：overview / activities / schedule-calendar / announcements / favorites
- Site Messages：categories / settings / targeted delivery / list / unread-count / read-all
- Dashboard 配置与工作区隔离
- 租户用户 / 角色边界
- Impersonation
- 搜索与站内信隔离
- Incident -> Rule -> Flow -> Approval
- Healing 查询面：node schema / search schema / list / detail / stats / dry-run / pending / dismissed
- Git / Playbook / Execution / Cancel / Once Schedule
- Execution 查询面：tasks / runs / schedules 的 list / detail / stats / trend / timeline / enable / disable
- `local` 与 `docker` 两种 executor 的真实执行链
- Plugin / Incidents / CMDB：search schema / list / detail / stats / sync / logs / maintenance / resume
- Notifications：`webhook` + `dingtalk` + `email` channels / templates / preview / send / logs / stats
- Notification 模板变量：真实执行触发 `on_start / on_success`，校验 `execution.* / task.* / stats.* / repository.* / system.*` 渲染
- Notification 失败分支：`webhook` / `dingtalk` / `email` 的 `test` 失败和 `send` 失败日志
- Notification 重试：失败通知在修正 channel 配置后，由后台调度器自动重试成功
- Notification 重试耗尽：达到 `max_retries` 后不再继续重试，`next_retry_at` 清空
- Notification 限流：单渠道 `rate_limit_per_minute` 命中后，第二次发送落 `failed`
- Notification 多租户重试：非默认租户下的失败通知也会被后台调度器自动重试成功
- Secrets 默认源回退：无显式默认时回退到最高优先级活跃源；无活跃源时报错
- Secrets 禁用使用：已禁用源不能被显式 `source_id` 使用，执行任务也不会偷偷继续用它
- Secrets 运行时覆盖：运行时传入的 `secrets_source_ids` 优先于任务模板默认配置
- Secrets 引用生命周期：任务/调度引用冲突、解绑后删除成功
- Secrets 更新约束：被任务/调度引用中的源不能修改实际配置，但仍可调整低风险字段
- 黑名单安全：命令黑名单规则 CRUD / 仿真 / 切换 / 豁免申请 / 审批通过 / 拒绝
- 黑名单豁免执行：审批通过后的豁免规则能够真实放行执行任务
- Audit 精细断言：动作筛选接口可用，高危记录带 `risk_reason`
- Tenant / Platform Audit：list / detail / stats / trend / ranking / high-risk / export
- Dashboard Overview：全 section 聚合、租户 users 作用域、关联 stats 汇总
- Secrets 冲突：被任务/调度引用时删除返回冲突
- 复杂筛选 / 分页 / 空态：tasks / runs / incidents / cmdb / notifications / git / playbooks / site-messages / audit
- query token / SSE

## 当前未完全覆盖项

以下项仍建议人工或后续脚本补测：

- 插件“部分记录失败”场景的故障注入
- Git 同步“最终状态持久化失败”场景的故障注入
- 更细粒度的页面态验证，例如复杂筛选条件组合、空态、分页边界
- 更多通知 provider 边界场景，例如签名错误、SMTP 认证失败、回执异常
- 审计高危列表当前只校验接口可用性，没有把具体 action 精确钉死成强断言
- 还没覆盖更多跨租户并发写入后的筛选一致性
- 更细的 provider 专属失败原因断言还可以继续补

## CI 建议

建议在 CI 中至少配置两档：

### 1. 快速档

触发时机：

- 每次 PR

建议执行：

```bash
bash tests/e2e/test_acceptance_auth.sh
bash tests/e2e/test_acceptance_platform_tenants.sh
bash tests/e2e/test_acceptance_settings_secrets_dictionaries.sh
bash tests/e2e/test_acceptance_common.sh
bash tests/e2e/test_acceptance_workbench_site_messages.sh
bash tests/e2e/test_acceptance_dashboard_overview_stats.sh
bash tests/e2e/test_acceptance_dashboard.sh
bash tests/e2e/test_acceptance_tenant_boundaries.sh
```

如果 PR 触达执行、自愈、插件或 CMDB，建议额外加：

```bash
bash tests/e2e/test_acceptance_healing.sh
bash tests/e2e/test_acceptance_healing_queries.sh
bash tests/e2e/test_acceptance_execution.sh
bash tests/e2e/test_acceptance_execution_queries.sh
bash tests/e2e/test_acceptance_plugin_cmdb.sh
bash tests/e2e/test_acceptance_notifications_audit.sh
bash tests/e2e/test_acceptance_notification_variables.sh
bash tests/e2e/test_acceptance_notification_failures.sh
bash tests/e2e/test_acceptance_notification_retry.sh
bash tests/e2e/test_acceptance_notification_retry_exhaustion.sh
bash tests/e2e/test_acceptance_notification_rate_limit.sh
bash tests/e2e/test_acceptance_notification_retry_tenant_scope.sh
bash tests/e2e/test_acceptance_secrets_default_fallback.sh
bash tests/e2e/test_acceptance_secrets_disabled_usage.sh
bash tests/e2e/test_acceptance_secrets_runtime_override.sh
bash tests/e2e/test_acceptance_secrets_reference_updates.sh
bash tests/e2e/test_acceptance_secrets_update_constraints.sh
bash tests/e2e/test_acceptance_blacklist_security.sh
bash tests/e2e/test_acceptance_blacklist_exemption_execution.sh
bash tests/e2e/test_acceptance_audit_action_assertions.sh
bash tests/e2e/test_acceptance_dashboard_overview_stats.sh
bash tests/e2e/test_acceptance_filters_pagination.sh
```

### 2. 完整档

触发时机：

- 合并到主干前
- 夜间回归

建议执行：

```bash
bash tests/e2e/test_acceptance_real.sh
```

## 失败排查顺序

1. 看脚本打印停在哪个阶段
2. 看 `acceptance artifacts` 目录中的 `server.log`
3. 看 mock 日志
4. 看 `docs/acceptance-automation-results-*.json`
5. 如有需要，再查询验收数据库中的 `incidents / flow_instances / approval_tasks / execution_runs`
