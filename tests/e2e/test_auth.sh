#!/bin/bash
# 端到端测试 - 认证模块
# 测试: 登录、获取当前用户、刷新 Token

set -e

API_BASE="${API_BASE:-http://localhost:8080/api/v1}"
USERNAME="${USERNAME:-admin}"
PASSWORD="${PASSWORD:-admin123456}"

echo "=========================================="
echo "  认证模块端到端测试"
echo "=========================================="

# 1. 登录
echo ""
echo "--- 1. 登录 ---"
LOGIN_RESULT=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")

ACCESS_TOKEN=$(echo "$LOGIN_RESULT" | jq -r '.access_token')
REFRESH_TOKEN=$(echo "$LOGIN_RESULT" | jq -r '.refresh_token')

if [ "$ACCESS_TOKEN" == "null" ] || [ -z "$ACCESS_TOKEN" ]; then
  echo "❌ 登录失败: $LOGIN_RESULT"
  exit 1
fi
echo "✅ 登录成功"

# 2. 获取当前用户
echo ""
echo "--- 2. 获取当前用户 ---"
ME_RESULT=$(curl -s "$API_BASE/auth/me" -H "Authorization: Bearer $ACCESS_TOKEN")
ME_USERNAME=$(echo "$ME_RESULT" | jq -r '.username')

if [ "$ME_USERNAME" != "$USERNAME" ]; then
  echo "❌ 获取用户失败: $ME_RESULT"
  exit 1
fi
echo "✅ 获取用户成功: $ME_USERNAME"

# 3. 刷新 Token
echo ""
echo "--- 3. 刷新 Token ---"
REFRESH_RESULT=$(curl -s -X POST "$API_BASE/auth/refresh" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")

NEW_TOKEN=$(echo "$REFRESH_RESULT" | jq -r '.access_token')
if [ "$NEW_TOKEN" == "null" ] || [ -z "$NEW_TOKEN" ]; then
  echo "❌ 刷新 Token 失败: $REFRESH_RESULT"
  exit 1
fi
echo "✅ 刷新 Token 成功"

echo ""
echo "=========================================="
echo "  认证模块测试通过 ✅"
echo "=========================================="
