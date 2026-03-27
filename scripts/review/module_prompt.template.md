# __LABEL__

- Session: `__SESSION__`
- Module ID: `__MODULE_ID__`
- Module Kind: `__MODULE_KIND__`
- Branch: `__BRANCH_NAME__`
- Worktree: `__WORKTREE_DIR__`
- Findings output: `__FINDINGS_OUTPUT__`
- Status CSV row: `__STATUS_CSV__`

## Module Note

`__MODULE_NOTE__`

## Module Scope

`__MODULE_SCOPE__`

## Shared Touchpoints

`__SHARED_TOUCHPOINTS__`

## Priority Focus

`__PRIORITY_FOCUS__`

## Work Prompt

你负责以下模块范围：
__MODULE_SCOPE__

共享触点：
__SHARED_TOUCHPOINTS__

要求：
1. 先在当前模块范围内做审查，确认问题后直接修复
2. 只改当前模块范围内的文件，不要越界；共享触点如必须修改，要在结果里说明
3. 优先找并修：安全、隔离、事务、一致性、并发、错误语义、静默降级、测试缺口
4. 每条 finding 必须带 file:line、触发条件、影响
5. 修复后运行与本模块相关的最小必要验证，并记录验证命令和结果
6. findings 写入 `__FINDINGS_OUTPUT__`
7. 处理完成后，更新 `__STATUS_CSV__` 中本模块的 status 和 notes
8. 不要切换 main，不要 merge，不要 rebase；只在当前模块分支内工作
9. 如果当前模块未发现明确问题，明确写“当前模块未发现新的明确问题”；如果已修复，明确列出修复项
