# Internal 目录结构说明

此目录包含 Auto-Healing 平台的核心代码，按照 Go 项目标准实践组织。

> 📘 完整开发规范请参考 [开发指南](../docs/development-guide.md)

## 分层架构

```
┌─────────────────────────────────────────────────────────────┐
│                      Handler Layer                          │
│  - HTTP 请求处理、DTO 定义与转换                              │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                          │
│  - 业务逻辑（只接受 Model 或原始类型）                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Repository Layer                         │
│  - 数据库 CRUD 操作                                          │
└─────────────────────────────────────────────────────────────┘
```

### 分层目录

| 目录 | 职责 | 文件命名规范 |
|------|------|-------------|
| `model/` | 数据模型（GORM struct） | `<entity>.go` |
| `repository/` | 数据库 CRUD 操作 | `<entity>.go` |
| `service/<module>/` | 业务逻辑处理 | `service.go` |
| `handler/` | HTTP API 路由和请求处理 | `<module>_handler.go`, `<module>_dto.go` |

## Handler 层规范

```
handler/
├── routes.go              # 统一路由注册
├── <module>_handler.go    # HTTP 处理函数
└── <module>_dto.go        # 请求/响应 DTO
```

**DTO 规则**：
- DTO 只存在于 Handler 层
- Service 只接受 Model 或原始类型
- DTO 必须提供 `ToModel()` 方法

---

## 独立功能模块

独立模块采用统一的 `interface.go` + `provider/` 结构：

| 模块 | 路径 | 结构 |
|------|------|------|
| **adapter** | `adapter/` | `interface.go` + `types/` + `provider/` |
| **engine** | `engine/` | `interface.go` + `provider/ansible/` |
| **scheduler** | `scheduler/` | `interface.go` + `provider/` |
| **secrets** | `secrets/` | `interface.go` + `provider/` |
| **notification** | `notification/` | `service.go` + `provider/` |

### 模块结构规范

```
module/
├── interface.go          # 接口定义、工厂函数、类型别名
├── types/                # 共享类型（可选，用于避免循环导入）
│   └── types.go
└── provider/             # 具体实现
    ├── impl_a.go
    └── impl_b.go
```

### 当前模块详情

#### adapter/（外部系统适配器）
```
adapter/
├── interface.go          # PluginAdapter 接口、NewAdapter 工厂
├── types/
│   └── types.go          # RawIncident, RawCMDBItem, FieldMapping
└── provider/
    ├── servicenow.go     # ServiceNow 适配器
    └── other.go          # Jira, Custom 适配器
```

#### engine/（执行引擎）
```
engine/
├── interface.go          # Executor 接口、NewExecutor 工厂
└── provider/
    └── ansible/          # Ansible 执行器
        ├── executor.go
        ├── docker_executor.go
        └── local_executor.go
```

#### scheduler/（调度器）
```
scheduler/
├── interface.go          # Manager、NewScheduler 工厂
└── provider/
    ├── plugin.go         # 插件同步调度
    ├── execution.go      # 执行调度
    └── git.go            # Git 同步调度
```

#### secrets/（密钥管理）
```
secrets/
├── interface.go          # Provider 接口、NewProvider 工厂
└── provider/
    ├── errors.go         # 共享错误定义
    ├── file.go           # 文件密钥
    ├── vault.go          # HashiCorp Vault
    └── webhook.go        # Webhook 密钥
```

#### notification/（通知推送）
```
notification/
├── service.go            # 主服务
├── template.go           # 模板解析
├── variable.go           # 变量构建
└── provider/
    ├── interface.go      # NotificationProvider 接口
    ├── dingtalk.go       # 钉钉
    ├── email.go          # 邮件
    └── webhook.go        # Webhook
```

---

## 基础设施目录

| 目录 | 职责 |
|------|------|
| `config/` | 配置文件加载 |
| `database/` | 数据库连接管理 |
| `middleware/` | HTTP 中间件（JWT、CORS、日志） |
| `pkg/` | 内部公共工具（logger、response、jwt） |
| `gitclient/` | Git 客户端封装 |

---

## 快速参考

### 新增业务模块

```
1. internal/model/<entity>.go
2. internal/repository/<entity>.go
3. internal/service/<module>/service.go
4. internal/handler/<module>_handler.go
5. internal/handler/<module>_dto.go
6. 在 routes.go 中注册路由
7. 更新 docs/openapi.yaml
```

### 新增独立模块 Provider

```
1. internal/<module>/provider/<impl>.go
2. 实现 Provider 接口
3. 在 interface.go 工厂函数中注册
```

### 同步更新检查清单

- [ ] 模型字段变更 → `migrations/*.sql` + `docs/openapi.yaml`
- [ ] 新增 API → `routes.go` + `docs/openapi.yaml`
- [ ] 新增模块 → 按上述规范创建文件
