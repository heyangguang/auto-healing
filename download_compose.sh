#!/bin/bash

# =============================================================
# 下载 Docker Compose Plugin（linux/amd64）
# 用于离线安装到 Rocky Linux 9.7 服务器
# =============================================================

set -e

OUTPUT_DIR="docker-images"
mkdir -p "${OUTPUT_DIR}"

# 获取最新版本号
echo "🔍 获取最新 Docker Compose 版本..."
VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$VERSION" ]; then
    # 如果获取失败，使用已知稳定版
    VERSION="v2.33.1"
    echo "⚠️  自动获取版本失败，使用默认版本 ${VERSION}"
else
    echo "✅ 最新版本: ${VERSION}"
fi

DOWNLOAD_URL="https://github.com/docker/compose/releases/download/${VERSION}/docker-compose-linux-x86_64"
OUTPUT_FILE="${OUTPUT_DIR}/docker-compose-linux-x86_64"

echo ""
echo "📥 下载中: ${DOWNLOAD_URL}"
curl -SL "${DOWNLOAD_URL}" -o "${OUTPUT_FILE}"
chmod +x "${OUTPUT_FILE}"

SIZE=$(du -sh "${OUTPUT_FILE}" | cut -f1)
echo ""
echo "✅ 下载完成: ${OUTPUT_FILE} (${SIZE})"
echo ""
echo "========================================="
echo "  上传并安装到服务器的步骤："
echo "========================================="
echo ""
echo "# 1. 上传到服务器"
echo "scp ${OUTPUT_FILE} user@server:/tmp/"
echo ""
echo "# 2. 在服务器上安装（以 root 执行）"
echo "mkdir -p /usr/local/lib/docker/cli-plugins"
echo "mv /tmp/docker-compose-linux-x86_64 /usr/local/lib/docker/cli-plugins/docker-compose"
echo "chmod +x /usr/local/lib/docker/cli-plugins/docker-compose"
echo ""
echo "# 3. 验证安装"
echo "docker compose version"
echo ""
