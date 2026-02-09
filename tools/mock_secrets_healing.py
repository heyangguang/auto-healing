#!/usr/bin/env python3
"""
模拟密钥服务 - 为自愈引擎执行节点测试提供凭证
端口: 5002

认证方式：Bearer Token (mock-token-12345)
查询方式：POST /api/secrets/{ip_or_hostname}
"""

from flask import Flask, jsonify, request
from functools import wraps

app = Flask(__name__)

# 模拟 Bearer Token
VALID_TOKEN = "mock-token-12345"

def require_auth(f):
    """Bearer Token 认证装饰器"""
    @wraps(f)
    def decorated(*args, **kwargs):
        auth_header = request.headers.get('Authorization', '')
        if not auth_header.startswith('Bearer '):
            return jsonify({"error": "Missing or invalid Authorization header"}), 401
        token = auth_header[7:]
        if token != VALID_TOKEN:
            return jsonify({"error": "Invalid token"}), 401
        return f(*args, **kwargs)
    return decorated

# 用户提供的 SSH 私钥 (auto_healing_key.bak)
SSH_PRIVATE_KEY = """-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACArtvC5DjeUhqUFJyw62t/0l7Ua5FsmSt5IOWo8zQjX/AAAAJg2q55UNque
VAAAAAtzc2gtZWQyNTUxOQAAACArtvC5DjeUhqUFJyw62t/0l7Ua5FsmSt5IOWo8zQjX/A
AAAECmVw1rGIkeJtfCtw2/1TWuE6twNNN7N+N0CAQzF4cO9Cu28LkON5SGpQUnLDra3/SX
tRrkWyZK3kg5ajzNCNf8AAAAEHJvb3RAZGV2ZWxvcG1lbnQBAgMEBQ==
-----END OPENSSH PRIVATE KEY-----
"""

# 模拟密钥数据 - 只保留 192.168 开头的主机
SECRETS = {
    # 密码认证主机
    "192.168.31.102": {
        "username": "root",
        "password": "123",
        "auth_type": "password"
    },
    "192.168.31.103": {
        "username": "root",
        "password": "123",
        "auth_type": "password"
    },
    "192.168.31.66": {
        "username": "root",
        "password": "123",
        "auth_type": "password"
    },
    # SSH 密钥认证主机
    "192.168.31.100": {
        "username": "root",
        "private_key": SSH_PRIVATE_KEY,
        "auth_type": "ssh_key"
    },
    "192.168.31.101": {
        "username": "root",
        "private_key": SSH_PRIVATE_KEY,
        "auth_type": "ssh_key"
    }
}

def get_secret_response(host):
    """获取凭证响应"""
    if host in SECRETS:
        secret = SECRETS[host]
        response = {
            "username": secret['username'],
            "auth_type": secret['auth_type']
        }
        if secret['auth_type'] == 'password':
            response['password'] = secret['password']
        else:
            response['private_key'] = secret['private_key']
        return response, 200
    else:
        return {"error": f"Host '{host}' not found"}, 404

@app.route('/api/secrets/<host>', methods=['POST'])
@require_auth
def get_secret_by_path(host):
    """通过 URL 路径查询凭证（POST + Bearer Token 认证）"""
    response, status = get_secret_response(host)
    return jsonify(response), status

@app.route('/api/secrets/query', methods=['POST'])
@require_auth
def query_secret():
    """通过 JSON body 查询凭证"""
    data = request.get_json() or {}
    host = data.get('host', '') or data.get('hostname', '') or data.get('ip_address', '')
    
    if not host:
        return jsonify({"error": "host is required"}), 400
    
    response, status = get_secret_response(host)
    if status == 200:
        return jsonify({"success": True, "data": response})
    return jsonify({"success": False, "error": response.get("error")}), status

@app.route('/api/secrets/hosts', methods=['GET'])
@require_auth
def list_hosts():
    """列出所有已配置凭证的主机"""
    hosts = list(SECRETS.keys())
    return jsonify({
        "success": True,
        "count": len(hosts),
        "data": hosts[:50]
    })

# ========== 测试 response_data_path 和字段映射 ==========

@app.route('/api/v2/secrets/<host>', methods=['POST'])
@require_auth
def get_secret_v2(host):
    """
    测试 response_data_path 功能
    凭证包在 result.credentials 里，需要配置:
      response_data_path: "result.credentials"
    """
    if host in SECRETS:
        secret = SECRETS[host]
        creds = {
            "user": secret['username'],  # 字段名不同，需要映射
            "secret_type": secret['auth_type']
        }
        if secret['auth_type'] == 'password':
            creds['pwd'] = secret['password']  # 需要映射 password -> pwd
        else:
            creds['key'] = secret['private_key']  # 需要映射 private_key -> key
        
        return jsonify({
            "code": 0,
            "message": "ok",
            "result": {
                "credentials": creds
            }
        })
    else:
        return jsonify({"code": 404, "message": "not found"}), 404

@app.route('/health', methods=['GET'])
def health():
    """健康检查（无需认证）"""
    return jsonify({"status": "healthy", "service": "mock-secrets", "hosts_count": len(SECRETS)})

if __name__ == '__main__':
    print("=" * 50)
    print("模拟密钥服务启动 (POST + Bearer Token)")
    print("端口: 5002")
    print("Token: mock-token-12345")
    print("=" * 50)
    print(f"\n已配置凭证的主机: {len(SECRETS)} 个")
    print("示例主机:")
    sample_hosts = list(SECRETS.keys())[:10]
    for host in sample_hosts:
        secret = SECRETS[host]
        print(f"  - {host}: {secret['username']} (auth: {secret['auth_type']})")
    print("  ... 更多主机请调用 /api/secrets/hosts")
    print("\n可用端点:")
    print("  POST /api/secrets/{ip_or_hostname} - 通过路径查询")
    print("  POST /api/secrets/query - 通过 JSON body 查询")
    print("  GET  /api/secrets/hosts - 列出主机")
    print("  GET  /health - 健康检查（无需认证）")
    print("\n示例:")
    print("  curl -X POST -H 'Authorization: Bearer mock-token-12345' http://localhost:5002/api/secrets/192.168.31.100")
    print("=" * 50)
    
    app.run(host='0.0.0.0', port=5002, debug=False)


