#!/bin/bash

# =============================================================
# Auto-Healing Platform - Docker 构建打包脚本
#
# 用法:
#   ./build_docker.sh <version> [选项]
#
# 选项 (可多选，默认全部构建):
#   --server      只构建后端主服务
#   --executor    只构建 Ansible 执行器
#   --ui          只构建前端 UI
#
# 示例:
#   ./build_docker.sh v1.0.1              # 全部构建
#   ./build_docker.sh v1.0.1 --server     # 只构建后端
#   ./build_docker.sh v1.0.1 --ui         # 只构建前端
#   ./build_docker.sh v1.0.1 --server --executor  # 后端+执行器
# =============================================================

set -e

APP_NAME="auto-healing"
EXECUTOR_NAME="auto-healing-executor"
UI_NAME="auto-healing-ui"
UI_DIR="../auto-healing-ui"
OUTPUT_DIR="docker-images"
PLATFORM="linux/amd64"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ── 参数解析 ──────────────────────────────────────
if [ -z "$1" ]; then
    echo -e "${RED}❌ 错误: 请提供版本号${NC}"
    echo ""
    echo "用法: $0 <version> [--server] [--executor] [--ui]"
    echo "示例: $0 v1.0.1 --server"
    exit 1
fi

VERSION=$1
shift

BUILD_SERVER=false
BUILD_EXECUTOR=false
BUILD_UI=false

# 解析剩余参数
if [ $# -eq 0 ]; then
    # 没有选项 → 全部构建
    BUILD_SERVER=true
    BUILD_EXECUTOR=true
    BUILD_UI=true
else
    for arg in "$@"; do
        case $arg in
            --server)   BUILD_SERVER=true ;;
            --executor) BUILD_EXECUTOR=true ;;
            --ui)       BUILD_UI=true ;;
            *) echo -e "${YELLOW}⚠️  未知选项: $arg，已忽略${NC}" ;;
        esac
    done
fi

# ── 构建信息 ──────────────────────────────────────
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Auto-Healing Docker 构建打包工具${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "  版本:     ${GREEN}${VERSION}${NC}"
echo -e "  平台:     ${GREEN}${PLATFORM}${NC}"
echo -e "  Git 提交: ${GREEN}${GIT_COMMIT}${NC}"
echo -e "  构建范围: ${GREEN}$([ "$BUILD_SERVER" = true ] && echo "后端 ")$([ "$BUILD_EXECUTOR" = true ] && echo "执行器 ")$([ "$BUILD_UI" = true ] && echo "前端")${NC}"
echo ""

mkdir -p "${OUTPUT_DIR}"

STEP=0
TOTAL=$(( (BUILD_SERVER ? 1 : 0) + (BUILD_EXECUTOR ? 1 : 0) + (BUILD_UI ? 1 : 0) ))
TOTAL=$(( TOTAL * 2 ))  # 每个都有 build + export 两步

# ── 编译后端主服务二进制（linux/amd64，直接部署到宿主机）────────────
SERVER_TAR=""
if [ "$BUILD_SERVER" = true ]; then
    STEP=$((STEP + 1))
    echo -e "${BLUE}[${STEP}] 🔨 编译后端主服务（linux/amd64 二进制）...${NC}"
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -trimpath \
        -ldflags "-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}" \
        -o "${OUTPUT_DIR}/server" \
        ./cmd/server
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
        -trimpath \
        -ldflags "-s -w" \
        -o "${OUTPUT_DIR}/init-admin" \
        ./cmd/init-admin
    echo -e "${GREEN}  ✅ 编译完成: ${OUTPUT_DIR}/server${NC}"
    echo ""

    STEP=$((STEP + 1))
    SERVER_TAR="${OUTPUT_DIR}/${APP_NAME}-${VERSION}.tar.gz"
    echo -e "${BLUE}[${STEP}] 💾 打包后端二进制...${NC}"
    tar -czf "${SERVER_TAR}" -C "${OUTPUT_DIR}" server init-admin
    SERVER_SIZE=$(du -sh "${SERVER_TAR}" | cut -f1)
    echo -e "${GREEN}  ✅ 打包完成: ${SERVER_TAR} (${SERVER_SIZE})${NC}"
    echo ""
fi

# ── 构建 Ansible 执行器 ───────────────────────────
EXECUTOR_TAR=""
if [ "$BUILD_EXECUTOR" = true ]; then
    STEP=$((STEP + 1))
    echo -e "${BLUE}[${STEP}] 📦 构建 Ansible 执行器...${NC}"
    docker build \
        --platform "${PLATFORM}" \
        -t "${EXECUTOR_NAME}:${VERSION}" \
        -f docker/ansible-executor/Dockerfile \
        docker/ansible-executor/
    echo -e "${GREEN}  ✅ 构建完成: ${EXECUTOR_NAME}:${VERSION}${NC}"
    echo ""

    STEP=$((STEP + 1))
    EXECUTOR_TAR="${OUTPUT_DIR}/${EXECUTOR_NAME}-${VERSION}.tar.gz"
    echo -e "${BLUE}[${STEP}] 💾 导出 Ansible 执行器...${NC}"
    docker save "${EXECUTOR_NAME}:${VERSION}" | gzip > "${EXECUTOR_TAR}"
    EXECUTOR_SIZE=$(du -sh "${EXECUTOR_TAR}" | cut -f1)
    echo -e "${GREEN}  ✅ 导出完成: ${EXECUTOR_TAR} (${EXECUTOR_SIZE})${NC}"
    echo ""
fi

# ── 构建前端 UI ───────────────────────────────────
UI_TAR=""
if [ "$BUILD_UI" = true ]; then
    STEP=$((STEP + 1))
    echo -e "${BLUE}[${STEP}] 🎨 构建前端 UI...${NC}"
    if [ ! -d "${UI_DIR}" ]; then
        echo -e "${YELLOW}  ⚠️  未找到前端目录 ${UI_DIR}，跳过${NC}"
    else
        docker build \
            --platform "${PLATFORM}" \
            -t "${UI_NAME}:${VERSION}" \
            "${UI_DIR}"
        echo -e "${GREEN}  ✅ 构建完成: ${UI_NAME}:${VERSION}${NC}"
        echo ""

        STEP=$((STEP + 1))
        UI_TAR="${OUTPUT_DIR}/${UI_NAME}-${VERSION}.tar.gz"
        echo -e "${BLUE}[${STEP}] 💾 导出前端 UI...${NC}"
        docker save "${UI_NAME}:${VERSION}" | gzip > "${UI_TAR}"
        UI_SIZE=$(du -sh "${UI_TAR}" | cut -f1)
        echo -e "${GREEN}  ✅ 导出完成: ${UI_TAR} (${UI_SIZE})${NC}"
        echo ""
    fi
fi

# ── 完成 ─────────────────────────────────────────
echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}  ✅ 构建打包完成！${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "生成文件:"
[ -n "${SERVER_TAR}" ]   && echo "  📦 ${SERVER_TAR} (${SERVER_SIZE})"
[ -n "${EXECUTOR_TAR}" ] && echo "  📦 ${EXECUTOR_TAR} (${EXECUTOR_SIZE})"
[ -n "${UI_TAR}" ]       && echo "  📦 ${UI_TAR} (${UI_SIZE})"
echo ""
echo "部署到目标服务器:"
echo "  # 1. 拷贝文件"
echo "  scp <上面的文件> root@server:/data/"
echo ""
if [ -n "${SERVER_TAR}" ]; then
    echo "  # 2a. 部署后端二进制"
    echo "  tar -xzf $(basename ${SERVER_TAR}) -C /data/"
    echo "  ./deployments/docker/start.sh server"
    echo ""
fi
if [ -n "${EXECUTOR_TAR}" ] || [ -n "${UI_TAR}" ]; then
    echo "  # 2b. 加载镜像"
    [ -n "${EXECUTOR_TAR}" ] && echo "  gunzip -c ${EXECUTOR_NAME}-${VERSION}.tar.gz | docker load"
    [ -n "${UI_TAR}" ]       && echo "  gunzip -c ${UI_NAME}-${VERSION}.tar.gz | docker load"
    echo ""
    echo "  # 3. 重启容器服务"
    [ -n "${EXECUTOR_TAR}" ] && echo "  EXECUTOR_VERSION=${VERSION} ./deployments/docker/start.sh"
    [ -n "${UI_TAR}" ]       && echo "  UI_VERSION=${VERSION} ./deployments/docker/start.sh ui"
fi
echo ""
