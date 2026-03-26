#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  scripts/review/setup_parallel_review.sh [session_name]

Optional env:
  REVIEW_ROOT        Override output root. Default: .parallel-review
  REVIEW_MODULES     Override module CSV. Default: scripts/review/backend_modules.csv

What it does:
  1. Creates a review session directory
  2. Generates review_status.csv and repair_plan.csv
  3. Generates one prompt per module
  4. Generates branch-aware create_worktrees.sh
EOF
}

repo_root() {
  git rev-parse --show-toplevel
}

session_name() {
  if [[ $# -gt 1 ]]; then
    usage
    exit 1
  fi
  if [[ $# -eq 1 ]]; then
    printf '%s\n' "$1"
    return
  fi
  date '+backend-review-%Y%m%d-%H%M%S'
}

review_root() {
  local root
  root="$(repo_root)"
  printf '%s/%s\n' "$root" "${REVIEW_ROOT:-.parallel-review}"
}

module_file() {
  local root
  root="$(repo_root)"
  printf '%s/%s\n' "$root" "${REVIEW_MODULES:-scripts/review/backend_modules.csv}"
}

branch_name() {
  local session="$1"
  local branch_suffix="$2"
  printf 'fix/%s/%s\n' "$session" "$branch_suffix"
}

worktree_dir() {
  local session="$1"
  local worktree_suffix="$2"
  local root repo_name parent
  root="$(repo_root)"
  repo_name="$(basename "$root")"
  parent="$(dirname "$root")"
  printf '%s/%s-%s-%s\n' "$parent" "$repo_name" "$session" "$worktree_suffix"
}

ensure_inputs() {
  local modules
  modules="$(module_file)"
  [[ -f "$modules" ]] || {
    printf 'module file not found: %s\n' "$modules" >&2
    exit 1
  }
}

write_status_csv() {
  local session_dir="$1"
  local session="$2"
  local modules="$3"
  {
    printf 'module_id,label,status,branch_name,worktree_dir,owner,findings_file,notes\n'
    tail -n +2 "$modules" | while IFS=, read -r module_id label worktree_suffix branch_suffix paths shared_touchpoints focus; do
      printf '%s,%s,TODO,%s,%s,,findings/%s.md,\n' \
        "$module_id" \
        "$label" \
        "$(branch_name "$session" "$branch_suffix")" \
        "$(worktree_dir "$session" "$worktree_suffix")" \
        "$module_id"
    done
  } >"$session_dir/review_status.csv"
}

write_repair_plan_csv() {
  local session_dir="$1"
  local session="$2"
  local modules="$3"
  {
    printf 'module_id,label,branch_name,worktree_dir,paths,shared_touchpoints,focus\n'
    tail -n +2 "$modules" | while IFS=, read -r module_id label worktree_suffix branch_suffix paths shared_touchpoints focus; do
      printf '%s,%s,%s,%s,%s,%s,%s\n' \
        "$module_id" \
        "$label" \
        "$(branch_name "$session" "$branch_suffix")" \
        "$(worktree_dir "$session" "$worktree_suffix")" \
        "$paths" \
        "$shared_touchpoints" \
        "$focus"
    done
  } >"$session_dir/repair_plan.csv"
}

write_prompt() {
  local prompt_file="$1"
  local session="$2"
  local module_id="$3"
  local label="$4"
  local worktree_suffix="$5"
  local branch_suffix="$6"
  local paths="$7"
  local shared_touchpoints="$8"
  local focus="$9"
  cat >"$prompt_file" <<EOF
# ${label}

- Session: \`${session}\`
- Module ID: \`${module_id}\`
- Branch: \`$(branch_name "$session" "$branch_suffix")\`
- Worktree: \`$(worktree_dir "$session" "$worktree_suffix")\`
- Findings output: \`.parallel-review/${session}/findings/${module_id}.md\`
- Status CSV row: \`.parallel-review/${session}/review_status.csv\`

## Audit Scope

\`${paths}\`

## Shared Touchpoints

\`${shared_touchpoints}\`

## Priority Focus

\`${focus}\`

## Audit Prompt

你只审计以下范围：
${paths}

共享触点：
${shared_touchpoints}

要求：
1. 先只做审计，不要改代码
2. 只看当前模块范围内的文件，不要越界
3. 优先找：安全、隔离、事务、一致性、并发、错误语义、静默降级、测试缺口
4. 每条 finding 必须带 file:line、触发条件、影响
5. findings 写入 \`.parallel-review/${session}/findings/${module_id}.md\`
6. 审计完成后，更新 \`.parallel-review/${session}/review_status.csv\` 中本模块的 status 和 notes
7. 如果当前模块未发现明确问题，明确写“当前模块未发现新的明确问题”
EOF
}

write_findings_stub() {
  local findings_file="$1"
  local label="$2"
  cat >"$findings_file" <<EOF
# ${label}

## Findings

- TODO
EOF
}

write_worktree_helper() {
  local helper_file="$1"
  local session="$2"
  local modules="$3"
  local root repo_name parent
  root="$(repo_root)"
  repo_name="$(basename "$root")"
  parent="$(dirname "$root")"

  {
    printf '#!/usr/bin/env bash\nset -euo pipefail\n\n'
    printf 'cd %q\n' "$root"
    printf 'base_ref="${REVIEW_BASE_BRANCH:-$(git branch --show-current)}"\n'
    printf '[[ -n "$base_ref" ]] || {\n'
    printf '  printf %q >&2\n' 'REVIEW_BASE_BRANCH is required when HEAD is detached\n'
    printf '  exit 1\n'
    printf '}\n'
    printf 'git rev-parse --verify "$base_ref" >/dev/null 2>&1 || {\n'
    printf '  printf %q "$base_ref" >&2\n' 'base ref not found: %s\n'
    printf '  exit 1\n'
    printf '}\n'
    printf 'parent=%q\n' "$parent"
    printf 'repo_name=%q\n\n' "$repo_name"
    tail -n +2 "$modules" | while IFS=, read -r module_id label worktree_suffix branch_suffix paths shared_touchpoints focus; do
      printf 'branch=%q\n' "$(branch_name "$session" "$branch_suffix")"
      printf 'dir=%q\n' "$(worktree_dir "$session" "$worktree_suffix")"
      printf 'if [[ -d "$dir" ]]; then\n'
      printf '  printf %q "$dir"\n' 'skip existing worktree: %s\n'
      printf 'elif git show-ref --verify --quiet "refs/heads/$branch"; then\n'
      printf '  git worktree add "$dir" "$branch"\n'
      printf 'else\n'
      printf '  git worktree add -b "$branch" "$dir" "$base_ref"\n'
      printf 'fi\n\n'
    done
  } >"$helper_file"

  chmod +x "$helper_file"
}

write_readme() {
  local session_dir="$1"
  local session="$2"
  cat >"$session_dir/README.md" <<EOF
# Parallel Review Session

- Session: \`${session}\`
- Review status: \`review_status.csv\`
- Repair plan: \`repair_plan.csv\`
- Prompt directory: \`prompts/\`
- Findings directory: \`findings/\`
- Worktree helper: \`create_worktrees.sh\`

## Recommended Flow

1. 先看 \`repair_plan.csv\`，确认模块边界、分支名和 worktree 目录
2. 运行 \`./create_worktrees.sh\` 为每个模块创建独立分支和 worktree
3. 手动开多个终端/SSH 会话，每个进程进入自己的 worktree
4. 在每个进程里打开 \`prompts/<module>.md\`，把内容贴给对应的 Codex 会话
5. findings 写入 \`findings/<module>.md\`
6. 总控只维护 \`review_status.csv\`
EOF
}

main() {
  ensure_inputs
  local session modules output_root session_dir
  session="$(session_name "$@")"
  modules="$(module_file)"
  output_root="$(review_root)"
  session_dir="${output_root}/${session}"

  mkdir -p "$session_dir/prompts" "$session_dir/findings"
  cp "$modules" "$session_dir/modules.csv"
  write_status_csv "$session_dir" "$session" "$modules"
  write_repair_plan_csv "$session_dir" "$session" "$modules"
  write_worktree_helper "$session_dir/create_worktrees.sh" "$session" "$modules"
  write_readme "$session_dir" "$session"

  tail -n +2 "$modules" | while IFS=, read -r module_id label worktree_suffix branch_suffix paths shared_touchpoints focus; do
    write_prompt \
      "$session_dir/prompts/${module_id}.md" \
      "$session" \
      "$module_id" \
      "$label" \
      "$worktree_suffix" \
      "$branch_suffix" \
      "$paths" \
      "$shared_touchpoints" \
      "$focus"
    write_findings_stub "$session_dir/findings/${module_id}.md" "$label"
  done

  printf 'Created review session: %s\n' "$session_dir"
  printf 'Next:\n'
  printf '  1. %s\n' "$session_dir/create_worktrees.sh"
  printf '  2. sed -n %q %s\n' '1,200p' "$session_dir/repair_plan.csv"
  printf '  3. 手动开多个终端，分别进入各模块 worktree，并把 prompts/*.md 贴给对应进程\n'
}

main "$@"
