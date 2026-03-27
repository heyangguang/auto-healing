#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

SESSION_ROOT="${TMP_DIR}/review"
SESSION_NAME="parallel-review-e2e"

REVIEW_ROOT="${SESSION_ROOT}" bash "${REPO_ROOT}/scripts/review/setup_parallel_review.sh" "${SESSION_NAME}"

SESSION_DIR="${SESSION_ROOT}/${SESSION_NAME}"
HELPER="${SESSION_DIR}/create_worktrees.sh"

test -f "${SESSION_DIR}/module_csv_validator.awk"
test -f "${HELPER}"
test -f "${SESSION_DIR}/README.md"

bash "${HELPER}" --help >/dev/null
LIST_OUTPUT="$(bash "${HELPER}" --list)"
printf '%s\n' "${LIST_OUTPUT}" | grep -q '^auth_middleware,'

FAKE_GIT_DIR="${TMP_DIR}/fakegit"
LOG_FILE="${FAKE_GIT_DIR}/git.log"
REAL_GIT="$(command -v git)"
mkdir -p "${FAKE_GIT_DIR}"
cat > "${FAKE_GIT_DIR}/git" <<EOF
#!/bin/bash
printf '%s\n' "\$*" >> "${LOG_FILE}"
if [[ "\${1:-}" == "worktree" && "\${2:-}" == "add" ]]; then
  printf 'unexpected worktree add\n' >> "${LOG_FILE}"
  exit 99
fi
exec "${REAL_GIT}" "\$@"
EOF
chmod +x "${FAKE_GIT_DIR}/git"
: > "${LOG_FILE}"

set +e
PATH="${FAKE_GIT_DIR}:$PATH" bash "${HELPER}" auth_middleware no_such_module
STATUS=$?
set -e

test "${STATUS}" -ne 0
! grep -q 'unexpected worktree add' "${LOG_FILE}"

echo "parallel review tooling e2e passed"
