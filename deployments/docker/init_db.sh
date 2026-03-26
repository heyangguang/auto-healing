#!/bin/bash
# Auto-Healing 数据库初始化脚本
#
# 用法:
#   ./init_db.sh          # 增量：只跑新增的 SQL migrations
#   ./init_db.sh --reset  # 全量重建（清库 → 启动 server → AutoMigrate → 初始化管理员）

set -euo pipefail

export DOCKER_HOST=unix:///run/podman/podman.sock

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

RESET_MODE=false
[ "${1:-}" = "--reset" ] && RESET_MODE=true

GREEN='\033[0;32m'; BLUE='\033[0;34m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
DB_NAME="auto_healing"
DB_USER="postgres"
POSTGRES_CONTAINER="auto-healing-postgres"
MIGRATION_TABLE="schema_migration_history"
MIGRATIONS_DIR="/data/migrations"

run_psql() {
    docker exec -i "$POSTGRES_CONTAINER" psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" "$@"
}

run_psql_file() {
    local file_path="$1"
    run_psql < "$file_path"
}

ensure_migration_history_table() {
    run_psql <<EOF
CREATE TABLE IF NOT EXISTS public.${MIGRATION_TABLE} (
    file_name TEXT PRIMARY KEY,
    checksum TEXT NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
EOF
}

is_migration_applied() {
    local file_name="$1"
    run_psql -tA -c "SELECT 1 FROM public.${MIGRATION_TABLE} WHERE file_name = '$file_name' LIMIT 1;" | grep -q '^1$'
}

record_migration() {
    local file_name="$1"
    local checksum="$2"
    run_psql -c "INSERT INTO public.${MIGRATION_TABLE} (file_name, checksum) VALUES ('$file_name', '$checksum');" >/dev/null
}

echo ""
echo -e "${BLUE}========================================${NC}"
[ "$RESET_MODE" = true ] && echo -e "${BLUE}  全量重建（--reset）${NC}" || echo -e "${BLUE}  增量迁移${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# ── 确保 postgres 运行 ─────────────────────────────
if ! docker ps --format '{{.Names}}' | grep -q "^${POSTGRES_CONTAINER}$"; then
    echo -e "${RED}❌ postgres 容器未运行，请先执行 ./start.sh${NC}"
    exit 1
fi

# ── 等待 postgres 健康 ─────────────────────────────
echo -e "${BLUE}⏳ 等待 PostgreSQL 就绪...${NC}"
for i in $(seq 1 30); do
    docker exec "$POSTGRES_CONTAINER" pg_isready -U "$DB_USER" -d "$DB_NAME" > /dev/null 2>&1 && break
    sleep 1
    [ $i -eq 30 ] && echo -e "${RED}❌ 超时${NC}" && exit 1
done
echo -e "${GREEN}✅ PostgreSQL 就绪${NC}"
echo ""

# ================================================================
# --reset 模式：清库 → 启动 server（让 AutoMigrate 建表） → init-admin
# ================================================================
if [ "$RESET_MODE" = true ]; then
    echo -e "${BLUE}⚠️  清空数据库...${NC}"
    run_psql <<'EOF'
DROP SCHEMA public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO postgres;
EOF
    echo -e "${GREEN}✅ 数据库已清空${NC}"
    echo ""

    # 启动 server，AutoMigrate 会建所有表 + 种子数据
    if [ ! -f /data/server ]; then
        echo -e "${RED}❌ /data/server 不存在，请先上传后端二进制${NC}"
        exit 1
    fi

    echo -e "${BLUE}🔄 启动后端（触发 AutoMigrate + 种子数据）...${NC}"
    pkill -f '/data/server' 2>/dev/null || true
    sleep 1
    chmod +x /data/server /data/init-admin 2>/dev/null || true
    mkdir -p /data/logs /data/workspace
    APP_CONFIG=/data/deployments/docker/config.yaml \
    nohup /data/server > /data/logs/server.log 2>&1 &

    echo -e "${BLUE}⏳ 等待后端就绪（最多 60s）...${NC}"
    for i in $(seq 1 30); do
        if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
            echo -e "${GREEN}✅ 后端已就绪${NC}"
            break
        fi
        sleep 2
        [ $i -eq 30 ] && echo -e "${RED}❌ 后端启动失败，查看 /data/logs/server.log${NC}" && exit 1
    done
    echo ""

    echo -e "${BLUE}👤 初始化平台管理员...${NC}"
    /data/init-admin
    echo ""

    echo -e "${BLUE}========================================${NC}"
    echo -e "${GREEN}  ✅ 初始化完成！${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    echo "账号: admin / admin123456"
    echo ""
    exit 0
fi

# ================================================================
# 增量模式：只跑新增的 SQL migrations（用于生产升级）
# ================================================================
if [ ! -d "$MIGRATIONS_DIR" ]; then
    echo -e "${RED}❌ 迁移目录 $MIGRATIONS_DIR 不存在${NC}"
    exit 1
fi

migration_files=("$MIGRATIONS_DIR"/*.up.sql)
if [ "${migration_files[0]}" = "$MIGRATIONS_DIR/*.up.sql" ]; then
    echo -e "${RED}❌ 未找到任何 .up.sql 迁移文件${NC}"
    exit 1
fi

ensure_migration_history_table

echo -e "${BLUE}📦 执行数据库迁移...${NC}"
COUNT=0
APPLIED=0
SKIPPED=0
for f in "${migration_files[@]}"; do
    file_name="$(basename "$f")"
    checksum="$(sha256sum "$f" | awk '{print $1}')"
    COUNT=$((COUNT + 1))
    printf "  %-55s" "$file_name"

    if is_migration_applied "$file_name"; then
        SKIPPED=$((SKIPPED + 1))
        echo -e "${YELLOW}SKIP${NC}"
        continue
    fi

    run_psql_file "$f" >/dev/null
    record_migration "$file_name" "$checksum"
    APPLIED=$((APPLIED + 1))
    echo -e "${GREEN}OK${NC}"
done
echo -e "${GREEN}✅ 迁移完成（总计 $COUNT，新增 $APPLIED，跳过 $SKIPPED）${NC}"
echo ""
