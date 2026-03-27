#!/usr/bin/env python3

import argparse
import sys
from pathlib import Path
from typing import Any, Dict, List

import yaml

ROOT = Path(__file__).resolve().parents[1]
SOURCE_DIR = ROOT / "api" / "openapi-src"
BUNDLE_PATH = ROOT / "api" / "openapi.yaml"
ROOT_FILE = SOURCE_DIR / "root.yaml"
PATHS_DIR = SOURCE_DIR / "paths"
COMPONENTS_DIR = SOURCE_DIR / "components"
SCHEMAS_DIR = COMPONENTS_DIR / "schemas"


class BuildError(RuntimeError):
    pass


def load_yaml_file(path: Path) -> Any:
    try:
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        raise BuildError(f"{path} 不是合法 YAML: {exc}") from exc
    return {} if data is None else data


def load_mapping(path: Path, label: str) -> Dict[str, Any]:
    data = load_yaml_file(path)
    if not isinstance(data, dict):
        raise BuildError(f"{label} {path} 顶层必须是 mapping")
    return data


def merge_mapping_files(files: List[Path], label: str) -> Dict[str, Any]:
    merged: Dict[str, Any] = {}
    for path in files:
        data = load_mapping(path, label)
        for key, value in data.items():
            if key in merged:
                raise BuildError(f"{label} 重复定义: {key} ({path})")
            merged[key] = value
    return merged


def build_document() -> Dict[str, Any]:
    root = load_mapping(ROOT_FILE, "openapi root")
    if "paths" in root or "components" in root:
        raise BuildError("openapi-src/root.yaml 不应直接包含 paths/components，请放到分片目录")

    document = dict(root)
    document["paths"] = merge_mapping_files(sorted(PATHS_DIR.glob("*.yaml")), "path")
    document["components"] = {
        "securitySchemes": load_mapping(COMPONENTS_DIR / "security-schemes.yaml", "security schemes"),
        "parameters": load_mapping(COMPONENTS_DIR / "parameters.yaml", "parameters"),
        "responses": load_mapping(COMPONENTS_DIR / "responses.yaml", "responses"),
        "schemas": merge_mapping_files(sorted(SCHEMAS_DIR.glob("*.yaml")), "schema"),
    }

    if not document["paths"]:
        raise BuildError("未构建出任何 paths")
    if not document["components"]["schemas"]:
        raise BuildError("未构建出任何 schemas")
    return document


def dump_document(document: Dict[str, Any]) -> str:
    return yaml.safe_dump(
        document,
        allow_unicode=True,
        sort_keys=False,
        width=120,
    )


def run_check() -> int:
    rendered = dump_document(build_document())
    current = BUNDLE_PATH.read_text(encoding="utf-8") if BUNDLE_PATH.exists() else ""
    if rendered != current:
        print("[FAIL] api/openapi.yaml 已过期，请先运行: python tools/build_openapi.py")
        return 1
    print("[OK] api/openapi.yaml 与 openapi-src 保持同步")
    return 0


def run_build() -> int:
    rendered = dump_document(build_document())
    BUNDLE_PATH.write_text(rendered, encoding="utf-8")
    print(f"[OK] built {BUNDLE_PATH}")
    return 0


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Build bundled OpenAPI spec from split source fragments.")
    parser.add_argument("--check", action="store_true", help="only verify api/openapi.yaml is up to date")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        return run_check() if args.check else run_build()
    except BuildError as exc:
        print(f"[FAIL] {exc}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
