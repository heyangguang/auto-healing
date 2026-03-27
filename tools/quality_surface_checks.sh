#!/bin/bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

cd "$ROOT_DIR"

echo "[1/5] validate openapi bundle freshness"
python3 tools/build_openapi.py --check

echo "[2/5] validate api contracts"
python3 tools/validate_api_contracts.py

echo "[3/5] validate helper scripts"
python3 tools/validate_helper_scripts.py

echo "[4/5] check shell syntax"
find deployments docker tests/e2e tools -type f -name '*.sh' -print0 | xargs -0 -r -n1 bash -n

echo "[5/5] check python syntax"
find tools tests/e2e -type f -name '*.py' -print0 | xargs -0 -r python3 -m py_compile

echo "[OK] quality surface checks passed"
