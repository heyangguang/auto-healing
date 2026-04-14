#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_ROOT="/opt/auto-healing-fault-lab"
REMOTE_SCRIPT="$REMOTE_ROOT/auto_healing_fault_lab.sh"
REMOTE_SERVICE="/etc/systemd/system/auto-healing-lab-http.service"
HOST_77_KEY="${HOST_77_KEY:-/etc/auto-healing/secrets/192.168.31.77/id_ed25519}"

copy_password() {
  local password="$1" src="$2" host="$3" dest="$4"
  sshpass -p "$password" scp -o StrictHostKeyChecking=no "$src" "root@$host:$dest"
}

run_password() {
  local password="$1" host="$2" cmd="$3"
  sshpass -p "$password" ssh -o StrictHostKeyChecking=no "root@$host" "$cmd"
}

copy_key() {
  local src="$1" host="$2" dest="$3"
  scp -i "$HOST_77_KEY" -o StrictHostKeyChecking=no "$src" "root@$host:$dest"
}

run_key() {
  local host="$1" cmd="$2"
  ssh -i "$HOST_77_KEY" -o StrictHostKeyChecking=no "root@$host" "$cmd"
}

deploy_password_host() {
  local password="$1" host="$2"
  run_password "$password" "$host" "mkdir -p $REMOTE_ROOT"
  copy_password "$password" "$SCRIPT_DIR/auto_healing_fault_lab.sh" "$host" "$REMOTE_SCRIPT"
  copy_password "$password" "$SCRIPT_DIR/auto-healing-lab-http.service" "$host" "$REMOTE_SERVICE"
  run_password "$password" "$host" "chmod +x $REMOTE_SCRIPT && $REMOTE_SCRIPT install-service && $REMOTE_SCRIPT status all"
}

deploy_key_host() {
  local host="$1"
  run_key "$host" "mkdir -p $REMOTE_ROOT"
  copy_key "$SCRIPT_DIR/auto_healing_fault_lab.sh" "$host" "$REMOTE_SCRIPT"
  copy_key "$SCRIPT_DIR/auto-healing-lab-http.service" "$host" "$REMOTE_SERVICE"
  run_key "$host" "chmod +x $REMOTE_SCRIPT && $REMOTE_SCRIPT install-service && $REMOTE_SCRIPT status all"
}

main() {
  [ -n "${FAULT_HOST_100_PASSWORD:-}" ] || { echo "缺少 FAULT_HOST_100_PASSWORD" >&2; exit 1; }
  [ -n "${FAULT_HOST_101_PASSWORD:-}" ] || { echo "缺少 FAULT_HOST_101_PASSWORD" >&2; exit 1; }
  [ -f "$HOST_77_KEY" ] || { echo "缺少 HOST_77_KEY 对应私钥: $HOST_77_KEY" >&2; exit 1; }

  deploy_password_host "$FAULT_HOST_100_PASSWORD" "192.168.31.100"
  deploy_password_host "$FAULT_HOST_101_PASSWORD" "192.168.31.101"
  deploy_key_host "192.168.31.77"
}

main "$@"
