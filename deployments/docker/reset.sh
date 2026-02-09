#!/bin/bash
# 重置数据库 - 清空数据并重新初始化
# ⚠️ 警告: 这将删除所有数据!
# 使用方法: ./reset.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "⚠️  警告: 这将删除所有数据!"
read -p "确认继续? (输入 yes 确认): " confirm

if [ "$confirm" != "yes" ]; then
    echo "❌ 已取消"
    exit 1
fi

echo ""
echo "🗑️  停止服务并删除数据..."
docker compose down -v

echo ""
echo "🚀 重新启动服务..."
docker compose up -d

echo ""
echo "⏳ 等待数据库初始化..."
sleep 10

echo ""
echo "✅ 数据库已重置!"
docker-compose ps
