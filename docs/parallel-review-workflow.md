# 并行审计工作流

这套脚手架现在按“手动多进程 + 分模块审计”设计，不再依赖 `tmux`。

核心模型：

- 主仓库：只看总表、做最终集成
- 每个模块：一个独立 `worktree`
- 每个模块：一个独立分支
- 每个模块：一个独立终端 / SSH 会话 / Codex 进程

## 目录

- 模块定义：[scripts/review/backend_modules.csv](/root/auto-healing-tooling/scripts/review/backend_modules.csv)
- session 生成脚本：[scripts/review/setup_parallel_review.sh](/root/auto-healing-tooling/scripts/review/setup_parallel_review.sh)
- 运行产物：`.parallel-review/<session>/`

`.parallel-review/` 已加入 [.gitignore](/root/auto-healing-tooling/.gitignore)，不会污染仓库。

## 一次完整流程

### 1. 生成 session

```bash
cd /root/auto-healing-tooling
bash scripts/review/setup_parallel_review.sh dev-20260326
```

会生成：

- `.parallel-review/<session>/modules.csv`
- `.parallel-review/<session>/review_status.csv`
- `.parallel-review/<session>/repair_plan.csv`
- `.parallel-review/<session>/prompts/*.md`
- `.parallel-review/<session>/findings/*.md`
- `.parallel-review/<session>/create_worktrees.sh`

### 2. 创建独立 worktree

```bash
bash .parallel-review/<session>/create_worktrees.sh
```

如果你当前是 detached HEAD，需要显式指定基线分支：

```bash
REVIEW_BASE_BRANCH=main bash .parallel-review/<session>/create_worktrees.sh
```

### 3. 看总分工表

```bash
sed -n '1,200p' .parallel-review/<session>/repair_plan.csv
```

这里会列出：

- 模块 ID
- 分支名
- worktree 路径
- 审计范围
- 共享触点
- 关注重点

### 4. 手动开多个进程

你自己开多个终端、标签页或 SSH 会话。每个进程只负责一个模块。

例如：

```bash
cd /root/auto-healing-dev-20260326-auth
git branch --show-current
sed -n '1,200p' /root/auto-healing-tooling/.parallel-review/dev-20260326/prompts/auth_middleware.md
codex
```

然后把刚才 prompt 文件内容贴给这个进程里的 Codex。

## Prompt 在哪里

所有模块 prompt 都在：

```bash
.parallel-review/<session>/prompts/
```

例如：

```bash
sed -n '1,200p' .parallel-review/dev-20260326/prompts/auth_middleware.md
sed -n '1,200p' .parallel-review/dev-20260326/prompts/tenant_user_role.md
sed -n '1,200p' .parallel-review/dev-20260326/prompts/execution_healing.md
```

这些 prompt 默认是：

- 先只做审计
- 不改代码
- 只输出 findings
- 带 file:line、触发条件、影响

## 你在每个模块进程里做什么

1. 进入自己的 worktree
2. 打开自己的 prompt 文件
3. 启动该进程里的 Codex
4. 把 prompt 内容贴进去
5. 审计结果写回 `findings/<module>.md`
6. 更新 `review_status.csv`

## 当前模块拆分

当前源码被拆成 11 组：

- `auth_middleware`
- `tenant_user_role`
- `execution_healing`
- `plugin_git_cmdb`
- `notification_dashboard`
- `http_platform_core`
- `platform_audit_ops`
- `secrets_logs`
- `model_engine_scheduler`
- `config_database_pkg`
- `quality_ops_surface`

如果你要继续拆细，就改 [backend_modules.csv](/root/auto-healing-tooling/scripts/review/backend_modules.csv)，然后重新跑一次 `setup_parallel_review.sh`。

## 最实用的使用方式

- 主仓库终端：只看 `repair_plan.csv` 和 `review_status.csv`
- 模块终端：各自审计各自范围
- 公共文件：只指定一个模块负责

像共享装配文件和跨模块接口，不要让多个模块同时处理。
