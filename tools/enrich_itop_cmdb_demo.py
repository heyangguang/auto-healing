#!/usr/bin/env python3
"""
Enrich the iTop demo CMDB with customer-facing hardware data.

The script only uses iTop REST APIs. It is safe to run repeatedly and does not
touch the iTop database directly.
"""

import json
import os
from base64 import b64encode
from typing import Any, Dict, List
from urllib.parse import quote
from urllib.request import Request, urlopen


ITOP_BASE_URL = os.getenv("ITOP_BASE_URL", "http://127.0.0.1:18084").rstrip("/")
ITOP_REST_PATH = os.getenv("ITOP_REST_PATH", "/webservices/rest.php")
ITOP_REST_VERSION = os.getenv("ITOP_REST_VERSION", "1.3")
ITOP_USER = os.getenv("ITOP_AUTH_USER", "admin")
ITOP_PASSWORD = os.getenv("ITOP_AUTH_PWD", "admin")

AUTH_HEADER = "Basic " + b64encode(f"{ITOP_USER}:{ITOP_PASSWORD}".encode("utf-8")).decode("ascii")

DEMO_CIS: List[Dict[str, Any]] = [
    {
        "class": "Server",
        "name": "e2e-target-01",
        "fields": {
            "managementip": "118.196.22.79",
            "cpu": "4 vCPU",
            "ram": "8 GB",
            "location_id": "3",
            "brand_id": "8",
            "model_id": "10",
            "serialnumber": "AHS-DEMO-0001",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "Server",
        "name": "Server1",
        "fields": {
            "managementip": "10.10.24.11",
            "cpu": "8 vCPU",
            "ram": "32 GB",
            "location_id": "1",
            "brand_id": "8",
            "model_id": "10",
            "serialnumber": "AHS-SRV-0001",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "Server",
        "name": "Server2",
        "fields": {
            "managementip": "10.10.24.12",
            "cpu": "8 vCPU",
            "ram": "64 GB",
            "location_id": "2",
            "brand_id": "8",
            "model_id": "10",
            "serialnumber": "AHS-SRV-0002",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "Server",
        "name": "Server3",
        "fields": {
            "managementip": "10.10.24.13",
            "cpu": "16 vCPU",
            "ram": "128 GB",
            "location_id": "3",
            "brand_id": "8",
            "model_id": "10",
            "serialnumber": "AHS-SRV-0003",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "Server",
        "name": "Server4",
        "fields": {
            "managementip": "10.10.24.14",
            "cpu": "16 vCPU",
            "ram": "128 GB",
            "location_id": "3",
            "brand_id": "8",
            "model_id": "10",
            "serialnumber": "AHS-SRV-0004",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "VirtualMachine",
        "name": "VM1",
        "fields": {
            "managementip": "10.10.31.21",
            "cpu": "2 vCPU",
            "ram": "8 GB",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "VirtualMachine",
        "name": "VM2",
        "fields": {
            "managementip": "10.10.31.22",
            "cpu": "4 vCPU",
            "ram": "16 GB",
            "osfamily_id": "13",
            "osversion_id": "15",
        },
    },
    {
        "class": "VirtualMachine",
        "name": "VM3",
        "fields": {
            "managementip": "10.10.31.23",
            "cpu": "4 vCPU",
            "ram": "16 GB",
            "osfamily_id": "12",
            "osversion_id": "14",
        },
    },
    {
        "class": "VirtualMachine",
        "name": "VM4",
        "fields": {
            "managementip": "10.10.31.24",
            "cpu": "8 vCPU",
            "ram": "32 GB",
            "osfamily_id": "13",
            "osversion_id": "15",
        },
    },
    {
        "class": "NetworkDevice",
        "name": "Router1",
        "fields": {
            "managementip": "10.10.1.1",
            "location_id": "1",
            "brand_id": "7",
            "serialnumber": "AHS-NET-0001",
        },
    },
    {
        "class": "NetworkDevice",
        "name": "Switch1",
        "fields": {
            "managementip": "10.10.1.2",
            "location_id": "2",
            "brand_id": "8",
            "model_id": "11",
            "serialnumber": "AHS-NET-0002",
        },
    },
]


def itop_call(payload: Dict[str, Any]) -> Dict[str, Any]:
    body = f"json_data={quote(json.dumps(payload, ensure_ascii=False))}".encode("utf-8")
    request = Request(
        f"{ITOP_BASE_URL}{ITOP_REST_PATH}?version={quote(ITOP_REST_VERSION)}",
        data=body,
        method="POST",
        headers={
            "Authorization": AUTH_HEADER,
            "Content-Type": "application/x-www-form-urlencoded",
            "Accept": "application/json",
        },
    )
    with urlopen(request, timeout=30) as response:
        data = json.loads(response.read().decode("utf-8"))
    if data.get("code") not in (0, "0", None):
        raise RuntimeError(json.dumps(data, ensure_ascii=False))
    return data


def update_ci(item: Dict[str, Any]) -> Dict[str, Any]:
    payload = {
        "operation": "core/update",
        "class": item["class"],
        "key": {"name": item["name"]},
        "comment": "AHS demo CMDB enrichment",
        "fields": item["fields"],
        "output_fields": "*",
    }
    return itop_call(payload)


def main() -> None:
    for item in DEMO_CIS:
        result = update_ci(item)
        objects = result.get("objects") or {}
        object_keys = ", ".join(objects.keys()) or "none"
        print(f"updated {item['class']} {item['name']}: {object_keys}")


if __name__ == "__main__":
    main()
