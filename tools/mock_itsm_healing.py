#!/usr/bin/env python3
"""
Mock ITSM Service for Auto-Healing E2E Testing
模拟 ServiceNow 等 ITSM 系统，返回可触发自愈流程的工单

端口: 5000

特性:
- 每次请求返回带有动态时间戳的工单 ID (避免重复)
- 工单标题包含 "E2E-HEALING" 以匹配规则
- affected_ci 指向真实测试主机: 192.168.31.103 / healing-test-host
"""

from flask import Flask, jsonify, request
from datetime import datetime
import json
import time

app = Flask(__name__)

def get_mock_incidents():
    """返回带有动态 ID 的工单列表"""
    timestamp = int(time.time())
    return [
        {
            "sys_id": f"E2E-INC-{timestamp}-001",
            "number": f"E2E-INC-{timestamp}-001",
            "short_description": "E2E-HEALING 磁盘空间不足告警",
            "description": "服务器磁盘使用率超过90%，需要自动清理。测试主机: 192.168.31.103",
            "priority": "1",
            "urgency": "1",
            "state": "1",  # 1=New
            "category": "hardware",
            "cmdb_ci": "healing-test-host",  # 使用 CMDB 中的主机名
            "business_service": "auto-healing-service",
            "assignment_group": "auto-healing",
            "assigned_to": "张三",
            "opened_by": "监控系统",
            "opened_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "sys_updated_on": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        },
        {
            "sys_id": f"E2E-INC-{timestamp}-002", 
            "number": f"E2E-INC-{timestamp}-002",
            "short_description": "E2E-HEALING 服务重启请求",
            "description": "应用服务需要重启",
            "priority": "2",
            "urgency": "2",
            "state": "1",
            "category": "software",
            "cmdb_ci": "192.168.31.103",  # 直接使用 IP
            "business_service": "web-service",
            "assignment_group": "auto-healing",
            "assigned_to": "李四",
            "opened_by": "运维人员",
            "opened_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "sys_updated_on": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        },
        {
            "sys_id": f"E2E-INC-{timestamp}-003", 
            "number": f"E2E-INC-{timestamp}-003",
            "short_description": "E2E-HEALING-FAIL 四主机混合认证测试",
            "description": "测试4台主机混合认证：密码+密钥+失败",
            "priority": "3",
            "urgency": "3",
            "state": "1",
            "category": "network",
            "cmdb_ci": "192.168.31.103,192.168.31.100,192.168.31.101,192.168.31.254",  # 4台: 密码+2密钥+失败
            "business_service": "network-service",
            "assignment_group": "auto-healing",
            "assigned_to": "王五",
            "opened_by": "告警系统",
            "opened_at": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "sys_updated_on": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
        },
    ]

@app.route('/api/now/table/incident', methods=['GET'])
def list_incidents():
    """获取工单列表 - ServiceNow 格式"""
    incidents = get_mock_incidents()
    print(f"[Mock ITSM] GET /api/now/table/incident")
    print(f"[Mock ITSM] 返回 {len(incidents)} 个工单 (动态ID)")
    for inc in incidents:
        print(f"  - {inc['number']}: {inc['short_description']} (cmdb_ci: {inc['cmdb_ci']})")
    return jsonify({"result": incidents})

@app.route('/api/now/table/incident/<sys_id>', methods=['GET'])
def get_incident(sys_id):
    """获取单个工单"""
    print(f"[Mock ITSM] GET /api/now/table/incident/{sys_id}")
    incidents = get_mock_incidents()
    for inc in incidents:
        if inc['sys_id'] == sys_id:
            return jsonify({"result": inc})
    return jsonify({"result": None}), 404

@app.route('/api/now/table/incident', methods=['POST'])
def create_incident():
    """创建工单"""
    data = request.get_json() or {}
    print(f"[Mock ITSM] POST /api/now/table/incident")
    print(f"[Mock ITSM] 请求数据: {json.dumps(data, ensure_ascii=False)}")
    new_id = f"E2E-INC-{int(time.time())}-NEW"
    return jsonify({"result": {"sys_id": new_id}})

@app.route('/api/now/table/incident/<sys_id>', methods=['PATCH', 'PUT'])
def update_incident(sys_id):
    """更新工单"""
    data = request.get_json() or {}
    print(f"[Mock ITSM] PATCH /api/now/table/incident/{sys_id}")
    print(f"[Mock ITSM] 更新数据: {json.dumps(data, ensure_ascii=False)}")
    return jsonify({"result": {"sys_id": sys_id, "state": data.get("state", "1")}})

# 连接测试端点
@app.route('/api/now/table/incident', methods=['HEAD'])
@app.route('/', methods=['GET'])
def health():
    """健康检查"""
    return jsonify({"status": "ok"})

if __name__ == '__main__':
    print("=" * 70)
    print("Mock ITSM Service for Auto-Healing E2E Testing")
    print("=" * 70)
    print(f"端口: 5000")
    print("")
    print("工单特性:")
    print("  - 动态 ID: 每次请求生成唯一 ID (基于时间戳)")
    print("  - 标题包含 'E2E-HEALING' 用于规则匹配")
    print("  - cmdb_ci: healing-test-host / 192.168.31.103")
    print("")
    print("对应资源:")
    print("  - Mock CMDB (5001): healing-test-host -> 192.168.31.103")
    print("  - Mock Secrets (5002): root / 123")
    print("")
    print("启动中...")
    print("=" * 70)
    app.run(host='0.0.0.0', port=5000, debug=False)
