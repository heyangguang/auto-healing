#!/usr/bin/env python3

from pathlib import Path
import re
import sys
from typing import Any, Dict, Iterable, List, Optional, Set, Tuple

import yaml

ROOT = Path(__file__).resolve().parents[1]
OPENAPI_PATH = ROOT / "api" / "openapi.yaml"
CONTRACTS_PATH = ROOT / "tools" / "dictionary_contracts.yaml"
SEED_FILES = sorted((ROOT / "internal/modules/ops/service").glob("dictionary_seeds*.go"))

SEED_CALL_RE = re.compile(r"\b(dInactive|d)\(\s*\"([^\"]+)\"\s*,\s*\"([^\"]+)\"")


class ValidationError(RuntimeError):
    pass


def load_yaml(path: Path) -> Dict[str, Any]:
    try:
        data = yaml.safe_load(path.read_text(encoding="utf-8"))
    except yaml.YAMLError as exc:
        raise ValidationError(f"{path} 不是合法 YAML: {exc}") from exc
    if not isinstance(data, dict):
        raise ValidationError(f"{path} 顶层必须是 mapping")
    return data


def load_seed_entries() -> Dict[str, Dict[str, bool]]:
    entries: Dict[str, Dict[str, bool]] = {}
    for path in SEED_FILES:
        text = path.read_text(encoding="utf-8")
        for helper, dict_type, dict_key in SEED_CALL_RE.findall(text):
            active = helper == "d"
            entries.setdefault(dict_type, {})
            if dict_key in entries[dict_type] and entries[dict_type][dict_key] != active:
                raise ValidationError(f"字典 seed 激活状态冲突: {dict_type}.{dict_key} ({path})")
            entries[dict_type][dict_key] = active
    return entries


def active_seed_keys(entries: Dict[str, Dict[str, bool]], dict_type: str) -> Set[str]:
    return {key for key, active in entries.get(dict_type, {}).items() if active}


def load_openapi_document() -> Dict[str, Any]:
    doc = load_yaml(OPENAPI_PATH)
    if not isinstance(doc.get("paths"), dict):
        raise ValidationError("openapi.yaml 缺少合法的 paths")
    return doc


def collect_dict_types(node: Any, found: Set[str]) -> None:
    if isinstance(node, dict):
        dict_type = node.get("x-dict-type")
        if isinstance(dict_type, str):
            found.add(dict_type)
        for value in node.values():
            collect_dict_types(value, found)
    elif isinstance(node, list):
        for value in node:
            collect_dict_types(value, found)


def parse_go_values(spec: Dict[str, Any]) -> Set[str]:
    if "literals" in spec:
        literals = spec.get("literals")
        if not isinstance(literals, list):
            raise ValidationError("go_sources.literals 必须是列表")
        return {value for value in literals if isinstance(value, str)}

    file_path = spec.get("file")
    if not isinstance(file_path, str):
        raise ValidationError("go_sources.file 必须是字符串")
    text = (ROOT / file_path).read_text(encoding="utf-8")

    if "const_prefix" in spec:
        prefix = spec["const_prefix"]
        pattern = re.compile(rf"\b{re.escape(prefix)}[A-Za-z0-9_]*\s*=\s*\"([^\"]+)\"")
        return set(pattern.findall(text))

    if "identifiers" in spec:
        identifiers = spec["identifiers"]
        if not isinstance(identifiers, list):
            raise ValidationError("go_sources.identifiers 必须是列表")
        values: Set[str] = set()
        for identifier in identifiers:
            if not isinstance(identifier, str):
                continue
            match = re.search(rf"\b{re.escape(identifier)}\s*=\s*\"([^\"]+)\"", text)
            if match:
                values.add(match.group(1))
        return values

    raise ValidationError("go_sources 必须配置 literals / const_prefix / identifiers 之一")


def get_schema(document: Dict[str, Any], name: str) -> Dict[str, Any]:
    schema = document.get("components", {}).get("schemas", {}).get(name)
    if not isinstance(schema, dict):
        raise ValidationError(f"OpenAPI schema 不存在: {name}")
    return schema


def dereference_schema(document: Dict[str, Any], node: Dict[str, Any]) -> Dict[str, Any]:
    current = node
    seen: Set[str] = set()
    while isinstance(current, dict) and isinstance(current.get("$ref"), str):
        ref = current["$ref"]
        if ref in seen:
            raise ValidationError(f"OpenAPI $ref 循环引用: {ref}")
        seen.add(ref)
        prefix = "#/components/schemas/"
        if not ref.startswith(prefix):
            raise ValidationError(f"暂不支持的 $ref: {ref}")
        current = get_schema(document, ref[len(prefix):])
    return current


def walk_mapping_path(node: Dict[str, Any], property_path: str) -> Dict[str, Any]:
    current: Any = node
    for part in property_path.split("."):
        if not isinstance(current, dict) or part not in current:
            raise ValidationError(f"OpenAPI 路径不存在: {property_path}")
        current = current[part]
    if not isinstance(current, dict):
        raise ValidationError(f"OpenAPI 目标不是 object 节点: {property_path}")
    return current


def get_parameter(document: Dict[str, Any], path: str, method: str, name: str) -> Dict[str, Any]:
    operation = document.get("paths", {}).get(path, {}).get(method.lower())
    if not isinstance(operation, dict):
        raise ValidationError(f"OpenAPI 操作不存在: {method.upper()} {path}")
    parameters = operation.get("parameters", [])
    for parameter in parameters:
        if isinstance(parameter, dict) and parameter.get("name") == name:
            return parameter
    raise ValidationError(f"OpenAPI 参数不存在: {method.upper()} {path} {name}")


def get_request_body_schema(document: Dict[str, Any], path: str, method: str) -> Dict[str, Any]:
    operation = document.get("paths", {}).get(path, {}).get(method.lower())
    if not isinstance(operation, dict):
        raise ValidationError(f"OpenAPI 操作不存在: {method.upper()} {path}")
    request_body = operation.get("requestBody")
    if not isinstance(request_body, dict):
        raise ValidationError(f"OpenAPI requestBody 不存在: {method.upper()} {path}")
    content = request_body.get("content")
    if not isinstance(content, dict):
        raise ValidationError(f"OpenAPI requestBody.content 不存在: {method.upper()} {path}")
    media = content.get("application/json")
    if not isinstance(media, dict):
        raise ValidationError(f"OpenAPI requestBody 缺少 application/json: {method.upper()} {path}")
    schema = media.get("schema")
    if not isinstance(schema, dict):
        raise ValidationError(f"OpenAPI requestBody.schema 缺失: {method.upper()} {path}")
    return dereference_schema(document, schema)


def resolve_target(document: Dict[str, Any], target: Dict[str, Any]) -> Dict[str, Any]:
    kind = target.get("kind")
    if kind == "schema_property":
        schema_name = target.get("schema")
        property_path = target.get("property_path")
        if not isinstance(schema_name, str) or not isinstance(property_path, str):
            raise ValidationError("schema_property 目标必须包含 schema/property_path")
        schema = dereference_schema(document, get_schema(document, schema_name))
        properties = schema.get("properties")
        if not isinstance(properties, dict):
            raise ValidationError(f"schema 缺少 properties: {schema_name}")
        return walk_mapping_path(properties, property_path)

    if kind == "parameter":
        path = target.get("path")
        method = target.get("method")
        name = target.get("name")
        if not all(isinstance(value, str) for value in (path, method, name)):
            raise ValidationError("parameter 目标必须包含 path/method/name")
        parameter = get_parameter(document, path, method, name)
        schema = parameter.get("schema")
        if not isinstance(schema, dict):
            raise ValidationError(f"OpenAPI 参数 schema 缺失: {method.upper()} {path} {name}")
        return schema

    if kind == "request_body_property":
        path = target.get("path")
        method = target.get("method")
        property_path = target.get("property_path")
        if not all(isinstance(value, str) for value in (path, method, property_path)):
            raise ValidationError("request_body_property 目标必须包含 path/method/property_path")
        schema = get_request_body_schema(document, path, method)
        properties = schema.get("properties")
        if not isinstance(properties, dict):
            raise ValidationError(f"OpenAPI requestBody schema 缺少 properties: {method.upper()} {path}")
        return walk_mapping_path(properties, property_path)

    raise ValidationError(f"不支持的 target kind: {kind}")


def node_enum_values(node: Dict[str, Any]) -> Optional[Set[str]]:
    if isinstance(node.get("enum"), list):
        return {value for value in node["enum"] if isinstance(value, str)}
    if node.get("type") == "array":
        items = node.get("items")
        if isinstance(items, dict) and isinstance(items.get("enum"), list):
            return {value for value in items["enum"] if isinstance(value, str)}
    return None


def validate_target(
    errors: List[str],
    document: Dict[str, Any],
    dict_type: str,
    target: Dict[str, Any],
    expected_keys: Set[str],
) -> None:
    node = resolve_target(document, target)
    expected_mode = target.get("mode")
    if expected_mode not in {"exact", "subset", "open"}:
        errors.append(f"{dict_type}: 非法 mode={expected_mode}")
        return

    if node.get("x-dict-type") != dict_type:
        errors.append(f"{dict_type}: OpenAPI 目标缺少或错误的 x-dict-type -> {target}")
    if node.get("x-dict-mode") != expected_mode:
        errors.append(f"{dict_type}: OpenAPI 目标缺少或错误的 x-dict-mode -> {target}")

    enum_values = node_enum_values(node)
    if enum_values is None or expected_mode == "open":
        return
    if expected_mode == "exact" and enum_values != expected_keys:
        errors.append(
            f"{dict_type}: OpenAPI enum 与激活字典值不一致 target={target} "
            f"openapi={sorted(enum_values)} dict={sorted(expected_keys)}"
        )
    if expected_mode == "subset" and not enum_values.issubset(expected_keys):
        errors.append(
            f"{dict_type}: OpenAPI enum 不是激活字典值子集 target={target} "
            f"openapi={sorted(enum_values)} dict={sorted(expected_keys)}"
        )


def validate_non_dictionary_target(errors: List[str], document: Dict[str, Any], target: Dict[str, Any]) -> None:
    node = resolve_target(document, target)
    if "x-dict-type" in node:
        errors.append(f"非字典字段不应带 x-dict-type: {target}")


def validate_contracts() -> List[str]:
    contracts = load_yaml(CONTRACTS_PATH)
    document = load_openapi_document()
    seed_entries = load_seed_entries()
    errors: List[str] = []
    declared_dict_types = set(contracts.get("dict_types", {}).keys())

    used_dict_types: Set[str] = set()
    collect_dict_types(document, used_dict_types)
    missing_contracts = sorted(used_dict_types - declared_dict_types)
    if missing_contracts:
        errors.append(f"OpenAPI 使用了未登记到 dictionary_contracts 的 dict_type: {missing_contracts}")

    for dict_type, config in contracts.get("dict_types", {}).items():
        if not isinstance(config, dict):
            errors.append(f"{dict_type}: 合同配置必须是 object")
            continue
        keys = active_seed_keys(seed_entries, dict_type)
        if not keys:
            errors.append(f"{dict_type}: 缺少激活字典种子")
            continue

        for source in config.get("go_sources", []):
            try:
                go_values = parse_go_values(source)
            except Exception as exc:
                errors.append(f"{dict_type}: Go 常量解析失败: {exc}")
                continue
            if not go_values:
                errors.append(f"{dict_type}: Go 常量未提取到任何值 -> {source}")
                continue
            source_mode = source.get("mode", "exact")
            if source_mode == "exact" and go_values != keys:
                errors.append(
                    f"{dict_type}: Go 常量值与激活字典值不一致 "
                    f"go={sorted(go_values)} dict={sorted(keys)}"
                )
            if source_mode == "subset" and not go_values.issubset(keys):
                errors.append(
                    f"{dict_type}: Go 常量值不是激活字典值子集 "
                    f"go={sorted(go_values)} dict={sorted(keys)}"
                )

        for target in config.get("openapi_targets", []):
            try:
                validate_target(errors, document, dict_type, target, keys)
            except Exception as exc:
                errors.append(f"{dict_type}: OpenAPI 目标校验失败: {exc}")

    for target in contracts.get("non_dictionary_fields", []):
        try:
            validate_non_dictionary_target(errors, document, target)
        except Exception as exc:
            errors.append(f"非字典字段校验失败: {exc}")

    return errors


def main() -> int:
    try:
        errors = validate_contracts()
    except ValidationError as exc:
        print(f"[FAIL] {exc}")
        return 1

    if errors:
        print("[FAIL] dictionary contracts 校验失败")
        for err in errors:
            print(f"- {err}")
        return 1

    print("[OK] dictionary contracts 校验通过")
    return 0


if __name__ == "__main__":
    sys.exit(main())
