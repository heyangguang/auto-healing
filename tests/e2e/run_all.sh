#!/bin/bash
# 运行所有端到端测试
# 用法: ./run_all.sh

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FAILED=0
PASSED=0

echo "=========================================="
echo "     Auto-Healing 端到端测试套件"
echo "=========================================="
echo ""

# 检查依赖
command -v curl >/dev/null 2>&1 || { echo "需要 curl"; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "需要 jq"; exit 1; }

# 定义测试列表
TESTS=(
  "test_auth.sh:认证模块"
  "test_plugins.sh:插件模块"
  "test_incident_close.sh:工单关闭"
  "test_secrets.sh:密钥管理"
  "test_git_repos.sh:Git仓库"
)

# 运行测试
for TEST in "${TESTS[@]}"; do
  SCRIPT="${TEST%%:*}"
  NAME="${TEST##*:}"
  
  echo ""
  echo "▶ 运行: $NAME ($SCRIPT)"
  echo "----------------------------------------"
  
  if bash "$SCRIPT_DIR/$SCRIPT"; then
    PASSED=$((PASSED + 1))
    echo "✅ $NAME 通过"
  else
    FAILED=$((FAILED + 1))
    echo "❌ $NAME 失败"
  fi
done

# 汇总
echo ""
echo "=========================================="
echo "     测试结果汇总"
echo "=========================================="
echo "  通过: $PASSED"
echo "  失败: $FAILED"
echo "  总计: $((PASSED + FAILED))"
echo "=========================================="

if [ $FAILED -gt 0 ]; then
  exit 1
fi
