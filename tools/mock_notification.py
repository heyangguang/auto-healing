#!/usr/bin/env python3
"""
Mock 通知服务
用于测试通知模块的 Webhook 和钉钉发送功能
"""

from flask import Flask, request, jsonify
import json
from datetime import datetime

app = Flask(__name__)

# 存储接收到的通知
received_notifications = []

@app.route('/webhook', methods=['POST'])
def webhook_receiver():
    """接收 Webhook 通知"""
    data = request.get_json()
    notification = {
        'type': 'webhook',
        'timestamp': datetime.now().isoformat(),
        'headers': dict(request.headers),
        'body': data
    }
    received_notifications.append(notification)
    print(f"[Webhook] 收到通知: {json.dumps(data, ensure_ascii=False, indent=2)}")
    return jsonify({"success": True, "message": "通知已接收"})

@app.route('/dingtalk', methods=['POST'])
def dingtalk_receiver():
    """模拟钉钉机器人 Webhook"""
    data = request.get_json()
    notification = {
        'type': 'dingtalk',
        'timestamp': datetime.now().isoformat(),
        'body': data
    }
    received_notifications.append(notification)
    print(f"[DingTalk] 收到通知: {json.dumps(data, ensure_ascii=False, indent=2)}")
    
    # 模拟钉钉响应
    return jsonify({
        "errcode": 0,
        "errmsg": "ok"
    })

@app.route('/fail', methods=['POST'])
def fail_receiver():
    """模拟失败的 Webhook（用于测试重试）"""
    return jsonify({"error": "模拟失败"}), 500

@app.route('/notifications', methods=['GET'])
def list_notifications():
    """查看所有接收到的通知"""
    return jsonify({
        "total": len(received_notifications),
        "notifications": received_notifications
    })

@app.route('/notifications/clear', methods=['POST'])
def clear_notifications():
    """清空通知记录"""
    global received_notifications
    received_notifications = []
    return jsonify({"message": "已清空"})

@app.route('/health', methods=['GET'])
def health():
    """健康检查"""
    return jsonify({"status": "ok"})

if __name__ == '__main__':
    print("=" * 50)
    print("Mock 通知服务已启动")
    print("=" * 50)
    print("Webhook 地址: http://localhost:9999/webhook")
    print("钉钉地址:     http://localhost:9999/dingtalk")
    print("失败测试:     http://localhost:9999/fail")
    print("查看记录:     http://localhost:9999/notifications")
    print("=" * 50)
    app.run(host='0.0.0.0', port=9999, debug=False, use_reloader=False)
