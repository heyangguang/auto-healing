FROM python:3.11-slim

LABEL maintainer="auto-healing"
LABEL description="Ansible Executor for Auto-Healing Platform"
LABEL version="2.0-compat"

# 安装 Ansible 2.14 (支持 Python 3.6+ 目标机器)
# Ansible 2.14 是最后一个支持 Python 3.6 目标机器的版本
# 参考: https://docs.ansible.com/ansible/latest/reference_appendices/release_and_maintenance.html
RUN pip install --no-cache-dir \
    ansible-core==2.14.18 \
    jmespath \
    netaddr \
    dnspython \
    passlib \
    paramiko

# 安装系统工具
RUN apt-get update && apt-get install -y --no-install-recommends \
    openssh-client \
    sshpass \
    git \
    curl \
    && rm -rf /var/lib/apt/lists/*

# 创建工作目录
WORKDIR /workspace

# 配置 Ansible - 增强兼容性
ENV ANSIBLE_HOST_KEY_CHECKING=False
ENV ANSIBLE_RETRY_FILES_ENABLED=False
# 使用 auto 模式自动发现 Python 解释器
ENV ANSIBLE_PYTHON_INTERPRETER=auto
# 设置回退列表，包含 RHEL/CentOS 的 platform-python
ENV ANSIBLE_INTERPRETER_PYTHON_FALLBACK="python3.11,python3.10,python3.9,python3.8,python3.7,python3.6,/usr/bin/python3,/usr/libexec/platform-python,python2.7,/usr/bin/python,python"

# 默认入口
ENTRYPOINT ["ansible-playbook"]
