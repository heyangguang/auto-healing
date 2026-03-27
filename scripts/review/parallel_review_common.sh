EXPECTED_MODULE_HEADER='module_id,label,module_kind,module_note,worktree_suffix,branch_suffix,paths,shared_touchpoints,focus'
PARALLEL_REVIEW_PYTHON_BIN=''

repo_root() {
	git rev-parse --show-toplevel
}

current_branch() {
	git branch --show-current
}

resolve_path() {
	local path="$1"
	if [[ "$path" = /* ]]; then
		printf '%s\n' "$path"
		return
	fi
	printf '%s/%s\n' "$(repo_root)" "$path"
}

template_file() {
	printf '%s/%s\n' "$PARALLEL_REVIEW_SCRIPT_DIR" "$1"
}

review_root() {
	resolve_path "${REVIEW_ROOT:-.parallel-review}"
}

module_file() {
	resolve_path "${REVIEW_MODULES:-scripts/review/backend_modules.csv}"
}

shell_quote() {
	printf '%q' "$1"
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

validate_session_name() {
	local session="$1"
	[[ -n "$session" ]] || {
		printf 'session name cannot be empty\n' >&2
		exit 1
	}
	[[ "$session" != -* ]] || {
		printf 'session name must not start with "-": %s\n' "$session" >&2
		exit 1
	}
	[[ "$session" =~ ^[A-Za-z0-9._-]+$ ]] || {
		printf 'invalid session name: %s\n' "$session" >&2
		printf 'allowed characters: A-Z a-z 0-9 . _ -\n' >&2
		exit 1
	}
}

validate_modules_csv() {
	local modules="$1"
	awk -F, -v expected="$EXPECTED_MODULE_HEADER" -f "$(template_file module_csv_validator.awk)" "$modules"
}

ensure_required_commands() {
	command -v git >/dev/null 2>&1 || {
		printf 'required command not found: git\n' >&2
		exit 1
	}
	command -v awk >/dev/null 2>&1 || {
		printf 'required command not found: awk\n' >&2
		exit 1
	}
	if command -v python3 >/dev/null 2>&1; then
		PARALLEL_REVIEW_PYTHON_BIN='python3'
		return
	fi
	if command -v python >/dev/null 2>&1; then
		PARALLEL_REVIEW_PYTHON_BIN='python'
		return
	fi
	printf 'required command not found: python3 or python\n' >&2
	exit 1
}

ensure_inputs() {
	local modules
	ensure_required_commands
	modules="$(module_file)"
	[[ -f "$modules" ]] || {
		printf 'module file not found: %s\n' "$modules" >&2
		exit 1
	}
	ensure_template_files
	validate_modules_csv "$modules"
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

render_template_with_context() {
	local template_path="$1"
	local output_file="$2"
	local context_name="$3"
	local -a context_args=()
	local token
	declare -n context="$context_name"
	for token in "${!context[@]}"; do
		context_args+=("$token" "${context[$token]}")
	done

	"$PARALLEL_REVIEW_PYTHON_BIN" - "$template_path" "$output_file" "${context_args[@]}" <<'PY'
import re
import sys
from pathlib import Path

template_path = Path(sys.argv[1])
output_path = Path(sys.argv[2])
pairs = sys.argv[3:]

if len(pairs) % 2 != 0:
    raise SystemExit("template context must be token/value pairs")

replacements = dict(zip(pairs[::2], pairs[1::2]))
template = template_path.read_text(encoding="utf-8")

if replacements:
    pattern = re.compile(
        "|".join(re.escape(token) for token in sorted(replacements, key=len, reverse=True))
    )
    rendered = pattern.sub(lambda match: replacements[match.group(0)], template)
else:
    rendered = template

output_path.write_text(rendered, encoding="utf-8")
PY
}

ensure_template_files() {
	local template_name template_path
	for template_name in \
		create_worktrees.template.sh \
		module_csv_validator.awk \
		module_prompt.template.md \
		session_readme.template.md; do
		template_path="$(template_file "$template_name")"
		[[ -r "$template_path" ]] || {
			printf 'template file not found or unreadable: %s\n' "$template_path" >&2
			exit 1
		}
	done
}
