#!/bin/bash

# ================================================================
# 条件判断节点 E2E 测试
# 
# 测试流程:
# start → host_extractor → cmdb_validator → approval → execution 
#       → condition (判断 exit_code) 
#           ├─ ==0 → notification_success → end
#           └─ !=0 → notification_failure → end
# ================================================================

set -e

API_BASE="${API_BASE:-http://localhost:8080}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

TEST_ID="COND-$(date +%Y%m%d%H%M%S)"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║         条件判断节点 E2E 测试                                          ║"
echo "╠════════════════════════════════════════════════════════════════════════╣"
echo "║  测试ID: $TEST_ID"
echo "║  API: $API_BASE"
echo "╚════════════════════════════════════════════════════════════════════════╝"
echo ""

# ============================================================================
# 步骤 1: 登录
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 1: 登录系统                                                       ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

LOGIN_RESP=$(curl -s -X POST "$API_BASE/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$USERNAME\", \"password\": \"$PASSWORD\"}")

TOKEN=$(echo "$LOGIN_RESP" | jq -r '.access_token')
if [ -z "$TOKEN" ] || [ "$TOKEN" = "null" ]; then
    echo "  ❌ 登录失败"
    exit 1
fi
echo "  ✅ 登录成功"
echo ""

# ============================================================================
# 步骤 2: 获取已有资源
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 2: 获取已有资源                                                   ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

# 获取密钥来源
SECRETS_SOURCE_ID=$(curl -s "$API_BASE/api/v1/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')
echo "  密钥来源ID: $SECRETS_SOURCE_ID"

# 获取 Git 仓库
GIT_REPO_ID=$(curl -s "$API_BASE/api/v1/git-repos" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')
echo "  Git仓库ID: $GIT_REPO_ID"

# 获取通知渠道
CHANNEL_ID=$(curl -s "$API_BASE/api/v1/channels" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')
echo "  通知渠道ID: $CHANNEL_ID"
echo ""

if [ -z "$SECRETS_SOURCE_ID" ] || [ -z "$GIT_REPO_ID" ]; then
    echo "  ❌ 缺少必要资源，请先运行 test_complete_workflow.sh"
    exit 1
fi

# ============================================================================
# 步骤 3: 创建包含条件节点的流程
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 3: 创建包含条件判断节点的流程                                     ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  流程结构:"
echo "  start → host_extractor → cmdb_validator → approval → execution"
echo "        → condition (判断 exit_code)"
echo "            ├─ exit_code==0 → notification_success → end_success"
echo "            └─ exit_code!=0 → notification_failure → end_failure"
echo ""

FLOW_NAME="条件分支测试-$TEST_ID"
FLOW_RESP=$(curl -s -X POST "$API_BASE/api/v1/healing/flows" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "'"$FLOW_NAME"'",
    "description": "测试条件判断节点的分支功能",
    "is_active": true,
    "nodes": [
      {"id": "start_1", "type": "start", "config": {}},
      {"id": "host_extractor_1", "type": "host_extractor", "config": {"source_field": "raw_data.cmdb_ci", "extract_mode": "split", "split_by": ","}},
      {"id": "cmdb_validator_1", "type": "cmdb_validator", "config": {"input_key": "hosts", "output_key": "validated_hosts"}},
      {"id": "approval_1", "type": "approval", "config": {"title": "条件分支测试审批", "description": "请确认是否执行", "timeout_hours": 1}},
      {"id": "execution_1", "type": "execution", "config": {"git_repo_id": "'$GIT_REPO_ID'", "executor_type": "local", "hosts_key": "validated_hosts", "secrets_source_id": "'$SECRETS_SOURCE_ID'", "extra_vars": {"service_name": "nginx", "service_action": "check"}}},
      {"id": "condition_1", "type": "condition", "config": {
        "conditions": [
          {"expression": "execution_result.exit_code == 0", "target": "notification_success"},
          {"expression": "execution_result.exit_code != 0", "target": "notification_failure"}
        ],
        "default_target": "notification_success"
      }},
      {"id": "notification_success", "type": "notification", "config": {"channel_ids": ["'$CHANNEL_ID'"], "subject": "✅ 自愈成功", "body": "执行成功，exit_code=0"}},
      {"id": "notification_failure", "type": "notification", "config": {"channel_ids": ["'$CHANNEL_ID'"], "subject": "❌ 自愈失败", "body": "执行失败，需要人工介入"}},
      {"id": "end_success", "type": "end", "config": {}},
      {"id": "end_failure", "type": "end", "config": {}}
    ],
    "edges": [
      {"source": "start_1", "target": "host_extractor_1"},
      {"source": "host_extractor_1", "target": "cmdb_validator_1"},
      {"source": "cmdb_validator_1", "target": "approval_1"},
      {"source": "approval_1", "target": "execution_1"},
      {"source": "execution_1", "target": "condition_1"},
      {"source": "notification_success", "target": "end_success"},
      {"source": "notification_failure", "target": "end_failure"}
    ]
  }')

echo "  创建响应:"
echo "$FLOW_RESP" | jq '{code, message, flow_id: .data.id, nodes_count: (.data.nodes | length)}'
echo ""

FLOW_ID=$(echo "$FLOW_RESP" | jq -r '.data.id')
if [ -z "$FLOW_ID" ] || [ "$FLOW_ID" = "null" ]; then
    echo "  ❌ 创建流程失败"
    exit 1
fi
echo "  ✅ 创建成功，流程ID: $FLOW_ID"
echo ""

# 显示详细的流程配置
echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📊 流程节点配置                                                     │"
echo "└──────────────────────────────────────────────────────────────────────┘"
echo ""
echo "  🔷 节点列表:"
echo "$FLOW_RESP" | jq -r '.data.nodes[] | "    [\(.type)] \(.id)"'
echo ""
echo "  🔷 边 (Edges):"
echo "$FLOW_RESP" | jq -r '.data.edges[] | "    \(.source) → \(.target)"'
echo ""
echo "  🔷 条件节点 condition_1 配置:"
echo "$FLOW_RESP" | jq '.data.nodes[] | select(.id == "condition_1") | .config'
echo ""

# ============================================================================
# 步骤 4: 禁用冲突规则并创建匹配规则
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 4: 禁用冲突规则并创建匹配规则                                     ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

# 禁用所有已有的活跃规则，避免冲突
echo "  禁用其他冲突规则..."
ACTIVE_RULE_IDS=$(curl -s "$API_BASE/api/v1/healing/rules?is_active=true" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[].id')
for RID in $ACTIVE_RULE_IDS; do
    curl -s -X POST "$API_BASE/api/v1/healing/rules/$RID/deactivate" \
      -H "Authorization: Bearer $TOKEN" > /dev/null
done

RULE_RESP=$(curl -s -X POST "$API_BASE/api/v1/healing/rules" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{
    \"name\": \"条件测试规则-$TEST_ID\",
    \"priority\": 100,
    \"trigger_mode\": \"auto\",
    \"match_mode\": \"all\",
    \"conditions\": [{\"field\": \"title\", \"operator\": \"contains\", \"value\": \"E2E-HEALING\"}],
    \"flow_id\": $FLOW_ID,
    \"is_active\": true
  }")

RULE_ID=$(echo "$RULE_RESP" | jq -r '.data.id')
echo "  ✅ 规则ID: $RULE_ID"
echo ""

# ============================================================================
# 步骤 5: 触发 ITSM 同步
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 5: 触发 ITSM 同步                                                 ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

# 获取已存在的 ITSM 插件
ITSM_PLUGIN_ID=$(curl -s "$API_BASE/api/v1/plugins?type=itsm" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')

if [ -z "$ITSM_PLUGIN_ID" ]; then
    echo "  ❌ 没有找到 ITSM 插件"
    exit 1
fi

echo "  ITSM插件ID: $ITSM_PLUGIN_ID"
echo "  触发同步..."

SYNC_RESP=$(curl -s -X POST "$API_BASE/api/v1/plugins/$ITSM_PLUGIN_ID/sync" \
  -H "Authorization: Bearer $TOKEN")
echo "  同步响应: $(echo "$SYNC_RESP" | jq -r '.message // .code')"

# 等待同步
for i in {1..6}; do
    sleep 2
    SYNC_STATUS=$(curl -s "$API_BASE/api/v1/plugins/$ITSM_PLUGIN_ID/logs?page=1&page_size=1" \
      -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].status')
    echo "    [$i/6] 同步状态: $SYNC_STATUS"
    if [ "$SYNC_STATUS" = "success" ]; then
        break
    fi
done
echo ""

# ============================================================================
# 步骤 6: 等待规则引擎触发并审批
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 6: 等待规则引擎触发并审批                                         ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

INSTANCE_ID=""
for i in {1..12}; do
    sleep 3
    INSTANCES=$(curl -s "$API_BASE/api/v1/healing/instances?flow_id=$FLOW_ID&page=1&page_size=1" \
      -H "Authorization: Bearer $TOKEN")
    
    INSTANCE_ID=$(echo "$INSTANCES" | jq -r '.data[0].id // empty')
    STATUS=$(echo "$INSTANCES" | jq -r '.data[0].status // empty')
    
    if [ -n "$INSTANCE_ID" ]; then
        echo "  [$i/12] 实例ID=$INSTANCE_ID 状态=$STATUS"
        if [ "$STATUS" = "waiting_approval" ]; then
            echo "  ✅ 流程进入审批等待状态"
            break
        fi
    else
        echo "  [$i/12] 等待实例创建..."
    fi
done

if [ -z "$INSTANCE_ID" ]; then
    echo "  ❌ 未检测到流程实例"
    exit 1
fi
echo ""

# 自动审批
if [ "$STATUS" = "waiting_approval" ]; then
    echo "  自动审批..."
    APPROVAL_TASK_ID=$(curl -s "$API_BASE/api/v1/healing/approvals/pending" \
      -H "Authorization: Bearer $TOKEN" | jq -r '.data[] | select(.flow_instance_id == '$INSTANCE_ID') | .id' | head -1)
    
    if [ -n "$APPROVAL_TASK_ID" ]; then
        curl -s -X POST "$API_BASE/api/v1/healing/approvals/$APPROVAL_TASK_ID/approve" \
          -H "Content-Type: application/json" \
          -H "Authorization: Bearer $TOKEN" \
          -d '{"comment": "条件节点测试自动批准"}' > /dev/null
        echo "  ✅ 审批通过"
    fi
fi
echo ""

# ============================================================================
# 步骤 7: 等待流程完成并验证条件分支
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 7: 等待流程完成并验证条件分支                                     ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

for i in {1..15}; do
    sleep 2
    INSTANCE_DETAIL=$(curl -s "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
      -H "Authorization: Bearer $TOKEN")
    
    STATUS=$(echo "$INSTANCE_DETAIL" | jq -r '.data.status')
    CURRENT_NODE=$(echo "$INSTANCE_DETAIL" | jq -r '.data.current_node_id')
    NODE_STATES=$(echo "$INSTANCE_DETAIL" | jq -r '.data.node_states | keys | join(", ") // "无"')
    
    echo "  [$i/15] 状态=$STATUS 当前节点=$CURRENT_NODE"
    echo "         已执行节点: $NODE_STATES"
    
    if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
        break
    fi
done
echo ""

# ============================================================================
# 结果总结
# ============================================================================
echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║                    条件判断节点测试结果                                ║"
echo "╚════════════════════════════════════════════════════════════════════════╝"
echo ""

FINAL_DETAIL=$(curl -s "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
  -H "Authorization: Bearer $TOKEN")

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📋 流程实例详情                                                     │"
echo "└──────────────────────────────────────────────────────────────────────┘"
echo "$FINAL_DETAIL" | jq '.data | {id, status, current_node_id, created_at, completed_at}'
echo ""

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📊 节点状态 (node_states)                                           │"
echo "└──────────────────────────────────────────────────────────────────────┘"
echo "$FINAL_DETAIL" | jq '.data.node_states'
echo ""

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 🔍 Ansible 执行结果                                                 │"
echo "└──────────────────────────────────────────────────────────────────────┘"
echo "$FINAL_DETAIL" | jq '.data.node_states.execution_1 | {exit_code, success, changed, failed, unreachable}'
echo ""

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 🔀 条件分支判断                                                     │"
echo "└──────────────────────────────────────────────────────────────────────┘"
EXIT_CODE=$(echo "$FINAL_DETAIL" | jq -r '.data.node_states.execution_1.exit_code // "N/A"')
FINAL_NODE=$(echo "$FINAL_DETAIL" | jq -r '.data.current_node_id')

echo "  执行结果 exit_code: $EXIT_CODE"
echo "  最终节点: $FINAL_NODE"
echo ""

if [ "$EXIT_CODE" = "0" ]; then
    echo "  📌 条件表达式: execution_result.exit_code == 0"
    echo "  ✅ 求值结果: true → 跳转到 notification_success → end_success"
else
    echo "  📌 条件表达式: execution_result.exit_code != 0"
    echo "  ⚠️  求值结果: true → 跳转到 notification_failure → end_failure"
fi
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  测试ID:  $TEST_ID"
echo "  流程ID:  $FLOW_ID"
echo "  规则ID:  $RULE_ID"
echo "  实例ID:  $INSTANCE_ID"
echo "  最终状态: $STATUS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "$STATUS" = "completed" ]; then
    echo "  ✅✅✅ 条件判断节点测试成功！ ✅✅✅"
else
    echo "  ❌ 测试失败，最终状态: $STATUS"
    exit 1
fi
echo ""
