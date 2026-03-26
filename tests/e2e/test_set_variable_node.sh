#!/bin/bash

# ================================================================
# 变量设置节点 E2E 测试
# 
# 测试流程:
# start → host_extractor → set_variable (设置 is_low_risk=true) 
#       → cmdb_validator → approval → execution → condition
#           ├─ is_low_risk==true → notification_success → end_success
#           └─ is_low_risk==false → notification_warning → end_warning
# ================================================================

set -e

API_BASE="${API_BASE:-http://localhost:8080}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

TEST_ID="SETVAR-$(date +%Y%m%d%H%M%S)"

echo ""
echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║         变量设置节点 E2E 测试                                          ║"
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

SECRETS_SOURCE_ID=$(curl -s "$API_BASE/api/v1/secrets-sources" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')
echo "  密钥来源ID: $SECRETS_SOURCE_ID"

GIT_REPO_ID=$(curl -s "$API_BASE/api/v1/git-repos" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')
echo "  Git仓库ID: $GIT_REPO_ID"

CHANNEL_ID=$(curl -s "$API_BASE/api/v1/channels" \
  -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id // empty')
echo "  通知渠道ID: $CHANNEL_ID"
echo ""

if [ -z "$SECRETS_SOURCE_ID" ] || [ -z "$GIT_REPO_ID" ]; then
    echo "  ❌ 缺少必要资源，请先运行 test_complete_workflow.sh"
    exit 1
fi

# ============================================================================
# 步骤 3: 创建使用 set_variable + condition 的流程
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 3: 创建使用变量设置节点的流程                                     ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"
echo ""
echo "  流程结构:"
echo "  start → host_extractor → set_variable (is_low_risk=true)"
echo "        → cmdb_validator → approval → execution"
echo "        → condition (判断 is_low_risk)"
echo "            ├─ is_low_risk==true → notification_success → end_success"
echo "            └─ is_low_risk==false → notification_warning → end_warning"
echo ""

FLOW_NAME="变量设置测试-$TEST_ID"
FLOW_RESP=$(curl -s -X POST "$API_BASE/api/v1/healing/flows" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "'"$FLOW_NAME"'",
    "description": "测试变量设置节点功能",
    "is_active": true,
    "nodes": [
      {"id": "start_1", "type": "start", "config": {}},
      {"id": "host_extractor_1", "type": "host_extractor", "config": {"source_field": "raw_data.cmdb_ci", "extract_mode": "split", "split_by": ","}},
      {"id": "set_var_1", "type": "set_variable", "config": {
        "variables": {
          "is_low_risk": false,
          "env": "test",
          "max_retries": 3,
          "custom_message": "何阳，你真TM帅"
        }
      }},
      {"id": "cmdb_validator_1", "type": "cmdb_validator", "config": {"input_key": "hosts", "output_key": "validated_hosts"}},
      {"id": "approval_1", "type": "approval", "config": {"title": "变量设置节点测试审批", "description": "请确认是否执行", "timeout_hours": 1}},
      {"id": "execution_1", "type": "execution", "config": {"git_repo_id": "'$GIT_REPO_ID'", "executor_type": "local", "hosts_key": "validated_hosts", "secrets_source_id": "'$SECRETS_SOURCE_ID'", "extra_vars": {"service_name": "nginx", "service_action": "check"}}},
      {"id": "condition_1", "type": "condition", "config": {
        "conditions": [
          {"expression": "is_low_risk == true", "target": "notification_success"},
          {"expression": "is_low_risk == false", "target": "notification_warning"}
        ],
        "default_target": "notification_success"
      }},
      {"id": "notification_success", "type": "notification", "config": {"channel_ids": ["'$CHANNEL_ID'"], "subject": "✅ 低风险执行成功", "body": "变量 is_low_risk=true，执行成功"}},
      {"id": "notification_warning", "type": "notification", "config": {"channel_ids": ["'$CHANNEL_ID'"], "subject": "⚠️ 高风险执行", "body": "变量 is_low_risk=false，需要额外关注"}},
      {"id": "end_success", "type": "end", "config": {}},
      {"id": "end_warning", "type": "end", "config": {}}
    ],
    "edges": [
      {"source": "start_1", "target": "host_extractor_1"},
      {"source": "host_extractor_1", "target": "set_var_1"},
      {"source": "set_var_1", "target": "cmdb_validator_1"},
      {"source": "cmdb_validator_1", "target": "approval_1"},
      {"source": "approval_1", "target": "execution_1"},
      {"source": "execution_1", "target": "condition_1"},
      {"source": "notification_success", "target": "end_success"},
      {"source": "notification_warning", "target": "end_warning"}
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
echo "  🔷 变量设置节点 set_var_1 配置:"
echo "$FLOW_RESP" | jq '.data.nodes[] | select(.id == "set_var_1") | .config'
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
    \"name\": \"变量设置测试规则-$TEST_ID\",
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
          -d '{"comment": "变量设置节点测试自动批准"}' > /dev/null
        echo "  ✅ 审批通过"
    fi
fi
echo ""

# ============================================================================
# 步骤 7: 等待流程完成
# ============================================================================
echo "┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓"
echo "┃ 步骤 7: 等待流程完成                                                   ┃"
echo "┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛"

for i in {1..15}; do
    sleep 2
    INSTANCE_DETAIL=$(curl -s "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
      -H "Authorization: Bearer $TOKEN")
    
    STATUS=$(echo "$INSTANCE_DETAIL" | jq -r '.data.status')
    CURRENT_NODE=$(echo "$INSTANCE_DETAIL" | jq -r '.data.current_node_id')
    
    echo "  [$i/15] 状态=$STATUS 当前节点=$CURRENT_NODE"
    
    if [ "$STATUS" = "completed" ] || [ "$STATUS" = "failed" ]; then
        break
    fi
done
echo ""

# ============================================================================
# 结果总结
# ============================================================================
echo "╔════════════════════════════════════════════════════════════════════════╗"
echo "║                    变量设置节点测试结果                                ║"
echo "╚════════════════════════════════════════════════════════════════════════╝"
echo ""

FINAL_DETAIL=$(curl -s "$API_BASE/api/v1/healing/instances/$INSTANCE_ID" \
  -H "Authorization: Bearer $TOKEN")

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📋 流程实例详情                                                     │"
echo "└──────────────────────────────────────────────────────────────────────┘"
echo "$FINAL_DETAIL" | jq '.data | {id, status, current_node_id}'
echo ""

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 📊 流程上下文 (context) - 包含 set_variable 设置的变量              │"
echo "└──────────────────────────────────────────────────────────────────────┘"
echo "$FINAL_DETAIL" | jq '.data.context | {is_low_risk, env, max_retries, custom_message}'
echo ""

echo "┌──────────────────────────────────────────────────────────────────────┐"
echo "│ 🔀 条件分支判断                                                     │"
echo "└──────────────────────────────────────────────────────────────────────┘"
IS_LOW_RISK=$(echo "$FINAL_DETAIL" | jq -r '.data.context.is_low_risk // "N/A"')
FINAL_NODE=$(echo "$FINAL_DETAIL" | jq -r '.data.current_node_id')

echo "  变量 is_low_risk: $IS_LOW_RISK"
echo "  最终节点: $FINAL_NODE"
echo ""

if [ "$IS_LOW_RISK" = "true" ]; then
    echo "  📌 条件表达式: is_low_risk == true"
    echo "  ✅ 求值结果: true → 跳转到 notification_success → end_success"
else
    echo "  📌 条件表达式: is_low_risk == false"
    echo "  ⚠️  求值结果: true → 跳转到 notification_warning → end_warning"
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

if [ "$STATUS" = "completed" ] && [ "$FINAL_NODE" = "end_warning" ]; then
    echo "  ✅✅✅ 变量设置节点测试成功！ ✅✅✅"
    echo "  - set_variable 正确设置了 is_low_risk=false"
    echo "  - condition 正确读取并判断自定义变量"
    echo "  - 流程走了 warning 分支"
else
    echo "  ❌ 测试失败"
    echo "  最终状态: $STATUS"
    echo "  最终节点: $FINAL_NODE"
    exit 1
fi
echo ""
