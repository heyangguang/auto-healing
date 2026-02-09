#!/bin/bash
# 停止并清理基础设施服务
# 使用方法: ./stop.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🛑 停止基础设施服务..."
docker compose down

echo ""
echo "✅ 服务已停止!"
echo ""
echo "💡 提示: 数据已保存在 Docker 卷中，下次启动数据不会丢失"
echo "   如需清理数据: docker compose down -v"
