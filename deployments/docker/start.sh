#!/bin/bash
# 启动基础设施服务 (PostgreSQL + Redis)
# 使用方法: ./start.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 启动基础设施服务..."
docker compose up -d

echo ""
echo "⏳ 等待服务启动..."
sleep 5

echo ""
echo "📊 检查服务状态..."
docker compose ps

echo ""
echo "✅ 基础设施启动完成!"
echo ""
echo "📝 连接信息:"
echo "   PostgreSQL: localhost:5432"
echo "     用户名: postgres"
echo "     密码: postgres"
echo "     数据库: auto_healing"
echo ""
echo "   Redis: localhost:6379"
echo ""
echo "🔧 常用命令:"
echo "   查看日志: docker-compose logs -f"
echo "   停止服务: docker-compose down"
echo "   重启服务: docker-compose restart"
