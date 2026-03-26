# 并行修复工作流

这套脚手架现在按“并行修复 + 全源码审查”设计，不再建议多个窗口共用主工作区直接改代码。正确模型是：

- 主仓库：只做总控、汇总、最终集成
- 每个模块：一个独立 `worktree`
- 每个模块：一个独立分支
- 每个模块：只改自己的写入边界

## 目录

- 模块定义：[scripts/review/backend_modules.csv](/root/auto-healing/scripts/review/backend_modules.csv)
- session 生成脚本：[scripts/review/setup_parallel_review.sh](/root/auto-healing/scripts/review/setup_parallel_review.sh)
- tmux 启动脚本：[scripts/review/start_tmux_parallel_review.sh](/root/auto-healing/scripts/review/start_tmux_parallel_review.sh)
- 运行产物：`.parallel-review/<session>/`

`.parallel-review/` 已加入 [/.gitignore](/root/auto-healing/.gitignore)，不会污染代码仓库。

默认覆盖目标是“仓库里的源码和交付物”，包括：

- `internal/`
- `cmd/`
- `pkg/`
- `api/`
- `configs/`
- `migrations/`
- `tests/`
- `deployments/`
- `docker/`
- `tools/`
- `scripts/`
- `.github/workflows/`
- `docs/api/`

默认不纳入分支拆分的是运行时和元数据目录，例如：

- `.git/`
- `.parallel-review/`
- `.codex-tasks/`
- `logs/`
- `bin/`

## 一次完整流程

### 1. 生成 session

```bash
cd /root/auto-healing
bash scripts/review/setup_parallel_review.sh backend-review-20260326
```

会生成：

- `.parallel-review/<session>/modules.csv`
- `.parallel-review/<session>/review_status.csv`
- `.parallel-review/<session>/repair_plan.csv`
- `.parallel-review/<session>/prompts/*.md`
- `.parallel-review/<session>/findings/*.md`
- `.parallel-review/<session>/create_worktrees.sh`

### 2. 先看 repair plan

先打开：

```bash
sed -n '1,200p' .parallel-review/<session>/repair_plan.csv
```

这个文件已经把下面这些信息列好了：

- 模块 ID
- 分支名
- worktree 路径
- 允许修改的路径范围
- 共享触点
- 重点关注点

默认分支命名规则是：

```text
fix/<session>/<module-branch-suffix>
```

例如：

- `fix/backend-review-20260326/auth-middleware`
- `fix/backend-review-20260326/tenant-user-role`

### 3. 创建独立 worktree

```bash
bash .parallel-review/<session>/create_worktrees.sh
```

脚本会按模块创建：

- 一个 worktree 目录
- 一个独立分支

例如：

- `/root/auto-healing-backend-review-20260326-auth`
- `/root/auto-healing-backend-review-20260326-tenant`
- `/root/auto-healing-backend-review-20260326-healing`

如果你当前是 detached HEAD，需要显式指定基线分支：

```bash
REVIEW_BASE_BRANCH=main bash .parallel-review/<session>/create_worktrees.sh
```

### 4. 启动 tmux

```bash
bash scripts/review/start_tmux_parallel_review.sh <session>
```

它会默认直接进入 `tmux`，并创建：

- `control` 窗口：看 `review_status.csv` 和 `repair_plan.csv`
- 每个模块一个窗口：优先进入对应 worktree

如果只想后台创建 session：

```bash
bash scripts/review/start_tmux_parallel_review.sh <session> --detach
```

## 你在每个模块窗口里做什么

### 模块窗口只做一件事

只处理自己模块的审查和修复，不越界。

先看模块 prompt：

```bash
sed -n '1,200p' .parallel-review/<session>/prompts/<module>.md
```

然后：

1. 先按模块范围审查
2. 把 findings 写到 `findings/<module>.md`
3. 在自己的 worktree 里修复
4. 更新 `review_status.csv`

## 为什么要这么做

共享主工作区多窗口同时改代码，会有两类问题：

- 工作区冲突：同一目录被多个窗口同时改
- 集成冲突：不同分支最后改到同一文件或同一接口

`worktree + 分支` 只能解决第一类，第二类仍然要靠边界拆分和总控集成。

所以模块 CSV 里除了写入范围，还显式列了 `shared_touchpoints`。这些文件默认不要随便改，避免把冲突从工作区层面搬到 merge 层面。

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

模块定义都在 [backend_modules.csv](/root/auto-healing/scripts/review/backend_modules.csv)，你要继续拆细，就直接改这个文件，然后重新跑一次 `setup_parallel_review.sh`。

## 最实用的使用方式

- 主仓库窗口：只看状态表、做最终集成
- 模块 worktree 窗口：各自审查和修复
- 公共文件：只指定一个模块改

像 [routes.go](/root/auto-healing/internal/handler/routes.go) 这种共享装配文件，不要多个模块同时碰。
