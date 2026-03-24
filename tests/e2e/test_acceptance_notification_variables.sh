#!/bin/bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3.11 "$SCRIPT_DIR/test_acceptance_real.py" --phase notification_variables
