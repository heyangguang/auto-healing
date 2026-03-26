#!/usr/bin/env python3

from pathlib import Path
import re
import subprocess
import sys
from typing import List

ROOT = Path(__file__).resolve().parents[1]


def read_text(relative_path: str) -> str:
    return (ROOT / relative_path).read_text(encoding="utf-8")


def require(errors: List[str], condition: bool, message: str) -> None:
    if not condition:
        errors.append(message)


def has_regex(content: str, pattern: str) -> bool:
    return re.search(pattern, content, re.S) is not None


def validate_notification_seed(errors: List[str]) -> None:
    content = read_text("tools/create_notification_data.sh")
    require(errors, ".access_token // empty" in content, "create_notification_data.sh 必须读取 login 原始 access_token")
    require(errors, "default_recipients" not in content, "create_notification_data.sh 仍在使用 default_recipients")
    require(errors, '"url": "http://localhost:5000/webhook/alert-' in content, "create_notification_data.sh 的 webhook 渠道配置仍未使用 config.url")


def validate_task_seed(errors: List[str]) -> None:
    content = read_text("tools/create_tasks.sh")
    require(errors, ".access_token // empty" in content, "create_tasks.sh 必须读取 login 原始 access_token")
    require(errors, "/playbooks?page=1&page_size=100" in content, "create_tasks.sh 必须先查询可用 Playbook")
    require(errors, "PLAYBOOKS=(" not in content, "create_tasks.sh 仍在硬编码 Playbook UUID")
    require(errors, "> /dev/null" not in content, "create_tasks.sh 仍在吞掉 API 错误输出")
    require(errors, ".code == 0 and .data.id != null" in content, "create_tasks.sh 缺少创建结果校验")


def validate_executor_build(errors: List[str]) -> None:
    content = read_text("docker/ansible-executor/build.sh")
    require(errors, 'IMAGE_NAME="${IMAGE_NAME:-auto-healing-executor}"' in content, "ansible executor 构建脚本默认镜像名未与部署配置统一")
    require(errors, 'EXECUTOR_VERSION' in content, "ansible executor 构建脚本未复用部署版本变量")


def validate_mock_notification(errors: List[str]) -> None:
    content = read_text("tools/mock_notification.py")
    require(errors, "use_reloader=False" in content, "mock_notification.py 未禁用 reloader")
    require(errors, "debug=True" not in content, "mock_notification.py 仍在开启 Flask debug")


def validate_deployment_scripts(errors: List[str]) -> None:
    start_content = read_text("deployments/docker/start.sh")
    stop_content = read_text("deployments/docker/stop.sh")
    reset_content = read_text("deployments/docker/reset.sh")

    require(errors, "set -euo pipefail" in start_content, "deployments/docker/start.sh 缺少严格模式，失败后仍可能打印成功")
    require(errors, 'if [ ! -x "$SERVER_BIN" ]; then' in start_content, "deployments/docker/start.sh 仍未校验 server 二进制存在性")
    require(errors, 'kill -0 "$server_pid"' in start_content, "deployments/docker/start.sh 仍未校验 server 进程真实存活")
    require(errors, "unix:///run/podman/podman.sock" in stop_content, "deployments/docker/stop.sh 未继承 Podman DOCKER_HOST")
    require(errors, "bind mount 目录 /data/postgres 和 /data/redis" in stop_content, "deployments/docker/stop.sh 仍错误提示 Docker 卷数据模型")
    require(errors, "./reset.sh" in stop_content, "deployments/docker/stop.sh 未提示使用 reset.sh 清理 bind mount 数据")
    require(errors, "unix:///run/podman/podman.sock" in reset_content, "deployments/docker/reset.sh 未继承 Podman DOCKER_HOST")


def validate_no_hardcoded_repo_paths(errors: List[str]) -> None:
    cmd = ["rg", "-n", "/root/auto-healing/", "tests/e2e", "-S"]
    result = subprocess.run(
        cmd,
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=False,
    )
    require(errors, result.returncode == 1, "tests/e2e 仍存在硬编码 /root/auto-healing 路径")


def validate_no_legacy_contract_fallbacks(errors: List[str]) -> None:
    patterns = [
        r"\.id // \.data\.id",
        r"\.data\.id // \.id",
        r"\.status // \.data\.status",
        r"\(\.items // \.data\)",
        r"\(\.data // \.items // \[\]\)",
        r"\.data\.subject_template // \.subject_template",
        r"\.data\.body_template // \.body_template",
        r"\.data\.available_variables // \.available_variables",
        r"\.access_token // \.data\.access_token",
        r"\.pagination\.total",
    ]
    cmd = ["rg", "-n", "-e", "|".join(patterns), "tests/e2e", "tools", "-g", "*.sh", "-S"]
    result = subprocess.run(
        cmd,
        cwd=ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        universal_newlines=True,
        check=False,
    )
    require(errors, result.returncode == 1, "tests/e2e 或 tools 仍保留旧契约 fallback 解析")


def validate_no_silent_e2e_fallbacks(errors: List[str]) -> None:
    forbidden_patterns = {
        "tests/e2e/test_incident_close.sh": ["跳过测试", "exit 0"],
        "tests/e2e/test_notification_e2e.sh": ["跳过执行任务测试"],
        "tests/e2e/test_secrets.sh": ["跳过测试"],
        "tests/e2e/test_healing_full_flow.sh": ["部分通过", "跳过审批步骤"],
        "tests/e2e/test_cancel_execution.sh": ["⚠️ 任务未处于 running 状态", "⚠️ 取消测试结果"],
        "tests/e2e/test_docker_cancel_execution.sh": ["⚠️ 任务未处于 running 状态", "⚠️ 取消测试结果"],
        "tests/e2e/test_healing_engine.sh": ["⚠️ 规则更新结果", "⚠️ 规则启用状态", "⚠️ 规则停用状态"],
        "tests/e2e/test_git_repos.sh": ["⚠️ 同步状态", "⚠️ 激活失败", "⚠️ 没有可用的测试仓库"],
        "tests/e2e/test_complete_workflow.sh": ["尝试使用已有", "尝试获取已有", "跳过审批步骤"],
        "tests/e2e/test_complete_workflow_local.sh": ["尝试使用已有", "尝试获取已有", "跳过审批步骤"],
        "tests/e2e/test_complete_workflow_docker.sh": ["尝试使用已有", "尝试获取已有", "跳过审批步骤"],
        "tests/e2e/test_complete_workflow_docker_fail.sh": ["尝试使用已有", "尝试获取已有", "跳过审批步骤"],
    }

    for relative_path, patterns in forbidden_patterns.items():
        content = read_text(relative_path)
        for pattern in patterns:
            require(errors, pattern not in content, f"{relative_path} 仍包含静默回退/假通过模式: {pattern}")

    git_repos_content = read_text("tests/e2e/test_git_repos.sh")
    require(errors, 'GITEA_TOKEN="${GITEA_TOKEN:-4d6b1e987572e22875b0d4700c8de14e582c936d}"' not in git_repos_content, "tests/e2e/test_git_repos.sh 仍在硬编码 Gitea token")
    require(errors, "❌ 未收到 SSE done 终态事件" in read_text("tests/e2e/test_async_sse.sh"), "tests/e2e/test_async_sse.sh 仍未对缺失 SSE done 事件显式失败")
    require(errors, "终态不是 success" in read_text("tests/e2e/test_async_sse.sh"), "tests/e2e/test_async_sse.sh 仍未对失败终态显式失败")
    require(
        errors,
        'echo "  ❌ 测试失败 (状态: $STATUS)"\n  exit 1' in read_text("tests/e2e/test_scheduled_execution.sh"),
        "tests/e2e/test_scheduled_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 执行任务测试失败"\n  exit 1' in read_text("tests/e2e/test_execution.sh"),
        "tests/e2e/test_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ CMDB 联动执行测试失败"\n  exit 1' in read_text("tests/e2e/test_cmdb_execution.sh"),
        "tests/e2e/test_cmdb_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 测试失败"\n  exit 1' in read_text("tests/e2e/test_system_cmdb_execution.sh"),
        "tests/e2e/test_system_cmdb_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ SSH 密钥认证测试失败"\n  exit 1' in read_text("tests/e2e/test_ssh_key_execution.sh"),
        "tests/e2e/test_ssh_key_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 混合认证测试失败"\n  exit 1' in read_text("tests/e2e/test_mixed_auth_execution.sh"),
        "tests/e2e/test_mixed_auth_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 结果与预期不完全匹配"\n  exit 1' in read_text("tests/e2e/test_mixed_auth_with_errors.sh"),
        "tests/e2e/test_mixed_auth_with_errors.sh 仍会在结果不符时假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 结果与预期不完全匹配"\n  exit 1' in read_text("tests/e2e/test_docker_mixed_auth.sh"),
        "tests/e2e/test_docker_mixed_auth.sh 仍会在结果不符时假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 测试失败"\n  exit 1' in read_text("tests/e2e/test_scheduled_system_cmdb_execution.sh"),
        "tests/e2e/test_scheduled_system_cmdb_execution.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 测试失败，最终状态: $STATUS"\n    exit 1' in read_text("tests/e2e/test_condition_node.sh"),
        "tests/e2e/test_condition_node.sh 仍会在失败终态下假绿退出",
    )
    require(
        errors,
        'echo "  ❌ 测试失败"\n    echo "  最终状态: $STATUS"\n    echo "  最终节点: $FINAL_NODE"\n    exit 1' in read_text("tests/e2e/test_set_variable_node.sh"),
        "tests/e2e/test_set_variable_node.sh 仍会在失败终态下假绿退出",
    )


def validate_ci_interface_smoke(errors: List[str]) -> None:
    ci_content = read_text(".github/workflows/ci.yml")
    readme_content = read_text("tests/e2e/README-ci.md")
    smoke_runner = read_text("tests/e2e/run_acceptance_smoke_ci.sh")
    require(errors, "Run acceptance auth smoke" in ci_content, "CI 缺少 acceptance auth smoke")
    require(errors, "Run acceptance interface contract smoke" in ci_content, "CI 缺少 interface_contract_smoke acceptance 校验")
    require(errors, ci_content.count("ACCEPTANCE_PHASE: interface_contract_smoke") == 1, "CI 中 interface_contract_smoke 步骤应且仅应出现一次")
    require(errors, "--phase interface_contract_smoke" in readme_content, "README-ci 未记录 interface_contract_smoke phase")
    require(errors, 'auto-healing-postgres-${ACCEPTANCE_PHASE}' in smoke_runner, "acceptance smoke runner 仍未按 phase 隔离 postgres 容器名")
    require(errors, 'auto-healing-redis-${ACCEPTANCE_PHASE}' in smoke_runner, "acceptance smoke runner 仍未按 phase 隔离 redis 容器名")
    require(errors, 'docker port "$container" "${container_port}/tcp"' in smoke_runner, "acceptance smoke runner 仍未通过 docker port 回读随机端口")
    require(errors, '-p "127.0.0.1::5432"' in smoke_runner, "acceptance smoke runner 仍未让 docker 自动分配 postgres 宿主端口")
    require(errors, '-p "127.0.0.1::6379"' in smoke_runner, "acceptance smoke runner 仍未让 docker 自动分配 redis 宿主端口")


def main() -> int:
    errors: List[str] = []
    validate_notification_seed(errors)
    validate_task_seed(errors)
    validate_executor_build(errors)
    validate_mock_notification(errors)
    validate_deployment_scripts(errors)
    validate_no_hardcoded_repo_paths(errors)
    validate_no_legacy_contract_fallbacks(errors)
    validate_no_silent_e2e_fallbacks(errors)
    validate_ci_interface_smoke(errors)

    if errors:
        for message in errors:
            print(f"[FAIL] {message}")
        return 1

    print("[OK] helper 脚本关键面校验通过")
    return 0


if __name__ == "__main__":
    sys.exit(main())
