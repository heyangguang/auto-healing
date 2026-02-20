# 全局搜索 API 文档

**路径**: `/api/v1/search`  
**权限**: 已登录用户

---

## 全局搜索

**GET** `/api/v1/search`

跨模块全局搜索，同时搜索多种资源类型。

### 查询参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `q` | string | ✅ | 搜索关键词 |
| `limit` | int | ❌ | 每种类型返回数量，默认 5 |

### 支持的资源类型

| 类型 | 说明 |
|------|------|
| `playbook` | Playbook 模板 |
| `task` | 执行任务模板 |
| `healing_flow` | 自愈流程 |
| `healing_rule` | 自愈规则 |
| `cmdb` | CMDB 配置项 |
| `incident` | 工单 |
| `git_repo` | Git 仓库 |
| `plugin` | 插件 |

### 响应

```json
{
  "code": 0,
  "data": {
    "results": [
      {
        "type": "playbook",
        "id": "uuid",
        "name": "磁盘清理",
        "description": "清理 /tmp 目录",
        "link": "/playbooks/uuid"
      },
      {
        "type": "task",
        "id": "uuid",
        "name": "磁盘清理任务",
        "description": "每日磁盘清理",
        "link": "/execution-tasks/uuid"
      }
    ],
    "total": 2
  }
}
```
