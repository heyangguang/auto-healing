# Auto-Healing 后端代码全量审计报告

> 审计日期：2026-03-25
> 审计范围：`/root/auto-healing` 全量后端 Go 代码（~208 个 .go 文件）
> 技术栈：Go + Gin + GORM + Zap + Redis，多租户 SaaS 架构

---

## 目录

**Part A — 项目结构与规范审计**

- [1. 项目架构概览](#1-项目架构概览)
- [A1. Handler 层结构审计](#a1-handler-层结构审计)
- [A2. Service 层结构审计](#a2-service-层结构审计)
- [A3. Repository 层结构审计](#a3-repository-层结构审计)
- [A4. 错误定义规范审计](#a4-错误定义规范审计)
- [A5. 分层职责违规](#a5-分层职责违规)
- [A6. 响应格式规范审计](#a6-响应格式规范审计)
- [A7. 接口定义缺失](#a7-接口定义缺失)
- [A8. God File 与文件组织](#a8-god-file-与文件组织)

**Part B — 代码质量与安全审计**

- [B1. P0 — 安全漏洞](#b1-p0--安全漏洞)
- [B2. P1 — 响应格式不一致（代码级）](#b2-p1--响应格式不一致代码级)
- [B3. P2 — Handler 参数解析不安全](#b3-p2--handler-参数解析不安全)
- [B4. P3 — 服务层错误处理缺陷](#b4-p3--服务层错误处理缺陷)
- [B5. P4 — 事务管理缺失](#b5-p4--事务管理缺失)
- [B6. P5 — 并发安全](#b6-p5--并发安全)
- [B7. P6 — 资源泄漏](#b7-p6--资源泄漏)
- [B8. P7 — 日志规范不一致](#b8-p7--日志规范不一致)
- [B9. P8 — 缺少统一分页工具](#b9-p8--缺少统一分页工具)

**Part C — 修复方案**

- [C1. 修复方案](#c1-修复方案)
- [C2. 验证方案](#c2-验证方案)
- [C3. 关键文件清单](#c3-关键文件清单)

---

## 1. 项目架构概览

```
cmd/
  server/main.go          ← HTTP 服务入口
  init-admin/main.go      ← 管理员初始化 CLI

internal/
  config/                 ← 配置管理 (Viper)
  database/               ← 数据库初始化 & 迁移
  handler/                ← HTTP 请求处理层 (Gin)
  middleware/             ← 中间件（认证/租户/审计/权限）
  model/                  ← 数据模型 (GORM)
  repository/             ← 数据访问层
  service/                ← 业务逻辑层
    auth/                 ← 认证服务
    execution/            ← 执行任务服务
    git/                  ← Git 仓库管理
    healing/              ← 自愈流程引擎
    playbook/             ← Playbook 管理
    plugin/               ← 插件管理
    schedule/             ← 调度管理
    secrets/              ← 密钥管理
  scheduler/provider/     ← 后台调度器
  engine/provider/ansible/← Ansible 执行引擎
  git/                    ← Git 客户端封装
  notification/           ← 通知渠道
  secrets/                ← 密钥源 Provider
  pkg/
    logger/               ← 日志 (Zap)
    jwt/                  ← JWT 认证
    response/             ← 统一响应
    crypto/               ← 加密工具
```

**分层调用链**：Handler → Service → Repository → GORM/DB

**多租户隔离**：通过 `repository.TenantDB(db, ctx)` 自动注入 `WHERE tenant_id = ?`

---

# Part A — 项目结构与规范审计

## A1. Handler 层结构审计

### A1.1 依赖注入模式：三种混合共存

项目声明的架构是 `Handler → Service → Repository`，但实际 handler 层存在 **三种不同的依赖模式**：

**模式 A — 注入 Service（正确做法）**：

| Handler | 注入的 Service |
|---------|---------------|
| `ExecutionHandler` | `*execution.Service` |
| `PlaybookHandler` | `*playbook.Service` |
| `GitRepoHandler` | `*gitSvc.Service` |
| `ScheduleHandler` | `*schedule.Service` |
| `SecretsHandler` | `*secretsSvc.Service` |
| `CMDBHandler` | `*plugin.CMDBService` |
| `CommandBlacklistHandler` | `*service.CommandBlacklistService` |
| `DictionaryHandler` | `*service.DictionaryService` |

**模式 B — 直接注入 Repository（绕过 Service 层）** :

| Handler | 直接注入的 Repository |
|---------|----------------------|
| `AuditHandler` | `*repository.AuditLogRepository` |
| `PlatformAuditHandler` | `*repository.PlatformAuditLogRepository` |
| `PreferenceHandler` | `*repository.UserPreferenceRepository` |
| `UserActivityHandler` | `*repository.UserActivityRepository` |
| `DashboardHandler` | `*repository.DashboardRepository` |
| `SearchHandler` | `*repository.SearchRepository` |
| `WorkbenchHandler` | `*repository.WorkbenchRepository` |

**模式 C — 混合注入（Service + Repository + 其他）** :

| Handler | 注入组合 |
|---------|---------|
| `HealingHandler` | 6 个 Repository + FlowExecutor + Scheduler（无 Service 层） |
| `UserHandler` | `*UserRepository` + `*RoleRepository` + `*authService.Service` |
| `TenantHandler` | `*TenantRepository` + `*RoleRepository` + `*UserRepository` + `*authService.Service` |
| `TenantUserHandler` | 同上 |
| `TenantMemberHandler` | 类似 |
| `NotificationHandler` | `*notification.Service` + `*NotificationRepository` |
| `BlacklistExemptionHandler` | `*BlacklistExemptionService` + `*ExecutionRepository` + `*CommandBlacklistRepository` |
| `AuthHandler` | `*authService.Service` + `*jwt.Service` + 3 个 Repository |

**问题**: 模式 B 和 C 破坏了分层架构。Handler 直接操作 Repository 意味着：
- 无法在 Service 层统一增加业务校验、审计、缓存等横切关注点
- Handler 和 Repository 强耦合，难以单元测试
- 同样的业务逻辑可能在多个 Handler 中重复

### A1.2 响应格式：三种不同模式

| 模式 | 使用的 Handler | 格式 |
|------|---------------|------|
| `response.Success/BadRequest/...` | 大多数 handler | `{"code": 0, "message": "success", "data": ...}` |
| `c.JSON(http.StatusOK, gin.H{...})` | `command_blacklist_handler.go`, `blacklist_exemption_handler.go` | `{"data": ..., "total": ..., "page": ...}` |
| 混合 | `healing_handler.go` | 两种都有 |

**`command_blacklist_handler.go:53-60`** — 完全未使用 `response` 包：
```go
c.JSON(http.StatusInternalServerError, gin.H{"error": "查询失败: " + err.Error()})
// ...
c.JSON(http.StatusOK, gin.H{"data": rules, "total": total, "page": page, "page_size": pageSize})
```

**`blacklist_exemption_handler.go:47-50`** — 同上：
```go
c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
c.JSON(http.StatusOK, gin.H{"data": items, "total": total})
```

### A1.3 DTO 组织：两种模式

| 模式 | 文件 | 说明 |
|------|------|------|
| 独立 `*_dto.go` 文件 | `auth_dto.go`, `execution_dto.go`, `git_dto.go`, `healing_dto.go`, `plugin_dto.go`, `secrets_dto.go` | 有 `ToModel()` 转换方法 |
| 内联定义在 handler 文件 | `playbook_handler.go`, `user_handler.go`, `tenant_handler.go` 等 | struct 定义在方法上方 |

**问题**: 无统一规则决定什么时候用独立 DTO 文件，什么时候内联。

### A1.4 Handler 构造函数签名

| 模式 | 示例 | 问题 |
|------|------|------|
| 无参数 | `NewPlaybookHandler()` | 内部 hardcode `repository.NewXxx()` |
| 传入 config | `NewAuthHandler(cfg *config.Config)` | 部分依赖注入 |
| 传入 service | `NewUserHandler(authSvc *authService.Service)` | 部分依赖注入 |

**问题**: 有的 handler 所有依赖在内部 new，有的部分由外部传入。无统一 DI 策略。

---

## A2. Service 层结构审计

### A2.1 Service 文件组织：两种并存

**模式 A — 独立子包**（8 个）：

| 包 | 文件数 | 主 Struct | 构造函数 |
|----|--------|-----------|---------|
| `service/auth/` | 1 | `Service` | `NewService(jwtSvc)` |
| `service/execution/` | 1 | `Service` | `NewService()` |
| `service/git/` | 2 | `Service` | `NewService()` |
| `service/healing/` | 11 | `FlowExecutor`, `Scheduler` | 各自构造 |
| `service/playbook/` | 1 | `Service` | `NewService()` |
| `service/plugin/` | 4 | `Service`, `CMDBService`, `IncidentService` | 各自构造 |
| `service/schedule/` | 1 | `Service` | `NewService()` |
| `service/secrets/` | 1 | `Service` | `NewService()` |

**模式 B — 散落在 `service/` 根目录的文件**（6 个）：

| 文件 | Struct |
|------|--------|
| `blacklist_exemption_service.go` | `BlacklistExemptionService` |
| `command_blacklist_service.go` | `CommandBlacklistService` |
| `dictionary_service.go` | `DictionaryService` |
| `dictionary_seeds.go` | （种子数据，无 struct） |
| `dictionary_seeds_extra.go` | （种子数据） |
| `platform_email.go` | `PlatformEmailService` |

**问题**:
- 没有统一规则决定 service 放子包还是根目录
- 根目录 service 用完整名称（`BlacklistExemptionService`），子包 service 用短名称（`Service`）
- `dictionary_seeds*.go` 是数据文件但放在 service 目录

### A2.2 Service 直接访问 `database.DB` 绕过 Repository

| 文件 | 行为 |
|------|------|
| `healing/scheduler.go` | `database.DB.WithContext(ctx).Where(...).Find(...)` 直接查询 |
| `healing/dryrun.go` | `database.DB.Where(...)` 直接查询 |
| `healing/executor.go` | `database.DB.WithContext(ctx)` 直接更新 |

**问题**: Service 应通过 Repository 访问数据库。直接用 `database.DB` 破坏分层，且无法被 mock 测试。

### A2.3 `healing/` 包职责过重

`internal/service/healing/` 包含 11 个 Go 文件，承担了：
- 流程执行引擎（`executor.go`, `nodes.go`）
- 调度器（`scheduler.go`, `scheduler_async.go`）
- 事件总线（`event_bus.go`）
- 表达式引擎（`expression.go`）
- 规则匹配器（`matcher.go`）
- 试运行（`dryrun.go`）

应考虑拆分为 `healing/engine/`、`healing/scheduler/`、`healing/matcher/` 等子包。

---

## A3. Repository 层结构审计

### A3.1 数据库引用模式：两种并存

**模式 A — 存储 `db *gorm.DB` 字段**（28 个 Repository）：

```go
type UserRepository struct { db *gorm.DB }
func NewUserRepository() *UserRepository {
    return &UserRepository{db: database.DB}
}
// 方法中: r.db.WithContext(ctx).Find(...)
```

所有使用此模式的 Repository：
`AuditLogRepository`, `BlacklistExemptionRepository`, `CMDBItemRepository`, `CommandBlacklistRepository`, `DashboardRepository`, `GitRepositoryRepository`, `HealingFlowRepository`, `HealingRuleRepository`, `FlowInstanceRepository`, `ApprovalTaskRepository`, `FlowLogRepository`, `ImpersonationRepository`, `InvitationRepository`, `NotificationRepository`, `PlatformAuditLogRepository`, `PlatformSettingsRepository`, `PluginRepository`, `PluginSyncLogRepository`, `IncidentRepository`, `PermissionRepository`, `RoleRepository`, `SearchRepository`, `SecretsSourceRepository`, `SiteMessageRepository`, `TenantRepository`, `UserRepository`, `UserActivityRepository`, `UserPreferenceRepository`, `WorkbenchRepository`, `WorkspaceRepository`

**模式 B — 空 struct，直接用 `database.DB` 全局变量**（4 个 Repository）：

```go
type ExecutionRepository struct{} // 空 struct
func NewExecutionRepository() *ExecutionRepository {
    return &ExecutionRepository{}
}
// 方法中: database.DB.WithContext(ctx).Find(...)
```

| Repository | 文件 |
|------------|------|
| `ExecutionRepository` | `execution.go` |
| `PlaybookRepository` | `playbook.go` |
| `ScheduleRepository` | `schedule.go` |
| `DictionaryRepository` | `dictionary_repository.go` |

**问题**: 空 struct 模式直接引用全局变量，无法在测试中注入 mock DB。

### A3.2 构造函数参数不一致

| 模式 | 示例 | 数量 |
|------|------|------|
| 无参数，内部用 `database.DB` | `NewUserRepository() → {db: database.DB}` | ~28 |
| 接受 `*gorm.DB` 参数 | `NewNotificationRepository(db *gorm.DB)` | 2 |
| 无参数，空 struct | `NewExecutionRepository() → {}` | 4 |

只有 `NotificationRepository` 和 `BlacklistExemptionRepository` 接受外部注入的 `db` 参数，其余全部 hardcode。

### A3.3 文件命名不一致

| 模式 | 文件 | 数量 |
|------|------|------|
| 域名命名 `xxx.go` | `user.go`, `healing.go`, `plugin.go`, `git.go`, `secrets.go` 等 | ~27 |
| 显式后缀 `*_repository.go` | `blacklist_exemption_repository.go`, `command_blacklist_repository.go`, `dictionary_repository.go`, `workspace_repository.go` | 4 |

### A3.4 单文件多 Repository（God File）

| 文件 | 包含的 Repository | 行数 |
|------|-------------------|------|
| `healing.go` | `HealingFlowRepository`, `HealingRuleRepository`, `FlowInstanceRepository`, `ApprovalTaskRepository`, `FlowLogRepository` | ~1200+ |
| `plugin.go` | `PluginRepository`, `PluginSyncLogRepository`, `IncidentRepository` | ~400 |
| `role.go` | `RoleRepository`, `PermissionRepository` | ~350 |

---

## A4. 错误定义规范审计

### A4.1 Sentinel Error 分布（散落各处）

| 文件 | 定义的错误 |
|------|-----------|
| `repository/user.go` | `ErrUserNotFound`, `ErrUserExists`, `ErrRoleNotFound`, `ErrPermissionNotFound` |
| `repository/healing.go` | `ErrHealingFlowNotFound`, `ErrHealingRuleNotFound`, `ErrFlowInstanceNotFound` 等 |
| `repository/cmdb.go` | `ErrCMDBItemNotFound` |
| `repository/plugin.go` | Plugin 相关错误 |
| `repository/tenant_scope.go` | `ErrTenantContextRequired` |
| `service/auth/service.go` | `ErrInvalidCredentials`, `ErrUserLocked`, `ErrUserNotFound` 等 |
| `git/errors.go` | Git 相关错误 |
| `secrets/provider/errors.go` | Secrets 相关错误 |

**没有错误定义的 Repository**（直接透传 GORM 错误）：
- `execution.go`
- `playbook.go`
- `schedule.go`
- `dictionary_repository.go`

**问题**:
- 没有统一的 errors 包或 errors 文件
- Service 和 Repository 都定义错误，职责不清
- 部分模块完全没有定义业务错误，直接透传 GORM 底层错误给 Handler

### A4.2 错误匹配方式不一致

| 方式 | 示例 | 使用位置 |
|------|------|---------|
| `errors.Is(err, ErrXxx)` | `errors.Is(err, repository.ErrUserNotFound)` | auth service 等 |
| `strings.Contains(err.Error(), "...")` | `strings.Contains(err.Error(), "密钥源不存在")` | secrets_handler.go |
| 直接 `err.Error()` 返回 | `response.BadRequest(c, err.Error())` | 多个 handler |

---

## A5. 分层职责违规

### A5.1 Handler 中包含数据库事务（应在 Service/Repository 层）

**`user_handler.go:214-233`**:
```go
if err := database.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
    if err := tx.Save(user).Error; err != nil { return err }
    if targetRole != nil {
        if err := tx.Where("user_id = ?", user.ID).Delete(&model.UserPlatformRole{}).Error; err != nil { return err }
        if err := tx.Table("user_platform_roles").Create(map[string]any{...}).Error; err != nil { return err }
        // ...
    }
    return nil
}); err != nil {
```

**问题**: Handler 直接操作 `database.DB` 和 GORM Transaction，绕过了所有层。

### A5.2 Handler 中包含复杂业务逻辑（应在 Service 层）

**`healing_handler.go`**: 整个 handler 是一个"大 Controller"，直接操作 6 个 Repository，包含大量业务判断逻辑（流程状态机、规则匹配、审批流）。应有对应的 `healing.Service` 统一封装。

**`user_handler.go`**: `UpdateUser` 方法内包含"最后一个平台管理员"校验逻辑、角色变更逻辑，应由 UserService 处理。

### A5.3 Service 直接访问 database.DB（应通过 Repository）

| 文件 | 违规代码 |
|------|---------|
| `healing/scheduler.go` | `database.DB.WithContext(ctx).Where("status = ?", "expired").Find(&expiredTasks)` |
| `healing/dryrun.go` | `database.DB.Where("id = ?", id).First(&flow)` |
| `healing/executor.go` | `database.DB.WithContext(ctx).Model(&model.HealingFlowInstance{}).Update(...)` |

---

## A6. 响应格式规范审计

### A6.1 三种并存的响应格式

**格式 1 — `response` 包（标准）**:
```json
{"code": 0, "message": "success", "data": {...}}
{"code": 40000, "message": "请求参数错误", "error_code": "VALIDATION_ERROR"}
```
使用者：大多数 handler、`access_denied.go` middleware

**格式 2 — 裸 `gin.H`（middleware）**:
```json
{"code": 40300, "message": "此接口为租户级资源..."}
```
使用者：`tenant.go`, `impersonation.go`

**格式 3 — 嵌套 `gin.H`（auth middleware）**:
```json
{"error": {"code": "UNAUTHORIZED", "message": "Missing authorization header"}}
```
使用者：`auth.go`

**格式 4 — 完全不同的 `gin.H`（部分 handler）**:
```json
{"data": [...], "total": 10, "page": 1, "page_size": 20}
{"error": "查询失败: ..."}
```
使用者：`command_blacklist_handler.go`, `blacklist_exemption_handler.go`

---

## A7. 接口定义缺失

### A7.1 Repository 层无接口

**现状**: 所有 34 个 Repository 均为具体 struct，**没有定义任何 interface**。

**影响**:
- Handler/Service 直接依赖具体类型，无法 mock
- 不可能写真正的单元测试（只能写集成测试）
- 替换实现（如从 PostgreSQL 迁移到 ClickHouse）需要改所有调用方

**建议**: 至少为核心 Repository 定义 interface：
```go
// repository/interfaces.go
type UserRepo interface {
    GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
    Create(ctx context.Context, user *model.User) error
    // ...
}
```

### A7.2 Service 层无接口

同样，所有 Service 都是具体 struct，Handler 直接依赖具体类型。

---

## A8. God File 与文件组织

### A8.1 超大文件

| 文件 | 行数 | 包含内容 | 建议 |
|------|------|---------|------|
| `repository/healing.go` | ~1200+ | 5 个 Repository struct | 拆为 `healing_flow.go`, `healing_rule.go`, `flow_instance.go`, `approval_task.go`, `flow_log.go` |
| `repository/plugin.go` | ~400 | 3 个 Repository struct | 拆为 `plugin.go`, `plugin_sync_log.go`, `incident.go` |
| `repository/role.go` | ~350 | `RoleRepository` + `PermissionRepository` | 拆为 `role.go` + `permission.go` |
| `service/healing/scheduler.go` | 大型 | 调度+过期处理+扫描 | 可拆为 `scheduler.go` + `expired_handler.go` |

### A8.2 命名规范不统一一览

| 维度 | 模式 A | 模式 B | 应统一为 |
|------|--------|--------|---------|
| Repository 文件名 | `user.go` (27个) | `workspace_repository.go` (4个) | 全部 `xxx.go`（在 repository 包内不需后缀） |
| Service 位置 | 子包 `service/git/` (8个) | 根文件 `service/xxx_service.go` (4个) | 全部子包 |
| Service struct 名 | `Service` (子包内) | `BlacklistExemptionService` (根文件) | 子包内用 `Service` |
| Repository 构造 | `New...() → {db: database.DB}` | `New...() → {}` | 统一存储 `db` 字段 |
| Handler 依赖 | 注入 Service | 注入 Repository | 统一注入 Service |
| 响应格式 | `response.Success(...)` | `c.JSON(http.StatusOK, gin.H{...})` | 统一用 `response` 包 |
| 错误匹配 | `errors.Is(err, sentinel)` | `strings.Contains(err.Error(), "...")` | 统一用 `errors.Is` |
| DTO 位置 | 独立 `*_dto.go` (6个) | 内联在 handler (其余) | 统一用 `*_dto.go` |

---

## 审计问题总表

### Part A — 结构与规范问题

| 编号 | 类别 | 问题数 | 严重程度 |
|------|------|--------|----------|
| A1 | Handler 依赖模式混乱 | 3种模式共存 | **High** |
| A2 | Service 组织混乱 | 2种模式共存 | **Medium** |
| A3 | Repository 模式不一致 | 4个异常 + 命名混乱 | **High** |
| A4 | 错误定义散落 | ~8个文件 | **Medium** |
| A5 | 分层职责违规 | ~6处 | **High** |
| A6 | 响应格式不统一 | 4种格式 | **High** |
| A7 | 接口定义缺失 | 全局 | **Critical** |
| A8 | God File / 命名混乱 | ~5个大文件 | **Medium** |

### Part B — 代码质量与安全问题

| 编号 | 类别 | 问题数 | 严重程度 |
|------|------|--------|----------|
| **B1** | 安全漏洞 | 3 | **Critical** |
| **B2** | 响应格式不一致（代码级） | ~25处 | High |
| **B3** | Handler 参数解析不安全 | 6 | High |
| **B4** | 服务层错误处理缺陷 | 5 | High |
| **B5** | 事务管理缺失 | 3 | High |
| **B6** | 并发安全 | 4 | High |
| **B7** | 资源泄漏 | 3 | Medium |
| **B8** | 日志规范不一致 | 4 | Medium |
| **B9** | 缺少统一分页工具 | 全局 | Low |

---

# Part B — 代码质量与安全审计

| 优先级 | 类别 | 问题数 | 严重程度 |
|--------|------|--------|----------|
| **P0** | 安全漏洞 | 3 | **Critical** |
| **P1** | 响应格式不一致 | ~25处 (3文件) | High |
| **P2** | Handler 参数解析不安全 | 6 | High |
| **P3** | 服务层错误处理缺陷 | 5 | High |
| **P4** | 事务管理缺失 | 3 | High |
| **P5** | 并发安全 | 4 | High |
| **P6** | 资源泄漏 | 3 | Medium |
| **P7** | 日志规范不一致 | 4 | Medium |
| **P8** | 缺少统一分页工具 | 全局 | Low |
| | **合计** | **40+** | |

---

## B1. P0 — 安全漏洞

### S1: Git 凭据泄露到错误消息

**文件**: `internal/git/client.go`
**行号**: 68, 121

**问题**: `Clone()` 和 `Pull()` 方法将 `stderr.String()` 直接嵌入错误消息。Git 的 stderr 在认证失败时会包含带凭据的 URL（如 `https://ghp_xxxxx@github.com/...`），导致 token/password 泄露到日志和前端。

**当前代码**:
```go
// client.go:68
return fmt.Errorf("克隆失败: %s", stderr.String())

// client.go:121
return fmt.Errorf("拉取失败: %s", stderr.String())
```

**修复**: 新增 `redactCredentials()` 函数，用正则替换 URL 中的认证信息为 `***`。

---

### S2: ValidateAndListBranches 同样存在凭据泄露

**文件**: `internal/git/client.go`
**行号**: 185-208

**问题**: `ValidateAndListBranches()` 方法在最后的 fallback 分支中：
```go
err = fmt.Errorf("仓库验证失败: %s", errMsg)
```
`errMsg` 来自 `stderr.String()`，可能包含认证信息。

---

### S3: 路径穿越 — symlink 绕过

**文件**: `internal/service/git/service.go`
**行号**: ~544-548

**问题**: 文件读取接口使用 `filepath.Rel` 检查路径是否在仓库目录内，但未解析 symlink。攻击者可在仓库中创建 symlink 指向 `/etc/passwd` 等系统文件，绕过目录限制。

**当前代码**:
```go
fullPath := filepath.Join(repo.LocalPath, path)
relPath, err := filepath.Rel(repo.LocalPath, fullPath)
if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
    return "", fmt.Errorf("非法路径")
}
```

**修复**: 在 `filepath.Rel` 之前先调用 `filepath.EvalSymlinks()` 解析真实路径。

---

## B2. P1 — 响应格式不一致（代码级）

### 问题描述

Handler 层通过 `response` 包返回统一的 `Response` JSON 结构：
```json
{"code": 40100, "message": "...", "error_code": "...", "details": ...}
```

但中间件层使用原始 `gin.H` 构建响应，导致**两种不同的 JSON 格式**从 API 出去。

### R1: `middleware/auth.go` — 嵌套 error 结构（5处）

**行号**: 37, 47, 59, 70, 84

**当前格式**（与 handler 不一致）:
```json
{"error": {"code": "UNAUTHORIZED", "message": "Missing authorization header"}}
```

**应该是**:
```json
{"code": 40100, "message": "Missing authorization header", "error_code": "UNAUTHORIZED"}
```

### R2: `middleware/tenant.go` — 扁平但缺字段（~15处）

**行号**: 51, 59, 68, 79, 90, 103, 112, 129, 136, 174, 182, 215, 223, 233, 243, 252, 282, 289, 323

**当前格式**:
```json
{"code": 40300, "message": "此接口为租户级资源..."}
```

**缺少**: `error_code` 字段，不符合统一 `Response` 结构。

### R3: `middleware/impersonation.go` — 同上模式（7处）

**行号**: 88, 98, 107, 117, 127, 136, 146

---

### 标杆参考

`middleware/access_denied.go` 已正确使用 `response.ErrorWithMetadata()` + `c.Abort()`，是中间件响应的标准范例。

---

## B3. P2 — Handler 参数解析不安全

### 问题描述

部分 handler 使用 `page, _ := strconv.Atoi(...)` 忽略解析错误。当用户传入非数字参数（如 `?page=abc`）时，`strconv.Atoi` 返回 `(0, error)`，但 error 被 `_` 丢弃，导致 `page=0`。

### V1: `handler/audit_handler.go`

**行号**: 32-33, 179, 203, 221, 238, 258
```go
page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
```

### V2: `handler/platform_audit_handler.go`

**行号**: 28-29 — 同上模式

### V3: `handler/execution_handler.go`

**行号**: 90-91 — 同上模式

### V4: `handler/git_handler.go`

**行号**: 49-50 — 同上模式

### V5: `handler/secrets_handler.go` — 暴露原始错误

**行号**: 55-56
```go
response.BadRequest(c, "请求参数错误: "+err.Error())
```
应使用 `FormatValidationError(err)` 格式化为中文友好信息。

### V6: `handler/secrets_handler.go` — 无详情错误

**行号**: 97-98
```go
response.BadRequest(c, "请求参数错误")
```
缺少具体字段错误信息，不利于前端调试。

---

### 标杆参考

`handler/cmdb_handler.go:68-77` — 安全解析:
```go
page := 1
if p := c.Query("page"); p != "" {
    if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
        page = parsed
    }
}
```

`handler/user_handler.go:86` — 正确的验证错误格式化:
```go
response.BadRequest(c, "请求参数错误: "+FormatValidationError(err))
```

---

## B4. P3 — 服务层错误处理缺陷

### E1: Git Service — Update 未检查 error

**文件**: `internal/service/git/service.go`
**行号**: ~72

```go
repo.LocalPath = localPath
s.repo.Update(ctx, repo) // error 被忽略
```

如果 `Update` 失败，`LocalPath` 不会持久化但后续代码继续执行。

### E2: Git Service — Rollback Delete 忽略 error

**文件**: `internal/service/git/service.go`
**行号**: ~77

```go
if err := s.syncRepoInternal(ctx, repo, "create"); err != nil {
    s.repo.Delete(ctx, repo.ID) // error 被忽略，可能留下孤儿记录
    return nil, fmt.Errorf("首次同步失败: %w", err)
}
```

### E3: Execution Service — 静默跳过通知

**文件**: `internal/service/execution/service.go`
**行号**: ~376-384

```go
run, err := s.repo.GetRunByID(ctx, runID)
if err == nil {
    // 发送通知...
}
// 如果 GetRunByID 失败，整个通知逻辑被静默跳过，无任何日志
```

### E4: Handler 暴露内部错误

**文件**: `internal/handler/audit_handler.go`
**行号**: 79

```go
response.InternalError(c, err.Error()) // 数据库错误细节直接返回给前端
```

应改为通用消息 + 服务端日志记录。全局存在类似问题的 handler 还有多个。

### E5: Secrets Handler 脆弱的字符串匹配

**文件**: `internal/handler/secrets_handler.go`
**行号**: 104-113

```go
if strings.Contains(err.Error(), "record not found") ||
   strings.Contains(err.Error(), "密钥源不存在") {
    response.NotFound(c, "密钥源不存在")
    return
}
if strings.Contains(err.Error(), "无法更新：") {
    response.Conflict(c, err.Error())
    return
}
```

用 `strings.Contains` 匹配错误消息极其脆弱 — 消息文案变更即导致误判。应使用 sentinel errors + `errors.Is()`。

---

## B5. P4 — 事务管理缺失

### T1: Healing Scheduler — processIncident 跨表更新无事务

**文件**: `internal/service/healing/scheduler.go`
**行号**: ~189-216

```go
// 三个独立的 DB 操作，任何一个失败都会导致数据不一致
instance := s.createFlowInstance(ctx, incident, matchedRule)
s.incidentRepo.MarkScanned(ctx, incident.ID, &matchedRule.ID, &instance.ID)
s.ruleRepo.UpdateLastRunAt(ctx, matchedRule.ID)
```

如果 `UpdateLastRunAt` 失败：incident 已标记为 scanned 但 rule 的 `last_run_at` 未更新。

### T2: Healing Scheduler — processExpiredApprovals 直接用 database.DB

**文件**: `internal/service/healing/scheduler.go`
**行号**: ~325-356

查询和更新使用 `database.DB.WithContext(ctx)` 直接操作，多个更新不在同一事务中。

### T3: Git Service — Create 方法无事务

**文件**: `internal/service/git/service.go`
**Create 方法**

`Create` → `SyncRepo` → `Update` 三步操作，中间失败可能导致数据库中存在状态不一致的记录。

---

## B6. P5 — 并发安全

### C1: Execution Service — 无界 goroutine 生成

**文件**: `internal/service/execution/service.go`
**行号**: ~319

```go
go s.executeInBackground(run.ID, task, playbook, gitRepo, execOpts, task.TenantID)
```

每次 API 调用都生成新 goroutine，无任何并发限制。高负载下可能耗尽系统资源。

> 对比：`healing/scheduler.go:51` 已使用 `sem: make(chan struct{}, 10)` 做限流

### C2: Execution Service — sync.Map 无生命周期

**文件**: `internal/service/execution/service.go`
**行号**: ~36

```go
runningExecutions sync.Map // map[uuid.UUID]context.CancelFunc
```

无 `Shutdown()` 方法。服务停止时运行中的 goroutine 无法优雅终止。

### C3: Git Service — 裸 goroutine 无追踪

**文件**: `internal/service/git/service.go`
**行号**: ~232-234, ~369

```go
go s.SyncRepoWithTrigger(asyncCtx, id, "branch_change")
go s.checkPlaybooksAfterSync(detachTenantContext(ctx, repo.TenantID), id)
```

无追踪/清理机制。服务停止时这些 goroutine 可能留下不完整操作。

> 注意：项目中已有 `internal/service/git/async.go` 和 `internal/service/healing/async.go` 的 `asyncLifecycle` 模式可复用。

### C4: Execution Service — cancel 多路径调用

**文件**: `internal/service/execution/service.go`
**行号**: ~344 + defer

`cancel()` 可能被 `CancelRun()`、`watchRunCancellation`、`executeInBackground` 的 defer 三个路径调用。虽然 Go context cancel 是幂等的，但缺乏注释说明这一设计决策。

---

## B7. P6 — 资源泄漏

### L1: Ansible LocalExecutor — 临时 inventory 文件未清理

**文件**: `internal/engine/provider/ansible/local_executor.go`
**行号**: ~251-256

```go
tmpFile, err := os.CreateTemp("", "ansible-inventory-*.ini")
if err == nil {
    tmpFile.WriteString("[all]\n")
    tmpFile.WriteString(req.Inventory)
    tmpFile.Close()
    args = append(args, "-i", tmpFile.Name())
}
// 缺少 defer os.Remove(tmpFile.Name())
```

### L2: Docker Executor — 容器停止失败无 fallback

**文件**: `internal/engine/provider/ansible/docker_executor.go`
**行号**: ~200-210

```go
case <-ctx.Done():
    e.stopContainer(containerName)
```

`stopContainer` 仅调用 `docker stop`，失败后无 `docker kill` fallback，容器可能持续运行。

### L3: Ansible LocalExecutor — WriteString 错误未检查

**文件**: `internal/engine/provider/ansible/local_executor.go`
**行号**: ~252-254

```go
tmpFile.WriteString("[all]\n")      // error 未检查
tmpFile.WriteString(req.Inventory)  // error 未检查
```

写入失败会导致 inventory 文件不完整，Ansible 执行时产生难以排查的错误。

---

## B8. P7 — 日志规范不一致

### LOG1: Middleware 使用 `zap.L()` 而非项目 logger

**文件**: `internal/middleware/audit.go`
**多处**

项目统一使用 `logger.API("模块").Info/Error(...)` 分类日志，但 audit middleware 直接调用 `zap.L().Error(...)`，日志不会进入分类文件。

### LOG2: Auth 中间件无安全日志

**文件**: `internal/middleware/auth.go`
**全文件**

认证失败（无效 token、被拉黑 token、账户禁用）仅返回 HTTP 响应，**无任何服务端日志记录**。对于安全审计至关重要的认证事件完全没有日志。

### LOG3: Handler 层不记录错误日志

**全局**

所有 handler 在返回 `response.InternalError()` 时仅返回 HTTP 响应，不调用 `logger` 记录原始错误。这意味着 500 错误的根因只能从前端响应推断，无法在服务端日志中追查。

### LOG4: Execution Service 静默跳过

**文件**: `internal/service/execution/service.go`
**行号**: ~376

`GetRunByID` 失败时静默跳过通知发送，无 warn 级别日志。与 E3 同一问题的日志维度。

---

## B9. P8 — 缺少统一分页工具

### 问题描述

7+ 个 handler 文件各自解析 `page` / `page_size` 参数，写法不一致：

| 文件 | 写法 | 安全性 |
|------|------|--------|
| `cmdb_handler.go:68-77` | 带错误检查 + 范围校验 | 安全 |
| `audit_handler.go:32-33` | `_, _ = strconv.Atoi(...)` | 不安全 |
| `execution_handler.go:90-91` | 同上 | 不安全 |
| `git_handler.go:49-50` | 同上 | 不安全 |
| `platform_audit_handler.go:28-29` | 同上 | 不安全 |

部分 handler 无 `pageSize` 上限保护（service 层有 `if pageSize > 100` 但 handler 层缺失）。

---

# Part C — 修复方案

## C1. 修复方案

### Phase 0: 结构规范统一（A 类问题）

#### 0.1 统一 Handler 依赖模式 [A1]

**目标**: 所有 Handler 仅注入 Service，不直接操作 Repository。

| Handler | 当前状态 | 修复 |
|---------|---------|------|
| `AuditHandler` | 直接注入 Repo | 新建 `service/audit/service.go` |
| `PlatformAuditHandler` | 直接注入 Repo | 同上 |
| `HealingHandler` | 6 Repo + 2 组件 | 新建 `service/healing/service.go` 统一封装 |
| `UserHandler` | 混合 Repo + Service | 将 DB 事务逻辑移入 `service/auth/` 或新建 `service/user/` |
| `TenantHandler` | 混合 Repo + Service | 新建 `service/tenant/service.go` |
| `PreferenceHandler` | 直接注入 Repo | 新建 `service/preference/service.go` 或归入 user service |
| `DashboardHandler` | 直接注入 Repo | 新建 `service/dashboard/service.go` |

#### 0.2 统一 Handler 响应格式 [A6]

将 `command_blacklist_handler.go` 和 `blacklist_exemption_handler.go` 中所有 `c.JSON(http.StatusXxx, gin.H{...})` 替换为 `response.Success/BadRequest/InternalError/List(...)` 调用。

#### 0.3 统一 Repository 模式 [A3]

将 4 个空 struct Repository（`ExecutionRepository`, `PlaybookRepository`, `ScheduleRepository`, `DictionaryRepository`）改为存储 `db *gorm.DB` 字段：

```go
// Before:
type ExecutionRepository struct{}
func (r *ExecutionRepository) Create(ctx context.Context, task *model.ExecutionTask) error {
    return database.DB.WithContext(ctx).Create(task).Error
}

// After:
type ExecutionRepository struct{ db *gorm.DB }
func NewExecutionRepository() *ExecutionRepository {
    return &ExecutionRepository{db: database.DB}
}
func (r *ExecutionRepository) Create(ctx context.Context, task *model.ExecutionTask) error {
    return r.db.WithContext(ctx).Create(task).Error
}
```

#### 0.4 统一 Service 文件组织 [A2]

将 4 个散落的根目录 service 文件迁移到各自的子包：
- `service/blacklist_exemption_service.go` → `service/blacklist/service.go`
- `service/command_blacklist_service.go` → `service/blacklist/command_service.go`
- `service/dictionary_service.go` → `service/dictionary/service.go`
- `service/platform_email.go` → `service/email/service.go`
- `service/dictionary_seeds*.go` → `database/seeds/` 或 `service/dictionary/seeds.go`

#### 0.5 拆分 God File [A8]

- `repository/healing.go` → 拆为 5 个文件
- `repository/plugin.go` → 拆为 3 个文件
- `repository/role.go` → 拆为 `role.go` + `permission.go`

#### 0.6 统一命名规范 [A8]

- Repository 文件：全部用 `xxx.go`（去掉 `_repository` 后缀）
- DTO 文件：全部用 `xxx_dto.go`（从内联迁移到独立文件）

#### 0.7 消除 Service 层直接 DB 访问 [A5]

将 `healing/scheduler.go`、`healing/dryrun.go`、`healing/executor.go` 中的 `database.DB.WithContext(...)` 调用迁移到对应的 Repository 方法中。

#### 0.8 统一错误定义 [A4]

在 `repository/` 目录下按模块集中定义 sentinel errors：
- 每个 Repository 文件顶部定义该模块的 `var ErrXxx = errors.New("...")`
- Handler 统一用 `errors.Is()` 判断

---

### Phase 1: 统一响应层 + 中间件规范化

| 步骤 | 文件 | 改动 |
|------|------|------|
| 1.1 | `internal/pkg/response/response.go` | 新增 `AbortBadRequest`, `AbortUnauthorized`, `AbortForbidden`, `AbortForbiddenWithDetails` 函数 |
| 1.2 | `internal/middleware/auth.go` | 5处 `c.AbortWithStatusJSON(gin.H{"error":...})` → `response.AbortUnauthorized(...)` |
| 1.3 | `internal/middleware/tenant.go` | ~15处 `c.AbortWithStatusJSON(gin.H{...})` → `response.AbortXxx(...)` |
| 1.4 | `internal/middleware/impersonation.go` | 7处同上 |
| 1.5 | `internal/middleware/access_denied.go` | `ErrorWithMetadata + c.Abort()` 改为一步 `AbortForbiddenWithDetails(...)` |
| 1.6 | `internal/middleware/auth.go` | 认证失败处添加 `logger.API("AUTH").Warn(...)` |

### Phase 2: Handler 规范统一

| 步骤 | 文件 | 改动 |
|------|------|------|
| 2.1 | `internal/handler/helpers.go` | 新增 `parsePagination(c) (page, pageSize int)` 统一解析 |
| 2.2 | 4个 handler 文件 | 所有 `page, _ := strconv.Atoi(...)` → `parsePagination(c)` |
| 2.3 | `secrets_handler.go` 等 | `ShouldBindJSON` 错误统一用 `FormatValidationError(err)` |
| 2.4 | 全局 handler | `InternalError(c, err.Error())` → 通用消息 + `logger` 记录原始错误 |

### Phase 3: 安全修复 + 服务层错误处理

| 步骤 | 文件 | 改动 |
|------|------|------|
| 3.1 | `internal/git/client.go` | 新增 `redactCredentials()`，应用到 Clone/Pull/Validate 的 stderr |
| 3.2 | `internal/service/git/service.go` | 路径校验加 `filepath.EvalSymlinks()` |
| 3.3 | `internal/service/git/service.go` | Update 加 error 检查，Delete rollback 加 error log |
| 3.4 | `internal/service/execution/service.go` | `GetRunByID` 失败时记录 warn 日志 |
| 3.5 | `internal/service/healing/scheduler.go` | `MarkScanned + UpdateLastRunAt` 包装在 `db.Transaction()` 中 |
| 3.6 | `secrets_handler.go` + `service/secrets/` | 定义 sentinel errors，用 `errors.Is()` 替换 `strings.Contains` |

### Phase 4: 并发安全 + 资源清理

| 步骤 | 文件 | 改动 |
|------|------|------|
| 4.1 | `internal/service/execution/service.go` | Service struct 加 `sem chan struct{}`，限流并发执行 |
| 4.2 | `internal/service/execution/service.go` + `cmd/server/main.go` | 新增 `Shutdown()` 方法，主函数 graceful shutdown 中调用 |
| 4.3 | `internal/service/git/service.go` | 裸 `go` 改用 `asyncLifecycle.Go()` 管理 |
| 4.4 | `internal/engine/provider/ansible/local_executor.go` | `buildArgs` 返回 cleanup 函数，调用者 `defer cleanup()`；检查 WriteString 错误 |
| 4.5 | `internal/engine/provider/ansible/docker_executor.go` | `stopContainer` 增加 `docker kill` fallback |

### Phase 5: 日志规范 + 代码卫生

| 步骤 | 文件 | 改动 |
|------|------|------|
| 5.1 | `internal/middleware/audit.go` | `zap.L()` → `logger.API("AUDIT")` |
| 5.2 | 全局 handler | 返回 500 的路径前加 `logger.API("模块").Error(...)` |
| 5.3 | `internal/service/execution/service.go` | cancel 多路径调用处加注释说明幂等设计 |

---

## C2. 验证方案

| 验证项 | 方法 |
|--------|------|
| 编译 | `go build ./...` 零错误 |
| 回归测试 | `go test ./...` 全通过 |
| 响应格式 | Table-driven test: 所有 middleware abort 响应 body 可 unmarshal 为 `response.Response{}` |
| 凭据脱敏 | 测试 `redactCredentials("https://ghp_xxx@github.com")` → `"https://***@github.com"` |
| 路径穿越 | 创建 symlink → 外部目录，断言返回 "非法路径" |
| 并发限流 | 测试 semaphore 满时返回错误而非无限排队 |
| 资源清理 | 测试 executor 执行后无残留 `/tmp/ansible-inventory-*.ini` |

---

## C3. 关键文件清单

| 文件 | 涉及问题 | 修改类型 |
|------|----------|----------|
| `internal/pkg/response/response.go` | R1-R3 | 新增 Abort 系列函数 |
| `internal/middleware/auth.go` | R1, LOG2 | 迁移到 response 包 + 加安全日志 |
| `internal/middleware/tenant.go` | R2 | 迁移到 response 包 |
| `internal/middleware/impersonation.go` | R3 | 迁移到 response 包 |
| `internal/middleware/access_denied.go` | R1 | 简化为一步 Abort 调用 |
| `internal/middleware/audit.go` | LOG1 | 日志统一 |
| `internal/handler/audit_handler.go` | V1, E4 | 安全分页 + 错误脱敏 |
| `internal/handler/platform_audit_handler.go` | V2 | 安全分页 |
| `internal/handler/execution_handler.go` | V3 | 安全分页 |
| `internal/handler/git_handler.go` | V4 | 安全分页 |
| `internal/handler/secrets_handler.go` | V5, V6, E5 | 验证格式化 + sentinel errors |
| `internal/handler/helpers.go` | PG1 | 新增 `parsePagination` |
| `internal/git/client.go` | S1, S2 | 凭据脱敏 |
| `internal/service/git/service.go` | S3, E1, E2, T3, C3 | 路径穿越 + 错误处理 + lifecycle |
| `internal/service/execution/service.go` | E3, C1, C2, C4, LOG4 | semaphore + shutdown + 错误处理 |
| `internal/service/healing/scheduler.go` | T1, T2 | 事务包装 |
| `internal/engine/provider/ansible/local_executor.go` | L1, L3 | 临时文件清理 |
| `internal/engine/provider/ansible/docker_executor.go` | L2 | 容器清理加固 |
| `cmd/server/main.go` | C2 | 调用 Shutdown |
