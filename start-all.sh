#!/bin/bash

# 运维自愈系统 - 快速启动脚本

set -e

DEFAULT_ADMIN_PASSWORD="admin123456"

echo "========================================"
echo "运维自愈系统 - 启动中"
echo "========================================"

# 检查基础设施
echo ""
echo "📦 检查 Docker 容器状态..."
cd /root/auto-healing/deployments/docker
if docker compose ps | grep -q "Up"; then
    echo "✅ PostgreSQL 和 Redis 正在运行"
else
    echo "🔄 启动 PostgreSQL 和 Redis..."
    ./start.sh
    sleep 3
fi

cd /root/auto-healing

# 检查是否需要初始化管理员
echo ""
echo "👤 检查超级管理员..."
ADMIN_EXISTS=$(psql -h localhost -U postgres -d auto_healing -tAc "SELECT COUNT(*) FROM users WHERE username='admin'" 2>/dev/null || echo "0")

if [ "$ADMIN_EXISTS" = "0" ]; then
    echo "🔄 初始化超级管理员..."
    export INIT_ADMIN_PASSWORD="${INIT_ADMIN_PASSWORD:-$DEFAULT_ADMIN_PASSWORD}"
    ./bin/init-admin
else
    echo "✅ 超级管理员已存在"
fi

# 启动 Mock ITSM
echo ""
echo "🧪 启动 Mock ITSM (测试服务)..."
pkill -f mock_itsm.py 2>/dev/null || true
cd tools
nohup python3 mock_itsm.py > /tmp/mock_itsm.log 2>&1 &
MOCK_PID=$!
cd ..

sleep 2

# 检查 Mock ITSM
if curl -s http://localhost:5000/health > /dev/null 2>&1; then
    echo "✅ Mock ITSM 启动成功 (PID: $MOCK_PID)"
else
    echo "❌ Mock ITSM 启动失败，请检查日志: /tmp/mock_itsm.log"
fi

# 启动主服务
echo ""
echo "🚀 启动主服务..."
pkill -f "bin/server" 2>/dev/null || true
./bin/server 2>&1 &
SERVER_PID=$!

sleep 3

# 检查主服务
if curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ 主服务启动成功 (PID: $SERVER_PID)"
else
    echo "❌ 主服务启动失败，请检查日志"
    exit 1
fi

echo ""
echo "========================================"
echo "✅ 所有服务已启动"
echo "========================================"
echo ""
echo "服务地址:"
echo "  主服务:    http://localhost:8080"
echo "  Mock ITSM: http://localhost:5000"
echo ""
echo "测试账号:"
echo "  用户名: admin"
echo "  密码:   ${INIT_ADMIN_PASSWORD:-$DEFAULT_ADMIN_PASSWORD}"
echo ""
echo "查看日志:"
echo "  主服务:  tail -f /tmp/server.log (或查看当前终端)"
echo "  Mock:    tail -f /tmp/mock_itsm.log"
echo ""
echo "测试指南: docs/api-testing-guide.md"
echo ""
echo "停止服务:"
echo "  pkill -f mock_itsm.py"
echo "  pkill -f bin/server"
echo "========================================"
