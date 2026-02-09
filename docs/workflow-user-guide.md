# 自愈工作流使用手册

本手册面向运维人员和管理员，介绍如何使用自愈引擎创建和管理自动化工作流。

## 目录

1. [概述](#概述)
2. [快速开始](#快速开始)
3. [前置资源准备](#前置资源准备)
4. [流程设计](#流程设计)
5. [规则配置](#规则配置)
6. [最佳实践](#最佳实践)
7. [故障排查](#故障排查)

---

## 概述

### 什么是自愈引擎？

自愈引擎是一个自动化运维工具，能够：
- 监控 ITSM 系统中的工单
- 根据规则自动触发修复流程
- 执行 Ansible Playbook 进行故障修复
- 发送通知报告执行结果

### 核心概念

| 概念 | 说明 |
|------|------|
| **流程（Flow）** | DAG 工作流定义，包含节点和边 |
| **规则（Rule）** | 匹配条件，决定哪些工单触发哪个流程 |
| **实例（Instance）** | 流程的一次执行 |
| **节点（Node）** | 流程中的执行单元 |

### 工作原理

```
ITSM 工单 → 规则匹配 → 创建流程实例 → 执行节点 → 发送通知
                  ↑                              ↓
               (每10秒扫描)              (记录到数据库)
```

---

## 快速开始

### 5 分钟创建第一个自愈流程

**步骤 1: 登录获取 Token**
```bash
TOKEN=$(curl -s -X POST "http://localhost:8080/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "admin123456"}' | jq -r '.access_token')
```

**步骤 2: 创建简单流程**
```bash
curl -X POST "http://localhost:8080/api/v1/healing/flows" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "我的第一个自愈流程",
    "is_active": true,
    "nodes": [
      {"id": "start", "type": "start", "config": {}},
      {"id": "extract", "type": "host_extractor", "config": {"source_field": "raw_data.cmdb_ci"}},
      {"id": "validate", "type": "cmdb_validator", "config": {}},
      {"id": "end", "type": "end", "config": {}}
    ],
    "edges": [
      {"source": "start", "target": "extract"},
      {"source": "extract", "target": "validate"},
      {"source": "validate", "target": "end"}
    ]
  }'
```

**步骤 3: 创建匹配规则**
```bash
curl -X POST "http://localhost:8080/api/v1/healing/rules" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "测试规则",
    "trigger_mode": "auto",
    "conditions": [{"field": "title", "operator": "contains", "value": "测试"}],
    "flow_id": "YOUR_FLOW_UUID",
    "is_active": true
  }'
```

---

## 前置资源准备

创建完整的自愈流程需要以下资源：

### 1. ITSM 插件

对接工单系统，获取待处理工单：

```bash
curl -X POST "$API_BASE/api/v1/plugins" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ServiceNow ITSM",
    "type": "itsm",
    "adapter": "servicenow",
    "sync_enabled": true,
    "sync_interval_minutes": 5,
    "config": {
      "endpoint": "https://your-instance.service-now.com",
      "username": "api_user",
      "password": "api_password"
    }
  }'
```

### 2. CMDB 插件

同步资产信息用于主机验证：

```bash
curl -X POST "$API_BASE/api/v1/plugins" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "CMDB",
    "type": "cmdb",
    "adapter": "servicenow",
    "config": {
      "endpoint": "https://your-cmdb.example.com",
      "username": "api_user",
      "password": "api_password"
    }
  }'
```

### 3. 密钥源

管理 SSH 凭证用于连接目标主机：

```bash
curl -X POST "$API_BASE/api/v1/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "生产环境密钥",
    "type": "webhook",
    "config": {
      "url": "http://your-vault.example.com/api/v1/secrets/query"
    }
  }'
```

### 4. Git 仓库

存储 Ansible Playbook：

```bash
# 创建仓库
curl -X POST "$API_BASE/api/v1/git-repos" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "运维 Playbooks",
    "url": "https://github.com/your-org/ansible-playbooks.git",
    "branch": "main"
  }'

# 同步仓库
curl -X POST "$API_BASE/api/v1/git-repos/{id}/sync" \
  -H "Authorization: Bearer $TOKEN"

# 激活仓库（设置入口 Playbook）
curl -X POST "$API_BASE/api/v1/git-repos/{id}/activate" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"main_playbook": "site.yml"}'
```

### 5. 通知渠道

接收执行结果通知：

```bash
curl -X POST "$API_BASE/api/v1/channels" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "运维企业微信",
    "type": "webhook",
    "config": {
      "url": "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=xxx"
    }
  }'
```

---

## 流程设计

### 标准流程模板

**生产环境推荐流程**（含审批）：
```
start → host_extractor → cmdb_validator → approval → execution → notification → end
```

**测试/开发环境流程**（无审批）：
```
start → host_extractor → cmdb_validator → execution → notification → end
```

### 节点配置要点

#### host_extractor

从工单中提取目标主机：
- `source_field`: 工单中主机信息的字段路径
- 常用路径: `raw_data.cmdb_ci`, `title`, `description`

#### cmdb_validator

验证主机是否存在且状态正常：
- 会检查 CMDB 中的 `status` 字段
- 返回主机的真实 IP 地址

#### approval

关键操作前的人工审批：
- 流程会暂停直到审批完成
- 超时后根据配置处理

#### execution

执行 Ansible Playbook：
- **必须** 指定 `git_repo_id`（已激活的仓库）
- 建议配置 `secrets_source_id` 用于 SSH 认证
- `extra_vars` 传递给 Playbook 的变量

#### notification

发送执行结果通知：
- 支持 40+ 模板变量
- 支持 Markdown 格式

详细参数见 [节点参考文档](workflow-node-reference.md)。

---

## 规则配置

### 条件匹配

规则决定哪些工单会触发自愈流程。

**匹配字段**：

| 字段 | 说明 |
|------|------|
| `title` | 工单标题 |
| `description` | 工单描述 |
| `severity` | 严重程度 |
| `priority` | 优先级 |
| `category` | 工单分类 |
| `status` | 工单状态 |

**匹配运算符**：

| 运算符 | 说明 | 示例 |
|--------|------|------|
| `equals` | 完全匹配 | `severity equals critical` |
| `contains` | 包含 | `title contains nginx` |
| `in` | 在列表中 | `priority in ["P1", "P2"]` |
| `regex` | 正则匹配 | `title regex "Nginx.*down"` |

**匹配模式**：
- `all`: 所有条件都满足
- `any`: 任一条件满足

### 触发模式

| 模式 | 说明 |
|------|------|
| `auto` | 自动触发，工单匹配后立即执行 |
| `manual` | 手动触发，需要人工确认 |

---

## 最佳实践

### 1. 流程设计

- ✅ **生产环境始终加入审批节点**
- ✅ 使用 `set_variable` 在流程开始时设置关键变量
- ✅ 添加条件分支处理成功/失败场景
- ✅ 确保每条路径都有通知节点

### 2. Playbook 规范

- ✅ 使用仓库的 `main_playbook` 作为入口
- ✅ Playbook 支持幂等执行
- ✅ 设置合理的 `timeout_minutes`
- ✅ 使用 `extra_vars` 参数化 Playbook

### 3. 规则优化

- ✅ 设置规则优先级避免冲突
- ✅ 使用精确匹配减少误触发
- ✅ 测试规则匹配逻辑后再启用

### 4. 监控与告警

- ✅ 检查流程实例执行状态
- ✅ 关注 `failed` 状态的实例
- ✅ 定期清理历史实例数据

---

## 故障排查

### 常见问题

#### 1. 流程实例未创建

**症状**：ITSM 同步成功但没有流程实例

**排查步骤**：
```bash
# 1. 检查工单是否已被扫描
curl "$API_BASE/api/v1/incidents?page=1&page_size=5" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[] | {id, title, scanned}'

# 2. 检查规则是否激活
curl "$API_BASE/api/v1/healing/rules" \
  -H "Authorization: Bearer $TOKEN" | jq '.data[] | {id, name, is_active}'

# 3. 查看服务器日志
tail -f /root/auto-healing/server.log | grep -E "规则|调度|Healing"
```

#### 2. 执行节点失败

**常见原因**：
- Git 仓库未同步或未激活
- 密钥源配置错误
- Ansible Playbook 语法错误
- SSH 连接超时

**调试方法**：
```bash
# 查看执行节点详情
curl "$API_BASE/api/v1/healing/instances/{id}" \
  -H "Authorization: Bearer $TOKEN" | jq '.data.node_states.execution_1'
```

#### 3. 审批节点卡住

**解决方法**：
```bash
# 获取待审批任务
curl "$API_BASE/api/v1/healing/approvals/pending" \
  -H "Authorization: Bearer $TOKEN"

# 批准任务
curl -X POST "$API_BASE/api/v1/healing/approvals/{id}/approve" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"comment": "手动批准"}'
```

#### 4. 通知未发送

**检查项**：
- 通知渠道是否配置正确
- Webhook URL 是否可访问
- 模板语法是否正确

### 调试选项

执行节点支持 `keep_credentials: true` 选项，保留：
- `.ssh_keys/` - SSH 私钥
- `inventory.ini` - 主机清单
- `ansible.cfg` - 配置文件

```json
{
  "type": "execution",
  "config": {
    "keep_credentials": true
  }
}
```

---

## 附录

### API 参考

| 操作 | 端点 |
|------|------|
| 流程列表 | `GET /api/v1/healing/flows` |
| 创建流程 | `POST /api/v1/healing/flows` |
| 规则列表 | `GET /api/v1/healing/rules` |
| 创建规则 | `POST /api/v1/healing/rules` |
| 实例列表 | `GET /api/v1/healing/instances` |
| 审批列表 | `GET /api/v1/healing/approvals/pending` |

### 相关文档

- [节点参考文档](workflow-node-reference.md) - 所有节点类型详解
- [API 测试指南](api-testing-guide.md) - 完整 API 使用示例
- [OpenAPI 规范](openapi.yaml) - API 定义
