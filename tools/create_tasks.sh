#!/bin/bash

TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d '{"username":"admin","password":"admin123456"}' | jq -r '.access_token')

# 5个可用的 Playbook ID
PLAYBOOKS=("c0809626-f4b7-4fd3-9767-fa3fc536edfa" "584f37f8-6d5a-416c-acd1-79aa995d8481" "6d6ad136-1ca5-4f8c-9b96-869ca291f967" "7d00236b-165e-4a64-a4d2-8b3dd6965a02" "d3e338ad-71c1-4df3-ad87-4bbf907b6606")

# 执行器类型
EXECUTORS=("local" "docker")

# 任务类别
CATEGORIES=("日志清理" "服务重启" "配置更新" "健康检查" "备份任务" "监控告警" "安全扫描" "性能优化" "数据同步" "系统维护")

# 环境
ENVS=("生产" "测试" "开发" "预发布" "灾备")

echo "开始创建 100 个任务模板..."

for i in $(seq 1 100); do
  # 随机选择
  playbook_idx=$((RANDOM % 5))
  executor_idx=$((RANDOM % 2))
  category_idx=$((RANDOM % 10))
  env_idx=$((RANDOM % 5))
  
  playbook_id="${PLAYBOOKS[$playbook_idx]}"
  executor="${EXECUTORS[$executor_idx]}"
  category="${CATEGORIES[$category_idx]}"
  env="${ENVS[$env_idx]}"
  
  name="${env}环境-${category}-任务${i}"
  desc="这是第 ${i} 个测试任务模板，用于 ${env} 环境的 ${category} 操作"
  host="192.168.$((RANDOM % 256)).$((RANDOM % 256))"
  
  curl -s -X POST "http://localhost:8080/api/v1/execution-tasks" \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"${name}\",
      \"playbook_id\": \"${playbook_id}\",
      \"target_hosts\": \"${host}\",
      \"executor_type\": \"${executor}\",
      \"description\": \"${desc}\"
    }" > /dev/null
  
  if [ $((i % 10)) -eq 0 ]; then
    echo "已创建 ${i} 个..."
  fi
done

echo "创建完成！"
