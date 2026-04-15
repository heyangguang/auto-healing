# Blacklist Demo Playbooks

这个仓库用于演示 AHS 的高危指令拦截能力。

特点：

- Playbook 内包含真实破坏性命令，而不是 `echo` 或 mock 文本
- 通过 Git 仓库接入 AHS，再走 Playbook / 任务模板 / 执行记录的真实链路
- 预期结果不是“执行成功”，而是被 AHS 在执行前安全扫描阶段拦截

## 入口文件

- `playbooks/destructive-demo.yml`

## 设计原则

- 命令内容必须能命中 AHS 内置黑名单规则
- 演示目标是验证 AHS 拦截，不是手工直接运行该 Playbook
- 该仓库只应用于受控演示环境
