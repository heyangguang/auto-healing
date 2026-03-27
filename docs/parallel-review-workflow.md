# 并行模块修复工作流

这套脚手架现在按“手动多进程 + 分模块修复”设计，不再依赖 `tmux`。

核心模型：

- 主仓库：只看总表、做最终集成
- 每个模块：一个独立 `worktree`
- 每个模块：一个独立分支
- 每个模块：一个独立终端 / SSH 会话 / Codex 进程

这些模块是“业务/平台/基础设施/质量”分组，不包含脚手架本身。
脚手架本身只在工具分支 `chore/parallel-review-tooling`，也不属于 `quality_ops_surface` 的审查范围。

## 目录

- 模块定义：[scripts/review/backend_modules.csv](/root/auto-healing-tooling/scripts/review/backend_modules.csv)
- session 入口脚本：[scripts/review/setup_parallel_review.sh](/root/auto-healing-tooling/scripts/review/setup_parallel_review.sh)
- session 公共逻辑：[scripts/review/parallel_review_common.sh](/root/auto-healing-tooling/scripts/review/parallel_review_common.sh)
- session 写入逻辑：[scripts/review/parallel_review_writer.sh](/root/auto-healing-tooling/scripts/review/parallel_review_writer.sh)
- 生成模板与校验器：[scripts/review/create_worktrees.template.sh](/root/auto-healing-tooling/scripts/review/create_worktrees.template.sh)、[scripts/review/module_csv_validator.awk](/root/auto-healing-tooling/scripts/review/module_csv_validator.awk)、[scripts/review/module_prompt.template.md](/root/auto-healing-tooling/scripts/review/module_prompt.template.md)、[scripts/review/session_readme.template.md](/root/auto-healing-tooling/scripts/review/session_readme.template.md)
- 运行产物：`.parallel-review/<session>/`

`.parallel-review/` 和 `.codex-tasks/` 已加入 [.gitignore](/root/auto-healing-tooling/.gitignore)，不会把工具运行产物带进提交。

## 一次完整流程

### 1. 生成 session

```bash
cd /root/auto-healing-tooling
bash scripts/review/setup_parallel_review.sh dev-20260326
```

补充约束：

- 运行 `setup_parallel_review.sh` 需要 `bash`、`git`、`awk`、`python3` 或 `python`
- session 名只能包含 `A-Z a-z 0-9 . _ -`
- session 名不能以 `-` 开头
- 如果 session 目录已存在，脚本会直接报错，避免静默覆盖
- `REVIEW_ROOT` 和 `REVIEW_MODULES` 都支持仓库相对路径和绝对路径
- `backend_modules.csv` 采用简单 9 列 CSV；不支持带英文逗号的字段，也不支持 quoted CSV
- `backend_modules.csv` 里的 `worktree_suffix` 和 `branch_suffix` 必须全局唯一
- 可用 `bash scripts/review/setup_parallel_review.sh --help` 查看说明

会生成：

- `.parallel-review/<session>/modules.csv`
- `.parallel-review/<session>/module_csv_validator.awk`
- `.parallel-review/<session>/review_status.csv`
- `.parallel-review/<session>/repair_plan.csv`
- `.parallel-review/<session>/prompts/*.md`
- `.parallel-review/<session>/findings/*.md`
- `.parallel-review/<session>/create_worktrees.sh`

### 2. 创建独立 worktree 和分支

先看模块清单和备注：

```bash
bash .parallel-review/<session>/create_worktrees.sh --list
```

然后只创建你这轮要处理的模块：

```bash
bash .parallel-review/<session>/create_worktrees.sh auth_middleware tenant_user_role
```

生成 session 时，`create_worktrees.sh` 会把当时所在分支固化成默认基线。
如果你确实要全量创建，再不带模块参数执行。
如果 session 是从 detached HEAD 生成的，或者你要显式覆盖默认基线分支，再传 `REVIEW_BASE_BRANCH`：

如果命令里混入了不存在的 `module_id`，helper 会先直接报错，不会先创建一部分 worktree 再失败。

```bash
REVIEW_BASE_BRANCH=main bash .parallel-review/<session>/create_worktrees.sh
```

### 3. 看总分工表

```bash
sed -n '1,200p' .parallel-review/<session>/repair_plan.csv
```

这里会列出：

- 模块 ID
- 模块类型
- 模块备注
- 分支名
- worktree 路径
- 审计范围
- 共享触点
- 关注重点

### 4. 手动开多个进程

你自己开多个终端、标签页或 SSH 会话。每个进程只负责一个模块，并在这个模块分支里完成“先审再修”。

例如：

```bash
cd /root/auto-healing-dev-20260326-auth
git branch --show-current
sed -n '1,200p' /root/auto-healing-tooling/.parallel-review/dev-20260326/prompts/auth_middleware.md
codex
```

然后把刚才 prompt 文件内容贴给这个进程里的 Codex。
生成的 prompt 已经写入主仓库 session 目录下的绝对回写路径，所以模块 worktree 里不需要自己拼 `.parallel-review/...` 相对路径。

## Prompt 在哪里

所有模块 prompt 都在主仓库的：

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

- 当前模块先审再修
- 不越界改别的模块
- findings 要带 `file:line`、触发条件、影响
- 修完要跑最小必要验证
- 不碰 `main`

## 你在每个模块进程里做什么

1. 进入自己的 worktree
2. 打开自己的 prompt 文件
3. 启动该进程里的 Codex
4. 把 prompt 内容贴进去
5. 在当前模块分支里先审再修
6. 跑本模块最小必要验证
7. 按 prompt 里的绝对路径把 findings / fixes / validation 写回 `findings/<module>.md`
8. 按 prompt 里的绝对路径更新 `review_status.csv`
9. 提交并 push 当前模块分支

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

模块类型大致分为：

- `business-core`
- `platform-core`
- `infra-core`
- `quality-surface`

如果你要继续拆细，就改 [backend_modules.csv](/root/auto-healing-tooling/scripts/review/backend_modules.csv)，然后重新跑一次 `setup_parallel_review.sh`。

## 最实用的使用方式

- 主仓库终端：只看 `repair_plan.csv` 和 `review_status.csv`
- 模块终端：各自负责自己的模块分支，先审再修
- 公共文件：只指定一个模块负责

像共享装配文件和跨模块接口，不要让多个模块同时处理。

## 自动化测试

这套并行审查脚手架现在有 4 层自动化测试：

- 单元测试：`make test-review-tooling-unit`
- 集成测试：`make test-review-tooling-integration`
- 接口/契约测试：`make test-review-tooling-interface`
- 端到端测试：`make test-review-tooling-e2e`

如果要一次跑完这 4 层：

```bash
make test-review-tooling
```
