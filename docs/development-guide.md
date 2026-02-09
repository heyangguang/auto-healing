# Auto-Healing 开发指南

> 本文档定义了 Auto-Healing 项目的架构设计、目录规范和开发标准。
> 所有新代码必须遵循本指南，以确保项目的一致性和可维护性。

## 目录

1. [项目概述](#项目概述)
2. [目录结构](#目录结构)
3. [分层架构](#分层架构)
4. [独立功能模块](#独立功能模块)
5. [DTO 规范](#dto-规范)
6. [代码规范](#代码规范)
7. [数据库规范](#数据库规范)
8. [API 规范](#api-规范)
9. [测试规范](#测试规范)

---

## 项目概述

Auto-Healing 是一个企业级自动化运维平台，支持：
- ITSM/CMDB 集成（插件化）
- 自愈流程编排（可视化）
- 多执行器支持（Docker/Local）
- 密钥管理（File/Vault/Webhook）
- 通知推送（钉钉/邮件/Webhook）

---

## 目录结构

```
auto-healing/
├── cmd/                          # 应用入口
│   └── server/                   # 主服务
│       └── main.go
├── internal/                     # 内部代码（不对外暴露）
│   ├── model/                    # 数据模型
│   ├── repository/               # 数据访问层
│   ├── service/                  # 业务逻辑层
│   │   ├── execution/            # 执行服务
│   │   ├── git/                  # Git 仓库服务
│   │   ├── healing/              # 自愈流程服务
│   │   ├── plugin/               # 插件服务
│   │   └── secrets/              # 密钥服务
│   ├── handler/                  # HTTP 处理层
│   │   ├── *_handler.go          # 处理器
│   │   ├── *_dto.go              # 请求/响应 DTO
│   │   └── routes.go             # 路由定义
│   ├── adapter/                  # 外部系统适配器
│   │   ├── interface.go
│   │   ├── types/
│   │   └── provider/
│   ├── engine/                   # 执行引擎
│   │   ├── interface.go
│   │   └── provider/ansible/
│   ├── scheduler/                # 调度器
│   │   ├── interface.go
│   │   └── provider/
│   ├── secrets/                  # 密钥提供者
│   │   ├── interface.go
│   │   └── provider/
│   ├── notification/             # 通知模块
│   │   ├── service.go
│   │   └── provider/
│   ├── database/                 # 数据库连接
│   ├── pkg/                      # 内部公共包
│   │   ├── logger/
│   │   ├── response/
│   │   └── jwt/
│   └── gitclient/                # Git 客户端
├── migrations/                   # 数据库迁移
├── docs/                         # 文档
│   ├── openapi.yaml              # API 文档
│   └── api-testing-guide.md      # API 测试指南
├── tests/                        # 测试
│   └── e2e/                      # 端到端测试
├── tools/                        # 开发工具
│   ├── mock_*.py                 # Mock 服务
│   └── scripts/                  # 脚本
└── ansible/                      # Ansible 资源
    └── playbooks/                # Playbook 模板
```

---

## 分层架构

### 三层架构原则

```
┌─────────────────────────────────────────────────────────────┐
│                      Handler Layer                          │
│  - HTTP 请求处理                                             │
│  - 参数验证 & 绑定                                           │
│  - DTO 定义 & 转换                                           │
│  - 响应格式化                                                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                          │
│  - 业务逻辑                                                  │
│  - 事务管理                                                  │
│  - 跨模块协调                                                │
│  - 只接受 Model 或原始类型                                    │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Repository Layer                         │
│  - 数据库操作                                                │
│  - CRUD 封装                                                 │
│  - 查询构建                                                  │
└─────────────────────────────────────────────────────────────┘
```

### 层级职责

| 层级 | 目录 | 职责 | 依赖 |
|------|------|------|------|
| Handler | `handler/` | HTTP 处理、DTO 转换 | Service |
| Service | `service/` | 业务逻辑、事务 | Repository、Model |
| Repository | `repository/` | 数据访问 | Model、Database |
| Model | `model/` | 数据结构定义 | 无 |

### 调用规则

```
✅ Handler → Service → Repository
✅ Handler → DTO.ToModel() → Service
❌ Service → Handler（禁止反向依赖）
❌ Repository → Service（禁止反向依赖）
```

---

## 独立功能模块

独立功能模块是不依赖三层架构的自包含模块，采用统一结构：

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

### 当前模块

| 模块 | 路径 | 说明 |
|------|------|------|
| adapter | `internal/adapter/` | ITSM/CMDB 适配器 |
| engine | `internal/engine/` | 执行引擎 |
| scheduler | `internal/scheduler/` | 定时调度器 |
| secrets | `internal/secrets/` | 密钥提供者 |
| notification | `internal/notification/` | 通知推送 |

### interface.go 规范

```go
// interface.go 必须包含：
package module

// 1. 接口定义
type Provider interface {
    DoSomething(ctx context.Context) error
}

// 2. 类型别名（向后兼容）
type SomeType = provider.SomeType

// 3. 工厂函数
func NewProvider(config Config) (Provider, error) {
    switch config.Type {
    case "type_a":
        return provider.NewTypeA(config)
    default:
        return nil, ErrUnsupportedType
    }
}

// 4. 错误定义
var (
    ErrUnsupportedType = errors.New("不支持的类型")
)
```

### 避免循环导入

当 provider 需要使用父包定义的类型时：

```
❌ 错误做法：provider 导入 parent，parent 导入 provider（循环）

✅ 正确做法：创建 types/ 子包
   - types/ 只包含类型定义，不导入其他子包
   - interface.go 和 provider/ 都导入 types/
```

---

## DTO 规范

### 核心原则

> **DTO 只存在于 Handler 层，Service 层只接受 Model 或原始类型**

### DTO 文件命名

```
handler/
├── execution_dto.go      # Execution 模块 DTO
├── git_dto.go            # Git 模块 DTO
├── plugin_dto.go         # Plugin 模块 DTO
├── secrets_dto.go        # Secrets 模块 DTO
└── healing_dto.go        # Healing 模块 DTO
```

### DTO 结构规范

```go
// XXXRequest - 请求 DTO
type CreatePluginRequest struct {
    Name   string     `json:"name" binding:"required"`
    Type   string     `json:"type" binding:"required,oneof=itsm cmdb"`
    Config model.JSON `json:"config"`
}

// ToModel - 转换为 Model
func (r *CreatePluginRequest) ToModel() *model.Plugin {
    return &model.Plugin{
        Name:   r.Name,
        Type:   r.Type,
        Config: r.Config,
    }
}

// XXXResponse - 响应 DTO（可选，通常直接返回 Model）
type PluginResponse struct {
    ID        string    `json:"id"`
    Name      string    `json:"name"`
    CreatedAt time.Time `json:"created_at"`
}

// FromModel - 从 Model 转换
func PluginResponseFromModel(p *model.Plugin) *PluginResponse {
    return &PluginResponse{
        ID:        p.ID.String(),
        Name:      p.Name,
        CreatedAt: p.CreatedAt,
    }
}
```

### Handler 使用示例

```go
func (h *PluginHandler) Create(c *gin.Context) {
    // 1. 绑定并验证 DTO
    var req CreatePluginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        response.Error(c, http.StatusBadRequest, err.Error())
        return
    }
    
    // 2. 转换为 Model 并调用 Service
    plugin := req.ToModel()
    result, err := h.service.Create(c.Request.Context(), plugin)
    if err != nil {
        response.Error(c, http.StatusInternalServerError, err.Error())
        return
    }
    
    // 3. 返回响应
    response.Success(c, result)
}
```

---

## 代码规范

### 命名规范

| 类型 | 规范 | 示例 |
|------|------|------|
| 包名 | 小写单词 | `handler`, `service` |
| 文件名 | 小写下划线 | `plugin_handler.go` |
| 接口 | 动词/名词 | `Provider`, `Executor` |
| 结构体 | Pascal | `PluginService` |
| 方法 | Pascal | `CreatePlugin` |
| 常量 | Pascal 或全大写 | `MaxRetries`, `HTTP_TIMEOUT` |
| 私有 | 小写开头 | `parseConfig` |

### 错误处理

```go
// 1. 定义模块级错误
var (
    ErrNotFound      = errors.New("资源未找到")
    ErrUnauthorized  = errors.New("未授权")
)

// 2. 包装错误添加上下文
if err != nil {
    return fmt.Errorf("创建插件失败: %w", err)
}

// 3. Handler 层统一处理
if errors.Is(err, service.ErrNotFound) {
    response.Error(c, http.StatusNotFound, "资源不存在")
    return
}
```

### 日志规范

```go
import "github.com/company/auto-healing/internal/pkg/logger"

// 使用格式化日志
logger.Info("开始同步插件: %s (ID: %s)", plugin.Name, plugin.ID)
logger.Error("同步失败: %v", err)
logger.Debug("查询参数: %+v", params)
```

### 上下文传递

```go
// 所有涉及 I/O 的方法必须接受 context
func (s *Service) Create(ctx context.Context, data *model.Data) error {
    return s.repo.Create(ctx, data)
}
```

---

## 数据库规范

### 迁移文件

```
migrations/
├── 001_create_users.up.sql
├── 001_create_users.down.sql
├── 002_create_plugins.up.sql
└── 002_create_plugins.down.sql
```

### 迁移规则

1. **新字段**：添加迁移文件 + 更新 Model
2. **字段改为可空**：`ALTER COLUMN DROP NOT NULL` + 类型改为指针
3. **删除字段**：先从代码移除引用，再添加迁移

### 主键规范

- 所有表使用 UUID 作为主键
- 类型：`uuid.UUID`
- 数据库：`UUID PRIMARY KEY DEFAULT uuid_generate_v4()`

---

## API 规范

### URL 格式

```
GET    /api/v1/resources          # 列表
POST   /api/v1/resources          # 创建
GET    /api/v1/resources/:id      # 详情
PUT    /api/v1/resources/:id      # 更新
DELETE /api/v1/resources/:id      # 删除
POST   /api/v1/resources/:id/action  # 自定义操作
```

### 响应格式

```json
// 成功
{
    "code": 0,
    "message": "success",
    "data": { ... }
}

// 失败
{
    "code": 400,
    "message": "参数错误: name 不能为空",
    "data": null
}
```

### 文档同步

代码修改后必须同步更新：
1. `docs/openapi.yaml` - API 文档
2. `docs/api-testing-guide.md` - 测试指南

---

## 测试规范

### 测试目录

```
tests/
├── e2e/                          # 端到端测试
│   ├── test_complete_workflow_docker.sh
│   └── test_complete_workflow_local.sh
├── integration/                  # 集成测试
└── unit/                         # 单元测试
```

### E2E 测试要求

- 覆盖完整业务流程
- 使用 Mock 服务模拟外部依赖
- 验证 Docker 和 Local 两种执行模式
- 包含审批流程验证

### Mock 服务

```
tools/
├── mock_itsm_healing.py     # ITSM Mock
├── mock_secrets_healing.py  # Secrets Mock
└── mock_notification.py     # 通知 Mock
```

---

## 检查清单

### 新增 API

- [ ] Handler 方法
- [ ] DTO 定义（在 `*_dto.go`）
- [ ] Service 方法（接受 Model/原始类型）
- [ ] Repository 方法（如需要）
- [ ] 路由注册（`routes.go`）
- [ ] OpenAPI 文档更新
- [ ] 测试指南更新

### 新增模块

- [ ] 创建 `interface.go`
- [ ] 创建 `provider/` 目录
- [ ] 实现 Provider 接口
- [ ] 工厂函数
- [ ] 错误定义

### 数据库变更

- [ ] 迁移文件（up/down）
- [ ] Model 更新
- [ ] OpenAPI 更新

---

## 版本历史

| 版本 | 日期 | 作者 | 说明 |
|------|------|------|------|
| 1.0 | 2026-01-06 | Claude | 初始版本 |
