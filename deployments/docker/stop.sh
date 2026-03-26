#!/bin/bash
# 停止并清理基础设施服务
# 使用方法: ./stop.sh

set -euo pipefail

export DOCKER_HOST=unix:///run/podman/podman.sock

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🛑 停止基础设施服务..."
docker compose down

echo ""
echo "✅ 服务已停止!"
echo ""
echo "💡 提示: 数据保存在 bind mount 目录 /data/postgres 和 /data/redis"
echo "   如需清理真实数据，请执行: ./reset.sh"
