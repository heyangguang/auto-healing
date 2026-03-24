# 整改说明

更新时间：2026-03-22

本文档用于说明本轮仓库整改涉及的修复点、业务影响、验证结果，以及上线前需要关注的事项。

## 结论

本轮修改的目标不是改变正常业务流程，而是修复以下问题：

- 越权访问
- 跨租户数据串写/串读
- 自愈流程、执行、审批、取消、调度的状态机错误
- Token 使用与日志泄露风险
- 代码语义与数据库约束不一致

对正常业务的影响原则如下：

- 正常、符合设计的业务路径应保持可用。
- 原本错误的行为会被阻止。
- 原本不稳定的数据行为会变得稳定。
- 个别接口语义被收紧后，前端/调用方需要按新约定对接。

## 已修复问题

### 1. 用户与权限边界

- 修复租户级用户管理错误复用平台级用户 CRUD 的问题。
- 修复租户级角色接口可跨租户读改删的问题。
- 修复平台 Impersonation 路由缺少细粒度权限控制的问题。
- 修复 Dashboard 工作区权限按用户全量租户角色计算的问题。

业务影响：

- 租户管理员现在只能操作当前租户成员，不能跨租户影响别的用户或角色。
- 平台用户只有具备相应权限时才能进入 Impersonation 申请流程。
- Dashboard 工作区权限将严格按当前租户角色生效。

### 2. 租户上下文与公共接口

- 修复 `/api/v1/common/*` 在缺失租户上下文时回落到默认租户的问题。
- 为公共路由补充 `CommonTenantMiddleware`。
- 让默认租户选择稳定化，避免多租户用户登录/刷新后默认租户漂移。

业务影响：

- `/api/v1/common/*` 将在租户用户场景下使用明确或稳定的默认租户上下文。
- 平台管理员只有在 Impersonation 场景下才拥有租户上下文。
- 搜索、工作台、偏好、收藏、最近访问等数据不再串租户。

### 3. 自愈流程与执行状态机

- 修复重复审批导致流程重复恢复的问题。
- 修复取消流程后被审批超时扫描覆盖回失败态的问题。
- 修复流程启动、进入待审批状态未可靠落库的问题。
- 修复执行 run 在 `pending -> running` 期间取消无效的问题。
- 修复调度器只判断“是否成功创建异步执行”而不看真实执行结果的问题。
- 修复 once 调度过早标记为完成的问题。
- 修复流程取消时不主动中断运行中的子执行的问题。

业务影响：

- 审批、取消、重试、自愈触发结果会更稳定，状态更可信。
- 调度失败计数、自动暂停、完成状态将更接近真实执行结果。
- 用户点击取消后，流程和子执行更符合“停止”的预期。

### 4. 多租户数据一致性

- 修复 `user_preferences` 的租户唯一键与代码语义不一致的问题。
- 修复 `site_message_reads` 的已读状态不是租户隔离的问题。
- 修复 `dashboard_configs` 的租户唯一键与 repository 行为不一致的问题。
- 修复 `user_tenant_roles` 单角色语义与数据库唯一约束不一致的问题。
- 修复 `user_favorites` / `user_recents` / Dashboard 配置在 `tenant_id IS NULL` 时唯一性不受保护的问题。

业务影响：

- 用户偏好、站内信已读、Dashboard 配置、收藏、最近访问将按租户正确隔离。
- 同一用户在不同租户的配置不应再互相覆盖。

### 5. Token 与日志安全

- 收紧 query token：仅 SSE 端点允许通过 query 传递 token。
- 对日志中的 `token` / `access_token` / `refresh_token` 做脱敏。
- 刷新 token 增加黑名单校验与轮换。
- 登出接口支持同时吊销 refresh token。

业务影响：

- 普通 API 不再允许通过 query token 访问。
- SSE 场景仍保留兼容能力。
- refresh token 被使用后会轮换，登出时如果前端同时传入 refresh token，则会一并失效。

## 新增/修改的数据库迁移

本轮新增：

- [051_fix_tenant_unique_constraints.up.sql](/root/auto-healing/migrations/051_fix_tenant_unique_constraints.up.sql)
- [052_user_tenant_roles_single_role.up.sql](/root/auto-healing/migrations/052_user_tenant_roles_single_role.up.sql)
- [053_fix_null_tenant_uniques.up.sql](/root/auto-healing/migrations/053_fix_null_tenant_uniques.up.sql)
- [054_dashboard_config_tenant_unique.up.sql](/root/auto-healing/migrations/054_dashboard_config_tenant_unique.up.sql)

同时补充了对应的 `down.sql`。

上线注意：

- 这些迁移会调整唯一键和部分历史数据语义。
- 涉及多租户配置、已读状态、工作区权限时，建议先在测试库验证迁移效果。
- 如果线上已有脏数据，迁移中的去重/回填逻辑会保留一份记录并删除重复项。

## 新增测试

新增或补充的测试覆盖了以下修复点：

- query token 仅限 SSE 端点
- refresh token 黑名单行为
- Dashboard 配置租户隔离
- Dashboard 工作区权限不串租户
- 默认租户顺序稳定
- Flow 实例状态 compare-and-set
- Execution run 被取消后不再被结果覆盖

## 联调/配合事项

### 前端

- `logout` 建议同时提交 `refresh_token`，这样后端可以一并吊销 refresh token。
- `/api/v1/common/*` 需要理解当前公共路由也带租户上下文语义。
- 如果页面明确切换租户，仍建议显式传 `X-Tenant-ID`。

### 测试

- 验证租户用户无法跨租户操作用户/角色。
- 验证 Dashboard 工作区在不同租户下的可见性。
- 验证取消流程、审批超时、调度失败计数、once 调度完成时机。
- 验证用户偏好、已读状态、收藏、最近访问在不同租户下互不覆盖。

### 运维

- 执行数据库迁移前建议备份相关表：
  - `user_preferences`
  - `site_message_reads`
  - `dashboard_configs`
  - `user_tenant_roles`
  - `user_favorites`
  - `user_recents`

## 验证结果

已执行：

```bash
go test ./...
```

当前通过。

## 仍需持续补强的方向

虽然本轮已补充一批测试，但以下方向仍建议继续增强：

- 更完整的 handler/e2e 级租户隔离测试
- 自愈流程审批/取消/重试的集成测试
- 调度器真实执行结果判定的端到端测试
- migration 在历史脏数据场景下的回归测试
