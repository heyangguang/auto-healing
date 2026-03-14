#!/bin/bash
# Auto-Healing 启动脚本
#
# 用法:
#   ./start.sh            # 启动所有（postgres/redis/ui 用 docker，server 用二进制）
#   ./start.sh server     # 只重启 server 二进制
#   ./start.sh ui         # 只重启 ui 容器
#   ./start.sh infra      # 只重启 postgres + redis

export DOCKER_HOST=unix:///run/podman/podman.sock
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

GREEN='\033[0;32m'; BLUE='\033[0;34m'; NC='\033[0m'

start_server() {
    echo -e "${BLUE}▶ 启动 server 二进制...${NC}"
    pkill -f '/data/server' 2>/dev/null || true
    sleep 1
    mkdir -p /data/logs
    export ANSIBLE_EXECUTOR_IMAGE="auto-healing-executor:${EXECUTOR_VERSION:-v1.0.0}"
    export ANSIBLE_WORKSPACE_DIR="/data/workspace"
    mkdir -p /data/workspace
    nohup /data/server > /data/logs/server.log 2>&1 &
    echo -e "${GREEN}  ✅ server 已启动 (PID: $!)${NC}"
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
