#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "$ROOT_DIR"

echo "[1/4] validate api contracts"
python3 tools/validate_api_contracts.py

echo "[2/4] validate helper scripts"
python3 tools/validate_helper_scripts.py

echo "[3/4] check shell syntax"
find deployments docker tests/e2e tools -type f -name '*.sh' -print0 | xargs -0 -r -n1 bash -n

echo "[4/4] check python syntax"
find tools tests/e2e -type f -name '*.py' -print0 | xargs -0 -r python3 -m py_compile

echo "[OK] quality surface checks passed"
