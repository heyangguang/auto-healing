write_status_csv() {
	local session_dir="$1"
	local session="$2"
	local modules="$3"
	{
		printf 'module_id,label,module_kind,module_note,status,branch_name,worktree_dir,owner,findings_file,notes\n'
		tail -n +2 "$modules" | while IFS=, read -r module_id label module_kind module_note worktree_suffix branch_suffix paths shared_touchpoints focus; do
			printf '%s,%s,%s,%s,TODO,%s,%s,,findings/%s.md,\n' \
				"$module_id" \
				"$label" \
				"$module_kind" \
				"$module_note" \
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
		printf 'module_id,label,module_kind,module_note,branch_name,worktree_dir,paths,shared_touchpoints,focus\n'
		tail -n +2 "$modules" | while IFS=, read -r module_id label module_kind module_note worktree_suffix branch_suffix paths shared_touchpoints focus; do
			printf '%s,%s,%s,%s,%s,%s,%s,%s,%s\n' \
				"$module_id" \
				"$label" \
				"$module_kind" \
				"$module_note" \
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
	local session_context_name="$2"
	local module_context_name="$3"
	local -A template_context=()
	declare -n session_ctx_ref="$session_context_name"
	declare -n module_ctx_ref="$module_context_name"

	template_context=(
		['__SESSION__']="${session_ctx_ref[session]}"
		['__MODULE_ID__']="${module_ctx_ref[module_id]}"
		['__MODULE_KIND__']="${module_ctx_ref[module_kind]}"
		['__BRANCH_NAME__']="$(branch_name "${session_ctx_ref[session]}" "${module_ctx_ref[branch_suffix]}")"
		['__WORKTREE_DIR__']="$(worktree_dir "${session_ctx_ref[session]}" "${module_ctx_ref[worktree_suffix]}")"
		['__FINDINGS_OUTPUT__']="${session_ctx_ref[session_dir]}/findings/${module_ctx_ref[module_id]}.md"
		['__STATUS_CSV__']="${session_ctx_ref[session_dir]}/review_status.csv"
		['__MODULE_NOTE__']="${module_ctx_ref[module_note]}"
		['__MODULE_SCOPE__']="${module_ctx_ref[paths]}"
		['__SHARED_TOUCHPOINTS__']="${module_ctx_ref[shared_touchpoints]}"
		['__PRIORITY_FOCUS__']="${module_ctx_ref[focus]}"
		['__LABEL__']="${module_ctx_ref[label]}"
	)

	render_template_with_context "$(template_file module_prompt.template.md)" "$prompt_file" template_context
}

write_findings_stub() {
	local findings_file="$1"
	local label="$2"
	local module_note="$3"
	cat >"$findings_file" <<EOF
# ${label}

> ${module_note}

## Findings

- TODO

## Fixes

- TODO

## Validation

- TODO
EOF
}

write_worktree_helper() {
	local helper_file="$1"
	local session="$2"
	local default_base_ref="$3"
	local root repo_name parent detached_note
	local -A template_context=()
	root="$(repo_root)"
	repo_name="$(basename "$root")"
	parent="$(dirname "$root")"
	detached_note='false'
	if [[ -z "$default_base_ref" ]]; then
		detached_note='true'
	fi

	template_context=(
		['__REPO_ROOT__']="$(shell_quote "$root")"
		['__SESSION_NAME__']="$(shell_quote "$session")"
		['__PARENT_DIR__']="$(shell_quote "$parent")"
		['__REPO_NAME__']="$(shell_quote "$repo_name")"
		['__SESSION_BASE_REF__']="$(shell_quote "$default_base_ref")"
		['__SESSION_FROM_DETACHED_HEAD__']="$(shell_quote "$detached_note")"
	)

	render_template_with_context "$(template_file create_worktrees.template.sh)" "$helper_file" template_context
	chmod +x "$helper_file"
}

write_module_csv_validator() {
	local session_dir="$1"
	cp "$(template_file module_csv_validator.awk)" "$session_dir/module_csv_validator.awk"
}

write_readme() {
	local session_dir="$1"
	local session="$2"
	local default_base_ref="$3"
	local default_base_ref_text="$default_base_ref"
	local -A template_context=()
	if [[ -z "$default_base_ref_text" ]]; then
		default_base_ref_text='detached HEAD at session generation time; set REVIEW_BASE_BRANCH when creating worktrees'
	fi
	template_context=(
		['__SESSION__']="$session"
		['__SESSION_DIR__']="$session_dir"
		['__DEFAULT_BASE_REF__']="$default_base_ref_text"
	)
	render_template_with_context "$(template_file session_readme.template.md)" "$session_dir/README.md" template_context
}

populate_module_outputs() {
	local session_dir="$1"
	local session="$2"
	local modules="$3"
	local -A session_context=(
		[session]="$session"
		[session_dir]="$session_dir"
	)
	tail -n +2 "$modules" | while IFS=, read -r module_id label module_kind module_note worktree_suffix branch_suffix paths shared_touchpoints focus; do
		local -A module_context=(
			[module_id]="$module_id"
			[label]="$label"
			[module_kind]="$module_kind"
			[module_note]="$module_note"
			[worktree_suffix]="$worktree_suffix"
			[branch_suffix]="$branch_suffix"
			[paths]="$paths"
			[shared_touchpoints]="$shared_touchpoints"
			[focus]="$focus"
		)
		write_prompt \
			"$session_dir/prompts/${module_id}.md" \
			session_context \
			module_context
		write_findings_stub "$session_dir/findings/${module_id}.md" "$label" "$module_note"
	done
}

print_next_steps() {
	local session_dir="$1"
	printf 'Created review session: %s\n' "$session_dir"
	printf 'Next:\n'
	printf '  1. sed -n %q %s\n' '1,200p' "$session_dir/repair_plan.csv"
	printf '  2. %s --list\n' "$session_dir/create_worktrees.sh"
	printf '  3. %s auth_middleware tenant_user_role\n' "$session_dir/create_worktrees.sh"
	printf '  4. 手动开多个终端，分别进入各模块 worktree，并把 prompts/*.md 贴给对应进程\n'
	printf '  5. prompt 已写入绝对回写路径；每个模块在自己的分支里先审再修，跑最小必要验证，再提交当前分支\n'
}
