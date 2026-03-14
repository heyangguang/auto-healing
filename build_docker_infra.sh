#!/bin/bash

# =============================================================
# Auto-Healing Platform - 基础设施镜像打包脚本（仅首次部署需要）
# 用法: ./build_docker_infra.sh
# =============================================================

set -e

OUTPUT_DIR="docker-images"
PLATFORM="linux/amd64"

GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  基础设施镜像打包（首次部署使用）${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

mkdir -p "${OUTPUT_DIR}"

# 导出函数
# 思路: 用 buildx --load 强制拉取单平台镜像加载到本地，再用 docker save 导出
export_image() {
    local src_image=$1   # 源镜像，如 postgres:15-alpine
    local tag=$2         # 临时本地 tag，如 export-postgres:15-alpine
    local output=$3      # 输出文件，如 docker-images/postgres-15-alpine.tar.gz
    local label=$4

    echo -e "${BLUE}📥 拉取 ${label} (${src_image}, ${PLATFORM})...${NC}"
    docker buildx build \
        --platform "${PLATFORM}" \
        --load \
        -t "${tag}" \
        --no-cache \
        - <<EOF
FROM ${src_image}
EOF

    # 重新打标签为原始镜像名（确保 docker load 后名称正确）
    docker tag "${tag}" "${src_image}"

    echo -e "  💾 导出压缩中..."
    docker save "${src_image}" | gzip > "${output}"

    # 清理临时 tag
    docker rmi "${tag}" > /dev/null 2>&1 || true

    local size
    size=$(du -sh "${output}" | cut -f1)
    echo -e "${GREEN}  ✅ 完成: ${output} (${size})${NC}"
    echo ""
}

# 导出 PostgreSQL
echo -e "${BLUE}[1/2] 处理 PostgreSQL...${NC}"
export_image \
    "postgres:15-alpine" \
    "export-postgres:15-alpine" \
    "${OUTPUT_DIR}/postgres-15-alpine.tar.gz" \
    "PostgreSQL"

# 导出 Redis
echo -e "${BLUE}[2/2] 处理 Redis...${NC}"
export_image \
    "redis:7-alpine" \
    "export-redis:7-alpine" \
    "${OUTPUT_DIR}/redis-7-alpine.tar.gz" \
    "Redis"

PG_SIZE=$(du -sh "${OUTPUT_DIR}/postgres-15-alpine.tar.gz" | cut -f1)
REDIS_SIZE=$(du -sh "${OUTPUT_DIR}/redis-7-alpine.tar.gz" | cut -f1)

echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}  ✅ 基础设施镜像打包完成！${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "生成文件:"
echo "  📦 ${OUTPUT_DIR}/postgres-15-alpine.tar.gz (${PG_SIZE})"
echo "  📦 ${OUTPUT_DIR}/redis-7-alpine.tar.gz (${REDIS_SIZE})"
echo ""
echo "上传并加载到服务器:"
echo "  scp ${OUTPUT_DIR}/postgres-*.tar.gz ${OUTPUT_DIR}/redis-*.tar.gz root@server:/data/"
echo "  gunzip -c postgres-15-alpine.tar.gz | docker load"
echo "  gunzip -c redis-7-alpine.tar.gz | docker load"
echo ""
echo -e "${BLUE}💡 提示: 此脚本只需首次部署时运行一次${NC}"
echo ""
