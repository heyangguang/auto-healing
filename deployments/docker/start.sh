#!/bin/bash
# Auto-Healing 启动脚本
#
# 用法:
#   ./start.sh            # 启动所有（postgres/redis/ui 用 docker，server 用二进制）
#   ./start.sh server     # 只重启 server 二进制
#   ./start.sh ui         # 只重启 ui 容器
#   ./start.sh infra      # 只重启 postgres + redis

set -euo pipefail

export DOCKER_HOST=unix:///run/podman/podman.sock
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

GREEN='\033[0;32m'; BLUE='\033[0;34m'; NC='\033[0m'

SERVER_BIN="/data/server"
SERVER_LOG="/data/logs/server.log"
SERVER_STARTUP_WAIT_SECONDS=2

start_server() {
    echo -e "${BLUE}▶ 启动 server 二进制...${NC}"
    pkill -f '/data/server' 2>/dev/null || true
    sleep 1
    mkdir -p /data/logs
    export ANSIBLE_EXECUTOR_IMAGE="auto-healing-executor:${EXECUTOR_VERSION:-v1.0.0}"
    export ANSIBLE_WORKSPACE_DIR="/data/workspace"
    mkdir -p /data/workspace
    if [ ! -x "$SERVER_BIN" ]; then
        echo "server 二进制不存在或不可执行: $SERVER_BIN" >&2
        return 1
    fi
    nohup "$SERVER_BIN" > "$SERVER_LOG" 2>&1 &
    local server_pid=$!
    sleep "$SERVER_STARTUP_WAIT_SECONDS"
    if ! kill -0 "$server_pid" 2>/dev/null; then
        echo "server 启动失败，请检查日志: $SERVER_LOG" >&2
        tail -n 20 "$SERVER_LOG" 2>/dev/null || true
        return 1
    fi
    echo -e "${GREEN}  ✅ server 已启动 (PID: $server_pid)${NC}"
}

start_infra() {
    echo -e "${BLUE}▶ 启动基础设施（postgres/redis）...${NC}"
    for svc in postgres redis; do
        docker stop "auto-healing-${svc}" 2>/dev/null || true
        docker rm -f "auto-healing-${svc}" 2>/dev/null || true
    done
    docker compose up -d postgres redis
}

start_ui() {
    echo -e "${BLUE}▶ 启动 UI...${NC}"
    docker stop auto-healing-ui 2>/dev/null || true
    docker rm -f auto-healing-ui 2>/dev/null || true
    UI_VERSION=${UI_VERSION:-v1.0.0} docker compose up -d --no-deps ui
}

case "${1:-all}" in
    server)  start_server ;;
    ui)      start_ui ;;
    infra)   start_infra ;;
    all)
        docker compose down 2>/dev/null || true
        docker compose up -d postgres redis ui
        sleep 3
        start_server
        ;;
    *)
        echo "用法: $0 [server|ui|infra|all]"
        exit 1
        ;;
esac

echo ""
IP=$(hostname -I | awk '{print $1}')
echo -e "${GREEN}✅ 完成${NC}"
echo "  前端: http://${IP}:8000"
echo "  后端: http://${IP}:8080"
echo ""
