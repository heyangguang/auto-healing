#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v python3.11 >/dev/null 2>&1; then
  echo "需要 python3.11"
  exit 1
fi

exec python3.11 "$SCRIPT_DIR/test_acceptance_real.py"
