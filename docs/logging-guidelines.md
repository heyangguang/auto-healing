# 日志规范指南

## 概述

Auto-Healing 平台采用结构化日志系统，支持分类标签、任务追踪ID和多文件输出。

## 日志分类

| 分类 | 标签 | 说明 | 文件 |
|------|------|------|------|
| API | `[API]` | HTTP 请求日志 | `logs/api.log` |
| 调度 | `[SCHED:*]` | 调度器相关 | `logs/scheduler.log` |
| 执行 | `[EXEC:*]` | 流程执行 | `logs/execution.log` |
| 同步 | `[SYNC:*]` | 数据同步 | `logs/sync.log` |
| 认证 | `[AUTH:*]` | 认证/密钥 | `logs/auth.log` |

## 子类标签

### SCHED 调度器
- `[SCHED:HEAL]` - 自愈规则调度
- `[SCHED:SYNC]` - 插件同步调度
- `[SCHED:GIT]` - Git 仓库同步调度
- `[SCHED:TASK]` - 执行任务调度

### EXEC 执行器
- `[EXEC:FLOW]` - 流程实例
- `[EXEC:NODE]` - 节点执行
- `[EXEC:ANSIBLE]` - Playbook 执行

### SYNC 同步
- `[SYNC:PLUGIN]` - 插件数据同步
- `[SYNC:GIT]` - Git 仓库同步

### AUTH 认证
- `[AUTH:SECRET]` - 密钥查询

## 任务追踪 ID

并发任务添加 8 位短 ID 前缀以区分：

```
[EXEC:FLOW] [db64b071] 开始执行流程实例
[EXEC:NODE] [db64b071] 执行节点 start_1
[EXEC:FLOW] [45f12ae2] 开始执行流程实例     ← 另一个流程
[EXEC:NODE] [45f12ae2] 执行节点 start_1
```

过滤特定任务：
```bash
grep "\[db64b071\]" logs/execution.log
```

## 日志格式

### 控制台
```
时间戳           级别  标签           消息
2026-01-06T07:34:34.365+0800 INFO [SCHED:HEAL] 发现 3 个未扫描工单
```

### API 请求（带颜色）
```
2026-01-06T07:34:30.766+0800 INFO [API] 200 POST /api/v1/auth/login → 390ms | ::1
                                       ↑绿色  ↑黄色(4xx)  ↑红色(5xx)
```

## 代码使用

### 基础用法
```go
import "github.com/company/auto-healing/internal/pkg/logger"

// 分类日志
logger.Sched("HEAL").Info("发现 %d 个未扫描工单", count)
logger.Exec("FLOW").Error("流程执行失败: %v", err)
logger.Sync_("PLUGIN").Warn("同步异常: %s", msg)
logger.Auth("SECRET").Info("密钥查询成功")
logger.API("").Info("%d %s %s", statusCode, method, path)

// 带追踪 ID
logger.Exec("NODE").Info("[%s] 执行节点 %s", shortID(instance), node.ID)
```

### 日志级别
- `Debug` - 调试信息（条件判断、变量值）
- `Info` - 正常操作（开始、完成、统计）
- `Warn` - 警告（部分失败、降级）
- `Error` - 错误（操作失败、需人工处理）

## 最佳实践

1. **不用 emoji 写入日志文件** - 避免乱码
2. **错误消息清理换行符** - 保持单行
3. **截断过长消息** - 限制 200 字符
4. **并发任务加 shortID** - 便于追踪
5. **使用分类标签** - 便于过滤和分析
