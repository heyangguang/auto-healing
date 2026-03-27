# Parallel Module Session

- Session: `__SESSION__`
- Session directory: `__SESSION_DIR__`
- Review status: `review_status.csv`
- Repair plan: `repair_plan.csv`
- Module CSV validator: `module_csv_validator.awk`
- Prompt directory: `prompts/`
- Findings directory: `findings/`
- Worktree helper: `create_worktrees.sh`
- Default worktree base ref: `__DEFAULT_BASE_REF__`

## Recommended Flow

1. 先看 `repair_plan.csv`，确认模块边界、分支名和 worktree 目录
2. 运行 `./create_worktrees.sh --list` 查看模块清单和备注
3. 按需运行 `./create_worktrees.sh auth_middleware tenant_user_role` 创建本轮要处理的模块
4. 手动开多个终端/SSH 会话，每个进程进入自己的 worktree
5. 在每个进程里打开 `prompts/<module>.md`，按 prompt 里的绝对路径回写 findings 和 review_status
6. 每个模块进程在自己的分支里完成“先审再修 + 最小验证”
7. findings 写入 `findings/<module>.md`
8. 总控维护 `review_status.csv`
