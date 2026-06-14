#!/usr/bin/env python3
import http.server
import json
import socketserver
import subprocess

LAB_SCRIPT = "/opt/auto-healing-fault-lab/auto_healing_fault_lab.sh"
SCENARIOS = {"service_down", "cpu_high", "disk_full", "clean_logs", "kill_process"}


class Handler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/fault-lab/status":
            return self.run_fault_command(["status", "all"])
        return super().do_GET()

    def do_POST(self):
        prefix = "/fault-lab/"
        if not self.path.startswith(prefix):
            self.send_error(404)
            return
        parts = self.path[len(prefix):].strip("/").split("/")
        if len(parts) != 2 or parts[0] not in {"inject", "reset", "status"} or parts[1] not in SCENARIOS:
            self.send_error(400, "invalid fault lab command")
            return
        return self.run_fault_command(parts)

    def run_fault_command(self, args):
        result = subprocess.run(
            [LAB_SCRIPT, *args],
            check=False,
            text=True,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            timeout=30,
        )
        payload = {
            "ok": result.returncode == 0,
            "return_code": result.returncode,
            "command": [LAB_SCRIPT, *args],
            "output": result.stdout.strip(),
        }
        body = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(200 if result.returncode == 0 else 409)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, fmt, *args):
        return


socketserver.TCPServer.allow_reuse_address = True
with socketserver.TCPServer(("0.0.0.0", 19081), Handler) as httpd:
    httpd.serve_forever()
