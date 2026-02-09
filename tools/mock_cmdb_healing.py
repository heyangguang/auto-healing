#!/usr/bin/env python3
"""
模拟 CMDB 服务 - 为自愈引擎执行节点测试提供主机信息
端口: 5001
"""

from flask import Flask, jsonify, request
import datetime

app = Flask(__name__)

# 模拟 CMDB 主机数据 - 手动定义的5个主机
CMDB_HOSTS = {
    "healing-test-host": {
        "name": "healing-test-host",
        "ip_address": "192.168.31.103",
        "sys_id": "host-001",
        "sys_class_name": "cmdb_ci_linux_server",
        "os": "Linux",
        "os_version": "CentOS 8",
        "cpu": "4 vCPU",
        "memory": "8GB",
        "disk": "100GB SSD",
        "status": "active",
        "environment": "production",
        "location": "北京数据中心",
        "owner": "ops-team",
        "department": "运维部",
        "category": "server",
        "description": "自愈测试主机",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": datetime.datetime.now().isoformat()
    },
    "web-server-01": {
        "name": "web-server-01",
        "ip_address": "192.168.31.103",
        "sys_id": "host-002",
        "sys_class_name": "cmdb_ci_linux_server",
        "os": "Linux",
        "os_version": "Ubuntu 22.04",
        "cpu": "8 vCPU",
        "memory": "16GB",
        "disk": "200GB SSD",
        "status": "active",
        "environment": "production",
        "location": "北京数据中心",
        "owner": "web-team",
        "department": "研发部",
        "category": "server",
        "description": "Web 服务器 01",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": datetime.datetime.now().isoformat()
    },
    "key-host-100": {
        "name": "key-host-100",
        "ip_address": "192.168.31.100",
        "sys_id": "host-100",
        "sys_class_name": "cmdb_ci_linux_server",
        "os": "Linux",
        "os_version": "Rocky 8.8",
        "cpu": "4 vCPU",
        "memory": "8GB",
        "disk": "100GB SSD",
        "status": "active",
        "environment": "production",
        "location": "上海数据中心",
        "owner": "ops-team",
        "department": "运维部",
        "category": "server",
        "description": "密钥认证主机 100",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": datetime.datetime.now().isoformat()
    },
    "key-host-101": {
        "name": "key-host-101",
        "ip_address": "192.168.31.101",
        "sys_id": "host-101",
        "sys_class_name": "cmdb_ci_linux_server",
        "os": "Linux",
        "os_version": "Rocky 8.8",
        "cpu": "4 vCPU",
        "memory": "16GB",
        "disk": "200GB SSD",
        "status": "active",
        "environment": "production",
        "location": "上海数据中心",
        "owner": "ops-team",
        "department": "运维部",
        "category": "server",
        "description": "密钥认证主机 101",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": datetime.datetime.now().isoformat()
    },
    "fail-host-01": {
        "name": "fail-host-01",
        "ip_address": "192.168.31.254",
        "sys_id": "host-003",
        "sys_class_name": "cmdb_ci_linux_server",
        "os": "Linux",
        "os_version": "Ubuntu 22.04",
        "cpu": "2 vCPU",
        "memory": "4GB",
        "disk": "50GB SSD",
        "status": "inactive",
        "environment": "test",
        "location": "测试环境",
        "owner": "test-team",
        "department": "QA部",
        "category": "server",
        "description": "测试主机 - 用于测试失败场景",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": datetime.datetime.now().isoformat()
    }
}

# 动态生成100个主机 (server-001 到 server-100)
for i in range(1, 101):
    host_name = f"server-{i:03d}"
    # IP 范围: 10.0.0.1 到 10.0.0.100
    ip_address = f"10.0.0.{i}"
    CMDB_HOSTS[host_name] = {
        "name": host_name,
        "ip_address": ip_address,
        "hostname": host_name,
        "sys_id": f"host-gen-{i:03d}",
        "sys_class_name": "cmdb_ci_linux_server",
        "os": "Linux",
        "os_version": "Rocky 8.8" if i % 2 == 0 else "Ubuntu 22.04",
        "cpu": f"{2 + (i % 4) * 2} vCPU",
        "memory": f"{4 + (i % 4) * 4}GB",
        "disk": f"{50 + (i % 5) * 50}GB SSD",
        "status": "active" if i % 10 != 0 else "inactive",  # 每10个有1个 inactive
        "environment": ["production", "staging", "development"][i % 3],
        "location": ["北京数据中心", "上海数据中心", "广州数据中心"][i % 3],
        "owner": ["ops-team", "dev-team", "qa-team"][i % 3],
        "department": ["运维部", "研发部", "QA部"][i % 3],
        "category": "server",
        "description": f"自动生成的测试主机 {i:03d}",
        "created_at": "2026-01-01T00:00:00Z",
        "updated_at": datetime.datetime.now().isoformat()
    }


@app.route('/api/now/table/cmdb_ci', methods=['GET'])
def list_hosts():
    """获取所有 CMDB 主机"""
    sysparm_limit = request.args.get('sysparm_limit', 100, type=int)
    sysparm_query = request.args.get('sysparm_query', '')
    
    hosts = list(CMDB_HOSTS.values())
    
    # 简单的查询过滤
    if sysparm_query:
        if 'name=' in sysparm_query:
            name_filter = sysparm_query.split('name=')[1].split('^')[0]
            hosts = [h for h in hosts if name_filter.lower() in h['name'].lower()]
        elif 'ip_address=' in sysparm_query:
            ip_filter = sysparm_query.split('ip_address=')[1].split('^')[0]
            hosts = [h for h in hosts if ip_filter == h['ip_address']]
    
    return jsonify({
        "result": hosts[:sysparm_limit]
    })

@app.route('/api/now/table/cmdb_ci/<host_id>', methods=['GET'])
def get_host(host_id):
    """获取单个主机详情"""
    # 通过 sys_id 或 name 查找
    for host in CMDB_HOSTS.values():
        if host['sys_id'] == host_id or host['name'] == host_id:
            return jsonify({"result": host})
    
    return jsonify({"error": {"message": "Host not found"}}), 404

@app.route('/api/cmdb/hosts/by-ci', methods=['GET'])
def get_host_by_ci():
    """通过 CI 名称获取主机 IP（自愈引擎使用）"""
    ci_name = request.args.get('ci_name', '')
    
    if ci_name in CMDB_HOSTS:
        host = CMDB_HOSTS[ci_name]
        return jsonify({
            "success": True,
            "data": {
                "name": host['name'],
                "ip_address": host['ip_address'],
                "status": host['status']
            }
        })
    
    # 如果精确匹配失败，尝试模糊匹配
    for name, host in CMDB_HOSTS.items():
        if ci_name.lower() in name.lower():
            return jsonify({
                "success": True,
                "data": {
                    "name": host['name'],
                    "ip_address": host['ip_address'],
                    "status": host['status']
                }
            })
    
    return jsonify({
        "success": False,
        "error": f"Host '{ci_name}' not found in CMDB"
    }), 404

@app.route('/api/cmdb/hosts', methods=['GET'])
def list_all_hosts():
    """列出所有主机（简化格式）"""
    hosts = []
    for name, host in CMDB_HOSTS.items():
        hosts.append({
            "name": host['name'],
            "ip": host['ip_address'],
            "status": host['status'],
            "environment": host['environment']
        })
    return jsonify({
        "success": True,
        "data": hosts
    })

@app.route('/health', methods=['GET'])
def health():
    """健康检查"""
    return jsonify({"status": "healthy", "service": "mock-cmdb"})

if __name__ == '__main__':
    print("=" * 50)
    print("模拟 CMDB 服务启动")
    print("端口: 5001")
    print("=" * 50)
    print("\n可用主机:")
    for name, host in CMDB_HOSTS.items():
        print(f"  - {name}: {host['ip_address']}")
    print("\n可用端点:")
    print("  GET /api/now/table/cmdb_ci - 列出所有主机")
    print("  GET /api/cmdb/hosts/by-ci?ci_name=xxx - 通过 CI 获取主机")
    print("  GET /health - 健康检查")
    print("=" * 50)
    
    app.run(host='0.0.0.0', port=5001, debug=False)
