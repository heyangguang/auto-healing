#!/bin/bash
# 重置数据库 - 清空数据并重新初始化
# ⚠️ 警告: 这将删除所有数据!
# 使用方法: ./reset.sh

set -euo pipefail

export DOCKER_HOST=unix:///run/podman/podman.sock

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

POSTGRES_DATA_DIR="${POSTGRES_DATA_DIR:-/data/postgres}"
REDIS_DATA_DIR="${REDIS_DATA_DIR:-/data/redis}"

echo "⚠️  警告: 这将删除以下目录中的持久化数据!"
echo "   - ${POSTGRES_DATA_DIR}"
echo "   - ${REDIS_DATA_DIR}"
read -p "确认继续? (输入 yes 确认): " confirm

if [ "$confirm" != "yes" ]; then
    echo "❌ 已取消"
    exit 1
fi

echo ""
echo "🗑️  停止服务并删除数据..."
docker compose down -v
rm -rf "${POSTGRES_DATA_DIR:?}" "${REDIS_DATA_DIR:?}"
mkdir -p "${POSTGRES_DATA_DIR}" "${REDIS_DATA_DIR}"

echo ""
echo "🚀 重新启动服务..."
docker compose up -d

echo ""
echo "⏳ 等待数据库初始化..."
sleep 10

echo ""
echo "✅ 数据库已重置!"
docker compose ps
