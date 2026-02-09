#!/bin/bash
# 异步执行 + SSE 实时日志流测试
set -e
URL="http://localhost:8080/api/v1"

echo "🔐 登录..."
TOKEN=$(curl -s -X POST "$URL/auth/login" -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')
echo "✅ 登录成功"

# 获取任务ID
TASK_ID=$(curl -s "$URL/execution-tasks" -H "Authorization: Bearer $TOKEN" | jq -r '.data[0].id')
TASK_NAME=$(curl -s "$URL/execution-tasks/$TASK_ID" -H "Authorization: Bearer $TOKEN" | jq -r '.data.name')
echo "📋 任务: $TASK_NAME ($TASK_ID)"

# 异步执行
echo ""
echo "🚀 发起异步执行..."
START=$(date +%s%3N)
RESULT=$(curl -s -X POST "$URL/execution-tasks/$TASK_ID/execute" -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" -d '{"triggered_by":"async-test"}')
END=$(date +%s%3N)
ELAPSED=$((END - START))

RUN_ID=$(echo $RESULT | jq -r '.data.id')
STATUS=$(echo $RESULT | jq -r '.data.status')
echo "⏱️  响应时间: ${ELAPSED}ms (异步应该 <100ms)"
echo "📝 执行记录: $RUN_ID"
echo "📊 初始状态: $STATUS"

echo ""
echo "📡 SSE 实时日志流..."
echo "   (按 Ctrl+C 取消)"
echo "================================================"

# SSE 实时日志
curl -N -s "$URL/execution-runs/$RUN_ID/stream" -H "Authorization: Bearer $TOKEN" | while read -r line; do
    if [[ $line == event:* ]]; then
        EVENT="${line#event:}"
    elif [[ $line == data:* ]]; then
        DATA="${line#data:}"
        if [[ $EVENT == "log" ]]; then
            STAGE=$(echo "$DATA" | jq -r '.stage')
            MSG=$(echo "$DATA" | jq -r '.message')
            LEVEL=$(echo "$DATA" | jq -r '.log_level')
            echo "[$LEVEL] [$STAGE] $MSG"
            
            # 如果是 output 阶段，显示详细的 ansible 输出
            if [[ $STAGE == "output" ]]; then
                STDOUT=$(echo "$DATA" | jq -r '.details.stdout // empty')
                if [[ -n "$STDOUT" ]]; then
                    echo "--- Ansible Output ---"
                    echo "$STDOUT"
                    echo "----------------------"
                fi
            fi
        elif [[ $EVENT == "done" ]]; then
            FINAL_STATUS=$(echo "$DATA" | jq -r '.status')
            EXIT_CODE=$(echo "$DATA" | jq -r '.exit_code')
            echo "================================================"
            echo "🏁 执行完成！状态: $FINAL_STATUS | 退出码: $EXIT_CODE"
            break
        fi
    fi
done

echo ""
echo "🎉 测试完成！"
