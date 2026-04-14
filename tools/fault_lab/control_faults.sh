#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="$SCRIPT_DIR/.fault_lab.env"
REMOTE_SCRIPT="/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh"
DEFAULT_KEY_77="/etc/auto-healing/secrets/192.168.31.77/id_ed25519"

if [ -f "$ENV_FILE" ]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

HOST_100_IP="${HOST_100_IP:-192.168.31.100}"
HOST_101_IP="${HOST_101_IP:-192.168.31.101}"
HOST_77_IP="${HOST_77_IP:-192.168.31.77}"
HOST_77_KEY="${HOST_77_KEY:-$DEFAULT_KEY_77}"

usage() {
  cat <<'EOF'
用法:
  tools/fault_lab/control_faults.sh matrix
  tools/fault_lab/control_faults.sh status 100|101|77|all
  tools/fault_lab/control_faults.sh inject matrix
  tools/fault_lab/control_faults.sh reset matrix
  tools/fault_lab/control_faults.sh inject 100 service_down
  tools/fault_lab/control_faults.sh inject 100 cpu_high [workers]
  tools/fault_lab/control_faults.sh inject 101 service_down
  tools/fault_lab/control_faults.sh inject 101 disk_full [target_percent]
  tools/fault_lab/control_faults.sh inject 77 cpu_high [workers]
  tools/fault_lab/control_faults.sh reset 100|101|77 scenario
EOF
}

print_matrix() {
  cat <<EOF
100 -> service_down, cpu_high
101 -> service_down, disk_full
77  -> cpu_high
EOF
}

require_password() {
  local name="$1" value="$2"
  [ -n "$value" ] || {
    printf '缺少 %s，请检查 %s 或环境变量\n' "$name" "$ENV_FILE" >&2
    exit 1
  }
}

run_sshpass() {
  local password="$1" host="$2" cmd="$3"
  sshpass -p "$password" ssh -o StrictHostKeyChecking=no "root@$host" "$cmd"
}

run_sshkey() {
  local host="$1" cmd="$2"
  [ -f "$HOST_77_KEY" ] || {
    printf '缺少 77 机器私钥: %s\n' "$HOST_77_KEY" >&2
    exit 1
  }
  ssh -i "$HOST_77_KEY" -o StrictHostKeyChecking=no "root@$host" "$cmd"
}

target_ip() {
  case "$1" in
    100) printf '%s' "$HOST_100_IP" ;;
    101) printf '%s' "$HOST_101_IP" ;;
    77) printf '%s' "$HOST_77_IP" ;;
    *) printf '未知目标: %s\n' "$1" >&2; exit 1 ;;
  esac
}

supports_scenario() {
  case "$1:$2" in
    100:service_down|100:cpu_high|101:service_down|101:disk_full|77:cpu_high) return 0 ;;
    *) return 1 ;;
  esac
}

remote_cmd() {
  local action="$1" scenario="$2" extra="${3:-}"
  if [ -n "$extra" ]; then
    printf '%s %s %s %s' "$REMOTE_SCRIPT" "$action" "$scenario" "$extra"
    return
  fi
  printf '%s %s %s' "$REMOTE_SCRIPT" "$action" "$scenario"
}

run_target() {
  local target="$1" cmd="$2"
  case "$target" in
    100)
      require_password "FAULT_HOST_100_PASSWORD" "${FAULT_HOST_100_PASSWORD:-}"
      run_sshpass "$FAULT_HOST_100_PASSWORD" "$(target_ip 100)" "$cmd"
      ;;
    101)
      require_password "FAULT_HOST_101_PASSWORD" "${FAULT_HOST_101_PASSWORD:-}"
      run_sshpass "$FAULT_HOST_101_PASSWORD" "$(target_ip 101)" "$cmd"
      ;;
    77)
      run_sshkey "$(target_ip 77)" "$cmd"
      ;;
    *)
      printf '未知目标: %s\n' "$target" >&2
      exit 1
      ;;
  esac
}

status_target() {
  if [ "$1" = "all" ]; then
    for target in 100 101 77; do
      printf '== %s ==\n' "$target"
      run_target "$target" "$REMOTE_SCRIPT status all"
    done
    return
  fi
  run_target "$1" "$REMOTE_SCRIPT status all"
}

scenario_action() {
  local action="$1" target="$2" scenario="$3" extra="${4:-}"
  if ! supports_scenario "$target" "$scenario"; then
    printf '目标 %s 不支持场景 %s\n' "$target" "$scenario" >&2
    print_matrix
    exit 1
  fi
  run_target "$target" "$(remote_cmd "$action" "$scenario" "$extra")"
}

matrix_action() {
  local action="$1"
  case "$action" in
    inject)
      printf '== 100 service_down ==\n'
      scenario_action inject 100 service_down
      printf '== 100 cpu_high ==\n'
      scenario_action inject 100 cpu_high 2
      printf '== 101 service_down ==\n'
      scenario_action inject 101 service_down
      printf '== 101 disk_full ==\n'
      scenario_action inject 101 disk_full 92
      printf '== 77 cpu_high ==\n'
      scenario_action inject 77 cpu_high 2
      ;;
    reset)
      printf '== 100 service_down ==\n'
      scenario_action reset 100 service_down
      printf '== 100 cpu_high ==\n'
      scenario_action reset 100 cpu_high
      printf '== 101 service_down ==\n'
      scenario_action reset 101 service_down
      printf '== 101 disk_full ==\n'
      scenario_action reset 101 disk_full
      printf '== 77 cpu_high ==\n'
      scenario_action reset 77 cpu_high
      ;;
    *)
      printf '不支持的矩阵动作: %s\n' "$action" >&2
      exit 1
      ;;
  esac
}

main() {
  local action="${1:-}"
  case "$action" in
    matrix)
      print_matrix
      ;;
    status)
      [ -n "${2:-}" ] || { usage; exit 1; }
      status_target "$2"
      ;;
    inject|reset)
      if [ "${2:-}" = "matrix" ]; then
        matrix_action "$action"
        exit 0
      fi
      [ -n "${2:-}" ] && [ -n "${3:-}" ] || { usage; exit 1; }
      scenario_action "$action" "$2" "$3" "${4:-}"
      ;;
    *)
      usage
      exit 1
      ;;
  esac
}

main "$@"
