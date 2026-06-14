#!/usr/bin/env bash
set -euo pipefail

REMOTE_ROOT="/opt/auto-healing-fault-lab"
STATE_DIR="$REMOTE_ROOT/state"
WWW_DIR="$REMOTE_ROOT/www"
SERVICE_NAME="auto-healing-lab-http.service"
SERVICE_PORT="19081"
DISK_FILL_FILE="$REMOTE_ROOT/disk-fill.bin"
DISK_TARGET_PERCENT_DEFAULT="92"
DISK_MIN_FREE_BYTES="536870912"
CPU_WORKERS_DEFAULT="2"
CPU_STATE_FILE="$STATE_DIR/cpu_high.pids"
LOG_DIR="$REMOTE_ROOT/logs"
LOG_STATE_FILE="$STATE_DIR/clean_logs.bytes"
LOG_FILE="$LOG_DIR/demo-app.log"
LOG_ARCHIVE_FILE="$LOG_DIR/demo-app.1.log"
LOG_BYTES_DEFAULT="8388608"
PROCESS_STATE_FILE="$STATE_DIR/kill_process.pids"
PROCESS_NAME="auto_healing_demo_worker"
PROCESS_WORKERS_DEFAULT="1"

log() {
  printf '[fault-lab] %s\n' "$*"
}

die() {
  printf '[fault-lab] ERROR: %s\n' "$*" >&2
  exit 1
}

ensure_root() {
  [ "$(id -u)" = "0" ] || die "请使用 root 执行"
}

ensure_dirs() {
  mkdir -p "$STATE_DIR" "$WWW_DIR" "$REMOTE_ROOT/reports" "$LOG_DIR"
}

ensure_service_page() {
  cat >"$WWW_DIR/index.html" <<'EOF'
auto-healing-lab-http: ok
EOF
}

install_service() {
  ensure_root
  ensure_dirs
  ensure_service_page
  systemctl daemon-reload
  systemctl enable --now "$SERVICE_NAME"
  systemctl is-active --quiet "$SERVICE_NAME" || die "测试服务启动失败"
  log "测试服务已安装并启动: $SERVICE_NAME"
}

service_down_inject() {
  systemctl stop "$SERVICE_NAME"
  systemctl is-active --quiet "$SERVICE_NAME" && die "服务停止失败"
  log "已注入 service_down"
}

service_down_reset() {
  systemctl start "$SERVICE_NAME"
  systemctl is-active --quiet "$SERVICE_NAME" || die "服务恢复失败"
  log "已恢复 service_down"
}

service_down_status() {
  if systemctl is-active --quiet "$SERVICE_NAME"; then
    log "service_down=healthy service=$SERVICE_NAME port=$SERVICE_PORT"
    return
  fi
  log "service_down=injected service=$SERVICE_NAME port=$SERVICE_PORT"
}

start_cpu_worker() {
  nohup bash -lc 'while :; do :; done' >/dev/null 2>&1 &
  echo $!
}

cpu_high_inject() {
  local workers="${1:-$CPU_WORKERS_DEFAULT}"
  [ ! -f "$CPU_STATE_FILE" ] || die "cpu_high 已经处于注入状态"
  : >"$CPU_STATE_FILE"
  for _ in $(seq 1 "$workers"); do
    start_cpu_worker >>"$CPU_STATE_FILE"
  done
  log "已注入 cpu_high workers=$workers"
}

cpu_high_reset() {
  [ -f "$CPU_STATE_FILE" ] || die "cpu_high 当前未注入"
  while read -r pid; do
    [ -n "$pid" ] || continue
    kill "$pid" 2>/dev/null || true
  done <"$CPU_STATE_FILE"
  rm -f "$CPU_STATE_FILE"
  log "已恢复 cpu_high"
}

cpu_high_status() {
  local running=0
  if [ -f "$CPU_STATE_FILE" ]; then
    while read -r pid; do
      [ -n "$pid" ] || continue
      if kill -0 "$pid" 2>/dev/null; then
        running=$((running + 1))
      fi
    done <"$CPU_STATE_FILE"
  fi
  if [ "$running" -gt 0 ]; then
    log "cpu_high=injected workers=$running"
    return
  fi
  log "cpu_high=healthy workers=0"
}

disk_current_bytes() {
  df -B1 --output=used / | tail -1 | tr -d ' '
}

disk_size_bytes() {
  df -B1 --output=size / | tail -1 | tr -d ' '
}

disk_avail_bytes() {
  df -B1 --output=avail / | tail -1 | tr -d ' '
}

allocate_file() {
  local bytes="$1"
  if command -v fallocate >/dev/null 2>&1; then
    fallocate -l "$bytes" "$DISK_FILL_FILE"
    return
  fi
  dd if=/dev/zero of="$DISK_FILL_FILE" bs=1M count=$((bytes / 1048576)) status=none
}

disk_full_inject() {
  local target_percent="${1:-$DISK_TARGET_PERCENT_DEFAULT}"
  local used size avail target additional
  [ ! -f "$DISK_FILL_FILE" ] || die "disk_full 已经处于注入状态"
  used="$(disk_current_bytes)"
  size="$(disk_size_bytes)"
  avail="$(disk_avail_bytes)"
  target=$((size * target_percent / 100))
  additional=$((target - used))
  [ "$additional" -gt 0 ] || die "当前磁盘占用已达到目标阈值，无需注入"
  [ $((avail - additional)) -ge "$DISK_MIN_FREE_BYTES" ] || die "注入后剩余空间过小，已拒绝执行"
  allocate_file "$additional"
  sync
  log "已注入 disk_full target_percent=$target_percent added_bytes=$additional"
}

disk_full_reset() {
  [ -f "$DISK_FILL_FILE" ] || die "disk_full 当前未注入"
  rm -f "$DISK_FILL_FILE"
  sync
  log "已恢复 disk_full"
}

disk_full_status() {
  local usage
  usage="$(df -h / | tail -1 | awk '{print $5}')"
  if [ -f "$DISK_FILL_FILE" ]; then
    log "disk_full=injected usage=$usage file=$DISK_FILL_FILE"
    return
  fi
  log "disk_full=healthy usage=$usage file=$DISK_FILL_FILE"
}

write_random_log() {
  local file="$1"
  local bytes="$2"
  dd if=/dev/zero bs=1024 count=$((bytes / 1024)) status=none | tr '\0' 'L' >"$file"
}

clean_logs_inject() {
  local bytes="${1:-$LOG_BYTES_DEFAULT}"
  ensure_dirs
  rm -f "$LOG_ARCHIVE_FILE"
  write_random_log "$LOG_FILE" "$bytes"
  printf '%s\n' "$bytes" >"$LOG_STATE_FILE"
  log "已注入 clean_logs bytes=$bytes dir=$LOG_DIR"
}

clean_logs_reset() {
  ensure_dirs
  : >"$LOG_FILE"
  rm -f "$LOG_ARCHIVE_FILE" "$LOG_STATE_FILE"
  log "已恢复 clean_logs dir=$LOG_DIR"
}

clean_logs_status() {
  ensure_dirs
  local total
  total="$(du -sb "$LOG_DIR" 2>/dev/null | awk '{print $1}')"
  if [ -f "$LOG_STATE_FILE" ] && [ "${total:-0}" -gt 1048576 ]; then
    log "clean_logs=injected bytes=$total dir=$LOG_DIR"
    return
  fi
  log "clean_logs=healthy bytes=${total:-0} dir=$LOG_DIR"
}

start_demo_process() {
  bash -c "exec -a $PROCESS_NAME bash -c 'while :; do :; done'" >/dev/null 2>&1 &
  echo $!
}

kill_process_inject() {
  local workers="${1:-$PROCESS_WORKERS_DEFAULT}"
  local running
  running="$(pgrep -fc "$PROCESS_NAME" || true)"
  if [ "${running:-0}" -gt 0 ]; then
    log "kill_process 已经处于注入状态 process=$PROCESS_NAME workers=$running"
    return
  fi
  rm -f "$PROCESS_STATE_FILE"
  : >"$PROCESS_STATE_FILE"
  for _ in $(seq 1 "$workers"); do
    start_demo_process >>"$PROCESS_STATE_FILE"
  done
  log "已注入 kill_process process=$PROCESS_NAME workers=$workers"
}

kill_process_reset() {
  local killed=0
  if [ -f "$PROCESS_STATE_FILE" ]; then
    while read -r pid; do
      [ -n "$pid" ] || continue
      if kill "$pid" 2>/dev/null; then
        killed=$((killed + 1))
      fi
    done <"$PROCESS_STATE_FILE"
  fi
  pkill -f "$PROCESS_NAME" 2>/dev/null || true
  rm -f "$PROCESS_STATE_FILE"
  log "已恢复 kill_process process=$PROCESS_NAME killed=$killed"
}

kill_process_status() {
  local running
  running="$(pgrep -fc "$PROCESS_NAME" || true)"
  if [ "${running:-0}" -gt 0 ]; then
    log "kill_process=injected process=$PROCESS_NAME workers=$running"
    return
  fi
  log "kill_process=healthy process=$PROCESS_NAME workers=0"
}

status_all() {
  service_down_status
  cpu_high_status
  disk_full_status
  clean_logs_status
  kill_process_status
}

usage() {
  cat <<'EOF'
用法:
  auto_healing_fault_lab.sh install-service
  auto_healing_fault_lab.sh inject service_down
  auto_healing_fault_lab.sh inject cpu_high [workers]
  auto_healing_fault_lab.sh inject disk_full [target_percent]
  auto_healing_fault_lab.sh inject clean_logs [bytes]
  auto_healing_fault_lab.sh inject kill_process [workers]
  auto_healing_fault_lab.sh reset service_down|cpu_high|disk_full|clean_logs|kill_process
  auto_healing_fault_lab.sh status service_down|cpu_high|disk_full|clean_logs|kill_process|all
EOF
}

main() {
  ensure_dirs
  local action="${1:-}"
  local scenario="${2:-}"
  case "$action" in
    install-service) install_service ;;
    inject)
      case "$scenario" in
        service_down) service_down_inject ;;
        cpu_high) cpu_high_inject "${3:-}" ;;
        disk_full) disk_full_inject "${3:-}" ;;
        clean_logs) clean_logs_inject "${3:-}" ;;
        kill_process) kill_process_inject "${3:-}" ;;
        *) usage; die "未知场景: $scenario" ;;
      esac
      ;;
    reset)
      case "$scenario" in
        service_down) service_down_reset ;;
        cpu_high) cpu_high_reset ;;
        disk_full) disk_full_reset ;;
        clean_logs) clean_logs_reset ;;
        kill_process) kill_process_reset ;;
        *) usage; die "未知场景: $scenario" ;;
      esac
      ;;
    status)
      case "$scenario" in
        service_down) service_down_status ;;
        cpu_high) cpu_high_status ;;
        disk_full) disk_full_status ;;
        clean_logs) clean_logs_status ;;
        kill_process) kill_process_status ;;
        all) status_all ;;
        *) usage; die "未知场景: $scenario" ;;
      esac
      ;;
    *) usage; exit 1 ;;
  esac
}

main "$@"
