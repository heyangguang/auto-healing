#!/bin/bash
# =============================================================================
# 执行任务 E2E 测试脚本（重构后版本）
# 测试新的任务模板 + 执行记录分离架构
# =============================================================================

set -e

# 配置
BASE_URL="${BASE_URL:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

# 颜色
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 统计
PASSED=0
FAILED=0

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   执行任务 E2E 测试 (重构后版本)${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# ==================== 辅助函数 ====================

test_case() {
    local name="$1"
    local result="$2"
    
    if [ "$result" == "0" ]; then
        echo -e "  ${GREEN}✅ $name${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "  ${RED}❌ $name${NC}"
        FAILED=$((FAILED + 1))
    fi
}

api_call() {
    local method="$1"
    local endpoint="$2"
    local data="$3"
    
    if [ -n "$data" ]; then
        curl -s -X "$method" "$BASE_URL$endpoint" \
            -H "Authorization: Bearer $TOKEN" \
            -H "Content-Type: application/json" \
            -d "$data"
    else
        curl -s -X "$method" "$BASE_URL$endpoint" \
            -H "Authorization: Bearer $TOKEN"
    fi
}

# ==================== 1. 登录 ====================
echo -e "${YELLOW}[1] 登录获取 Token${NC}"

TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" \
    -H "Content-Type: application/json" \
    -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}" \
    | jq -r '.access_token')

if [ -n "$TOKEN" ] && [ "$TOKEN" != "null" ]; then
    test_case "登录成功" "0"
    echo -e "     Token: ${TOKEN:0:20}..."
else
    test_case "登录成功" "1"
    echo -e "${RED}无法登录，请检查服务是否运行${NC}"
    exit 1
fi

# ==================== 2. 获取已激活的仓库 ====================
echo ""
echo -e "${YELLOW}[2] 获取已激活的 Git 仓库${NC}"

REPOS=$(api_call GET "/git-repos?is_active=true")
REPO_ID=$(echo "$REPOS" | jq -r '.data[0].id // empty')
REPO_NAME=$(echo "$REPOS" | jq -r '.data[0].name // empty')

if [ -n "$REPO_ID" ]; then
    test_case "找到已激活的仓库" "0"
    echo -e "     仓库ID: $REPO_ID"
    echo -e "     仓库名称: $REPO_NAME"
else
    test_case "找到已激活的仓库" "1"
    echo -e "${RED}没有找到已激活的仓库，请先激活一个仓库${NC}"
    exit 1
fi

# ==================== 3. 创建任务模板 ====================
echo ""
echo -e "${YELLOW}[3] 创建任务模板${NC}"

TASK_RESPONSE=$(api_call POST "/execution-tasks" "{
    \"name\": \"E2E 测试任务\",
    \"repository_id\": \"$REPO_ID\",
    \"target_hosts\": \"localhost\",
    \"extra_vars\": {
        \"target_host\": \"localhost\"
    },
    \"executor_type\": \"local\"
}")

TASK_ID=$(echo "$TASK_RESPONSE" | jq -r '.id // empty')

if [ -n "$TASK_ID" ]; then
    test_case "创建任务模板" "0"
    echo -e "     任务ID: $TASK_ID"
    echo -e "     任务名称: $(echo "$TASK_RESPONSE" | jq -r '.name')"
else
    test_case "创建任务模板" "1"
    echo -e "     错误: $(echo "$TASK_RESPONSE" | jq -r '.error // "未知错误"')"
    exit 1
fi

# ==================== 4. 获取任务模板列表 ====================
echo ""
echo -e "${YELLOW}[4] 获取任务模板列表${NC}"

TASKS_RESPONSE=$(api_call GET "/execution-tasks")
TASK_COUNT=$(echo "$TASKS_RESPONSE" | jq -r '.total // 0')

if [ "$TASK_COUNT" -gt 0 ]; then
    test_case "获取任务列表" "0"
    echo -e "     总数: $TASK_COUNT"
else
    test_case "获取任务列表" "1"
fi

# ==================== 5. 执行任务 ====================
echo ""
echo -e "${YELLOW}[5] 执行任务（创建执行记录）${NC}"

RUN_RESPONSE=$(api_call POST "/execution-tasks/$TASK_ID/execute" "{
    \"triggered_by\": \"e2e-test\"
}")

RUN_ID=$(echo "$RUN_RESPONSE" | jq -r '.id // empty')
RUN_STATUS=$(echo "$RUN_RESPONSE" | jq -r '.status // empty')
EXIT_CODE=$(echo "$RUN_RESPONSE" | jq -r '.exit_code // "null"')

if [ -n "$RUN_ID" ]; then
    test_case "执行任务" "0"
    echo -e "     执行记录ID: $RUN_ID"
    echo -e "     状态: $RUN_STATUS"
    echo -e "     退出码: $EXIT_CODE"
    
    # 显示统计信息
    STATS=$(echo "$RUN_RESPONSE" | jq '.stats // {}')
    echo -e "     统计: $STATS"
else
    test_case "执行任务" "1"
    echo -e "     错误: $(echo "$RUN_RESPONSE" | jq -r '.error // "未知错误"')"
fi

# ==================== 6. 获取执行历史 ====================
echo ""
echo -e "${YELLOW}[6] 获取任务的执行历史${NC}"

RUNS_RESPONSE=$(api_call GET "/execution-tasks/$TASK_ID/runs")
RUNS_COUNT=$(echo "$RUNS_RESPONSE" | jq -r '.total // 0')

if [ "$RUNS_COUNT" -gt 0 ]; then
    test_case "获取执行历史" "0"
    echo -e "     执行记录数: $RUNS_COUNT"
else
    test_case "获取执行历史" "1"
fi

# ==================== 7. 获取执行记录详情 ====================
echo ""
echo -e "${YELLOW}[7] 获取执行记录详情${NC}"

if [ -n "$RUN_ID" ]; then
    RUN_DETAIL=$(api_call GET "/execution-runs/$RUN_ID")
    RUN_DETAIL_STATUS=$(echo "$RUN_DETAIL" | jq -r '.status // empty')
    
    if [ -n "$RUN_DETAIL_STATUS" ]; then
        test_case "获取执行记录详情" "0"
        echo -e "     状态: $RUN_DETAIL_STATUS"
        echo -e "     触发者: $(echo "$RUN_DETAIL" | jq -r '.triggered_by')"
    else
        test_case "获取执行记录详情" "1"
    fi
else
    test_case "获取执行记录详情" "1"
    echo -e "     (跳过: 没有执行记录ID)"
fi

# ==================== 8. 获取执行日志 ====================
echo ""
echo -e "${YELLOW}[8] 获取执行日志${NC}"

if [ -n "$RUN_ID" ]; then
    LOGS_RESPONSE=$(api_call GET "/execution-runs/$RUN_ID/logs")
    LOG_COUNT=$(echo "$LOGS_RESPONSE" | jq 'length // 0')
    
    if [ "$LOG_COUNT" -gt 0 ]; then
        test_case "获取执行日志" "0"
        echo -e "     日志条数: $LOG_COUNT"
        echo -e "     日志示例:"
        echo "$LOGS_RESPONSE" | jq -r '.[0:3] | .[] | "       [\(.stage)] \(.message)"'
    else
        test_case "获取执行日志" "1"
    fi
else
    test_case "获取执行日志" "1"
fi

# ==================== 9. 再次执行验证多次执行 ====================
echo ""
echo -e "${YELLOW}[9] 再次执行（验证多次执行历史）${NC}"

RUN2_RESPONSE=$(api_call POST "/execution-tasks/$TASK_ID/execute" "{
    \"triggered_by\": \"e2e-test-round2\"
}")

RUN2_ID=$(echo "$RUN2_RESPONSE" | jq -r '.id // empty')

if [ -n "$RUN2_ID" ] && [ "$RUN2_ID" != "$RUN_ID" ]; then
    test_case "二次执行生成新记录" "0"
    echo -e "     新执行记录ID: $RUN2_ID"
    
    # 验证执行历史数量增加
    RUNS2_RESPONSE=$(api_call GET "/execution-tasks/$TASK_ID/runs")
    RUNS2_COUNT=$(echo "$RUNS2_RESPONSE" | jq -r '.total // 0')
    echo -e "     执行历史总数: $RUNS2_COUNT"
else
    test_case "二次执行生成新记录" "1"
fi

# ==================== 10. 删除任务模板（级联删除） ====================
echo ""
echo -e "${YELLOW}[10] 删除任务模板（级联删除执行记录和日志）${NC}"

DELETE_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "$BASE_URL/execution-tasks/$TASK_ID" \
    -H "Authorization: Bearer $TOKEN")

if [ "$DELETE_RESPONSE" == "204" ]; then
    test_case "删除任务模板" "0"
    
    # 验证执行记录也被删除
    RUN_CHECK=$(api_call GET "/execution-runs/$RUN_ID")
    RUN_CHECK_ERROR=$(echo "$RUN_CHECK" | jq -r '.error // empty')
    
    if [ -n "$RUN_CHECK_ERROR" ]; then
        test_case "级联删除执行记录" "0"
    else
        test_case "级联删除执行记录" "1"
    fi
else
    test_case "删除任务模板" "1"
    echo -e "     HTTP 状态码: $DELETE_RESPONSE"
fi

# ==================== 结果汇总 ====================
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}   测试结果汇总${NC}"
echo -e "${BLUE}========================================${NC}"
echo -e "  ${GREEN}通过: $PASSED${NC}"
echo -e "  ${RED}失败: $FAILED${NC}"
echo ""

if [ "$FAILED" -eq 0 ]; then
    echo -e "${GREEN}🎉 所有测试通过！${NC}"
    exit 0
else
    echo -e "${RED}❌ 有 $FAILED 个测试失败${NC}"
    exit 1
fi
