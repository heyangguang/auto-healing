# Auto-Healing System

运维自愈系统 - 基于 Go Gin 的企业级自动化运维平台

## 功能特性

- **🔌 插件模块**: ITSM/CMDB 多数据源插件化接入（ServiceNow、Jira、自定义）
- **🔄 自愈引擎**: 可视化工作流编排，支持主机提取、CMDB 验证、审批、执行、通知等节点
- **⚡ 执行模块**: Ansible 执行引擎，支持 Docker 和 Local 两种模式
- **🔐 密钥管理**: 多源密钥支持（File、HashiCorp Vault、Webhook）
- **📢 通知模块**: 多渠道通知（钉钉、邮件、Webhook），支持 40+ 模板变量
- **📝 日志系统**: 完善的审计日志和实时执行日志
- **🔒 权限控制**: RBAC 权限模型，支持细粒度控制

## 技术栈

| 组件 | 版本 |
|------|------|
| Go | 1.21+ |
| Gin | 1.9+ |
| PostgreSQL | 14+ |
| Ansible | 2.14+ |
| Docker | 24+ (可选) |

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 配置环境变量

```bash
cp configs/.env.example configs/.env
# 编辑 .env 配置数据库连接信息
```

### 3. 初始化数据库

```bash
# 执行所有迁移脚本
for f in migrations/*.up.sql; do
    psql -h localhost -U postgres -d auto_healing -f "$f"
done
```

### 4. 编译并启动

```bash
# 编译
go build -o bin/server ./cmd/server

# 启动
./bin/server
```

服务将在 `http://localhost:8080` 启动

## 项目结构

```
auto-healing/
├── cmd/server/              # 应用入口
├── internal/                # 核心代码
│   ├── model/               # 数据模型
│   ├── repository/          # 数据访问层
│   ├── service/             # 业务逻辑层
│   │   ├── execution/       # 执行服务
│   │   ├── git/             # Git 仓库服务
│   │   ├── healing/         # 自愈引擎
│   │   ├── plugin/          # 插件服务
│   │   └── secrets/         # 密钥服务
│   ├── handler/             # HTTP 处理层
│   ├── adapter/             # 外部系统适配器
│   │   ├── interface.go
│   │   ├── types/
│   │   └── provider/
│   ├── engine/              # 执行引擎
│   │   ├── interface.go
│   │   └── provider/ansible/
│   ├── scheduler/           # 调度器
│   │   ├── interface.go
│   │   └── provider/
│   ├── secrets/             # 密钥提供者
│   │   ├── interface.go
│   │   └── provider/
│   ├── notification/        # 通知模块
│   │   ├── service.go
│   │   └── provider/
│   ├── database/            # 数据库连接
│   ├── middleware/          # HTTP 中间件
│   └── pkg/                 # 内部公共包
├── migrations/              # 数据库迁移
├── docs/                    # 文档
│   ├── openapi.yaml         # API 文档
│   ├── development-guide.md # 开发指南
│   └── api-testing-guide.md # API 测试指南
├── tests/                   # 测试
│   └── e2e/                 # 端到端测试
├── tools/                   # 开发工具
│   └── mock_*.py            # Mock 服务
└── ansible/                 # Ansible 资源
    └── playbooks/
```

## 文档

| 文档 | 说明 |
|------|------|
| [开发指南](docs/development-guide.md) | 架构设计、目录规范、编码标准 |
| [API 文档](docs/openapi.yaml) | OpenAPI 3.0 规范 |
| [API 测试指南](docs/api-testing-guide.md) | cURL 示例和测试流程 |
| [Internal README](internal/README.md) | 内部目录结构说明 |

## 测试

### 端到端测试

```bash
# 启动 Mock 服务
cd tools
python3 mock_itsm_healing.py &
python3 mock_secrets_healing.py &
python3 mock_notification.py &

# 运行 E2E 测试
cd tests/e2e
./test_complete_workflow_docker.sh  # Docker 模式
./test_complete_workflow_local.sh   # Local 模式
```

## 架构特点

### 分层架构
```
Handler (DTO) → Service (Model) → Repository → Database
```

### 独立模块统一结构
```
module/
├── interface.go      # 接口 + 工厂函数
├── types/            # 共享类型（可选）
└── provider/         # 具体实现
```

### DTO 规范
- DTO 只存在于 Handler 层
- Service 只接受 Model 或原始类型
- 避免业务逻辑泄漏到传输层

## License

MIT
