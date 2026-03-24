# 验收实测报告

更新时间：2026-03-22

本文档记录本轮基于真实接口与真实数据库的验收实测结果。

## 测试环境

本次使用两套运行方式验证：

- `127.0.0.1:18080`
  当前工作区代码通过 Go 容器运行，连接独立验收数据库 `auto_healing_acceptance`
- `127.0.0.1:18081`
  当前工作区代码编译为静态宿主机 binary 运行，用于验证本机 `ansible-playbook` 相关执行链路

共享资源：

- PostgreSQL: `auto-healing-postgres`
- Redis: `auto-healing-redis`
- 验收数据库: `auto_healing_acceptance`

辅助服务：

- Mock ITSM: `127.0.0.1:5005`
- Mock CMDB: `127.0.0.1:5006`

## 本轮实测中发现并修复的问题

### 1. fresh-db + AutoMigrate 场景下 `role_workspaces` 缺列

现象：

- Dashboard 配置读取与工作区创建在全新数据库场景下直接 500
- 根因是 `role_workspaces` 先被 many-to-many join table 形态创建，缺 `tenant_id / id / created_at`

修复：

- 在 `AutoMigrate` 后追加 schema 修正逻辑，自动补齐 `role_workspaces` 业务所需列和索引

相关文件：

- [database.go](/root/auto-healing/internal/database/database.go)

### 2. 待审批流程取消后审批单仍可继续走审批动作

现象：

- 流程已取消，但审批单仍保留可操作状态
- 之后再点审批，会造成审批状态和流程状态分叉

修复：

- 取消流程时同步关闭待审批单
- 审批时再校验流程实例当前状态

相关文件：

- [healing_handler.go](/root/auto-healing/internal/handler/healing_handler.go)
- [healing.go](/root/auto-healing/internal/repository/healing.go)

### 3. 审批节点“创建审批单 + 切换待审批”不是同事务

现象：

- 并发状态变化时可能留下孤儿审批单

修复：

- 改为 repository 层事务方法 `CreateAndEnterWaiting`

相关文件：

- [executor.go](/root/auto-healing/internal/service/healing/executor.go)
- [healing.go](/root/auto-healing/internal/repository/healing.go)

## 已真实验证通过的场景

### 一、认证与会话

已验证：

- `login` 成功获取 `access_token` / `refresh_token`
- `auth/me` 正常返回用户信息
- `refresh` 成功后旧 refresh token 立即失效
- `logout + refresh_token` 后：
  - access token 失效
  - refresh token 失效

结果：通过

### 二、多租户公共数据隔离

使用同一用户 `tenantadmin`，分别在租户 A、租户 B 下操作：

- 用户偏好 `common/user/preferences`
- 收藏 `common/user/favorites`
- 最近访问 `common/user/recents`

结果：

- A/B 数据互不覆盖
- 不传 `X-Tenant-ID` 时使用稳定默认租户
- 显式传租户头时切换正确

结果：通过

### 三、Dashboard 配置与工作区

已验证：

- 同一用户在租户 A/B 保存不同 Dashboard 配置
- 读取配置时严格返回当前租户对应值
- 在租户 A/B 分别创建系统工作区
- 列表不串租户

结果：通过

### 四、租户用户 / 角色边界

已验证：

- 租户 A 上下文下访问租户 B 用户详情 → 被拒绝
- 租户 A 上下文下访问租户 B 自定义角色 → 被拒绝
- 正确租户上下文下访问同一资源 → 成功

结果：通过

### 五、Impersonation 审批链路

已验证：

- 租户设置审批人
- 平台管理员提交申请
- 租户审批人查看 pending 列表
- 审批通过
- 平台管理员进入租户视角
- 使用 Impersonation 头访问租户数据
- 退出租户视角
- 撤销申请后再尝试审批 → 被拒绝

结果：通过

### 六、搜索租户隔离

已验证：

- 在租户 A、B 创建不同插件数据
- `/api/v1/common/search` 在默认租户与显式租户头场景下返回不同结果

结果：通过

### 七、站内信已读状态隔离

已验证：

- 平台侧定向向租户 A/B 发送同一标题消息
- 同一用户在租户 A 标记已读
- 租户 B 的未读数不受影响

结果：通过

### 八、Git / Playbook 业务链路

已验证：

- 创建本地 Git 仓库并提交 playbook 文件
- 通过 API 创建 Git 仓库记录
- 获取 commits / files
- 创建 playbook 记录

结果：通过

### 九、自愈主链路（incident -> rule -> flow -> approval）

已验证：

- 通过 Mock ITSM 插件同步真实 incident
- 创建手动触发规则
- 创建最小审批流 `start -> approval -> end`
- 调度器完成 incident 规则匹配
- 待触发列表可见
- 手动触发后实例进入 `waiting_approval`
- 审批通过后实例进入 `completed`
- 待审批取消后：
  - 实例变 `cancelled`
  - 审批单变 `cancelled`
  - 再次审批返回冲突/已处理

结果：通过

### 十、执行与取消链路

已验证：

- 宿主机版服务创建 Git 仓库 / Playbook / Task
- 使用 `connection: local` playbook 执行本机命令
- 任务成功执行，run 进入 `success`
- 长任务执行后手动取消，run 进入 `cancelled`
- 日志 SSE 在终态返回 `done`

结果：通过

### 十一、SSE query token 策略

已验证：

- 普通 API 带 `?token=` → 返回未授权
- 站内信 SSE 带 `?token=` → 正常建立连接并收到 `init`

结果：通过

## 数据库侧结果快照

验收过程中，数据库中已观察到：

- `incidents`: 已同步并完成规则匹配
- `flow_instances`: 存在 `completed` 与 `cancelled`
- `approval_tasks`: 存在 `approved` 与 `cancelled`
- `execution_runs`: 存在 `success`、`failed`、`cancelled`

说明关键业务链路已真正落库，不是仅接口假返回。

## 自动化验证

执行通过：

```bash
go test ./...
go vet ./...
git diff --check
```

并新增/保留了以下关键测试：

- [auth_test.go](/root/auto-healing/internal/middleware/auth_test.go)
- [jwt_test.go](/root/auto-healing/internal/pkg/jwt/jwt_test.go)
- [multitenancy_test.go](/root/auto-healing/internal/repository/multitenancy_test.go)
- [state_machine_test.go](/root/auto-healing/internal/repository/state_machine_test.go)

当前覆盖率快照（按已补重点包）：

- `internal/middleware`: 0.7%
- `internal/pkg/jwt`: 54.0%
- `internal/repository`: 1.8%
- `internal/service/healing`: 3.7%

说明：

- 总覆盖率仍不高，但高风险修复点已开始有针对性保护。
- 下一阶段应继续补 handler / e2e 级用例，而不是单纯追求数字。

## 尚未完整覆盖的项

以下项还没有做到完全端到端覆盖，建议放进正式验收或下一轮补测：

- `partial` 执行结果的真实生成场景（当前已修代码并验证终态处理，但未人工造出真实 partial run）
- cron 长期调度下的连续失败自动暂停
- 插件同步“部分记录入库失败”的真实场景
- Git 同步“最终状态落库失败”的真实场景（代码已修，未做故障注入式实测）

## 当前建议

可以进入正式的最终验收测试。

建议使用：

- [final-acceptance-test-checklist-2026-03-22.md](/root/auto-healing/docs/final-acceptance-test-checklist-2026-03-22.md)

作为测试执行清单。
