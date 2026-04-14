#!/usr/bin/env python3
import json
import os
import sys
from typing import Optional
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


HOSTS = {
    "Server1": {"username": "ops", "password": "Server1Pass!2026"},
    "Server2": {"username": "ops", "password": "Server2Pass!2026"},
    "Server3": {"username": "ops", "password": "Server3Pass!2026"},
    "Server4": {"username": "ops", "password": "Server4Pass!2026"},
    "cmdb-page-232605": {"username": "ops", "password": "CMDBPagePass!2026"},
}


def env(name: str, default: str) -> str:
    return os.getenv(name, default).strip() or default


def request(method: str, url: str, token: str, payload: Optional[dict] = None) -> dict:
    body = None
    headers = {"X-Vault-Token": token}
    if payload is not None:
        body = json.dumps(payload).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = Request(url, data=body, method=method, headers=headers)
    try:
        with urlopen(req, timeout=20) as resp:
            raw = resp.read().decode("utf-8")
    except HTTPError as exc:
        raise RuntimeError(f"OpenBao HTTP {exc.code}: {exc.read().decode('utf-8', 'replace')}") from exc
    except URLError as exc:
        raise RuntimeError(f"OpenBao unavailable: {exc}") from exc
    return json.loads(raw) if raw else {}


def enable_kv_v2(base_url: str, token: str) -> None:
    url = f"{base_url}/v1/sys/mounts/secret"
    payload = {"type": "kv", "options": {"version": "2"}}
    try:
        request("POST", url, token, payload)
    except RuntimeError as exc:
        if "path is already in use" not in str(exc):
            raise


def put_secret(base_url: str, token: str, host: str, data: dict) -> None:
    url = f"{base_url}/v1/secret/data/hosts/{host}"
    request("POST", url, token, {"data": data})


def main() -> int:
    base_url = env("OPENBAO_ADDR", "http://127.0.0.1:18200").rstrip("/")
    token = env("OPENBAO_ROOT_TOKEN", "root")
    enable_kv_v2(base_url, token)
    for host, secret in HOSTS.items():
        put_secret(base_url, token, host, secret)
    print(json.dumps({"status": "ok", "hosts": sorted(HOSTS.keys())}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main())
