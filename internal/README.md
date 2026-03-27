# Internal 目录结构说明

此目录包含 Auto-Healing 平台的核心后端代码。

> 完整开发规范请参考 [开发指南](../docs/development-guide.md)

## 当前组织方式

当前后端不再按顶层 `handler / service / repository` 三层目录组织业务代码，而是按业务域拆到 `internal/modules/*`，共享运行时能力收口到 `internal/platform/*`，统一 HTTP 装配收口到 `internal/app/httpapi`。

```
internal/
├── app/httpapi/                 # 统一路由装配与模块注册入口
├── modules/                     # 按业务域拆分的核心模块
│   ├── access/
│   ├── automation/
│   ├── engagement/
│   ├── integrations/
│   ├── ops/
│   └── secrets/
├── platform/                    # 跨模块共享的 HTTP / 生命周期 / 事件 / 仓储辅助
├── middleware/                  # HTTP 中间件
├── model/                       # GORM 模型
├── notification/                # 通知引擎与 provider
├── engine/                      # 执行引擎
├── scheduler/                   # 后台调度器
├── secrets/                     # 密钥 provider
├── git/                         # Git 基础能力
├── config/                      # 配置加载
├── database/                    # DB / Redis 初始化与迁移
└── pkg/                         # 通用工具
```

## 模块结构

每个业务域模块统一放在 `internal/modules/<domain>/` 下，内部再按职责拆分：

```
internal/modules/<domain>/
├── module.go                    # 模块构造与依赖装配
├── httpapi/                     # HTTP handler、DTO、路由注册器
├── service/                     # 领域服务 / use case
└── repository/                  # 该业务域自己的仓储实现
```

当前业务域模块：

| 模块 | 职责 |
|------|------|
| `access` | 认证、租户、用户、角色、权限、Impersonation |
| `automation` | 执行、自愈、调度 |
| `engagement` | Dashboard、Workbench、Search、Notification、Site Message、Preference |
| `integrations` | Plugin、Git、CMDB、Playbook |
| `ops` | Audit、Dictionary、Blacklist、Platform Settings |
| `secrets` | 密钥源管理与查询 |

## 共享运行时层

跨模块共享的运行时能力不再放在业务仓储目录里，而是放到 `internal/platform/*`：

| 目录 | 职责 |
|------|------|
| `platform/httpx/` | 分页、校验、SSE、资源错误回包、查询过滤等 HTTP 通用能力 |
| `platform/lifecycle/` | 生命周期与 goroutine cleanup |
| `platform/events/` | 站内信事件总线等共享事件能力 |
| `platform/repository/` | 跨模块共享的仓储实现（如 `audit / cmdb / incident / settings`） |
| `platform/repositoryx/` | 共享 tenant context / scoped DB helper |

## 统一路由入口

统一 HTTP 入口在：

```
internal/app/httpapi/router.go
internal/app/httpapi/modules.go
```

这里负责：
- 创建各业务域模块
- 组装模块级 route registrar
- 将 `/common`、`/platform`、`/tenant` 路由绑定到对应模块

新增 API 时，不再修改旧的 `internal/handler/routes.go`，而是：
1. 在对应业务域的 `httpapi/` 中新增 handler 或 registrar
2. 在对应模块的 `module.go` 中装配依赖
3. 在 `internal/app/httpapi/modules.go` 中接入模块依赖

## 独立 provider / runtime 模块

这类目录仍保持自己的 provider 结构：

| 模块 | 路径 | 结构 |
|------|------|------|
| `engine` | `internal/modules/automation/engine/` | `interface` + `provider/ansible` |
| `scheduler` | `internal/scheduler/` | 调度器与 provider |
| `secrets` | `internal/secrets/` | provider 实现 |
| `notification` | `internal/modules/engagement/service/notification/` | 通知服务与 provider |

## 基础设施目录

| 目录 | 职责 |
|------|------|
| `config/` | 配置加载 |
| `database/` | 数据库、Redis、迁移、种子初始化 |
| `middleware/` | 通用 HTTP 中间件 |
| `model/` | 数据模型 |
| `pkg/` | 内部通用工具 |

## 快速参考

### 新增业务接口

```
1. 如需新增模型，先更新 internal/model/<entity>.go
2. 在目标业务域下新增或扩展 internal/modules/<domain>/repository/
3. 在目标业务域下新增或扩展 internal/modules/<domain>/service/
4. 在目标业务域下新增或扩展 internal/modules/<domain>/httpapi/
5. 在 internal/modules/<domain>/module.go 中装配依赖
6. 在 internal/app/httpapi/modules.go 中接入模块 registrar
7. 更新 docs/openapi.yaml
8. 如涉及存储结构变化，补 migrations/*.sql
```

### 新增 provider

```
1. 在对应独立模块下新增 provider 实现，例如 internal/<module>/provider/<impl>.go
2. 实现接口并在工厂或注册表中接入
3. 补齐对应测试
```

### 同步更新检查清单

- [ ] 模型字段变更 → `migrations/*.sql` + `docs/openapi.yaml`
- [ ] 新增 API → 对应 `internal/modules/<domain>/httpapi/` + `internal/app/httpapi/modules.go` + `docs/openapi.yaml`
- [ ] 新增共享 HTTP/运行时能力 → `internal/platform/*`
- [ ] 新增业务模块代码 → `internal/modules/<domain>/...`
